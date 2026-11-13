package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed ui
var uiFS embed.FS

var lastPing atomic.Int64

type appState struct {
	App
	Running bool `json:"running"`
	Found   bool `json:"found"`
}

type gameState struct {
	Game
	Running bool `json:"running"`
	Found   bool `json:"found"`
}

type stateResp struct {
	Config    Config               `json:"config"`
	Apps      []appState           `json:"apps"`
	Games     []gameState          `json:"games"`
	Status    Status               `json:"status"`
	ExePath   string               `json:"exePath"`
	CfgPath   string               `json:"cfgPath"`
	Version   string               `json:"version"`
	Tray      bool                 `json:"tray"`
	Autostart bool                 `json:"autostart"`
	Lang      string               `json:"lang"`
	Update    UpdateInfo           `json:"update"`
	Stats     map[string]GameStats `json:"stats"`
}

var trayMode bool

// justUpdated : relancé par une mise à jour (l'ancienne instance libère le port dans la seconde).
var justUpdated bool

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// guard : les requêtes qui modifient quelque chose doivent venir de notre page
// (en-tête personnalisé, impossible à envoyer depuis un autre site sans accord CORS).
func guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-SimLauncher") != "1" {
			writeErr(w, http.StatusForbidden, TE("forbidden"))
			return
		}
		h(w, r)
	}
}

func buildState() (stateResp, error) {
	c, err := loadConfig()
	if err != nil {
		return stateResp{}, err
	}
	running, _ := sysImpl.Running()
	exe, _ := os.Executable()
	s := stateResp{Config: c, Status: getStatus(), ExePath: exe, CfgPath: configPath(), Version: version, Tray: trayMode, Autostart: getAutostart(), Lang: lang(), Update: getUpdate(), Stats: gameStats(c)}
	for _, a := range c.Apps {
		s.Apps = append(s.Apps, appState{App: a, Running: isRunning(running, a.Process), Found: sysImpl.FileExists(a.Path)})
	}
	for _, g := range c.Games {
		found := g.SteamAppID != "" || sysImpl.FileExists(g.ExePath)
		s.Games = append(s.Games, gameState{Game: g, Running: isRunning(running, g.Process), Found: found})
	}
	return s, nil
}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".bmp": true}

func isImagePath(p string) bool {
	return imageExts[strings.ToLower(filepath.Ext(p))]
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(uiFS, "ui")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().Unix())
		s, err := buildState()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, s)
	})

	mux.HandleFunc("/api/config", guard(func(w http.ResponseWriter, r *http.Request) {
		var c Config
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&c); err != nil {
			writeErr(w, 400, TE("bad_data", err))
			return
		}
		c.Version = 1
		if err := c.validate(); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := saveConfig(c); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/reset", guard(func(w http.ResponseWriter, r *http.Request) {
		if err := saveConfig(defaultConfig()); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/launch/", guard(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/launch/")
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		if _, ok := c.game(id); !ok {
			writeErr(w, 404, TE("game_unknown", id))
			return
		}
		profile := r.URL.Query().Get("profile")
		if _, ok := c.profile(profile); profile != "" && !ok {
			writeErr(w, 404, TE("profile_unknown", profile))
			return
		}
		if getStatus().Busy {
			writeErr(w, 409, TE("busy", getStatus().Game))
			return
		}
		go func() {
			if err := launchGameProfile(c, id, profile); err != nil {
				log.Print(err)
				return
			}
			maybeWatch(c, id)
		}()
		writeJSON(w, 202, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/app/", guard(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/app/"), "/")
		if len(parts) != 2 {
			writeErr(w, 404, TE("not_found"))
			return
		}
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		a, ok := c.app(parts[0])
		if !ok {
			writeErr(w, 404, TE("app_unknown", parts[0]))
			return
		}
		switch parts[1] {
		case "start":
			_, err = startApp(a)
		case "stop":
			setStep("%s", T("app_closing", a.Name))
			err = sysImpl.Kill(a.Process)
		default:
			err = TE("action_unknown")
		}
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/pick", guard(func(w http.ResponseWriter, r *http.Request) {
		p, err := pickFile(r.URL.Query().Get("initial"), r.URL.Query().Get("kind") == "image")
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]string{"path": p})
	}))

	// Image d'un jeu : ne sert que le fichier choisi dans les réglages, et seulement si c'est une image.
	mux.HandleFunc("/api/image/", func(w http.ResponseWriter, r *http.Request) {
		c, err := loadConfig()
		if err != nil {
			http.Error(w, "config", 500)
			return
		}
		g, ok := c.game(strings.TrimPrefix(r.URL.Path, "/api/image/"))
		if !ok || g.Image == "" || !isImagePath(g.Image) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, g.Image)
	})

	mux.HandleFunc("/api/update/check", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, checkUpdate())
	}))
	mux.HandleFunc("/api/update/install", guard(func(w http.ResponseWriter, r *http.Request) {
		if st := getStatus(); st.Busy {
			writeErr(w, 409, TE("busy", st.Game))
			return
		}
		if err := installUpdate(); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 202, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/end", guard(func(w http.ResponseWriter, r *http.Request) {
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		closed, err := endSession(c)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]any{"closed": closed})
	}))
	mux.HandleFunc("/api/steam/scan", guard(func(w http.ResponseWriter, r *http.Request) {
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		l, err := scanSteam(c, r.URL.Query().Get("all") == "1")
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, l)
	}))
	mux.HandleFunc("/api/steam/import", guard(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AppIDs []string `json:"appIds"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		added, err := importSteam(body.AppIDs)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, added)
	}))
	// Copie l'image Steam d'un jeu dans le dossier de SimLauncher et renvoie son chemin (le brouillon des réglages l'utilise).
	mux.HandleFunc("/api/steam/copycover", guard(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AppID  string `json:"appId"`
			GameID string `json:"gameId"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&body); err != nil || !idRe.MatchString(body.GameID) || !digitsRe.MatchString(body.AppID) {
			writeErr(w, 400, TE("forbidden"))
			return
		}
		p, err := copyCover(body.AppID, body.GameID)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, map[string]string{"path": p})
	}))
	// Aperçu d'une image du cache Steam (liste de détection).
	mux.HandleFunc("/api/steam/preview/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/steam/preview/")
		p := ""
		if digitsRe.MatchString(id) {
			p = steamCover(steamRoot(), id)
		}
		if p == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, p)
	})
	mux.HandleFunc("/api/webhook/test/", guard(func(w http.ResponseWriter, r *http.Request) {
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		url, event := c.Settings.WebhookStart, "session_start"
		if strings.HasSuffix(r.URL.Path, "/end") {
			url, event = c.Settings.WebhookEnd, "session_end"
		}
		if url == "" {
			writeErr(w, 400, TE("webhook_empty"))
			return
		}
		if err := fireWebhook(url, event, Game{ID: "test", Name: "SimLauncher test"}); err != nil {
			writeErr(w, 502, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/quit", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"ok": true})
		go func() { time.Sleep(300 * time.Millisecond); quitApp(0) }()
	}))

	mux.HandleFunc("/api/language", guard(func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, TE("bad_data", err))
			return
		}
		c, err := loadConfig()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		c.Settings.Language = b.Language
		if err := c.validate(); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := saveConfig(c); err != nil {
			writeErr(w, 500, err)
			return
		}
		setLang(b.Language)
		writeJSON(w, 200, map[string]string{"lang": lang()})
	}))

	mux.HandleFunc("/api/autostart", guard(func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := setAutostart(b.On); err != nil {
			writeErr(w, 500, TE("autostart_fail", err))
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true, "on": getAutostart()})
	}))
	return mux
}

// runUI démarre le serveur local. tray=true : icône de notification, pas de fenêtre
// et pas d'arrêt automatique. Si une instance tourne déjà, on lui ouvre une fenêtre.
func runUI(c Config, tray, openWindow bool) error {
	addr := fmt.Sprintf("127.0.0.1:%d", c.Settings.UIPort)
	url := "http://" + addr + "/"
	ln, err := net.Listen("tcp", addr)
	for i := 0; err != nil && justUpdated && i < 30; i++ {
		time.Sleep(500 * time.Millisecond) // l'ancienne version est en train de quitter
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		if openWindow {
			openUI(url) // une instance tourne déjà
		}
		return nil
	}
	trayMode = tray
	lastPing.Store(time.Now().Unix())
	go updateLoop()
	go statsLoop()
	if tray {
		go func() {
			if err := http.Serve(ln, newMux()); err != nil {
				log.Print(err)
			}
		}()
		if openWindow && !justUpdated {
			openUI(url)
		}
		return runTray(url)
	}
	// Fermeture automatique quand la fenêtre est fermée (plus de ping) et que rien n'est en cours.
	go func() {
		for range time.Tick(15 * time.Second) {
			if st := getStatus(); time.Now().Unix()-lastPing.Load() > 45 && !st.Busy && st.Watching == 0 {
				log.Print("Interface fermée, arrêt du lanceur")
				os.Exit(0)
			}
		}
	}()
	if !justUpdated { // après une mise à jour, la fenêtre déjà ouverte se recharge toute seule
		openUI(url)
	}
	return http.Serve(ln, newMux())
}
