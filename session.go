package main

// Fin de session (ferme le jeu et les logiciels) et webhooks de début / fin de session
// (Home Assistant, Node-RED, IFTTT… : un simple POST JSON).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	endMu    sync.Mutex
	lastEnd  time.Time // évite d'envoyer deux fois « fin de session » (bouton + surveillance)
	hookHTTP = &http.Client{Timeout: 5 * time.Second}
)

func validWebhook(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	return u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// fireWebhook envoie {"event":…, "game_id":…, "game":…, "time":…} à l'URL. Bloquant, 5 s maximum.
func fireWebhook(url, event string, g Game) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"event": event, "game_id": g.ID, "game": g.Name, "time": time.Now().Format(time.RFC3339), "source": "SimLauncher"})
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SimLauncher/"+version)
	resp, err := hookHTTP.Do(req)
	if err != nil {
		log.Printf("Webhook %s : %v", event, err)
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		err = fmt.Errorf("HTTP %d", resp.StatusCode)
		log.Printf("Webhook %s : %v", event, err)
		return err
	}
	log.Printf("Webhook %s envoyé", event)
	return nil
}

func fireStart(c Config, g Game) {
	if c.Settings.WebhookStart != "" {
		go fireWebhook(c.Settings.WebhookStart, "session_start", g)
	}
}

func fireEnd(c Config, g Game) {
	if c.Settings.WebhookEnd == "" {
		return
	}
	endMu.Lock()
	dup := time.Since(lastEnd) < 30*time.Second
	if !dup {
		lastEnd = time.Now()
	}
	endMu.Unlock()
	if !dup {
		_ = fireWebhook(c.Settings.WebhookEnd, "session_end", g)
	}
}

// endSession ferme les jeux en cours puis tous les logiciels compagnons qui tournent.
func endSession(c Config) (closed []string, err error) {
	running, err := sysImpl.Running()
	if err != nil {
		return nil, err
	}
	setStep("%s", T("end_start"))
	var last Game
	for _, g := range c.Games {
		if g.Process != "" && isRunning(running, g.Process) {
			setStep("%s", T("app_closing", g.Name))
			if err := sysImpl.Kill(g.Process); err != nil {
				setStep("⚠ %s : %v", g.Name, err)
			} else {
				closed = append(closed, g.Name)
				last = g
			}
		}
	}
	for _, a := range c.Apps {
		if a.Process != "" && isRunning(running, a.Process) {
			setStep("%s", T("app_closing", a.Name))
			if err := sysImpl.Kill(a.Process); err != nil {
				setStep("⚠ %s : %v", a.Name, err)
			} else {
				closed = append(closed, a.Name)
			}
		}
	}
	fireEnd(c, last)
	if len(closed) == 0 {
		setStep("%s", T("end_nothing"))
	} else {
		setStep("%s", T("end_done", len(closed)))
	}
	return closed, nil
}
