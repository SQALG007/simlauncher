package main

import (
	"regexp"
	"testing"
)

var verbRe = regexp.MustCompile(`%[vsdqw]`)

func TestCatalogComplete(t *testing.T) {
	for key, m := range catalog {
		ref := verbRe.FindAllString(m["en"], -1)
		for _, l := range languages {
			s, ok := m[l]
			if !ok || s == "" {
				t.Errorf("%s : traduction %s manquante", key, l)
				continue
			}
			got := verbRe.FindAllString(s, -1)
			if len(got) != len(ref) {
				t.Errorf("%s/%s : paramètres %v au lieu de %v", key, l, got, ref)
				continue
			}
			for i := range got {
				if got[i] != ref[i] {
					t.Errorf("%s/%s : paramètres %v au lieu de %v", key, l, got, ref)
				}
			}
		}
	}
}

func TestResolveLang(t *testing.T) {
	if resolveLang("pt") != "pt" || resolveLang("ZH") != "zh" {
		t.Error("langue explicite ignorée")
	}
	t.Setenv("LANG", "es_ES.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	if resolveLang("auto") != "es" {
		t.Error("détection système")
	}
	t.Setenv("LANG", "de_DE")
	if resolveLang("") != "en" {
		t.Error("repli anglais")
	}
	setLang("fr")
	if T("done_ok", "AMS2") != "✔ AMS2 lancé" {
		t.Error(T("done_ok", "AMS2"))
	}
	setLang("zh")
	if T("done_ok", "AMS2") != "✔ AMS2 已启动" {
		t.Error(T("done_ok", "AMS2"))
	}
	setLang("en")
}
