package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bcsConfigFixture(root string) []byte {
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = fmt.Sprintf("unchanged setting %d", i)
	}
	lines[0], lines[49], lines[50] = root, "old native folder", "false"
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func TestBCSConfigurationPreservesOtherSettingsAndEncoding(t *testing.T) {
	root := filepath.Join(t.TempDir(), "OMSI 2")
	adapter := filepath.Join(t.TempDir(), "São João €", "PeDePeAdapter")
	plain := bcsConfigFixture(root)
	for _, bom := range [][]byte{nil, {0xef, 0xbb, 0xbf}, {0xff, 0xfe}, {0xfe, 0xff}} {
		before, err := encodeBCSConfig(string(plain), bom)
		if err != nil {
			t.Fatal(err)
		}
		after, err := updatedBCSSettings(before, root, adapter)
		if err != nil {
			t.Fatal(err)
		}
		oldLines, newLines := splitConfigLines(decodeBCSConfig(before)), splitConfigLines(decodeBCSConfig(after))
		for i := range oldLines {
			if i != 49 && i != 50 && newLines[i] != oldLines[i] {
				t.Fatalf("setting %d changed", i)
			}
		}
		if newLines[49] != adapter+string(filepath.Separator) || newLines[50] != "true" {
			t.Fatal(newLines[49:51])
		}
		if len(bom) > 0 && !bytes.HasPrefix(after, bom) {
			t.Fatal("encoding changed")
		}
		if !strings.Contains(decodeBCSConfig(after), "\r\n") {
			t.Fatal("newline format changed")
		}
	}
	if _, err := updatedBCSSettings(plain, root, filepath.Join(t.TempDir(), "東京")); err == nil {
		t.Fatal("unrepresentable path accepted")
	}
	for _, before := range [][]byte{[]byte("unknown config"), bytes.Replace(plain, []byte("\r\nfalse\r\n"), []byte("\r\nfuture mode\r\n"), 1), bcsConfigFixture(t.TempDir())} {
		if _, err := updatedBCSSettings(before, root, adapter); err == nil {
			t.Fatal("unknown or unrelated settings accepted")
		}
	}
}

func TestExplicitBCSConfigurationUsesRedirectBackupAndClosedCheck(t *testing.T) {
	root, redirect := t.TempDir(), t.TempDir()
	jarDir := filepath.Join(root, "Busbetrieb-Simulator")
	_ = os.MkdirAll(jarDir, 0755)
	_ = os.WriteFile(filepath.Join(jarDir, "busbetrieb-simulator.jar"), []byte("original jar"), 0600)
	_ = os.WriteFile(filepath.Join(jarDir, "gad.txt"), []byte(redirect+"\r\n"), 0600)
	path := filepath.Join(redirect, "config.cfg")
	before := bcsConfigFixture(root)
	_ = os.WriteFile(path, before, 0600)
	dir := appDirectory(root)
	_ = ensureLayout(dir)
	_ = os.WriteFile(filepath.Join(adapterDirectory(dir), "openomsi.exe"), []byte("MZ"), 0600)
	if _, err := configureBCS(root, dir, func() error { return fmt.Errorf("bcs running") }); err == nil {
		t.Fatal("running BCS accepted")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, before) {
		t.Fatal("settings changed while BCS open")
	}
	backup, err := configureBCS(root, dir, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(backup); !bytes.Equal(got, before) {
		t.Fatal("backup did not preserve original bytes")
	}
	if ready, err := inspectBCS(root, dir); err != nil || !ready {
		t.Fatal(ready, err)
	}
	if got, _ := os.ReadFile(filepath.Join(jarDir, "busbetrieb-simulator.jar")); string(got) != "original jar" {
		t.Fatal("BCS JAR modified")
	}
	if _, err := os.Stat(filepath.Join(root, "bbs.start")); !os.IsNotExist(err) {
		t.Fatal("BCS marker fabricated")
	}
}

func TestLanguageAndUpdatePreferencesRemainIndependent(t *testing.T) {
	dir := appDirectory(t.TempDir())
	off := false
	if err := writePreferences(dir, "pt-BR", &off); err != nil {
		t.Fatal(err)
	}
	if err := saveLanguage(dir, "de"); err != nil {
		t.Fatal(err)
	}
	p := readPreferences(dir)
	if p.AutomaticUpdates || p.Language != "de" {
		t.Fatal("language reset update choice", p)
	}
	defer setLanguage("pt-BR")
	path := filepath.Join(configDirectory(dir), "settings.json")
	_ = os.WriteFile(path, []byte(`{"language":"de","automatic_updates":false,"future_option":123}`), 0600)
	if err := saveLanguage(dir, "en"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !bytes.Contains(b, []byte(`"future_option": 123`)) {
		t.Fatal("future preference discarded")
	}
	_ = os.WriteFile(path, []byte("broken settings"), 0600)
	if err := saveLanguage(dir, "de"); err == nil {
		t.Fatal("corrupt preferences overwritten")
	}
}
