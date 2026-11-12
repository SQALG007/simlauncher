//go:build !windows

package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// fakeSys : simulation utilisée pour développer et tester hors Windows.
type fakeSys struct {
	mu      sync.Mutex
	running map[string]bool
	Calls   []string
}

func newSys() Sys { return &fakeSys{running: map[string]bool{}} }

func (f *fakeSys) Running() (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := map[string]bool{}
	for k, v := range f.running {
		m[k] = v
	}
	return m, nil
}
func (f *fakeSys) Start(path, args, dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "start "+path)
	i := strings.LastIndexAny(path, `\/`)
	f.running[strings.ToLower(path[i+1:])] = true
	return nil
}
func (f *fakeSys) OpenURL(url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "url "+url)
	return nil
}
func (f *fakeSys) Kill(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "kill "+p)
	delete(f.running, strings.ToLower(p))
	return nil
}
func (f *fakeSys) FileExists(path string) bool {
	if os.Getenv("SIMLAUNCHER_FAKE_MISSING") != "" && strings.Contains(path, os.Getenv("SIMLAUNCHER_FAKE_MISSING")) {
		return false
	}
	return true
}

func openUI(url string)                       { fmt.Println("UI:", url) }
func messageBox(title, text string)           { fmt.Println(title+":", text) }
func pickFile(initial string) (string, error) { return "", TE("no_pick") }

func systemLang() string { return envLang() }

var fakeAutostart bool

func runTray(url string) error      { select {} }
func trayNotify(title, text string) {}
func getAutostart() bool            { return fakeAutostart }
func setAutostart(on bool) error    { fakeAutostart = on; return nil }
func quitApp(code int)              { os.Exit(code) }
