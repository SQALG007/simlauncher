<p align="center"><img src="assets/logo.png" width="128" alt="Logo SimLauncher"></p>

# SimLauncher

**Un clic (ou une touche de Stream Deck) pour démarrer toute une session de sim racing sous Windows.**

Une session de sim racing en VR, c'est souvent quatre ou cinq logiciels à ouvrir dans le bon ordre avant le jeu : le runtime VR, le logiciel de la base volant, SimHub, un ingénieur ou spotter comme Crew Chief… SimLauncher démarre chacun s'il ne tourne pas déjà, attend qu'ils soient prêts, puis lance le jeu via Steam.

[English](README.md)

![SimLauncher](docs/screenshot-games.png)

## Fonctionnalités

- **Une séquence par jeu** : choisis les logiciels à démarrer avant chaque jeu, leur ordre et le délai après chacun.
- **Déjà lancé ? On passe.** Un logiciel ou un jeu déjà ouvert n'est pas relancé.
- **Feux de départ** : cinq feux suivent la séquence et passent au vert quand le jeu est lancé.
- **Fermeture automatique (option)** : quand le jeu se termine, ferme les logiciels que tu as cochés (par exemple Crew Chief).
- **Tes propres images de jeux** : choisis une image .jpg, .png ou .webp dans Réglages, elle devient le fond de la carte du jeu.
- **Mises à jour intégrées** : SimLauncher vérifie les nouvelles versions sur GitHub et se met à jour en un clic, ou tout seul si tu l'autorises (jamais pendant une session). Le fichier téléchargé est vérifié avec l'empreinte SHA-256 de la release.
- **Prêt pour le Stream Deck** : chaque jeu a sa commande à coller dans une touche *Système › Ouvrir*. Des icônes de touches sont fournies dans `assets/streamdeck`.
- **Icône près de l'horloge** : démarrage avec Windows en icône. Clic droit pour lancer un jeu, double-clic pour ouvrir l'interface.
- **Serveur MCP** : un assistant IA (Claude Desktop et autres) peut lister et lancer tes jeux.
- **5 langues** : English, Français, Português (Brasil), Español, 简体中文. Suit la langue de Windows par défaut.
- **Un seul .exe portable**, sans installation. Les réglages sont dans `%APPDATA%\SimLauncher\config.json`.

La configuration par défaut vise un setup Pimax + MOZA + SimHub + Crew Chief avec AMS2, ACC, Assetto Corsa, AC EVO, AC Rally, Le Mans Ultimate, RaceRoom et BeamNG.drive. Tout se modifie dans **Réglages** : ajoute n'importe quel logiciel ou jeu (AppID Steam ou exécutable).

## Installation

1. Télécharge `SimLauncher.exe` depuis la [dernière version](../../releases/latest).
2. Place-le dans le dossier de ton choix et double-clique dessus.
3. Windows SmartScreen peut signaler une application non signée : *Informations complémentaires › Exécuter quand même*.

## Utilisation

| Comment | Effet |
|---|---|
| Double-clic sur `SimLauncher.exe` | Ouvre l'interface (fenêtre d'application Edge). Elle s'arrête à la fermeture de la fenêtre, sauf en mode icône. |
| Réglages › *Démarrer avec Windows* | Démarre caché à l'ouverture de session, avec l'icône près de l'horloge (`--tray`). |
| Stream Deck *Système › Ouvrir* avec `"C:\…\SimLauncher.exe" --game ams2` | Lance la séquence sans fenêtre. Un message n'apparaît qu'en cas de problème. |

### Ligne de commande

```
SimLauncher.exe                 ouvre l'interface
SimLauncher.exe --game <id>     lance la séquence d'un jeu (Stream Deck, raccourcis)
SimLauncher.exe --game <id> --apps    démarre seulement les logiciels compagnons
SimLauncher.exe --game <id> --nowait  ne surveille pas la fermeture du jeu
SimLauncher.exe --tray          icône près de l'horloge, sans fenêtre
SimLauncher.exe --mcp           serveur MCP sur stdio
SimLauncher.exe --list          affiche les identifiants des jeux
```

Les identifiants sont visibles dans l'onglet **Stream Deck**.

## Serveur MCP (assistants IA)

SimLauncher parle le [Model Context Protocol](https://modelcontextprotocol.io) sur stdio. Outils : `list_games`, `launch_game`, `list_apps`, `start_app`, `stop_app`, `get_status`. Si SimLauncher tourne déjà en icône, le serveur MCP lui transmet les commandes : l'interface et la surveillance de fin de jeu restent synchronisées.

Claude Desktop, `claude_desktop_config.json` :

```json
{
  "mcpServers": {
    "simlauncher": {
      "command": "C:\\Chemin\\Vers\\SimLauncher.exe",
      "args": ["--mcp"]
    }
  }
}
```

Ensuite, demande : *« Lance Le Mans Ultimate »* ou *« Est-ce que SimHub tourne ? »*

## Compiler

Go 1.22 ou plus. Compile l'exe Windows depuis n'importe quel système :

```sh
go install github.com/akavel/rsrc@v0.10.2   # optionnel, intègre l'icône
./build.sh                                   # → dist/SimLauncher.exe
```

Les tests tournent partout (les appels Windows sont simulés) : `go test ./...`. Node.js, s'il est présent, sert à vérifier que chaque langue de l'interface a toutes ses chaînes.

## Traductions

- Moteur et icône : `i18n.go` (`catalog`).
- Interface : l'objet `I18N` en haut du script de `ui/index.html`.

Pour ajouter une langue : son code dans `languages` (`i18n.go`), une colonne dans `catalog`, un bloc dans `I18N` et une `<option>` dans le menu des langues. `go test` échoue si une chaîne ou un paramètre manque.

## Licence

MIT. SimLauncher n'est affilié ni à Pimax, ni à MOZA Racing, SimHub, Crew Chief, Elgato, Valve, ni à aucun éditeur de jeu. Les noms des jeux sont des marques de leurs propriétaires ; les icônes Stream Deck fournies sont des créations originales.
