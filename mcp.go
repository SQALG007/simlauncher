package main

// Serveur MCP (Model Context Protocol) sur stdio : permet à un assistant IA
// (Claude Desktop, etc.) de lister et lancer les jeux de RigStart.
// Démarrage : RigStart.exe --mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

var mcpProtocols = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func noArgs() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}

func idArg(name, desc string) map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{name: map[string]any{"type": "string", "description": desc}},
		"required": []string{name}, "additionalProperties": false}
}

var mcpTools = []mcpTool{
	{"list_games", "List the sim racing games configured in RigStart, with their ID, name and whether they are running.", noArgs()},
	{"launch_game", "Run the launch sequence for a game: start its companion apps (VR runtime, wheel software, SimHub, Crew Chief...) if they are not running, wait, then start the game. Returns the steps performed.", map[string]any{"type": "object", "properties": map[string]any{
		"game_id": map[string]any{"type": "string", "description": "Game ID from list_games, e.g. \"ams2\""},
		"profile": map[string]any{"type": "string", "description": "Optional sequence profile ID from list_games, e.g. \"vr\" or \"flat\""},
	}, "required": []string{"game_id"}, "additionalProperties": false}},
	{"list_apps", "List the companion apps (VR runtime, wheel base software, SimHub...) and whether each is running or missing.", noArgs()},
	{"start_app", "Start one companion app if it is not already running.", idArg("app_id", "App ID from list_apps, e.g. \"simhub\"")},
	{"stop_app", "Close one companion app.", idArg("app_id", "App ID from list_apps")},
	{"get_status", "Current launcher status: whether a launch is in progress, the current step and the recent log.", noArgs()},
}

// --- accès à l'instance en cours (zone de notification), sinon action locale

type backend interface {
	state() (stateResp, error)
	launch(id, profile string) ([]string, error)
	app(id, action string) error
}

type remoteBackend struct{ base string }

func (r remoteBackend) do(method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, r.base+path, rd)
	req.Header.Set("X-RigStart", "1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("%s", e["error"])
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (r remoteBackend) state() (stateResp, error) {
	var s stateResp
	return s, r.do("GET", "/api/state", nil, &s)
}

func (r remoteBackend) launch(id, profile string) ([]string, error) {
	before, err := r.state()
	if err != nil {
		return nil, err
	}
	n0 := len(before.Status.History)
	if err := r.do("POST", "/api/launch/"+id+"?profile="+url.QueryEscape(profile), struct{}{}, nil); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		s, err := r.state()
		if err != nil {
			return nil, err
		}
		if !s.Status.Busy {
			h := s.Status.History
			if n0 > len(h) || len(h) >= 60 { // historique tronqué : on renvoie la fin
				n0 = 0
				if len(h) > 15 {
					n0 = len(h) - 15
				}
			}
			return h[n0:], nil
		}
	}
	return nil, fmt.Errorf("timeout")
}

func (r remoteBackend) app(id, action string) error {
	return r.do("POST", "/api/app/"+id+"/"+action, struct{}{}, nil)
}

type localBackend struct{}

func (localBackend) state() (stateResp, error) { return buildState() }

func (localBackend) launch(id, profile string) ([]string, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	n0 := len(getStatus().History)
	err = launchGameProfile(c, id, profile)
	h := getStatus().History
	if n0 > len(h) {
		n0 = 0
	}
	if err == nil {
		go maybeWatch(c, id) // le processus MCP reste en vie tant que l'assistant est ouvert
	}
	return h[n0:], err
}

func (localBackend) app(id, action string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	a, ok := c.app(id)
	if !ok {
		return TE("app_unknown", id)
	}
	if action == "stop" {
		setStep("%s", T("app_closing", a.Name))
		return sysImpl.Kill(a.Process)
	}
	_, err = startApp(a)
	return err
}

func pickBackend(c Config) backend {
	r := remoteBackend{base: fmt.Sprintf("http://127.0.0.1:%d", c.Settings.UIPort)}
	if _, err := r.state(); err == nil {
		return r
	}
	return localBackend{}
}

// --- outils

func toolText(s string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": s}}, "isError": isErr}
}

func callTool(name string, args map[string]any) map[string]any {
	c, err := loadConfig()
	if err != nil {
		return toolText(err.Error(), true)
	}
	b := pickBackend(c)
	str := func(k string) string { v, _ := args[k].(string); return strings.TrimSpace(v) }

	switch name {
	case "end_session", "scan_steam_games", "import_steam_games":
		var text string
		switch name {
		case "end_session":
			text, err = mcpEndSession(c)
		case "scan_steam_games":
			text, err = mcpScanSteam(c, args)
		default:
			text, err = mcpImportSteam(c, args)
		}
		if err != nil {
			return toolText(err.Error(), true)
		}
		return toolText(text, false)
	case "add_game", "add_app":
		f := mcpAddGame
		if name == "add_app" {
			f = mcpAddApp
		}
		text, err := f(args)
		if err != nil {
			return toolText(err.Error(), true)
		}
		return toolText(text, false)
	case "list_games", "list_apps", "get_status":
		s, err := b.state()
		if err != nil {
			return toolText(err.Error(), true)
		}
		var out any
		switch name {
		case "list_games":
			type g struct {
				ID       string  `json:"id"`
				Name     string  `json:"name"`
				Enabled  bool    `json:"enabled"`
				Running  bool    `json:"running"`
				Hours    float64 `json:"playtime_hours"`
				Sessions int     `json:"sessions"`
				Last     string  `json:"last_played,omitempty"`
			}
			type p struct {
				ID   string   `json:"id"`
				Name string   `json:"name"`
				Apps []string `json:"apps"`
			}
			var l []g
			for _, x := range s.Games {
				st := s.Stats[x.ID]
				e := g{ID: x.ID, Name: x.Name, Enabled: x.Enabled, Running: x.Running, Hours: float64(st.Seconds/36) / 100, Sessions: st.Sessions}
				if st.LastPlayed > 0 {
					e.Last = time.Unix(st.LastPlayed, 0).Format("2006-01-02 15:04")
				}
				l = append(l, e)
			}
			var ps []p
			for _, x := range s.Config.Profiles {
				ps = append(ps, p{x.ID, x.Name, x.Apps})
			}
			out = map[string]any{"games": l, "profiles": ps}
		case "list_apps":
			type a struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Running bool   `json:"running"`
				Found   bool   `json:"found"`
			}
			var l []a
			for _, x := range s.Apps {
				l = append(l, a{x.ID, x.Name, x.Running, x.Found})
			}
			out = l
		default:
			h := s.Status.History
			if len(h) > 20 {
				h = h[len(h)-20:]
			}
			out = map[string]any{"busy": s.Status.Busy, "game": s.Status.Game, "step": s.Status.Step, "result": s.Status.Result, "log": h}
		}
		j, _ := json.MarshalIndent(out, "", "  ")
		return toolText(string(j), false)
	case "launch_game":
		id := str("game_id")
		if _, ok := c.game(id); !ok {
			return toolText(T("game_unknown", id), true)
		}
		if p := str("profile"); p != "" {
			if _, ok := c.profile(p); !ok {
				return toolText(T("profile_unknown", p), true)
			}
		}
		steps, err := b.launch(id, str("profile"))
		text := strings.Join(steps, "\n")
		if err != nil {
			return toolText(strings.TrimSpace(text+"\n"+err.Error()), true)
		}
		return toolText(text, false)
	case "start_app", "stop_app":
		id := str("app_id")
		if _, ok := c.app(id); !ok {
			return toolText(T("app_unknown", id), true)
		}
		action := "start"
		if name == "stop_app" {
			action = "stop"
		}
		if err := b.app(id, action); err != nil {
			return toolText(err.Error(), true)
		}
		return toolText("ok", false)
	}
	return toolText("unknown tool: "+name, true)
}

func handleRPC(req rpcReq) *rpcResp {
	resp := &rpcResp{JSONRPC: "2.0", ID: req.ID}
	isNotif := len(req.ID) == 0 || string(req.ID) == "null"
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		pv := mcpProtocols[0]
		for _, v := range mcpProtocols {
			if v == p.ProtocolVersion {
				pv = v
			}
		}
		resp.Result = map[string]any{
			"protocolVersion": pv,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "rigstart", "version": version},
			"instructions":    "RigStart starts a sim racing session: companion apps (VR, wheel, SimHub, Crew Chief) then the game. Call list_games to get IDs, then launch_game. add_game and add_app add new entries to the configuration.",
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": mcpTools}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			resp.Error = &rpcErr{-32602, "invalid params"}
			break
		}
		resp.Result = callTool(p.Name, p.Arguments)
	default:
		if isNotif {
			return nil // notifications/initialized, notifications/cancelled…
		}
		resp.Error = &rpcErr{-32601, "method not found: " + req.Method}
	}
	if isNotif {
		return nil
	}
	return resp
}

// runMCP lit des messages JSON-RPC (un par ligne) sur stdin et répond sur stdout.
func runMCP(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcReq
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResp{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcErr{-32700, "parse error"}})
			continue
		}
		if r := handleRPC(req); r != nil {
			if err := enc.Encode(r); err != nil {
				return err
			}
		}
	}
	log.Print("MCP : fin de l'entrée standard")
	return sc.Err()
}
