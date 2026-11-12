//go:build !windows

package main

import (
	"strings"
	"testing"
)

func TestMCPAddAppAndGame(t *testing.T) {
	setup(t)
	c := defaultConfig()
	c.Settings.UIPort = freePort(t)
	c.Settings.Language = "en"
	_ = saveConfig(c)
	r := mcpRun(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add_app","arguments":{"name":"Open Kneeboard","path":"C:\\Program Files\\OpenKneeboard\\bin\\OpenKneeboardApp.exe","add_to_games":["ams2","lmu"]}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"add_game","arguments":{"name":"iRacing","steam_app_id":266410,"apps":["pimax","openkneeboard"]}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"add_game","arguments":{"name":"Sans lanceur"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"add_game","arguments":{"name":"X","steam_app_id":"1","apps":["nope"]}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"add_app","arguments":{"name":"Y","path":"C:\\y.exe","add_to_games":["nope"]}}}`,
	)
	if txt, isErr := toolResultText(t, r[0]); isErr || !strings.Contains(txt, `"id": "openkneeboard"`) || !strings.Contains(txt, "ams2, lmu") {
		t.Fatalf("add_app : %v %s", isErr, txt)
	}
	if txt, isErr := toolResultText(t, r[1]); isErr || !strings.Contains(txt, "--game iracing") {
		t.Fatalf("add_game : %v %s", isErr, txt)
	}
	for i := 2; i <= 4; i++ {
		if _, isErr := toolResultText(t, r[i]); !isErr {
			t.Errorf("réponse %d : erreur attendue", i+1)
		}
	}
	c, _ = loadConfig()
	g, ok := c.game("iracing")
	if !ok || g.SteamAppID != "266410" || strings.Join(g.Apps, ",") != "pimax,openkneeboard" {
		t.Fatalf("jeu mal enregistré : %+v", g)
	}
	a, ok := c.app("openkneeboard")
	if !ok || a.Process != "OpenKneeboardApp.exe" || a.DelayAfter != 2 {
		t.Fatalf("logiciel mal enregistré : %+v", a)
	}
	ams2, _ := c.game("ams2")
	if !strings.Contains(strings.Join(ams2.Apps, ","), "openkneeboard") {
		t.Error("le logiciel n'a pas été ajouté à AMS2")
	}
	if _, ok := c.app("y"); ok {
		t.Error("un ajout refusé ne doit rien enregistrer")
	}
}
