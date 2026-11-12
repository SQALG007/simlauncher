package main

import (
	"os/exec"
	"strings"
	"testing"
)

// Vérifie avec Node (si présent) que chaque langue de l'interface a toutes les clés de l'anglais.
func TestUIDictionaryComplete(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node absent")
	}
	b, _ := uiFS.ReadFile("ui/index.html")
	s := string(b)
	i, j := strings.Index(s, "const I18N="), strings.Index(s, "let L='en';")
	if i < 0 || j < 0 {
		t.Fatal("dictionnaire introuvable")
	}
	js := s[i:j] + `
const ref=Object.keys(I18N.en); let bad=[];
for(const l of ['fr','en','pt','es','zh']){ if(!I18N[l]){bad.push(l+':absent');continue}
  for(const k of ref){ const v=I18N[l][k]; if(!v) bad.push(l+':'+k);
    const a=(I18N.en[k].match(/\{\w+\}/g)||[]).join(), b2=(v||'').match(/\{\w+\}/g)||[]; if(a!==b2.join()) bad.push(l+':'+k+':params'); }
  for(const k of Object.keys(I18N[l])) if(!(k in I18N.en)) bad.push(l+':extra:'+k); }
const used=new Set([...document_src.matchAll(/(?:t\('|data-i18n(?:-html)?=")(\w+)/g)].map(m=>m[1]));
for(const k of used) if(!(k in I18N.en)) bad.push('missing-used:'+k);
console.log(bad.join('\n'));`
	js = "const document_src=process.env.DOC_SRC;\n" + js
	cmd := exec.Command("node", "-e", js)
	cmd.Env = append(cmd.Environ(), "DOC_SRC="+s)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if r := strings.TrimSpace(string(out)); r != "" {
		t.Errorf("problèmes de traduction :\n%s", r)
	}
}
