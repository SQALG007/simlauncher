//go:build !windows

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEndSessionAndWebhooks(t *testing.T) {
	f := setup(t)
	var mu sync.Mutex
	var events []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		events = append(events, m["event"].(string)+":"+m["game_id"].(string))
		mu.Unlock()
	}))
	defer srv.Close()
	c := defaultConfig()
	c.Settings.WebhookStart, c.Settings.WebhookEnd = srv.URL+"/start", srv.URL+"/end"
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if err := launchGame(c, "ams2"); err != nil {
		t.Fatal(err)
	}
	f.running["ams2avx.exe"] = true
	closed, err := endSession(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 5 || closed[0] != "Automobilista 2" {
		t.Fatalf("fermés : %v", closed)
	}
	if r, _ := f.Running(); len(r) != 0 {
		t.Fatalf("encore en cours : %v", r)
	}
	// Une seconde fin rapprochée (surveillance du jeu) ne renvoie pas le webhook.
	fireEnd(c, Game{ID: "ams2"})
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	got := strings.Join(events, ",")
	mu.Unlock()
	if !strings.Contains(got, "session_start:ams2") || strings.Count(got, "session_end") != 1 || !strings.Contains(got, "session_end:ams2") {
		t.Fatalf("webhooks reçus : %s", got)
	}

	bad := defaultConfig()
	bad.Settings.WebhookEnd = "ftp://x"
	if bad.validate() == nil {
		t.Fatal("un webhook non http doit être refusé")
	}
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSteamScanAndImport(t *testing.T) {
	setup(t)
	root := t.TempDir()
	lib2 := t.TempDir()
	t.Setenv("RIGSTART_STEAM", root)
	writeFile(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0" { "path" "`+strings.ReplaceAll(root, `\`, `\\`)+`" "apps" { "1066890" "1" } }
	"1" { "path" "`+strings.ReplaceAll(lib2, `\`, `\\`)+`" }
}`)
	manifest := func(dir, id, name string) {
		writeFile(t, filepath.Join(dir, "steamapps", "appmanifest_"+id+".acf"), `"AppState"
{
	"appid"		"`+id+`"
	"name"		"`+name+`"
	"installdir"	"x" // commentaire
}`)
	}
	manifest(root, "1066890", "Automobilista 2")
	manifest(root, "228980", "Steamworks Common Redistributables")
	manifest(lib2, "365960", "rFactor 2")
	manifest(lib2, "620", "Portal 2")
	writeFile(t, filepath.Join(root, "appcache", "librarycache", "365960", "abc123", "library_header.jpg"), "JPEGDATA")

	c := defaultConfig()
	_ = saveConfig(c)
	l, err := scanSteam(c, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 2 || l[0].Name != "Automobilista 2" || !l[0].Configured || l[1].ID != "rf2" || !l[1].HasCover || l[1].Process != "rFactor2.exe" {
		t.Fatalf("scan : %+v", l)
	}
	all, _ := scanSteam(c, true)
	if len(all) != 3 || all[2].Name != "Portal 2" {
		t.Fatalf("scan complet : %+v", all)
	}
	added, err := importSteam([]string{"365960", "1066890"})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0].ID != "rf2" || !added[0].HideName || len(added[0].Apps) != 4 {
		t.Fatalf("import : %+v", added)
	}
	b, _ := os.ReadFile(added[0].Image)
	if string(b) != "JPEGDATA" || !strings.HasPrefix(added[0].Image, configDir()) {
		t.Fatalf("image non copiée : %s", added[0].Image)
	}
	if _, err := importSteam([]string{"999"}); err == nil {
		t.Fatal("jeu non installé : erreur attendue")
	}
	// Outils MCP
	r := mcpRun(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"scan_steam_games","arguments":{}}}`)
	if txt, isErr := toolResultText(t, r[0]); isErr || !strings.Contains(txt, "rf2 — rFactor 2") {
		t.Fatalf("scan_steam_games : %s", txt)
	}
	_ = io.EOF
}
