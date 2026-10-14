package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestControlsPersistIndependentlyAndSoloNeedsNoInvitation(t *testing.T) {
	dir := appDirectory(t.TempDir())
	off, on := false, true
	if err := writeControls(dir, &off, &on); err != nil {
		t.Fatal(err)
	}
	automatic := false
	if err := writePreferences(dir, "de", &automatic); err != nil {
		t.Fatal(err)
	}
	defer setLanguage("pt-BR")
	p := readPreferences(dir)
	if p.MultiplayerEnabled || !p.ProgramDisabled || p.AutomaticUpdates || p.Language != "de" {
		t.Fatal(p)
	}
	cfg := localConfig{Version: 3, Player: "Solo"}
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfig(dir); err != nil {
		t.Fatal("solo requires invitation", err)
	}
	got := withoutLANArguments([]string{"--line", "4310", "--lan-join", "https://server", "--lan-name=Maycon", "--autostart"})
	if len(got) != 3 || got[0] != "--line" || got[2] != "--autostart" {
		t.Fatal(got)
	}
}
func TestDeactivateOnlyOurBCSAdapterAndPreserveSettings(t *testing.T) {
	root := t.TempDir()
	dir := appDirectory(t.TempDir())
	jar := filepath.Join(root, "Busbetrieb-Simulator")
	os.MkdirAll(jar, 0755)
	os.WriteFile(filepath.Join(jar, "busbetrieb-simulator.jar"), []byte("vendor"), 0600)
	before := bcsConfigFixture(root)
	active, err := updatedBCSSettings(before, root, adapterDirectory(dir))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(jar, "config.cfg")
	os.WriteFile(path, active, 0600)
	backup, err := deactivateBCS(root, dir, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(backup); !bytes.Equal(b, active) {
		t.Fatal("backup mismatch")
	}
	b, _ := os.ReadFile(path)
	lines := splitConfigLines(decodeBCSConfig(b))
	if lines[50] != "false" {
		t.Fatal("still enabled")
	}
	for i, l := range splitConfigLines(decodeBCSConfig(active)) {
		if i != 50 && l != lines[i] {
			t.Fatal("unrelated setting changed", i)
		}
	}
	other, _ := updatedBCSSettings(before, root, filepath.Join(t.TempDir(), "other"))
	os.WriteFile(path, other, 0600)
	if _, err := deactivateBCS(root, dir, func() error { return nil }); err == nil {
		t.Fatal("foreign adapter overwritten")
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, other) {
		t.Fatal("foreign settings changed")
	}
}

func TestSoloLaunchDoesNotNeedDirectory(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("uses a local shell fixture")
	}
	root := t.TempDir()
	dir := appDirectory(t.TempDir())
	ensureLayout(dir)
	os.WriteFile(filepath.Join(root, "Omsi.exe"), []byte("original"), 0600)
	os.MkdirAll(filepath.Join(dir, "server"), 0755)
	os.WriteFile(filepath.Join(dir, "server", "openomsi.exe"), []byte("server"), 0600)
	game := filepath.Join(t.TempDir(), "openomsi")
	os.WriteFile(game, []byte("#!/bin/sh\nexit 0\n"), 0755)
	off := false
	writeControls(dir, &off, nil)
	cfg := localConfig{Version: 3, Root: root, Game: game, Player: "Solo"}
	saveConfig(dir, cfg)
	if err := run(context.Background(), dir, []string{"--no-menu"}); err != nil {
		t.Fatal("solo tried to access directory", err)
	}
}

func TestDeactivateKnownPrevious3xFolder(t *testing.T) {
	root := t.TempDir()
	dir := appDirectory(t.TempDir())
	old := adapterDirectory(appDirectory(t.TempDir()))
	os.MkdirAll(old, 0755)
	payload := []byte("MZknown previous v3 adapter")
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	previousV3Adapters[hash] = true
	defer delete(previousV3Adapters, hash)
	os.WriteFile(filepath.Join(old, "openomsi.exe"), payload, 0600)
	jar := filepath.Join(root, "Busbetrieb-Simulator")
	os.MkdirAll(jar, 0755)
	os.WriteFile(filepath.Join(jar, "busbetrieb-simulator.jar"), []byte("vendor"), 0600)
	active, err := updatedBCSSettings(bcsConfigFixture(root), root, old)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(jar, "config.cfg")
	os.WriteFile(path, active, 0600)
	if _, err = deactivateBCS(root, dir, func() error { return nil }); err != nil {
		t.Fatal("old 3.x folder prevents disable", err)
	}
	b, _ := os.ReadFile(path)
	if splitConfigLines(decodeBCSConfig(b))[50] != "false" {
		t.Fatal("not disabled")
	}
}
