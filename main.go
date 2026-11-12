package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const version = "1.2.0"

func main() {
	gameID := flag.String("game", "", "lance le jeu indiqué (ex. ams2) sans interface — pour le Stream Deck")
	list := flag.Bool("list", false, "affiche la liste des jeux configurés")
	startApps := flag.Bool("apps", false, "démarre seulement les logiciels compagnons du jeu indiqué par -game")
	noWait := flag.Bool("nowait", false, "ne surveille pas la fermeture du jeu")
	mcp := flag.Bool("mcp", false, "serveur MCP sur stdio (pour Claude Desktop et autres assistants IA)")
	tray := flag.Bool("tray", false, "démarre en icône de notification, sans fenêtre (démarrage avec Windows)")
	flag.BoolVar(&justUpdated, "updated", false, "usage interne : relancé après une mise à jour")
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
		if *startApps {
			for _, id := range g.Apps {
				if a, ok := c.app(id); ok {
					if _, err := startApp(a); err != nil {
						log.Print(err)
					}
				}
			}
			return
		}
		if err := launchGame(c, g.ID); err != nil {
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
