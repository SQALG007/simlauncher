package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.2.1", "1.2.0", true}, {"1.10.0", "1.9.9", true}, {"v2.0.0", "1.99.0", true},
		{"1.2.0", "1.2.0", false}, {"v1.1.9", "1.2.0", false}, {"1.3.0-beta", "1.3.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q,%q)=%v", c.a, c.b, got)
		}
	}
}

func TestUpdateCheckAndDownload(t *testing.T) {
	exe := append([]byte("MZ"), make([]byte, 1000)...)
	sum := sha256.Sum256(exe)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprintf(w, `{"tag_name":"v99.0.0","html_url":"https://x/notes","assets":[
			  {"name":"ams2.png","browser_download_url":"%[1]s/dl/ams2.png","size":3},
			  {"name":"SimLauncher.exe","browser_download_url":"%[1]s/dl/SimLauncher.exe","size":%[2]d,"digest":"sha256:%[3]s"}]}`,
				srv.URL, len(exe), hex.EncodeToString(sum[:]))
		case "/dl/SimLauncher.exe":
			w.Write(exe)
		}
	}))
	defer srv.Close()
	oldAPI, oldPrefix := releasesAPI, downloadPrefix
	releasesAPI, downloadPrefix = srv.URL+"/latest", srv.URL+"/dl/"
	defer func() { releasesAPI, downloadPrefix = oldAPI, oldPrefix }()

	u := checkUpdate()
	if !u.Available || u.Latest != "99.0.0" || u.NotesURL != "https://x/notes" {
		t.Fatalf("mise à jour non détectée : %+v", u)
	}
	dst := filepath.Join(t.TempDir(), "new.exe")
	if err := download(u, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); len(b) != len(exe) {
		t.Fatal("fichier incomplet")
	}
	// Empreinte fausse : refusé.
	bad := u
	bad.digest = "sha256:" + hex.EncodeToString(make([]byte, 32))
	if err := download(bad, dst); err == nil {
		t.Fatal("une empreinte fausse devrait être refusée")
	}
	// Un fichier hébergé ailleurs que dans les releases du projet est ignoré.
	downloadPrefix = "https://github.com/SQALG007/simlauncher/releases/download/"
	if u := checkUpdate(); u.Available {
		t.Fatal("un exe hors des releases du projet ne doit pas être proposé")
	}
}

func TestRemoveOldExes(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "SimLauncher.exe")
	for _, n := range []string{"SimLauncher.exe", "SimLauncher.exe.old", "SimLauncher.exe.old-1794517000", "Autre.exe.old"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	if left := removeOldExes(exe); left != 0 {
		t.Fatalf("%d fichiers non supprimés", left)
	}
	l, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(l) != 2 {
		t.Fatalf("restants : %v", l)
	}
}
