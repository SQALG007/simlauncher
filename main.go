package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const version = "1.6.0"

func main() {
	gameID := flag.String("game", "", "lance le jeu indiqué (ex. ams2) sans interface — pour le Stream Deck")
	list := flag.Bool("list", false, "affiche la liste des jeux configurés")
	profile := flag.String("profile", "", "profil de séquence (ex. vr, ecran) — avec -game")
	startApps := flag.Bool("apps", false, "démarre seulement les logiciels compagnons du jeu indiqué par -game")
	noWait := flag.Bool("nowait", false, "ne surveille pas la fermeture du jeu")
	mcp := flag.Bool("mcp", false, "serveur MCP sur stdio (pour Claude Desktop et autres assistants IA)")
	tray := flag.Bool("tray", false, "démarre en icône de notification, sans fenêtre (démarrage avec Windows)")
	flag.BoolVar(&justUpdated, "updated", false, "usage interne : relancé après une mise à jour")
	end := flag.Bool("end", false, "fin de session : ferme le jeu et tous les logiciels compagnons — pour le Stream Deck")
	flag.Parse()

	setupLog()
	c, err := loadConfig()
	if err != nil && !*mcp {
		messageBox("SimLauncher", T("cli_cfg", err, configPath()))
		os.Exit(1)
	}

	switch {
	case *mcp:
		if err := runMCP(os.Stdin, os.Stdout); err != nil {
			log.Print(err)
			os.Exit(1)
		}
	case *end:
		// Si SimLauncher tourne déjà, c'est lui qui ferme (et qui envoie le webhook une seule fois).
		r := remoteBackend{base: fmt.Sprintf("http://127.0.0.1:%d", c.Settings.UIPort)}
		if _, err := r.state(); err == nil {
			if err := r.do("POST", "/api/end", struct{}{}, nil); err == nil {
				return
			}
		}
		if _, err := endSession(c); err != nil {
			messageBox("SimLauncher", err.Error())
			os.Exit(3)
		}
	case *list:
		for _, g := range c.Games {
			fmt.Printf("%-10s %s\n", g.ID, g.Name)
		}
	case *gameID != "":
		g, ok := c.game(*gameID)
		if !ok {
			messageBox("SimLauncher", T("cli_unknown", *gameID))
			os.Exit(2)
		}
		apps, _, err := c.sequence(g, *profile)
		if err != nil {
			messageBox("SimLauncher", err.Error())
			os.Exit(2)
		}
		if *startApps {
			for _, id := range apps {
				if a, ok := c.app(id); ok {
					if _, err := startApp(a); err != nil {
						log.Print(err)
					}
				}
			}
			return
		}
		if err := launchGameProfile(c, g.ID, *profile); err != nil {
			messageBox("SimLauncher", err.Error())
			os.Exit(3)
		}
		// Signale les logiciels introuvables (le jeu a quand même été lancé).
		var missing []string
		for _, h := range getStatus().History {
			if len(h) > 10 && containsWarn(h) {
				missing = append(missing, h[10:])
			}
		}
		if len(missing) > 0 {
			messageBox(T("warn_title"), strings.Join(missing, "\n"))
		}
		if !*noWait {
			maybeWatch(c, g.ID)
		}
	default:
		cleanupOldExe()
		if err := runUI(c, *tray); err != nil {
			messageBox("SimLauncher", err.Error())
			os.Exit(1)
		}
	}
}

func containsWarn(s string) bool {
	for _, r := range s {
		if r == '⚠' {
			return true
		}
	}
	return false
}
