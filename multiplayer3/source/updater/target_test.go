package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManualUpdaterUsesItsExistingInstallation(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "OpenOMSI-BBS", "config"), 0700)
	_ = os.WriteFile(filepath.Join(root, "Setup.exe"), []byte("MZ"), 0700)
	_ = os.WriteFile(filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json"), []byte(`{"version":3}`), 0600)
	called := false
	got, err := updateTarget("", filepath.Join(root, "Atualizar.exe"), func()(string,error){ called=true;return "",errUpdateCancelled })
	if err != nil || got != root || called { t.Fatalf("direct launch failed: %s %v picker=%v", got, err, called) }
}

func TestManualUpdaterSelectsTargetAndPreservesCancellation(t *testing.T) {
	root := t.TempDir()
	got, err := updateTarget("", filepath.Join(root, "Atualizar.exe"), func()(string,error){return "chosen",nil})
	if err != nil || got != "chosen" { t.Fatal(got, err) }
	_, err = updateTarget("", filepath.Join(root, "Atualizar.exe"), func()(string,error){return "",errUpdateCancelled})
	if !errors.Is(err,errUpdateCancelled) { t.Fatal("cancel was lost",err) }
	got, err = updateTarget(root, "ignored",func()(string,error){t.Fatal("automatic update opened folder picker");return "",nil})
	if err != nil || got != root { t.Fatal(got,err) }
}

func TestManualUpdaterDoesNotRecognizeUnconfiguredOr2xFolder(t *testing.T) {
	root := t.TempDir()
	if configuredUpdateRoot(root) { t.Fatal("empty folder accepted") }
	_ = os.MkdirAll(filepath.Join(root, "OpenOMSI-BBS", "config"), 0700)
	_ = os.WriteFile(filepath.Join(root, "Setup.exe"), []byte("MZ"), 0700)
	_ = os.WriteFile(filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json"), []byte(`{"version":2}`), 0600)
	if configuredUpdateRoot(root) { t.Fatal("2.x folder accepted") }
}
