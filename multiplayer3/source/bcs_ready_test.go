package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBCSReadyPluginWaitsForRealPlayerVehicle(t *testing.T) {
	vehicleGate := strings.Index(bcsReadyPluginSource, "omsi.has_vehicle()")
	marker := strings.Index(bcsReadyPluginSource, "] spawned at")
	if vehicleGate < 0 || marker < 0 {
		t.Fatalf("readiness plugin must contain the real vehicle gate and BCS marker")
	}
	if vehicleGate > marker {
		t.Fatalf("BCS readiness marker must be guarded by omsi.has_vehicle()")
	}
	if !strings.Contains(bcsReadyPluginSource, "if sent or not omsi.has_vehicle() then") {
		t.Fatalf("readiness must not be published before a player vehicle exists")
	}
}

func TestPrepareBCSReadyPluginWritesCompanionBesideForwardedPlugins(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "OMSI 2")
	client := filepath.Join(base, "client", "0.2.37")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(client, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := localConfig{Root: root, Game: filepath.Join(client, "openomsi.exe")}
	if err := prepareBCSReadyPlugin(cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(client, "Plugins", bcsReadyPluginName)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != bcsReadyPluginSource {
		t.Fatalf("unexpected readiness plugin contents")
	}
}
