package main

// Temps de jeu : l'instance principale (fenêtre ou icône) surveille les processus des jeux
// configurés, même lancés hors de RigStart, et cumule le temps dans stats.json.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type GameStats struct {
	Seconds    int64 `json:"seconds"`
	Sessions   int   `json:"sessions"`
	LastPlayed int64 `json:"lastPlayed,omitempty"` // horodatage Unix de la dernière session
}

var (
	statsMu   sync.Mutex
	statsData map[string]GameStats
)

func statsPath() string { return filepath.Join(configDir(), "stats.json") }

func loadStats() map[string]GameStats {
	statsMu.Lock()
	defer statsMu.Unlock()
	if statsData == nil {
		statsData = map[string]GameStats{}
		if b, err := os.ReadFile(statsPath()); err == nil {
			var f struct {
				Games map[string]GameStats `json:"games"`
			}
			if json.Unmarshal(b, &f) == nil && f.Games != nil {
				statsData = f.Games
			}
		}
	}
	out := make(map[string]GameStats, len(statsData))
	for k, v := range statsData {
		out[k] = v
	}
	return out
}

func saveStatsLocked() {
	b, _ := json.MarshalIndent(map[string]any{"games": statsData}, "", "  ")
	tmp := statsPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, statsPath())
	}
}

// statsTracker : appelé régulièrement avec la liste des processus ; cumule le temps des jeux en cours.
type statsTracker struct {
	last    time.Time
	running map[string]bool
	dirty   bool
	saved   time.Time
}

func (t *statsTracker) tick(c Config, procs map[string]bool, now time.Time) {
	loadStats()
	statsMu.Lock()
	defer statsMu.Unlock()
	if t.running == nil {
		t.running = map[string]bool{}
	}
	delta := int64(0)
	if !t.last.IsZero() {
		delta = int64(now.Sub(t.last).Seconds())
		if delta > 120 || delta < 0 { // PC en veille ou horloge changée : on ne compte pas le trou
			delta = 0
		}
	}
	t.last = now
	seen := map[string]bool{}
	for _, g := range c.Games {
		if g.Process == "" || seen[g.ID] {
			continue
		}
		seen[g.ID] = true
		if !isRunning(procs, g.Process) {
			if t.running[g.ID] {
				delete(t.running, g.ID)
				t.dirty = true
			}
			continue
		}
		st := statsData[g.ID]
		if !t.running[g.ID] {
			t.running[g.ID] = true
			st.Sessions++
		} else {
			st.Seconds += delta
		}
		st.LastPlayed = now.Unix()
		statsData[g.ID] = st
		t.dirty = true
	}
	if t.dirty && (now.Sub(t.saved) > time.Minute || len(t.running) == 0) {
		saveStatsLocked()
		t.dirty, t.saved = false, now
	}
}

func statsLoop() {
	var t statsTracker
	for {
		if c, err := loadConfig(); err == nil {
			if procs, err := sysImpl.Running(); err == nil {
				t.tick(c, procs, time.Now())
			}
		}
		time.Sleep(15 * time.Second)
	}
}

// --- temps de jeu enregistré par Steam (userdata/<compte>/config/localconfig.vdf)

var (
	steamPTMu    sync.Mutex
	steamPTFile  string
	steamPTMod   time.Time
	steamPTCache map[string]GameStats
)

// steamPlaytime renvoie, par AppID, le temps de jeu (minutes → secondes) et la dernière partie
// enregistrés par Steam pour le compte utilisé le plus récemment sur ce PC.
func steamPlaytime() map[string]GameStats {
	root := steamRoot()
	if root == "" {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "config", "localconfig.vdf"))
	var best string
	var bestMod time.Time
	for _, f := range files {
		if st, err := os.Stat(f); err == nil && st.ModTime().After(bestMod) {
			best, bestMod = f, st.ModTime()
		}
	}
	if best == "" {
		return nil
	}
	steamPTMu.Lock()
	defer steamPTMu.Unlock()
	if best == steamPTFile && bestMod.Equal(steamPTMod) {
		return steamPTCache
	}
	b, err := os.ReadFile(best)
	if err != nil {
		return nil
	}
	apps := parseVDF(string(b)).sub("UserLocalConfigStore").sub("Software").sub("Valve").sub("Steam").sub("apps")
	out := map[string]GameStats{}
	for id, v := range apps {
		a, ok := v.(vdf)
		if !ok {
			continue
		}
		min, _ := strconv.ParseInt(a.str("Playtime"), 10, 64)
		last, _ := strconv.ParseInt(a.str("LastPlayed"), 10, 64)
		if min > 0 || last > 0 {
			out[id] = GameStats{Seconds: min * 60, LastPlayed: last}
		}
	}
	steamPTFile, steamPTMod, steamPTCache = best, bestMod, out
	return out
}

// gameStats combine le temps de Steam (historique complet) et celui mesuré par RigStart
// (jeux hors Steam, session en cours que Steam n'a pas encore enregistrée).
func gameStats(c Config) map[string]GameStats {
	own := loadStats()
	steam := steamPlaytime()
	out := map[string]GameStats{}
	for _, g := range c.Games {
		st := own[g.ID]
		if s, ok := steam[g.SteamAppID]; ok && g.SteamAppID != "" {
			if s.Seconds > st.Seconds {
				st.Seconds = s.Seconds
			}
			if s.LastPlayed > st.LastPlayed {
				st.LastPlayed = s.LastPlayed
			}
		}
		if st.Seconds > 0 || st.LastPlayed > 0 {
			out[g.ID] = st
		}
	}
	return out
}
