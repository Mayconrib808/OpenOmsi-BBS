package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupShowsOnlyBCSTimeShift(t *testing.T) {
	if strings.Contains(setupPage, `id="zone"`) || strings.Contains(setupPage, `for="zone"`) {
		t.Fatal("reference timezone must not be visible in Setup")
	}
	if !strings.Contains(setupPage, `type="hidden" name="zone" value="Europe/Berlin"`) {
		t.Fatal("BCS reference timezone must remain internal")
	}
	if !strings.Contains(setupPage, `name="offset"`) || !strings.Contains(setupPage, `name="offset" type="number" min="-24" max="24" step="any" value="{{.Offset}}" placeholder="{{t "clockplaceholder"}}" required`) {
		t.Fatal("BCS Time shift must be the visible required clock field")
	}
}

func TestUnknownDedicatedPluginHostIsBackedUpAndReplaced(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "compat"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDirectory(dir), 0700); err != nil {
		t.Fatal(err)
	}

	bundled := []byte("MZ-new-3.0.4-helper")
	if err := os.WriteFile(filepath.Join(dir, "compat", "omsi-plugin-host32.exe"), bundled, 0600); err != nil {
		t.Fatal(err)
	}
	previousBundledHash := bundledPluginHostSHA256
	defer func() { bundledPluginHostSHA256 = previousBundledHash }()
	bundledPluginHostSHA256 = fmt.Sprintf("%x", sha256.Sum256(bundled))

	old := []byte("MZ-unregistered-old-helper")
	target := filepath.Join(root, installedPluginHostName)
	if err := os.WriteFile(target, old, 0755); err != nil {
		t.Fatal(err)
	}
	oldHash := fmt.Sprintf("%x", sha256.Sum256(old))
	if previousPluginHosts[oldHash] {
		t.Fatal("test fixture unexpectedly matches a registered helper")
	}

	got, err := preparePluginHost(localConfig{Root: root}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("unexpected helper path: %s", got)
	}
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(bundled) {
		t.Fatal("stale helper was not replaced")
	}
	backup, err := os.ReadFile(filepath.Join(configDirectory(dir), "helper-backups", oldHash+".bak"))
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(old) {
		t.Fatal("unknown previous helper backup does not match original bytes")
	}
}
