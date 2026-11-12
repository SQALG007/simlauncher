package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
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
	Config    Config      `json:"config"`
	Apps      []appState  `json:"apps"`
	Games     []gameState `json:"games"`
	Status    Status      `json:"status"`
	ExePath   string      `json:"exePath"`
	CfgPath   string      `json:"cfgPath"`
	Version   string      `json:"version"`
	Tray      bool        `json:"tray"`
	Autostart bool        `json:"autostart"`
	Lang      string      `json:"lang"`
}

var trayMode bool

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
	s := stateResp{Config: c, Status: getStatus(), ExePath: exe, CfgPath: configPath(), Version: version, Tray: trayMode, Autostart: getAutostart(), Lang: lang()}
	for _, a := range c.Apps {
		s.Apps = append(s.Apps, appState{App: a, Running: isRunning(running, a.Process), Found: sysImpl.FileExists(a.Path)})
	}
	for _, g := range c.Games {
		found := g.SteamAppID != "" || sysImpl.FileExists(g.ExePath)
		s.Games = append(s.Games, gameState{Game: g, Running: isRunning(running, g.Process), Found: found})
	}
	return s, nil
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
		if getStatus().Busy {
			writeErr(w, 409, TE("busy", getStatus().Game))
			return
		}
		go func() {
			if err := launchGame(c, id); err != nil {
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
		p, err := pickFile(r.URL.Query().Get("initial"))
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]string{"path": p})
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
func runUI(c Config, tray bool) error {
	addr := fmt.Sprintf("127.0.0.1:%d", c.Settings.UIPort)
	url := "http://" + addr + "/"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if !tray {
			openUI(url) // une instance tourne déjà
		}
		return nil
	}
	trayMode = tray
	lastPing.Store(time.Now().Unix())
	if tray {
		go func() {
			if err := http.Serve(ln, newMux()); err != nil {
				log.Print(err)
			}
		}()
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
	openUI(url)
	return http.Serve(ln, newMux())
}
