package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Le premier lancement de RigStart reprend la configuration de SimLauncher (1.7 et avant).
func TestMigrateLegacy(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("RIGSTART_HOME", "")
	t.Setenv("SIMLAUNCHER_HOME", "")
	t.Setenv("APPDATA", appdata)
	old := filepath.Join(appdata, "SimLauncher")
	os.MkdirAll(filepath.Join(old, "covers"), 0o755)
	os.WriteFile(filepath.Join(old, "covers", "ams2.jpg"), []byte("img"), 0o644)
	os.WriteFile(filepath.Join(old, "stats.json"), []byte(`{"ams2":{"seconds":60}}`), 0o644)
	c := defaultConfig()
	c.Games[0].Image = filepath.Join(old, "covers", "ams2.jpg")
	c.Games[1].Image = filepath.Join(appdata, "ailleurs.png")
	c.Settings.CardStyle = "band"
	b, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(filepath.Join(old, "config.json"), b, 0o644)

	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	nd := filepath.Join(appdata, "RigStart")
	if got.Settings.CardStyle != "band" {
		t.Fatal("réglages non repris")
	}
	if want := filepath.Join(nd, "covers", "ams2.jpg"); got.Games[0].Image != want {
		t.Fatalf("image = %q, attendu %q", got.Games[0].Image, want)
	}
	if got.Games[1].Image != filepath.Join(appdata, "ailleurs.png") {
		t.Fatal("une image hors du dossier covers ne doit pas changer")
	}
	for _, f := range []string{"stats.json", filepath.Join("covers", "ams2.jpg")} {
		if _, err := os.Stat(filepath.Join(nd, f)); err != nil {
			t.Fatalf("%s non copié", f)
		}
	}
	if _, err := os.Stat(filepath.Join(old, "config.json")); err != nil {
		t.Fatal("l'ancien dossier ne doit pas être touché")
	}
}

// Le fichier d'un autre logiciel nommé SimLauncher n'est pas repris.
func TestMigrateLegacyIgnoresOtherApp(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("RIGSTART_HOME", "")
	t.Setenv("SIMLAUNCHER_HOME", "")
	t.Setenv("APPDATA", appdata)
	old := filepath.Join(appdata, "SimLauncher")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"profiles":{"a":1},"games":{}}`), 0o644)
	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Games) != len(defaultConfig().Games) {
		t.Fatal("la configuration par défaut était attendue")
	}
}

// Pendant la transition, l'exe peut s'appeler RigStart.exe ou SimLauncher.exe ; RigStart.exe est préféré.
func TestUpdateAssetNames(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer srv.Close()
	oldAPI, oldPrefix := releasesAPI, downloadPrefix
	releasesAPI, downloadPrefix = srv.URL, srv.URL+"/dl/"
	defer func() { releasesAPI, downloadPrefix = oldAPI, oldPrefix }()

	body = fmt.Sprintf(`{"tag_name":"v99.0.0","assets":[{"name":"SimLauncher.exe","browser_download_url":"%s/dl/SimLauncher.exe","size":5}]}`, srv.URL)
	if u := checkUpdate(); !u.Available || !strings.HasSuffix(u.assetURL, "/SimLauncher.exe") {
		t.Fatalf("ancien nom refusé : %+v", u)
	}
	body = fmt.Sprintf(`{"tag_name":"v99.0.0","assets":[{"name":"RigStart.exe","browser_download_url":"%[1]s/dl/RigStart.exe","size":5},{"name":"SimLauncher.exe","browser_download_url":"%[1]s/dl/SimLauncher.exe","size":5}]}`, srv.URL)
	if u := checkUpdate(); !strings.HasSuffix(u.assetURL, "/RigStart.exe") {
		t.Fatalf("RigStart.exe attendu : %+v", u)
	}
}
