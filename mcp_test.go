//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
)

func mcpRun(t *testing.T, msgs ...string) []map[string]any {
	var out bytes.Buffer
	if err := runMCP(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("réponse non JSON : %s", l)
		}
		res = append(res, m)
	}
	return res
}

func freePort(t *testing.T) int {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func toolResultText(t *testing.T, r map[string]any) (string, bool) {
	res := r["result"].(map[string]any)
	c := res["content"].([]any)[0].(map[string]any)
	return c["text"].(string), res["isError"].(bool)
}

func TestMCPLocal(t *testing.T) {
	f := setup(t)
	c := defaultConfig()
	c.Settings.UIPort = freePort(t) // aucune instance : mode local
	c.Settings.Language = "en"
	_ = saveConfig(c)
	r := mcpRun(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_games","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"launch_game","arguments":{"game_id":"lmu"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"launch_game","arguments":{"game_id":"nope"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"foo"}`,
		`not json`,
	)
	if len(r) != 7 {
		t.Fatalf("7 réponses attendues (la notification n'en a pas), obtenu %d", len(r))
	}
	if r[0]["result"].(map[string]any)["protocolVersion"] != "2025-03-26" {
		t.Error("version de protocole non négociée")
	}
	if n := len(r[1]["result"].(map[string]any)["tools"].([]any)); n != 6 {
		t.Errorf("6 outils attendus, %d", n)
	}
	if txt, _ := toolResultText(t, r[2]); !strings.Contains(txt, `"id": "ams2"`) {
		t.Errorf("list_games : %s", txt)
	}
	txt, isErr := toolResultText(t, r[3])
	if isErr || !strings.Contains(txt, "Le Mans Ultimate launched") {
		t.Errorf("launch_game : %v %s", isErr, txt)
	}
	if !strings.Contains(strings.Join(f.Calls, ","), "steam://rungameid/2399420") {
		t.Errorf("LMU non lancé : %v", f.Calls)
	}
	if _, isErr := toolResultText(t, r[4]); !isErr {
		t.Error("jeu inconnu non signalé")
	}
	if r[5]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Error("méthode inconnue")
	}
	if r[6]["error"].(map[string]any)["code"].(float64) != -32700 {
		t.Error("erreur de parsing")
	}
}

func TestMCPRemote(t *testing.T) {
	f := setup(t)
	port := freePort(t)
	c := defaultConfig()
	c.Settings.UIPort = port
	_ = saveConfig(c)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: newMux()}
	go srv.Serve(ln)
	defer srv.Close()
	if _, ok := pickBackend(c).(remoteBackend); !ok {
		t.Fatal("l'instance en cours devrait être utilisée")
	}
	r := mcpRun(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"launch_game","arguments":{"game_id":"acc"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_status","arguments":{}}}`)
	txt, isErr := toolResultText(t, r[0])
	if isErr || !strings.Contains(txt, "805550") && !strings.Contains(txt, "Competizione") {
		t.Errorf("launch via instance : %v %s", isErr, txt)
	}
	if !strings.Contains(strings.Join(f.Calls, ","), "steam://rungameid/805550") {
		t.Errorf("ACC non lancé : %v", f.Calls)
	}
	if txt, _ := toolResultText(t, r[1]); !strings.Contains(txt, `"busy": false`) {
		t.Errorf("get_status : %s", txt)
	}
}
