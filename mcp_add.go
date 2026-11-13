package main

// Outils MCP add_game et add_app : ajoutent une entrée dans config.json.
// L'instance en cours relit la configuration à chaque requête, la nouveauté
// apparaît donc dans l'interface sans redémarrer.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func init() {
	mcpTools = append(mcpTools,
		mcpTool{"add_game", "Add a game to RigStart. Give a Steam AppID (preferred, found in the store URL store.steampowered.com/app/<id>) or the full path of the game's .exe. By default every companion app is started before it.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string", "description": "Display name, e.g. \"iRacing\""},
				"id":           map[string]any{"type": "string", "description": "Short ID for the Stream Deck command (letters and digits). Generated from the name if omitted."},
				"steam_app_id": map[string]any{"type": "string", "description": "Steam AppID, e.g. \"266410\""},
				"exe_path":     map[string]any{"type": "string", "description": "Full path of the game executable, for games not on Steam"},
				"exe_args":     map[string]any{"type": "string", "description": "Command-line arguments for exe_path"},
				"process":      map[string]any{"type": "string", "description": "Game process name, e.g. \"iRacingSim64DX11.exe\". Used to close companion apps when the game exits."},
				"apps":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "IDs of the companion apps to start first (see list_apps). Default: all."},
				"color":        map[string]any{"type": "string", "description": "Card colour, e.g. \"#1e88e5\""},
				"image":        map[string]any{"type": "string", "description": "Full path of a .jpg/.png/.webp picture for the card"},
				"hide_name":    map[string]any{"type": "boolean", "description": "Hide the name on the card when the picture already shows the logo"},
			},
			"required": []string{"name"}, "additionalProperties": false,
		}},
		mcpTool{"add_app", "Add a companion app (VR runtime, wheel software, telemetry, overlay...) that RigStart can start before games.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "Display name, e.g. \"OpenKneeboard\""},
				"path":          map[string]any{"type": "string", "description": "Full path of the app executable"},
				"id":            map[string]any{"type": "string", "description": "Short ID (letters and digits). Generated from the name if omitted."},
				"args":          map[string]any{"type": "string", "description": "Command-line arguments"},
				"process":       map[string]any{"type": "string", "description": "Process name used to detect it is running. Default: the executable file name."},
				"delay_after":   map[string]any{"type": "integer", "description": "Seconds to wait after starting it (default 2)"},
				"close_on_exit": map[string]any{"type": "boolean", "description": "Close it when the game exits (if auto-close is on)"},
				"add_to_games":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Game IDs that should start it, or [\"all\"]. Default: none."},
			},
			"required": []string{"name", "path"}, "additionalProperties": false,
		}},
	)
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{"end_session", "End the sim racing session: close the running game and every companion app, then call the end-of-session webhook if one is set.", noArgs()},
		mcpTool{"scan_steam_games", "List the games installed in Steam on this PC. By default only known sim racing games; set all to true for every installed game. Shows which ones are already in RigStart.", map[string]any{
			"type": "object", "properties": map[string]any{"all": map[string]any{"type": "boolean", "description": "Include every installed Steam game, not only racing games"}}, "additionalProperties": false}},
		mcpTool{"import_steam_games", "Add installed Steam games to RigStart with their Steam picture, process name and every companion app. Without app_ids, adds every racing game found that is not configured yet.", map[string]any{
			"type": "object", "properties": map[string]any{"app_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Steam AppIDs from scan_steam_games"}}, "additionalProperties": false}},
	)
}

func mcpEndSession(c Config) (string, error) {
	r := remoteBackend{base: fmt.Sprintf("http://127.0.0.1:%d", c.Settings.UIPort)}
	if _, err := r.state(); err == nil {
		var out struct {
			Closed []string `json:"closed"`
		}
		if err := r.do("POST", "/api/end", struct{}{}, &out); err != nil {
			return "", err
		}
		return endText(out.Closed), nil
	}
	closed, err := endSession(c)
	return endText(closed), err
}

func endText(closed []string) string {
	if len(closed) == 0 {
		return "Nothing was running."
	}
	return "Closed: " + strings.Join(closed, ", ")
}

func mcpScanSteam(c Config, args map[string]any) (string, error) {
	all, _ := args["all"].(bool)
	l, err := scanSteam(c, all)
	if err != nil {
		return "", err
	}
	if len(l) == 0 {
		return "No matching Steam game installed.", nil
	}
	return steamSummary(l), nil
}

func mcpImportSteam(c Config, args map[string]any) (string, error) {
	ids, given := argList(args, "app_ids")
	if !given || len(ids) == 0 {
		l, err := scanSteam(c, false)
		if err != nil {
			return "", err
		}
		for _, s := range l {
			if !s.Configured {
				ids = append(ids, s.AppID)
			}
		}
	}
	if len(ids) == 0 {
		return "Every installed racing game is already in RigStart.", nil
	}
	added, err := importSteam(ids)
	if err != nil {
		return "", err
	}
	if len(added) == 0 {
		return "Nothing new to add.", nil
	}
	var b strings.Builder
	b.WriteString("Added:\n")
	for _, g := range added {
		pic := "no picture"
		if g.Image != "" {
			pic = "Steam picture"
		}
		fmt.Fprintf(&b, "%s — %s (%s, Stream Deck: --game %s)\n", g.ID, g.Name, pic, g.ID)
	}
	return b.String(), nil
}

func argStr(args map[string]any, k string) string {
	switch v := args[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

func argList(args map[string]any, k string) ([]string, bool) {
	raw, ok := args[k]
	if !ok || raw == nil {
		return nil, false
	}
	var out []string
	switch v := raw.(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case string:
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out, true
}

// slugID fabrique un identifiant court et unique à partir d'un nom.
func slugID(name string, taken func(string) bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
		if b.Len() >= 16 {
			break
		}
	}
	base := b.String()
	if base == "" {
		base = "item"
	}
	id := base
	for i := 2; taken(id); i++ {
		id = fmt.Sprintf("%s%d", base, i)
	}
	return id
}

func exeBase(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}

var cardColors = []string{"#e53935", "#1e88e5", "#43a047", "#fb8c00", "#8e24aa", "#00acc1", "#fdd835", "#6d4c41"}

func mcpAddGame(args map[string]any) (string, error) {
	c, err := loadConfig()
	if err != nil {
		return "", err
	}
	g := Game{Name: argStr(args, "name"), Enabled: true, SteamAppID: argStr(args, "steam_app_id"), ExePath: argStr(args, "exe_path"),
		ExeArgs: argStr(args, "exe_args"), Process: argStr(args, "process"), Color: argStr(args, "color"), Image: argStr(args, "image")}
	g.HideName, _ = args["hide_name"].(bool)
	g.ID = argStr(args, "id")
	if g.ID == "" {
		g.ID = slugID(g.Name, func(s string) bool { _, ok := c.game(s); return ok })
	}
	if g.Process == "" && g.ExePath != "" {
		g.Process = exeBase(g.ExePath)
	}
	if g.Color == "" {
		g.Color = cardColors[len(c.Games)%len(cardColors)]
	}
	if apps, ok := argList(args, "apps"); ok {
		for _, id := range apps {
			if _, ok := c.app(id); !ok {
				return "", TE("app_unknown", id)
			}
		}
		g.Apps = apps
	} else {
		for _, a := range c.Apps {
			g.Apps = append(g.Apps, a.ID)
		}
	}
	if g.Apps == nil {
		g.Apps = []string{}
	}
	c.Games = append(c.Games, g)
	if err := c.validate(); err != nil {
		return "", err
	}
	if err := saveConfig(c); err != nil {
		return "", err
	}
	var notes []string
	if g.ExePath != "" && !sysImpl.FileExists(g.ExePath) {
		notes = append(notes, "warning: exe_path not found on this PC")
	}
	if g.Image != "" && !sysImpl.FileExists(g.Image) {
		notes = append(notes, "warning: image not found on this PC")
	}
	if g.Process == "" {
		notes = append(notes, "tip: set process so companion apps can be closed when the game exits")
	}
	return addResult("game", g, notes, fmt.Sprintf("Stream Deck command: \"RigStart.exe\" --game %s", g.ID)), nil
}

func mcpAddApp(args map[string]any) (string, error) {
	c, err := loadConfig()
	if err != nil {
		return "", err
	}
	a := App{Name: argStr(args, "name"), Path: argStr(args, "path"), Args: argStr(args, "args"), Process: argStr(args, "process"), DelayAfter: 2}
	if v, ok := args["delay_after"].(float64); ok {
		a.DelayAfter = int(v)
	}
	a.CloseOnExit, _ = args["close_on_exit"].(bool)
	a.ID = argStr(args, "id")
	if a.ID == "" {
		a.ID = slugID(a.Name, func(s string) bool { _, ok := c.app(s); return ok })
	}
	if a.Process == "" {
		a.Process = exeBase(a.Path)
	}
	c.Apps = append(c.Apps, a)
	var added []string
	if games, ok := argList(args, "add_to_games"); ok {
		all := len(games) == 1 && strings.EqualFold(games[0], "all")
		for i := range c.Games {
			want := all
			for _, id := range games {
				if id == c.Games[i].ID {
					want = true
				}
			}
			if want {
				c.Games[i].Apps = append(c.Games[i].Apps, a.ID)
				added = append(added, c.Games[i].ID)
			}
		}
		if !all {
			for _, id := range games {
				if _, ok := c.game(id); !ok {
					return "", TE("game_unknown", id)
				}
			}
		}
	}
	if err := c.validate(); err != nil {
		return "", err
	}
	if err := saveConfig(c); err != nil {
		return "", err
	}
	var notes []string
	if !sysImpl.FileExists(a.Path) {
		notes = append(notes, "warning: path not found on this PC")
	}
	if len(added) == 0 {
		notes = append(notes, "not started by any game yet: pass add_to_games, or tick it in Settings")
	}
	extra := ""
	if len(added) > 0 {
		extra = "started before: " + strings.Join(added, ", ")
	}
	return addResult("app", a, notes, extra), nil
}

func addResult(kind string, v any, notes []string, extra string) string {
	j, _ := json.MarshalIndent(v, "", "  ")
	s := "Added " + kind + ":\n" + string(j)
	if extra != "" {
		s += "\n" + extra
	}
	for _, n := range notes {
		s += "\n" + n
	}
	return s
}
