package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Sys isole tout ce qui dépend de Windows, pour pouvoir tester la logique ailleurs.
type Sys interface {
	Running() (map[string]bool, error) // noms de processus en minuscules
	Start(path, args, dir string) error
	OpenURL(url string) error
	Kill(process string) error
	FileExists(path string) bool
}

var sysImpl Sys = newSys()

// sleep est remplaçable dans les tests.
var sleep = time.Sleep

type Status struct {
	Busy     bool     `json:"busy"`
	Watching int      `json:"watching"`
	Done     int      `json:"done"`
	Total    int      `json:"total"`
	Result   string   `json:"result"` // "", "ok", "warn", "error"
	Game     string   `json:"game"`
	Step     string   `json:"step"`
	History  []string `json:"history"`
}

var (
	stMu   sync.Mutex
	status Status
)

func setStep(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	log.Print(msg)
	stMu.Lock()
	status.Step = msg
	status.History = append(status.History, time.Now().Format("15:04:05")+"  "+msg)
	if len(status.History) > 60 {
		status.History = status.History[len(status.History)-60:]
	}
	stMu.Unlock()
}

func getStatus() Status {
	stMu.Lock()
	defer stMu.Unlock()
	s := status
	s.History = append([]string(nil), status.History...)
	return s
}

func isRunning(running map[string]bool, proc string) bool {
	return proc != "" && running[strings.ToLower(proc)]
}

// startApp démarre un logiciel s'il ne tourne pas. Retourne true s'il a été démarré.
func startApp(a App) (bool, error) {
	running, err := sysImpl.Running()
	if err != nil {
		return false, err
	}
	if isRunning(running, a.Process) {
		setStep("%s", T("app_already", a.Name))
		return false, nil
	}
	if !sysImpl.FileExists(a.Path) {
		return false, TE("app_missing", a.Name, a.Path)
	}
	setStep("%s", T("app_starting", a.Name))
	if err := sysImpl.Start(a.Path, a.Args, filepath.Dir(a.Path)); err != nil {
		return false, fmt.Errorf("%s : %w", a.Name, err)
	}
	return true, nil
}

// launchGame exécute la séquence : logiciels compagnons, pause, puis le jeu.
func launchGame(c Config, gameID string) error {
	g, ok := c.game(gameID)
	if !ok {
		return TE("game_unknown", gameID)
	}
	stMu.Lock()
	if status.Busy {
		stMu.Unlock()
		return TE("busy", status.Game)
	}
	status.Busy, status.Game = true, g.Name
	status.Done, status.Total, status.Result = 0, len(g.Apps)+2, ""
	stMu.Unlock()
	tick := func() { stMu.Lock(); status.Done++; stMu.Unlock() }
	defer func() {
		stMu.Lock()
		status.Busy = false
		stMu.Unlock()
	}()

	setStep("%s", T("seq_start", g.Name))
	fireStart(c, g)
	started := 0
	var errs []string
	for _, id := range g.Apps {
		a, ok := c.app(id)
		if !ok {
			continue
		}
		did, err := startApp(a)
		tick()
		if err != nil {
			setStep("⚠ %v", err)
			errs = append(errs, err.Error())
			continue
		}
		if did {
			started++
			if a.DelayAfter > 0 {
				sleep(time.Duration(a.DelayAfter) * time.Second)
			}
		}
	}
	if started > 0 && c.Settings.DelayBeforeGame > 0 {
		setStep("%s", T("pause", c.Settings.DelayBeforeGame))
		sleep(time.Duration(c.Settings.DelayBeforeGame) * time.Second)
	}

	tick()
	running, _ := sysImpl.Running()
	if isRunning(running, g.Process) {
		setStep("%s", T("game_running", g.Name))
	} else {
		setStep("%s", T("game_launch", g.Name))
		var err error
		if g.SteamAppID != "" {
			err = sysImpl.OpenURL("steam://rungameid/" + g.SteamAppID)
		} else {
			err = sysImpl.Start(g.ExePath, g.ExeArgs, filepath.Dir(g.ExePath))
		}
		if err != nil {
			setStep("%s", T("game_fail", g.Name, err))
			stMu.Lock()
			status.Result = "error"
			stMu.Unlock()
			return err
		}
	}
	stMu.Lock()
	status.Done = status.Total
	if len(errs) > 0 {
		status.Result = "warn"
	} else {
		status.Result = "ok"
	}
	stMu.Unlock()
	if len(errs) > 0 {
		setStep("%s", T("done_warn", len(errs)))
	} else {
		setStep("%s", T("done_ok", g.Name))
	}

	return nil
}

// maybeWatch surveille le jeu si l'option « fermer les logiciels » est active.
func maybeWatch(c Config, gameID string) {
	g, ok := c.game(gameID)
	if !ok || g.Process == "" || (!c.Settings.CloseAppsOnExit && c.Settings.WebhookEnd == "") {
		return
	}
	stMu.Lock()
	status.Watching++
	stMu.Unlock()
	defer func() { stMu.Lock(); status.Watching--; stMu.Unlock() }()
	watchAndClose(c, g)
}

// watchAndClose attend que le jeu démarre puis se ferme, et ferme les logiciels marqués.
func watchAndClose(c Config, g Game) {
	setStep("%s", T("watch", g.Process))
	deadline := time.Now().Add(5 * time.Minute)
	for {
		r, err := sysImpl.Running()
		if err == nil && isRunning(r, g.Process) {
			break
		}
		if time.Now().After(deadline) {
			setStep("%s", T("watch_timeout", g.Process))
			return
		}
		sleep(3 * time.Second)
	}
	for {
		sleep(5 * time.Second)
		r, err := sysImpl.Running()
		if err == nil && !isRunning(r, g.Process) {
			break
		}
	}
	setStep("%s", T("game_closed", g.Name))
	defer fireEnd(c, g)
	if !c.Settings.CloseAppsOnExit {
		return
	}
	for _, id := range g.Apps {
		a, ok := c.app(id)
		if ok && a.CloseOnExit && a.Process != "" {
			setStep("%s", T("app_closing", a.Name))
			if err := sysImpl.Kill(a.Process); err != nil {
				setStep("⚠ %s : %v", a.Name, err)
			}
		}
	}
}

func setupLog() {
	_ = os.MkdirAll(configDir(), 0o755)
	f, err := os.OpenFile(filepath.Join(configDir(), "launcher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		if st, _ := f.Stat(); st != nil && st.Size() > 2<<20 {
			_ = f.Truncate(0)
		}
		log.SetOutput(f)
	}
	log.SetFlags(log.LstdFlags)
	// En cas de plantage, la trace d'erreur de Go est écrite ici (sinon elle est perdue : pas de console).
	if cf, err := os.OpenFile(filepath.Join(configDir(), "crash.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_ = debug.SetCrashOutput(cf, debug.CrashOptions{})
	}
}
