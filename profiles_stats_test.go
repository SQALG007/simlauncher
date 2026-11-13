//go:build !windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestProfiles(t *testing.T) {
	f := setup(t)
	c := defaultConfig()
	c.Profiles = []Profile{{ID: "vr", Name: "VR", Apps: []string{"pimax", "pithouse", "simhub", "crewchief"}}, {ID: "flat", Name: "Écran", Apps: []string{"pithouse", "simhub", "crewchief"}}}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if err := launchGameProfile(c, "ams2", "flat"); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(f.Calls, ",")
	if strings.Contains(calls, "Pimax") || !strings.Contains(calls, "SimHub") {
		t.Fatalf("profil Écran : %s", calls)
	}
	if err := launchGameProfile(c, "ams2", "nope"); err == nil {
		t.Fatal("profil inconnu : erreur attendue")
	}
	// BeamNG n'a pas Crew Chief : le profil ne l'ajoute pas.
	apps, _, _ := c.sequence(mustGame(t, c, "beamng"), "vr")
	if strings.Contains(strings.Join(apps, ","), "crewchief") {
		t.Fatal("un profil ne doit pas ajouter de logiciel au jeu")
	}
	bad := c
	bad.Profiles = []Profile{{ID: "x", Name: "X", Apps: []string{"inconnu"}}}
	if bad.validate() == nil {
		t.Fatal("logiciel inconnu dans un profil : erreur attendue")
	}
	// MCP (port libre : pas d'instance en cours)
	c.Settings.UIPort = freePort(t)
	_ = saveConfig(c)
	r := mcpRun(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_games","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"launch_game","arguments":{"game_id":"lmu","profile":"zzz"}}}`)
	if txt, _ := toolResultText(t, r[0]); !strings.Contains(txt, `"profiles"`) || !strings.Contains(txt, `"flat"`) {
		t.Fatalf("list_games : %s", txt)
	}
	if _, isErr := toolResultText(t, r[1]); !isErr {
		t.Fatal("profil inconnu via MCP : erreur attendue")
	}
}

func mustGame(t *testing.T, c Config, id string) Game {
	g, ok := c.game(id)
	if !ok {
		t.Fatal(id)
	}
	return g
}

func TestPlaytime(t *testing.T) {
	setup(t)
	statsData = nil
	c := defaultConfig()
	var tr statsTracker
	t0 := time.Unix(1_800_000_000, 0)
	on := map[string]bool{"ams2avx.exe": true}
	for i := 0; i <= 8; i++ { // 2 minutes de jeu, relevé toutes les 15 s
		tr.tick(c, on, t0.Add(time.Duration(i*15)*time.Second))
	}
	tr.tick(c, map[string]bool{}, t0.Add(135*time.Second))
	tr.tick(c, on, t0.Add(10*time.Minute)) // nouvelle session
	tr.tick(c, map[string]bool{}, t0.Add(10*time.Minute+15*time.Second))
	statsData = nil // relecture depuis le disque
	st := loadStats()["ams2"]
	if st.Seconds != 120 || st.Sessions != 2 || st.LastPlayed != t0.Add(10*time.Minute).Unix() {
		t.Fatalf("stats : %+v", st)
	}
	// Mise en veille : un trou de 2 h n'est pas compté.
	tr.tick(c, on, t0.Add(20*time.Minute))
	tr.tick(c, on, t0.Add(140*time.Minute))
	if s := loadStats()["ams2"]; s.Seconds != 120 {
		t.Fatalf("veille comptée : %+v", s)
	}
}

func TestSteamPlaytime(t *testing.T) {
	setup(t)
	statsData = nil
	root := t.TempDir()
	t.Setenv("RIGSTART_STEAM", root)
	writeFile(t, root+"/userdata/123/config/localconfig.vdf", `"UserLocalConfigStore"
{
	"Software" { "Valve" { "Steam" { "apps" {
		"1066890" { "LastPlayed" "1794478266" "Playtime" "3114" }
		"211500" { "LastPlayed" "1746376880" }
	} } } }
	"apps" { "999" { "Playtime" "5" } }
}`)
	c := defaultConfig()
	s := gameStats(c)
	if s["ams2"].Seconds != 3114*60 || s["ams2"].LastPlayed != 1794478266 {
		t.Fatalf("AMS2 : %+v", s["ams2"])
	}
	if s["raceroom"].Seconds != 0 || s["raceroom"].LastPlayed != 1746376880 {
		t.Fatalf("RaceRoom : %+v", s["raceroom"])
	}
	if _, ok := s["acc"]; ok {
		t.Fatal("ACC n'a pas de données")
	}
}

func TestProfileDisplayName(t *testing.T) {
	setLang("en")
	defer setLang("fr")
	if n := (Profile{ID: "ecran", Name: "Écran"}).DisplayName(); n != "Flat" {
		t.Fatalf("Écran en anglais : %s", n)
	}
	if n := (Profile{ID: "ecran", Name: "Mon écran 49 pouces"}).DisplayName(); n != "Mon écran 49 pouces" {
		t.Fatalf("nom personnalisé : %s", n)
	}
	if n := (Profile{ID: "vr", Name: "VR"}).DisplayName(); n != "VR" {
		t.Fatal(n)
	}
}
