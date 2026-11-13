package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// App : un logiciel compagnon (PimaxEVO, Pit House, SimHub, Crew Chief...).
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`        // chemin complet de l'exécutable
	Args        string `json:"args"`        // arguments optionnels
	Process     string `json:"process"`     // nom du processus (ex. SimHubWPF.exe) pour savoir s'il tourne
	DelayAfter  int    `json:"delayAfter"`  // secondes à attendre après l'avoir démarré
	CloseOnExit bool   `json:"closeOnExit"` // fermer quand le jeu se termine (si l'option globale est active)
}

// Game : un jeu de simulation.
type Game struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	SteamAppID string   `json:"steamAppId"` // lancement via steam://rungameid/<id>
	ExePath    string   `json:"exePath"`    // sinon, exécutable direct
	ExeArgs    string   `json:"exeArgs"`
	Process    string   `json:"process"` // processus du jeu (pour surveiller sa fermeture)
	Apps       []string `json:"apps"`    // IDs des logiciels à démarrer avant ce jeu
	Color      string   `json:"color"`
	Image      string   `json:"image,omitempty"`    // photo choisie par l'utilisateur pour la carte du jeu
	HideName   bool     `json:"hideName,omitempty"` // n'affiche pas le nom sur la carte (l'image contient déjà le logo)
}

type Settings struct {
	DelayBeforeGame int    `json:"delayBeforeGame"` // pause après le démarrage des logiciels
	CloseAppsOnExit bool   `json:"closeAppsOnExit"` // fermer les logiciels marqués quand le jeu se termine
	UIPort          int    `json:"uiPort"`
	Language        string `json:"language"`                // "auto", fr, en, pt, es, zh
	NoUpdateCheck   bool   `json:"noUpdateCheck,omitempty"` // ne pas vérifier les nouvelles versions
	AutoUpdate      bool   `json:"autoUpdate,omitempty"`    // installer les mises à jour sans demander
	WebhookStart    string `json:"webhookStart,omitempty"`  // URL appelée (POST) au début d'une session
	WebhookEnd      string `json:"webhookEnd,omitempty"`    // URL appelée à la fin (jeu fermé ou « Fin de session »)
	CardStyle       string `json:"cardStyle,omitempty"`     // classic (défaut), band, sheet, row
	KeepInTray      bool   `json:"keepInTray,omitempty"`    // rester près de l'horloge quand la fenêtre est fermée
	ForceClose      bool   `json:"forceClose,omitempty"`    // fermer les logiciels tout de suite, sans fermeture douce
}

// Profile : une variante de séquence (ex. « VR » ou « Écran ») qui limite les logiciels lancés.
type Profile struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Apps []string `json:"apps"` // logiciels autorisés dans ce profil (parmi ceux du jeu)
}

type Config struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Apps     []App     `json:"apps"`
	Games    []Game    `json:"games"`
	Profiles []Profile `json:"profiles,omitempty"`
}

var cfgMu sync.Mutex

func configDir() string {
	if d := os.Getenv("RIGSTART_HOME"); d != "" {
		return d
	}
	base := os.Getenv("APPDATA")
	if base == "" {
		h, _ := os.UserHomeDir()
		base = filepath.Join(h, ".config")
	}
	return filepath.Join(base, "RigStart")
}

// legacyDir : dossier de l'ancien nom du projet (SimLauncher, jusqu'à la version 1.7).
func legacyDir() string {
	if os.Getenv("RIGSTART_HOME") != "" {
		return ""
	}
	if d := os.Getenv("SIMLAUNCHER_HOME"); d != "" {
		return d
	}
	if base := os.Getenv("APPDATA"); base != "" {
		return filepath.Join(base, "SimLauncher")
	}
	return ""
}

// migrateLegacy recopie la configuration, les statistiques et les images de
// %APPDATA%\SimLauncher vers %APPDATA%\RigStart au premier lancement.
// L'ancien dossier n'est pas modifié. Un autre logiciel nommé SimLauncher utilise
// le même dossier : on ne reprend le fichier que s'il a bien notre format.
func migrateLegacy() bool {
	old := legacyDir()
	if old == "" || old == configDir() {
		return false
	}
	b, err := os.ReadFile(filepath.Join(old, "config.json"))
	if err != nil {
		return false
	}
	var c Config
	if json.Unmarshal(b, &c) != nil || len(c.Games) == 0 || len(c.Apps) == 0 {
		return false
	}
	newDir := configDir()
	if os.MkdirAll(filepath.Join(newDir, "covers"), 0o755) != nil {
		return false
	}
	if ents, err := os.ReadDir(filepath.Join(old, "covers")); err == nil {
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if data, err := os.ReadFile(filepath.Join(old, "covers", e.Name())); err == nil {
				_ = os.WriteFile(filepath.Join(newDir, "covers", e.Name()), data, 0o644)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(old, "stats.json")); err == nil {
		_ = os.WriteFile(filepath.Join(newDir, "stats.json"), data, 0o644)
	}
	oldCovers := strings.ToLower(filepath.Join(old, "covers")) + string(filepath.Separator)
	for i, g := range c.Games {
		if strings.HasPrefix(strings.ToLower(g.Image), oldCovers) {
			c.Games[i].Image = filepath.Join(newDir, "covers", g.Image[len(oldCovers):])
		}
	}
	out, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return false
	}
	return os.WriteFile(configPath(), out, 0o644) == nil
}

func configPath() string { return filepath.Join(configDir(), "config.json") }

func defaultConfig() Config {
	all := []string{"pimax", "pithouse", "simhub", "crewchief"}
	noCC := []string{"pimax", "pithouse", "simhub"}
	return Config{
		Version:  1,
		Settings: Settings{DelayBeforeGame: 8, CloseAppsOnExit: false, UIPort: 47821, Language: "auto"},
		Apps: []App{
			{ID: "pimax", Name: "PimaxEVO", Path: `C:\Program Files\Pimax\PimaxClient\pimaxui\PimaxClient.exe`, Process: "PimaxClient.exe", DelayAfter: 2},
			{ID: "pithouse", Name: "MOZA Pit House", Path: `C:\Program Files (x86)\MOZA Pit House\MOZA Pit House.exe`, Process: "MOZA Pit House.exe", DelayAfter: 2},
			{ID: "simhub", Name: "SimHub", Path: `C:\Program Files (x86)\SimHub\SimHubWPF.exe`, Process: "SimHubWPF.exe", DelayAfter: 2},
			{ID: "crewchief", Name: "Crew Chief", Path: `C:\Program Files (x86)\Britton IT Ltd\CrewChiefV4\CrewChiefV4.exe`, Process: "CrewChiefV4.exe", DelayAfter: 1, CloseOnExit: true},
		},
		Games: []Game{
			{ID: "ams2", Name: "Automobilista 2", Enabled: true, SteamAppID: "1066890", Process: "AMS2AVX.exe", Apps: all, Color: "#c8102e"},
			{ID: "acc", Name: "Assetto Corsa Competizione", Enabled: true, SteamAppID: "805550", Process: "AC2-Win64-Shipping.exe", Apps: all, Color: "#1f6feb"},
			{ID: "ac", Name: "Assetto Corsa", Enabled: true, SteamAppID: "244210", Process: "acs.exe", Apps: all, Color: "#d62828"},
			{ID: "acevo", Name: "Assetto Corsa EVO", Enabled: true, SteamAppID: "3058630", Process: "AssettoCorsaEVO.exe", Apps: all, Color: "#e85d04"},
			{ID: "acrally", Name: "Assetto Corsa Rally", Enabled: true, SteamAppID: "3917090", Process: "", Apps: noCC, Color: "#6a994e"},
			{ID: "lmu", Name: "Le Mans Ultimate", Enabled: true, SteamAppID: "2399420", Process: "Le Mans Ultimate.exe", Apps: all, Color: "#0b3d91"},
			{ID: "raceroom", Name: "RaceRoom", Enabled: true, SteamAppID: "211500", Process: "RRRE64.exe", Apps: all, Color: "#7b2cbf"},
			{ID: "beamng", Name: "BeamNG.drive", Enabled: true, SteamAppID: "284160", Process: "BeamNG.drive.x64.exe", Apps: noCC, Color: "#f4a261"},
		},
	}
}

func loadConfig() (Config, error) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	b, err := os.ReadFile(configPath())
	if errors.Is(err, os.ErrNotExist) && migrateLegacy() {
		b, err = os.ReadFile(configPath())
	}
	if errors.Is(err, os.ErrNotExist) {
		c := defaultConfig()
		return c, saveConfigLocked(c)
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, TE("cfg_bad", err)
	}
	if c.Settings.UIPort == 0 {
		c.Settings.UIPort = 47821
	}
	setLang(c.Settings.Language)
	forceClose.Store(c.Settings.ForceClose)
	return c, nil
}

// forceClose : copie du réglage, lue par Sys.Kill (qui ne reçoit pas la configuration).
var forceClose atomic.Bool

func saveConfig(c Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return saveConfigLocked(c)
}

func saveConfigLocked(c Config) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}

var idRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// validate vérifie la cohérence d'une configuration envoyée par l'interface.
func (c *Config) validate() error {
	c.Settings.WebhookStart = strings.TrimSpace(c.Settings.WebhookStart)
	c.Settings.WebhookEnd = strings.TrimSpace(c.Settings.WebhookEnd)
	if !validWebhook(c.Settings.WebhookStart) || !validWebhook(c.Settings.WebhookEnd) {
		return TE("v_webhook")
	}
	seen := map[string]bool{}
	for i := range c.Apps {
		a := &c.Apps[i]
		a.ID = strings.ToLower(strings.TrimSpace(a.ID))
		if !idRe.MatchString(a.ID) {
			return TE("v_app_id", a.ID)
		}
		if seen["app:"+a.ID] {
			return TE("v_app_dup", a.ID)
		}
		seen["app:"+a.ID] = true
		if strings.TrimSpace(a.Name) == "" {
			return TE("v_app_name", a.ID)
		}
		if a.DelayAfter < 0 || a.DelayAfter > 120 {
			return TE("v_app_delay", a.Name)
		}
	}
	for i := range c.Games {
		g := &c.Games[i]
		g.ID = strings.ToLower(strings.TrimSpace(g.ID))
		if !idRe.MatchString(g.ID) {
			return TE("v_game_id", g.ID)
		}
		if seen["game:"+g.ID] {
			return TE("v_game_dup", g.ID)
		}
		seen["game:"+g.ID] = true
		if strings.TrimSpace(g.Name) == "" {
			return TE("v_game_name", g.ID)
		}
		if g.Image != "" && !isImagePath(g.Image) {
			return TE("v_game_image", g.Name)
		}
		if strings.TrimSpace(g.SteamAppID) == "" && strings.TrimSpace(g.ExePath) == "" {
			return TE("v_game_launch", g.Name)
		}
		if g.SteamAppID != "" && !regexp.MustCompile(`^\d+$`).MatchString(g.SteamAppID) {
			return TE("v_game_appid", g.Name)
		}
		for _, a := range g.Apps {
			if !seen["app:"+a] {
				return TE("v_game_app", g.Name, a)
			}
		}
	}
	for i := range c.Profiles {
		p := &c.Profiles[i]
		p.ID = strings.ToLower(strings.TrimSpace(p.ID))
		if !idRe.MatchString(p.ID) {
			return TE("v_profile_id", p.ID)
		}
		if seen["profile:"+p.ID] {
			return TE("v_profile_dup", p.ID)
		}
		seen["profile:"+p.ID] = true
		if strings.TrimSpace(p.Name) == "" {
			return TE("v_profile_name", p.ID)
		}
		if p.Apps == nil {
			p.Apps = []string{}
		}
		for _, a := range p.Apps {
			if !seen["app:"+a] {
				return TE("v_game_app", p.Name, a)
			}
		}
	}
	switch c.Settings.CardStyle {
	case "", "classic", "band", "sheet", "row":
	default:
		return TE("v_card_style", c.Settings.CardStyle)
	}
	if c.Settings.DelayBeforeGame < 0 || c.Settings.DelayBeforeGame > 120 {
		return TE("v_pause")
	}
	if c.Settings.UIPort < 1024 || c.Settings.UIPort > 65535 {
		return TE("v_port")
	}
	if l := c.Settings.Language; l != "" && l != "auto" {
		ok := false
		for _, x := range languages {
			ok = ok || l == x
		}
		if !ok {
			return TE("v_lang", l)
		}
	}
	return nil
}

func (c *Config) game(id string) (Game, bool) {
	for _, g := range c.Games {
		if strings.EqualFold(g.ID, id) {
			return g, true
		}
	}
	return Game{}, false
}

// DisplayName : les profils par défaut (vr, ecran/flat) suivent la langue de l'interface,
// sauf si l'utilisateur les a renommés.
func (p Profile) DisplayName() string {
	switch strings.ToLower(p.ID) {
	case "ecran", "flat", "screen":
		if p.Name == "" || p.Name == "Ecran" || p.Name == "Screen" || p.Name == "Flat screen" {
			return T("prof_flat")
		}
		for _, v := range catalog["prof_flat"] {
			if p.Name == v {
				return T("prof_flat")
			}
		}
	}
	return p.Name
}

func (c *Config) profile(id string) (Profile, bool) {
	for _, p := range c.Profiles {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return Profile{}, false
}

// sequence renvoie les logiciels à démarrer pour un jeu, filtrés par le profil s'il y en a un.
func (c *Config) sequence(g Game, profileID string) ([]string, string, error) {
	if profileID == "" {
		return g.Apps, "", nil
	}
	p, ok := c.profile(profileID)
	if !ok {
		return nil, "", TE("profile_unknown", profileID)
	}
	allowed := map[string]bool{}
	for _, a := range p.Apps {
		allowed[a] = true
	}
	var apps []string
	for _, a := range g.Apps {
		if allowed[a] {
			apps = append(apps, a)
		}
	}
	return apps, p.DisplayName(), nil
}

func (c *Config) app(id string) (App, bool) {
	for _, a := range c.Apps {
		if a.ID == id {
			return a, true
		}
	}
	return App{}, false
}
