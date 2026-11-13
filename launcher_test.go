//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) *fakeSys {
	t.Setenv("RIGSTART_HOME", t.TempDir())
	f := &fakeSys{running: map[string]bool{}}
	sysImpl = f
	sleep = func(time.Duration) {}
	status = Status{}
	return f
}

func TestDefaultConfigValid(t *testing.T) {
	c := defaultConfig()
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchSequence(t *testing.T) {
	f := setup(t)
	f.running["simhubwpf.exe"] = true // déjà lancé
	c := defaultConfig()
	if err := launchGame(c, "ams2"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.Calls, "\n")
	want := []string{"start C:\\Program Files\\Pimax", "start C:\\Program Files (x86)\\MOZA", "start C:\\Program Files (x86)\\Britton", "url steam://rungameid/1066890"}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("appel manquant %q dans\n%s", w, got)
		}
	}
	if strings.Contains(got, "SimHub") {
		t.Errorf("SimHub ne devait pas être relancé:\n%s", got)
	}
	// ordre : le jeu en dernier
	if !strings.HasPrefix(f.Calls[len(f.Calls)-1], "url steam://") {
		t.Errorf("le jeu doit être lancé en dernier: %v", f.Calls)
	}
}

func TestNoCrewChiefForBeamNG(t *testing.T) {
	f := setup(t)
	if err := launchGame(defaultConfig(), "beamng"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.Calls, ","), "CrewChief") {
		t.Errorf("Crew Chief lancé pour BeamNG: %v", f.Calls)
	}
}

func TestGameAlreadyRunning(t *testing.T) {
	f := setup(t)
	f.running["ams2avx.exe"] = true
	_ = launchGame(defaultConfig(), "ams2")
	for _, c := range f.Calls {
		if strings.HasPrefix(c, "url") {
			t.Errorf("le jeu ne doit pas être relancé")
		}
	}
}

func TestMissingAppStillLaunchesGame(t *testing.T) {
	f := setup(t)
	t.Setenv("RIGSTART_FAKE_MISSING", "SimHub")
	if err := launchGame(defaultConfig(), "acc"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f.Calls[len(f.Calls)-1], "url steam://rungameid/805550") {
		t.Errorf("le jeu doit être lancé même si un logiciel manque: %v", f.Calls)
	}
	warn := false
	for _, h := range getStatus().History {
		if containsWarn(h) && strings.Contains(h, "SimHub") {
			warn = true
		}
	}
	if !warn {
		t.Errorf("avertissement SimHub absent")
	}
}

func TestWatchCloses(t *testing.T) {
	f := setup(t)
	c := defaultConfig()
	c.Settings.CloseAppsOnExit = true
	_ = launchGame(c, "ams2")
	f.running["ams2avx.exe"] = true
	n := 0
	sleep = func(time.Duration) {
		n++
		if n == 2 {
			f.mu.Lock()
			delete(f.running, "ams2avx.exe")
			f.mu.Unlock()
		}
	}
	maybeWatch(c, "ams2")
	got := strings.Join(f.Calls, ",")
	if !strings.Contains(got, "kill CrewChiefV4.exe") {
		t.Errorf("Crew Chief aurait dû être fermé: %s", got)
	}
	if strings.Contains(got, "kill SimHubWPF.exe") {
		t.Errorf("SimHub ne doit pas être fermé (non marqué): %s", got)
	}
}

func TestValidation(t *testing.T) {
	c := defaultConfig()
	c.Games[0].Apps = append(c.Games[0].Apps, "inconnu")
	if err := c.validate(); err == nil {
		t.Error("logiciel inconnu accepté")
	}
	c = defaultConfig()
	c.Games[1].ID = c.Games[0].ID
	if err := c.validate(); err == nil {
		t.Error("ID en double accepté")
	}
	c = defaultConfig()
	c.Games[0].SteamAppID, c.Games[0].ExePath = "", ""
	if err := c.validate(); err == nil {
		t.Error("jeu sans lancement accepté")
	}
}

func TestAPIGuardAndSave(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	// sans en-tête : refusé
	r, _ := http.Post(srv.URL+"/api/launch/ams2", "application/json", nil)
	if r.StatusCode != 403 {
		t.Errorf("attendu 403, obtenu %d", r.StatusCode)
	}
	// sauvegarde
	c := defaultConfig()
	c.Settings.DelayBeforeGame = 3
	b, _ := json.Marshal(c)
	req, _ := http.NewRequest("POST", srv.URL+"/api/config", bytes.NewReader(b))
	req.Header.Set("X-RigStart", "1")
	r, _ = http.DefaultClient.Do(req)
	if r.StatusCode != 200 {
		t.Fatalf("sauvegarde: %d", r.StatusCode)
	}
	r, _ = http.Get(srv.URL + "/api/state")
	var s stateResp
	_ = json.NewDecoder(r.Body).Decode(&s)
	if s.Config.Settings.DelayBeforeGame != 3 || len(s.Games) != 8 {
		t.Errorf("état inattendu: %+v", s.Config.Settings)
	}
	r, _ = http.Get(srv.URL + "/")
	if r.StatusCode != 200 {
		t.Errorf("UI: %d", r.StatusCode)
	}
}
