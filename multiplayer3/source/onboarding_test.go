package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDropInLayoutDetectsOMSIAndPreservesOriginal(t *testing.T) {
	root := t.TempDir()
	original := []byte("original Steam executable")
	_ = os.WriteFile(filepath.Join(root, "Omsi.exe"), original, 0600)
	dir := appDirectory(root)
	if e := ensureLayout(dir); e != nil {
		t.Fatal(e)
	}
	if detectRoot(dir) != root {
		t.Fatal("root detection failed")
	}
	c := localConfig{Version: 3, Root: root, Player: "Maycon", Invitation: "https://directory.example/rooms/" + strings.Repeat("a", 32)}
	server := filepath.Join(dir, "server")
	_ = os.MkdirAll(server, 0755)
	_ = os.WriteFile(filepath.Join(server, "openomsi.exe"), []byte("dedicated"), 0600)
	if e := validateInstall(c, dir); e != nil {
		t.Fatal("drop-in rejected", e)
	}
	if e := saveConfig(dir, c); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(configPath(dir), filepath.Join("OpenOMSI-BBS", "config")) {
		t.Fatal("configuration outside payload")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "Omsi.exe")); !bytes.Equal(b, original) {
		t.Fatal("original executable changed")
	}
}
func TestLanguageFirstAndTranslationsComplete(t *testing.T) {
	dir := appDirectory(t.TempDir())
	if loadLanguage(dir) {
		t.Fatal("language chosen without user")
	}
	defer setLanguage("pt-BR")
	re := regexp.MustCompile(`t "([a-z]+)"`)
	for _, lang := range []string{"pt-BR", "en", "de"} {
		if e := saveLanguage(dir, lang); e != nil {
			t.Fatal(e)
		}
		if !loadLanguage(dir) || language() != lang {
			t.Fatal("language not persisted")
		}
		for _, m := range re.FindAllStringSubmatch(setupPage, -1) {
			a, ok := translations[m[1]]
			if !ok || a[0] == "" || a[1] == "" || a[2] == "" {
				t.Fatal("missing UI translation", m[1])
			}
		}
		p := template.Must(template.New("setup").Funcs(template.FuncMap{"t": tr}).Parse(setupPage))
		for _, stage := range []string{"language", "setup", "created", "done"} {
			var b bytes.Buffer
			v := newSetupView(localConfig{}, dir, "token")
			v.Stage = stage
			v.Invite = "https://example.test/rooms/invite"
			if e := p.Execute(&b, v); e != nil {
				t.Fatal(stage, e)
			}
			if strings.Contains(b.String(), "ZgotmplZ") {
				t.Fatal("template blocked translated JS")
			}
		}
		if lang != "pt-BR" {
			for _, err := range []error{fmt.Errorf("a versão selecionada não confirma protocolo 6 e ônibus livres"), fmt.Errorf("cole o convite HTTPS completo da empresa"), fmt.Errorf("horário inválido"), fmt.Errorf("SHA-256 incorreto")} {
				message := friendlyError(err)
				if strings.Contains(message, "não") || strings.Contains(message, "ônibus") {
					t.Fatal("untranslated critical popup", lang, message)
				}
			}
		}
	}
}
func TestOldCompanySettingsMigrateWithoutChangingInvitation(t *testing.T) {
	launcher := t.TempDir()
	c := localConfig{Version: 3, Root: t.TempDir(), Invitation: "https://directory.example/rooms/" + strings.Repeat("b", 32), Player: "Maycon"}
	b, _ := json.Marshal(c)
	_ = os.WriteFile(filepath.Join(launcher, "multiplayer.json"), b, 0600)
	_ = os.WriteFile(filepath.Join(launcher, "company-admin.json"), []byte(`{"admin_token":"private"}`), 0600)
	dir := appDirectory(launcher)
	migrateConfig(launcher, dir)
	got, e := readConfig(dir)
	if e != nil || got.Invitation != c.Invitation {
		t.Fatal(got, e)
	}
	if _, e := os.Stat(adminPath(dir)); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(launcher, "multiplayer.json")); e != nil {
		t.Fatal("old settings removed")
	}
}
func TestClientPreservesNestedResourcesAndRejectsTraversal(t *testing.T) {
	z := makeRuntimeZip(t, map[string]string{"pkg/openomsi.exe": "game", "pkg/steam_api64.dll": "steam", "pkg/assets/font.bin": "resource"})
	dir := filepath.Join(t.TempDir(), "client")
	r, e := unpackClient(z, dir, "0.2.27", strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	if r.Files["assets/font.bin"] == "" {
		t.Fatal("client resources discarded")
	}
	if e = verifyClient(dir, r); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"pkg/../escape", "C:/escape", "pkg\\escape"} {
		z := makeRuntimeZip(t, map[string]string{"pkg/openomsi.exe": "game", p: "bad"})
		if _, e = unpackClient(z, filepath.Join(t.TempDir(), "client"), "0.2.27", ""); e == nil {
			t.Fatal("unsafe client archive", p)
		}
	}
}
