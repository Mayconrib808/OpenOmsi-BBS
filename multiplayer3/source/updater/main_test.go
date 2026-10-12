package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testPayload(t *testing.T, extra string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	r := receipt{Product: "bbs-openomsi-multiplayer", Version: "3.0.2-dev.1", Files: map[string]string{}}
	for name := range allowed {
		data := []byte("MZ source-backed component " + name)
		f, _ := z.Create(name)
		f.Write(data)
		r.Files[name] = fmtDigest(data)
	}
	meta, _ := json.Marshal(r)
	f, _ := z.Create("package.json")
	f.Write(meta)
	if extra != "" {
		f, _ := z.Create(extra)
		f.Write([]byte("unexpected"))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func fmtDigest(b []byte) string {
	h := sha256.Sum256(b)
	const hex = "0123456789abcdef"
	var out []byte
	for _, v := range h {
		out = append(out, hex[v>>4], hex[v&15])
	}
	return string(out)
}

func TestUpdatePreservesPreferencesCompanyAndDocumentationIsOptional(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"config", "updater", "PeDePeAdapter", "compat", "logs"} {
		_ = os.MkdirAll(filepath.Join(root, "OpenOMSI-BBS", name), 0755)
	}
	config := []byte(`{"version":3,"invitation":"same-company","player":"same-player"}`)
	path := filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json")
	_ = os.WriteFile(path, config, 0600)
	prefs := []byte(`{"language":"de","automatic_updates":false}`)
	prefPath := filepath.Join(root, "OpenOMSI-BBS", "config", "settings.json")
	_ = os.WriteFile(prefPath, prefs, 0600)
	_ = os.WriteFile(filepath.Join(root, "Configurar.exe"), []byte("old setup"), 0600)
	backup, err := apply(root, testPayload(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, config) {
		t.Fatal("company or player overwritten")
	}
	if b, _ := os.ReadFile(prefPath); !bytes.Equal(b, prefs) {
		t.Fatal("preferences overwritten")
	}
	if b, _ := os.ReadFile(filepath.Join(backup, "Configurar.exe")); string(b) != "old setup" {
		t.Fatal("backup missing")
	}
	if _, err := os.Stat(filepath.Join(root, "LEIA-ME.txt")); !os.IsNotExist(err) {
		t.Fatal("documentation required or installed")
	}
}
func TestUpdaterRefusesUnexpectedFilesAnd2x(t *testing.T) {
	for _, name := range []string{"../outside.exe", "OpenOMSI-BBS/config/multiplayer.json", "busbetrieb-simulator.jar"} {
		if _, err := unpack(testPayload(t, name)); err == nil {
			t.Fatal("unexpected component accepted", name)
		}
	}
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "OpenOMSI-BBS", "config"), 0755)
	_ = os.WriteFile(filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json"), []byte(`{"version":2}`), 0600)
	if _, err := apply(root, testPayload(t, "")); err == nil {
		t.Fatal("3.x updater replaced 2.x")
	}
}
