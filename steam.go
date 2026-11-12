package main

// Détection des jeux Steam installés et récupération de leur image depuis le cache local de Steam.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var digitsRe = regexp.MustCompile(`^[0-9]{1,10}$`)

// Jeux de course connus : AppID Steam → identifiant, processus et couleur proposés.
type knownGame struct {
	ID, Process, Color string
	NoCrewChief        bool // jeu non pris en charge par Crew Chief
}

var racingGames = map[string]knownGame{
	"1066890": {"ams2", "AMS2AVX.exe", "#e53935", false},
	"805550":  {"acc", "AC2-Win64-Shipping.exe", "#1e88e5", false},
	"244210":  {"ac", "acs.exe", "#c62828", false},
	"3058630": {"acevo", "AssettoCorsaEVO.exe", "#ef6c00", false},
	"3917090": {"acrally", "", "#2e7d32", true},
	"2399420": {"lmu", "Le Mans Ultimate.exe", "#1565c0", false},
	"211500":  {"raceroom", "RRRE64.exe", "#6a1b9a", false},
	"284160":  {"beamng", "BeamNG.drive.x64.exe", "#f57c00", true},
	"365960":  {"rf2", "rFactor2.exe", "#00838f", false},
	"266410":  {"iracing", "iRacingSim64DX11.exe", "#0d47a1", false},
	"431600":  {"ams1", "AMS.exe", "#b71c1c", false},
	"378860":  {"pcars2", "pCARS2AVX.exe", "#37474f", false},
	"1849250": {"eawrc", "WRC.exe", "#1b5e20", false},
	"690790":  {"dirt2", "dirtrally2.exe", "#e65100", false},
	"2488620": {"f124", "F1_24.exe", "#d50000", false},
	"3059520": {"f125", "F1_25.exe", "#d50000", false},
	"2440510": {"forzams", "", "#283593", true},
	"1551360": {"fh5", "ForzaHorizon5.exe", "#ad1457", true},
	"406350":  {"kartkraft", "", "#558b2f", true},
}

// Outils Steam qui ne sont pas des jeux.
var steamTools = map[string]bool{"228980": true, "250820": true, "1070560": true, "1391110": true, "1493710": true, "1628350": true}

type SteamGame struct {
	AppID      string `json:"appId"`
	Name       string `json:"name"`
	ID         string `json:"id"`
	Process    string `json:"process"`
	Racing     bool   `json:"racing"`
	Configured bool   `json:"configured"`
	HasCover   bool   `json:"hasCover"`
}

// --- lecteur minimal du format VDF/ACF de Valve : "clé" "valeur" et "clé" { … }
type vdf map[string]any

func parseVDF(s string) vdf {
	toks := []string{}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			j := i + 1
			var b strings.Builder
			for ; j < len(s) && s[j] != '"'; j++ {
				if s[j] == '\\' && j+1 < len(s) {
					j++
				}
				b.WriteByte(s[j])
			}
			toks = append(toks, "\x00"+b.String())
			i = j
		case c == '{' || c == '}':
			toks = append(toks, string(c))
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		}
	}
	pos := 0
	var parse func() vdf
	parse = func() vdf {
		m := vdf{}
		for pos < len(toks) {
			t := toks[pos]
			if t == "}" {
				pos++
				return m
			}
			pos++
			key := strings.TrimPrefix(t, "\x00")
			if pos >= len(toks) {
				break
			}
			if toks[pos] == "{" {
				pos++
				m[strings.ToLower(key)] = parse()
			} else {
				m[strings.ToLower(key)] = strings.TrimPrefix(toks[pos], "\x00")
				pos++
			}
		}
		return m
	}
	return parse()
}

func (v vdf) sub(k string) vdf    { m, _ := v[strings.ToLower(k)].(vdf); return m }
func (v vdf) str(k string) string { s, _ := v[strings.ToLower(k)].(string); return s }

// steamLibraries renvoie les dossiers de bibliothèque Steam (dont le dossier principal).
func steamLibraries(root string) []string {
	libs := []string{root}
	for _, p := range []string{filepath.Join(root, "steamapps", "libraryfolders.vdf"), filepath.Join(root, "config", "libraryfolders.vdf")} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, v := range parseVDF(string(b)).sub("libraryfolders") {
			if lib, ok := v.(vdf); ok && lib.str("path") != "" {
				libs = append(libs, lib.str("path"))
			}
		}
		break
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range libs {
		k := strings.ToLower(filepath.Clean(l))
		if !seen[k] {
			seen[k] = true
			out = append(out, l)
		}
	}
	return out
}

// scanSteam liste les jeux installés. Sans all, seulement les jeux de course connus.
func scanSteam(c Config, all bool) ([]SteamGame, error) {
	root := steamRoot()
	if root == "" {
		return nil, TE("steam_missing")
	}
	configured := map[string]bool{}
	for _, g := range c.Games {
		if g.SteamAppID != "" {
			configured[g.SteamAppID] = true
		}
	}
	found := map[string]SteamGame{}
	for _, lib := range steamLibraries(root) {
		files, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			st := parseVDF(string(b)).sub("appstate")
			id, name := st.str("appid"), strings.TrimSpace(st.str("name"))
			if id == "" || name == "" || steamTools[id] || strings.HasPrefix(name, "Steamworks") || strings.Contains(name, "Proton") || strings.Contains(name, "Redistributable") {
				continue
			}
			k, racing := racingGames[id]
			if !racing && !all {
				continue
			}
			sg := SteamGame{AppID: id, Name: name, Racing: racing, Configured: configured[id], HasCover: steamCover(root, id) != ""}
			sg.ID, sg.Process = k.ID, k.Process
			if sg.ID == "" {
				sg.ID = slugID(name, func(string) bool { return false })
			}
			found[id] = sg
		}
	}
	out := make([]SteamGame, 0, len(found))
	for _, g := range found {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Racing != out[j].Racing {
			return out[i].Racing
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// steamCover cherche la bannière du jeu (avec son logo) dans le cache de la bibliothèque Steam.
func steamCover(root, appID string) string {
	cache := filepath.Join(root, "appcache", "librarycache")
	for _, name := range []string{"library_header.jpg", "header.jpg", "library_hero.jpg"} {
		// Nouveau format : librarycache/<appid>/<empreinte>/<fichier> ou librarycache/<appid>/<fichier>
		if m, _ := filepath.Glob(filepath.Join(cache, appID, "*", name)); len(m) > 0 {
			return m[0]
		}
		if p := filepath.Join(cache, appID, name); fileExists(p) {
			return p
		}
	}
	// Ancien format : librarycache/<appid>_header.jpg
	for _, name := range []string{appID + "_header.jpg", appID + "_library_hero.jpg"} {
		if p := filepath.Join(cache, name); fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// copyCover copie l'image Steam dans le dossier de SimLauncher (Steam renomme ses fichiers de cache).
func copyCover(appID, gameID string) (string, error) {
	src := steamCover(steamRoot(), appID)
	if src == "" {
		return "", TE("steam_no_cover")
	}
	dir := filepath.Join(configDir(), "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, gameID+strings.ToLower(filepath.Ext(src)))
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	return dst, out.Close()
}

// importSteam ajoute les jeux Steam choisis, avec leur image et tous les logiciels compagnons.
func importSteam(appIDs []string) ([]Game, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	avail, err := scanSteam(c, true)
	if err != nil {
		return nil, err
	}
	byID := map[string]SteamGame{}
	for _, s := range avail {
		byID[s.AppID] = s
	}
	var added []Game
	for _, appID := range appIDs {
		s, ok := byID[appID]
		if !ok {
			return nil, TE("steam_not_installed", appID)
		}
		if s.Configured {
			continue
		}
		id := s.ID
		if _, taken := c.game(id); taken {
			id = slugID(s.Name, func(x string) bool { _, ok := c.game(x); return ok })
		}
		g := Game{ID: id, Name: s.Name, Enabled: true, SteamAppID: appID, Process: s.Process, Apps: []string{}}
		if k, ok := racingGames[appID]; ok {
			g.Color = k.Color
		} else {
			g.Color = cardColors[len(c.Games)%len(cardColors)]
		}
		for _, a := range c.Apps {
			if racingGames[appID].NoCrewChief && strings.Contains(strings.ToLower(a.Path+a.Process), "crewchief") {
				continue
			}
			g.Apps = append(g.Apps, a.ID)
		}
		if p, err := copyCover(appID, id); err == nil {
			g.Image, g.HideName = p, true
		}
		c.Games = append(c.Games, g)
		added = append(added, g)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return added, saveConfig(c)
}

func steamSummary(l []SteamGame) string {
	var b strings.Builder
	for _, g := range l {
		state := "new"
		if g.Configured {
			state = "already in SimLauncher"
		}
		fmt.Fprintf(&b, "%s — %s (AppID %s, %s)\n", g.ID, g.Name, g.AppID, state)
	}
	return b.String()
}
