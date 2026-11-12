<p align="center"><img src="assets/logo.png" width="128" alt="SimLauncher logo"></p>

# SimLauncher

**One click, or one Stream Deck key, to start a whole sim racing session on Windows.**

A VR sim racing session usually means opening four or five programs in the right order before the game: the VR runtime, the wheel base software, SimHub, a spotter such as Crew Chief… SimLauncher starts each one if it isn't already running, waits for it to be ready, then launches the game. When you're done, one more click closes everything.

[Français](README.fr.md) · [Download the latest release](../../releases/latest)

![SimLauncher](docs/screenshot-games.png)

## Contents

- [Features](#features)
- [Install](#install)
- [Quick start](#quick-start)
- [Settings](#settings)
- [Stream Deck](#stream-deck)
- [Home Assistant and webhooks](#home-assistant-and-webhooks)
- [AI assistants (MCP)](#ai-assistants-mcp)
- [Command line](#command-line)
- [Updates](#updates)
- [Files and privacy](#files-and-privacy)
- [Troubleshooting](#troubleshooting)
- [Build from source](#build-from-source)
- [Version history](#version-history)

## Features

**Launching**
- **A sequence per game**: choose which companion apps start before each game, in which order, with a delay after each one.
- **Already running? Skipped.** Nothing is started twice.
- **Starting lights**: five lights follow the sequence and turn green when the game is launched.
- **End session**: one button, Stream Deck key or tray menu entry closes the game and every companion app.
- **Optional auto-close**: when the game exits, SimLauncher closes the apps you marked (Crew Chief, for example).

**Your library**
- **Steam detection**: finds the racing games installed in Steam and adds the ones you tick, with their Steam picture, process name and companion apps.
- **Game pictures**: use the Steam banner or any .jpg, .png or .webp image as the card background, and hide the name when the picture already shows the logo.

**Integrations**
- **Stream Deck**: a ready-to-paste command for every game and for *End session*. Original key icons are included.
- **Home Assistant and webhooks**: a POST request at the start and end of every session, to switch on the cockpit lights or set a scene.
- **AI assistants (MCP)**: Claude Desktop and other assistants can launch games, end the session, add games and apps, or import your Steam games.

**Everyday use**
- **Tray icon**: starts with Windows as an icon next to the clock. Right-click to launch a game or end the session.
- **Built-in updates**: a banner when a new version is out, one-click install, or fully automatic if you allow it (never during a session). Every download is checked against the release's SHA-256.
- **5 languages**: English, Français, Português (Brasil), Español, 简体中文. Follows the Windows language by default.
- **Single portable .exe**: no installer, nothing else to install.

## Install

1. Download `SimLauncher.exe` from the [latest release](../../releases/latest) and put it in a folder of your choice (the Desktop works; avoid `Program Files`, which blocks self-updates).
2. Double-click it. Windows SmartScreen may warn about an unsigned app: *More info › Run anyway*.
3. The interface opens in its own window.

From then on, SimLauncher updates itself.

## Quick start

1. **Settings › Games › Find my Steam games**: tick your racing games and click *Add selected*. They arrive with their Steam picture.
2. **Settings › Companion apps**: check the path of each app (VR runtime, wheel software, SimHub, Crew Chief…). A missing app shows *Executable not found* in the Games tab. Add yours with *Add an app*.
3. Tick **Start with Windows** if you want the tray icon, then click **Save**.
4. In **Games**, click **Launch** on a card. The lights follow each step.
5. At the end of the session, click **End session**.

The default setup is a Pimax + MOZA + SimHub + Crew Chief rig with AMS2, ACC, Assetto Corsa, AC EVO, AC Rally, Le Mans Ultimate, RaceRoom and BeamNG.drive. Everything can be changed.

## Settings

| Section | What you set |
|---|---|
| **Sequence** | Pause before the game, auto-close of apps when the game exits, start with Windows, interface port. |
| **Updates** | Check for new versions automatically (on by default), install them automatically (off by default), *Check now*. |
| **Home Assistant and webhooks** | Two URLs called at the start and end of a session, each with a *Test* button. See [below](#home-assistant-and-webhooks). |
| **Companion apps** | Name, path, arguments, process name, delay after start, *Close after* (closed when the game exits). The order of the list is the start order. |
| **Games** | *Find my Steam games*, then for each game: name, ID, Steam AppID or executable, arguments, process name, colour, card picture (*Browse* or *Steam picture*), *Hide the name*, apps to start first. |

**Process names** (for example `AMS2AVX.exe`) let SimLauncher know whether something is already running, and close it at the end. The exact name is in Task Manager › Details.

**Steam detection** reads your Steam library on disk. It knows the main racing titles (AMS2, ACC, AC, AC EVO, AC Rally, LMU, RaceRoom, BeamNG.drive, rFactor 2, iRacing, Automobilista, Project CARS 2, EA WRC, DiRT Rally 2.0, F1 24/25, Forza…); tick *All games* to list everything else. Pictures are copied into SimLauncher's folder, so they keep working when Steam renames its cache. If a game has no picture, open it once in your Steam library.

## Stream Deck

1. Open the **Stream Deck** tab and copy a command, for example:
   `"C:\Users\you\Desktop\SimLauncher.exe" --game ams2`
2. In the Stream Deck app, drag a **System › Open** action onto a key and paste the command.
3. Set a key image: the release includes original icons, and `assets/streamdeck` has the sources.

The *End session* key uses `--end`. Launching from the Stream Deck shows no window; a message appears only if something goes wrong.

## Home Assistant and webhooks

SimLauncher sends a `POST` request with this JSON body:

```json
{ "event": "session_start", "game_id": "ams2", "game": "Automobilista 2", "time": "2026-11-12T21:30:00+01:00", "source": "SimLauncher" }
```

`event` is `session_start` when a launch begins, and `session_end` when the game closes or you click *End session* (sent only once per session). Any service that accepts a webhook works: Home Assistant, Node-RED, IFTTT…

**Home Assistant example** (`automations.yaml`):

```yaml
- alias: Sim rig on
  triggers:
    - trigger: webhook
      webhook_id: simlauncher_start
      allowed_methods: [POST]
      local_only: true
  actions:
    - action: light.turn_on
      target: { entity_id: light.cockpit }
    - action: notify.notify
      data: { message: "Session started: {{ trigger.json.game }}" }

- alias: Sim rig off
  triggers:
    - trigger: webhook
      webhook_id: simlauncher_end
      allowed_methods: [POST]
      local_only: true
  actions:
    - action: light.turn_off
      target: { entity_id: light.cockpit }
```

Then, in SimLauncher: *Start* = `http://homeassistant.local:8123/api/webhook/simlauncher_start`, *End* = `http://homeassistant.local:8123/api/webhook/simlauncher_end`. Save, then click *Test*.

## AI assistants (MCP)

SimLauncher speaks the [Model Context Protocol](https://modelcontextprotocol.io) over stdio. In Claude Desktop, add this to `claude_desktop_config.json` and restart it:

```json
{
  "mcpServers": {
    "simlauncher": {
      "command": "C:\\Path\\To\\SimLauncher.exe",
      "args": ["--mcp"]
    }
  }
}
```

| Tool | What it does |
|---|---|
| `list_games`, `list_apps`, `get_status` | Games, apps (running or missing) and the current step |
| `launch_game` | Runs the full sequence for a game |
| `start_app`, `stop_app` | Starts or closes one app |
| `end_session` | Closes the game and every app |
| `add_game`, `add_app` | Adds a game (Steam AppID or .exe) or an app, optionally to some games |
| `scan_steam_games`, `import_steam_games` | Lists installed Steam games and adds them with their picture |

Examples: *"Launch Le Mans Ultimate"*, *"Is SimHub running?"*, *"Add my new Steam racing games"*, *"Add OpenKneeboard before AMS2 and LMU"*, *"End the session"*.

When SimLauncher is already running in the tray, the MCP server relays commands to it, so the interface stays in sync. Claude Desktop keeps its own `SimLauncher.exe --mcp` processes running while it is open; that is expected.

## Command line

```
SimLauncher.exe                         open the interface
SimLauncher.exe --game <id>             run the launch sequence (Stream Deck, shortcuts)
SimLauncher.exe --game <id> --apps      only start the companion apps
SimLauncher.exe --game <id> --nowait    don't watch for the game exit
SimLauncher.exe --end                   end the session: close the game and every app
SimLauncher.exe --tray                  tray icon, no window
SimLauncher.exe --mcp                   MCP server on stdio
SimLauncher.exe --list                  print game IDs
```

## Updates

SimLauncher checks [GitHub Releases](../../releases) at startup and every 6 hours. When a new version is out, a green banner offers **Update now** and **What's new**. The new exe is downloaded, its size and SHA-256 are checked against the release, then it replaces the old one and SimLauncher restarts. With *Install updates automatically*, this happens on its own, but never while a launch is running or a game is being watched.

After an update, restart Claude Desktop so the MCP server uses the new version.

## Files and privacy

Everything stays on your PC, in `%APPDATA%\SimLauncher`:

| File | Content |
|---|---|
| `config.json` | Your settings |
| `covers\` | Pictures copied from Steam |
| `launcher.log` | Log of the last sessions |
| `crash.log` | Error details, if SimLauncher ever crashes |

SimLauncher only connects to GitHub (update check and download) and to the webhook URLs you enter. The interface listens on `127.0.0.1:47821` only, and every request that changes something must carry an `X-SimLauncher` header, so web pages can't trigger a launch.

## Troubleshooting

- **An app shows "Executable not found"**: fix its path in Settings › Companion apps.
- **Apps don't close when the game exits**: the game's process name is wrong or empty. Check it in Task Manager › Details.
- **"Find my Steam games" finds nothing**: Steam must be installed for the current Windows user. Games in other libraries are found through Steam's `libraryfolders.vdf`.
- **Two `SimLauncher.exe` in Task Manager after quitting**: they are the MCP servers started by Claude Desktop (`--mcp`). They close with Claude Desktop.
- **The update fails with "Access denied"**: SimLauncher is in a protected folder such as `Program Files`. Move the exe to a folder you own.
- **Something else**: send `launcher.log` (and `crash.log` if it isn't empty) in an [issue](../../issues).

## Build from source

Requires Go 1.23 or later. Builds the Windows exe from any OS:

```sh
go install github.com/akavel/rsrc@v0.10.2   # optional, embeds the icon
./build.sh                                   # → dist/SimLauncher.exe
```

`go test ./...` runs on any OS (Windows calls are replaced by a fake). Node.js is used, when present, to check that every interface language has every string.

| File | Role |
|---|---|
| `launcher.go`, `session.go` | Launch sequence, end of session, webhooks |
| `steam.go` | Steam library and pictures |
| `update.go` | Self-update |
| `config.go`, `server.go`, `ui/index.html` | Settings, local API, interface |
| `mcp.go`, `mcp_add.go` | MCP server |
| `tray_windows.go`, `sys_windows.go` | Windows tray, processes, dialogs |
| `i18n.go` | Engine and tray strings |

**Translations**: engine strings are in `i18n.go` (`catalog`), interface strings in the `I18N` object of `ui/index.html`. To add a language, add its code to `languages`, a column in `catalog`, a block in `I18N` and an `<option>` in the language menu; `go test` fails if a string or a placeholder is missing. Corrections from native speakers are very welcome.

## Version history

| Version | Changes |
|---|---|
| 1.4.1 | Updates work even when Claude Desktop's MCP server still uses the previous exe. |
| 1.4.0 | Steam detection with pictures, *End session* (button, tray, `--end`, MCP), Home Assistant webhooks, MCP tools `end_session`, `scan_steam_games`, `import_steam_games`. |
| 1.3.0 | MCP tools `add_game` and `add_app`. |
| 1.2.0 | Game pictures, *Hide the name*, built-in updates, version shown, crash log; fixed the *Browse* buttons. |
| 1.1.0 | First public release: launch sequences, starting lights, tray icon, Stream Deck, MCP server, 5 languages. |

## License

MIT. SimLauncher is not affiliated with Pimax, MOZA Racing, SimHub, Crew Chief, Elgato, Valve, Home Assistant or any game publisher. Game names and Steam pictures belong to their owners; SimLauncher ships no game artwork, and the bundled Stream Deck icons are original.
