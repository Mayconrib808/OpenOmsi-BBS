package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneBCSConfigurationWithoutBridge2(t *testing.T) {
	root := t.TempDir()
	dir := appDirectory(t.TempDir())
	if err := ensureLayout(dir); err != nil {
		t.Fatal(err)
	}
	jarDir := filepath.Join(root, "Busbetrieb-Simulator")
	os.MkdirAll(jarDir, 0755)
	os.WriteFile(filepath.Join(root, "Omsi.exe"), []byte("original"), 0600)
	os.WriteFile(filepath.Join(jarDir, "busbetrieb-simulator.jar"), []byte("vendor"), 0600)
	os.WriteFile(filepath.Join(jarDir, "config.cfg"), bcsConfigFixture(root), 0600)
	os.WriteFile(filepath.Join(adapterDirectory(dir), "openomsi.exe"), []byte("MZadapter"), 0600)
	helper := []byte("MZbundled v3 helper")
	os.MkdirAll(filepath.Join(dir, "compat"), 0755)
	os.WriteFile(filepath.Join(dir, "compat", "omsi-plugin-host32.exe"), helper, 0600)
	old := bundledPluginHostSHA256
	bundledPluginHostSHA256 = fmt.Sprintf("%x", sha256.Sum256(helper))
	defer func() { bundledPluginHostSHA256 = old }()
	if _, err := configureStandaloneBCS(root, dir, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, installedPluginHostName)); !bytes.Equal(got, helper) {
		t.Fatal("v3 helper not installed")
	}
	if ready, err := inspectBCS(root, dir); err != nil || !ready {
		t.Fatal("BCS not configured", err)
	}
	for _, name := range []string{"OpenOMSI_BCS_PluginHost32.exe", "bbs.start", "OpenOMSI_BCS_Bridge.exe"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("old bridge/marker unexpectedly created", name)
		}
	}
}

func TestStandaloneClientIgnoresInheritedContent(t *testing.T) {
	t.Setenv("OMSI_CONTENT", filepath.Join(t.TempDir(), "old-bridge"))
	game := filepath.Join(t.TempDir(), "openomsi.exe")
	if got := contentFolder(localConfig{Game: game}); got != filepath.Dir(game) {
		t.Fatal("old content inherited", got)
	}
}
