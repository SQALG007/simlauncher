package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGameImage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RIGSTART_HOME", dir)
	img := filepath.Join(dir, "ams2.png")
	os.WriteFile(img, []byte("\x89PNG\r\n\x1a\nfake"), 0o644)
	secret := filepath.Join(dir, "secret.txt")
	os.WriteFile(secret, []byte("nope"), 0o644)

	c := defaultConfig()
	c.Games[0].Image = secret
	if err := c.validate(); err == nil {
		t.Fatal("une image .txt devrait être refusée")
	}
	c.Games[0].Image = img
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if err := saveConfig(c); err != nil {
		t.Fatal(err)
	}
	mux := newMux()
	get := func(path string) int {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code
	}
	if code := get("/api/image/" + c.Games[0].ID); code != 200 {
		t.Fatalf("image attendue, code %d", code)
	}
	if code := get("/api/image/" + c.Games[1].ID); code != 404 {
		t.Fatalf("jeu sans image : 404 attendu, code %d", code)
	}
	if code := get("/api/image/inconnu"); code != 404 {
		t.Fatalf("jeu inconnu : 404 attendu, code %d", code)
	}
}
