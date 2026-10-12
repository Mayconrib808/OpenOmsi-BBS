package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func shiftedBCSFixture(root string, prefix int) []byte {
	lines := make([]string, prefix+60)
	for i := range lines {
		lines[i] = fmt.Sprintf("unchanged setting %d", i)
	}
	lines[prefix] = root
	lines[prefix+49] = "old native folder"
	lines[prefix+50] = "false"
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func TestCurrentBCSConfigCanHavePrefixLines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "OMSI 2")
	adapter := filepath.Join(t.TempDir(), "PeDePeAdapter")
	after, err := updatedBCSSettings(shiftedBCSFixture(root, 3), root, adapter)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitConfigLines(decodeBCSConfig(after))
	if lines[52] != adapter+string(filepath.Separator) || lines[53] != "true" {
		t.Fatal(lines[52:54])
	}
}

func TestBCSSettingsPathAcceptsQuotedEnvironmentRedirect(t *testing.T) {
	root := t.TempDir()
	jarDir := filepath.Join(root, "Busbetrieb-Simulator")
	configDir := t.TempDir()
	if err := os.MkdirAll(jarDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jarDir, "busbetrieb-simulator.jar"), []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.cfg"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BBS_V3_CONFIG", configDir)
	if err := os.WriteFile(filepath.Join(jarDir, "gad.txt"), []byte("\r\n\"%BBS_V3_CONFIG%\"\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := bcsSettingsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(got, filepath.Join(configDir, "config.cfg")) {
		t.Fatalf("unexpected BCS config: %q", got)
	}
}

func TestBCSSettingsPathIgnoresStaleGadWhenLocalConfigExists(t *testing.T) {
	root := t.TempDir()
	jarDir := filepath.Join(root, "Busbetrieb-Simulator")
	if err := os.MkdirAll(jarDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jarDir, "busbetrieb-simulator.jar"), []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jarDir, "gad.txt"), []byte(filepath.Join(t.TempDir(), "old-profile")+"\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(jarDir, "config.cfg")
	if err := os.WriteFile(local, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := bcsSettingsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(got, local) {
		t.Fatalf("unexpected BCS config: %q", got)
	}
}
