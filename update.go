package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Mise à jour depuis les releases GitHub du projet.
var (
	releasesAPI    = "https://api.github.com/repos/SQALG007/simlauncher/releases/latest"
	downloadPrefix = "https://github.com/SQALG007/simlauncher/releases/download/"
	assetName      = "SimLauncher.exe"
)

type UpdateInfo struct {
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Available  bool   `json:"available"`
	NotesURL   string `json:"notesUrl,omitempty"`
	Checked    int64  `json:"checked,omitempty"` // horodatage Unix de la dernière vérification
	Error      string `json:"error,omitempty"`
	Installing bool   `json:"installing"`
	assetURL   string
	assetSize  int64
	digest     string
}

var (
	updMu     sync.Mutex
	updState  = UpdateInfo{Current: version}
	updNotify bool // notification déjà affichée pour cette version
)

func getUpdate() UpdateInfo {
	updMu.Lock()
	defer updMu.Unlock()
	return updState
}

// parseVersion transforme "v1.2.3" en [1 2 3].
func parseVersion(s string) [3]int {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	for i, p := range strings.SplitN(s, ".", 3) {
		n, _ := strconv.Atoi(p)
		v[i] = n
	}
	return v
}

func newerVersion(latest, current string) bool {
	a, b := parseVersion(latest), parseVersion(current)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// checkUpdate interroge GitHub et met à jour updState.
func checkUpdate() UpdateInfo {
	info, err := fetchLatest()
	updMu.Lock()
	defer updMu.Unlock()
	installing := updState.Installing
	if err != nil {
		updState.Error = err.Error()
		updState.Checked = time.Now().Unix()
		return updState
	}
	info.Installing = installing
	updState = info
	return updState
}

func fetchLatest() (UpdateInfo, error) {
	info := UpdateInfo{Current: version, Checked: time.Now().Unix()}
	req, _ := http.NewRequest("GET", releasesAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "SimLauncher/"+version)
	resp, err := httpClient.Do(req)
	if err != nil {
		return info, TE("upd_fail", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return info, TE("upd_fail", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
		Pre     bool   `json:"prerelease"`
		Assets  []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel); err != nil {
		return info, TE("upd_fail", err)
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.NotesURL = rel.HTMLURL
	for _, a := range rel.Assets {
		if a.Name == assetName && strings.HasPrefix(a.URL, downloadPrefix) {
			info.assetURL, info.assetSize, info.digest = a.URL, a.Size, a.Digest
		}
	}
	info.Available = !rel.Draft && !rel.Pre && info.assetURL != "" && newerVersion(rel.TagName, version)
	return info, nil
}

// installUpdate télécharge le nouvel exe, le met à la place de l'actuel et relance SimLauncher.
func installUpdate() error {
	updMu.Lock()
	if updState.Installing {
		updMu.Unlock()
		return nil
	}
	if !updState.Available {
		updMu.Unlock()
		return TE("upd_none")
	}
	updState.Installing = true
	info := updState
	updMu.Unlock()

	fail := func(err error) error {
		updMu.Lock()
		updState.Installing = false
		updState.Error = err.Error()
		updMu.Unlock()
		log.Printf("Mise à jour : %v", err)
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fail(TE("upd_fail", err))
	}
	log.Printf("Mise à jour vers %s…", info.Latest)
	// Nom unique pour l'ancien exe : un autre processus (le serveur MCP lancé par Claude Desktop,
	// par exemple) peut encore utiliser celui d'une mise à jour précédente.
	newPath, oldPath := exe+".new", fmt.Sprintf("%s.old-%d", exe, time.Now().Unix())
	removeOldExes(exe)
	if err := download(info, newPath); err != nil {
		os.Remove(newPath)
		return fail(err)
	}
	// Windows autorise à renommer un exe en cours d'exécution, pas à l'écraser.
	if err := os.Rename(exe, oldPath); err != nil {
		os.Remove(newPath)
		return fail(TE("upd_fail", err))
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Rename(oldPath, exe)
		return fail(TE("upd_fail", err))
	}
	args := []string{"--updated"}
	if trayMode {
		args = append(args, "--tray")
	}
	if err := exec.Command(exe, args...).Start(); err != nil {
		os.Rename(exe, newPath)
		os.Rename(oldPath, exe)
		return fail(TE("upd_fail", err))
	}
	log.Printf("Mise à jour installée, redémarrage en %s", info.Latest)
	go func() { time.Sleep(500 * time.Millisecond); quitApp(0) }()
	return nil
}

func download(info UpdateInfo, dst string) error {
	req, _ := http.NewRequest("GET", info.assetURL, nil)
	req.Header.Set("User-Agent", "SimLauncher/"+version)
	resp, err := httpClient.Do(req)
	if err != nil {
		return TE("upd_fail", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return TE("upd_fail", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	f, err := os.Create(dst)
	if err != nil {
		return TE("upd_fail", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 200<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return TE("upd_fail", err)
	}
	if info.assetSize > 0 && n != info.assetSize {
		return TE("upd_bad_file")
	}
	if d, ok := strings.CutPrefix(info.digest, "sha256:"); ok && !strings.EqualFold(d, hex.EncodeToString(h.Sum(nil))) {
		return TE("upd_bad_file")
	}
	// Doit être un exécutable Windows.
	b := make([]byte, 2)
	if f, err := os.Open(dst); err == nil {
		io.ReadFull(f, b)
		f.Close()
	}
	if string(b) != "MZ" {
		return TE("upd_bad_file")
	}
	return nil
}

// removeOldExes supprime les anciens exe laissés par les mises à jour (ceux encore utilisés restent).
func removeOldExes(exe string) (left int) {
	old, _ := filepath.Glob(exe + ".old*")
	for _, p := range old {
		if os.Remove(p) != nil {
			left++
		}
	}
	return left
}

// cleanupOldExe réessaie pendant 20 s : l'ancienne instance met un moment à se fermer.
func cleanupOldExe() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	go func() {
		for i := 0; i < 20 && removeOldExes(exe) > 0; i++ {
			time.Sleep(time.Second)
		}
	}()
}

// updateLoop vérifie au démarrage puis toutes les 6 heures, et installe si l'utilisateur l'a demandé.
func updateLoop() {
	time.Sleep(5 * time.Second)
	for {
		c, err := loadConfig()
		if err == nil && !c.Settings.NoUpdateCheck {
			u := checkUpdate()
			if u.Available {
				st := getStatus()
				idle := !st.Busy && st.Watching == 0
				if c.Settings.AutoUpdate && idle {
					trayNotify("SimLauncher", T("upd_installing_n", u.Latest))
					_ = installUpdate()
				} else if c.Settings.AutoUpdate {
					time.Sleep(10 * time.Minute) // une session est en cours : on réessaie plus tard
					continue
				} else {
					updMu.Lock()
					first := !updNotify
					updNotify = true
					updMu.Unlock()
					if first {
						trayNotify("SimLauncher", T("upd_available_n", u.Latest))
					}
				}
			}
		}
		time.Sleep(6 * time.Hour)
	}
}
