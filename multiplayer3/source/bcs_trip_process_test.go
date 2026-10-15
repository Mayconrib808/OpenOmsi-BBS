package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGuardedBCSLaunchChild(t *testing.T) {
	if os.Getenv("BBS_GUARDED_LAUNCH_TEST") != "1" {
		return
	}
	folder := filepath.Join(filepath.Dir(os.Args[0]), "Plugins")
	paths, _ := filepath.Glob(filepath.Join(folder, "bbs_next_trip_*.lua"))
	if len(paths) != 1 {
		t.Fatal("missing private verifier", paths)
	}
	source, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^local expected = omsi.json.decode\((.*)\)$`).FindSubmatch(source)
	if len(m) != 2 {
		t.Fatal("missing selection payload")
	}
	var raw string
	var selected bcsTripSelection
	if json.Unmarshal(m[1], &raw) != nil || json.Unmarshal([]byte(raw), &selected) != nil {
		t.Fatal("invalid selection payload")
	}
	ready, _ := os.ReadFile(filepath.Join(folder, bcsReadyPluginName))
	if strings.Contains(string(ready), "] spawned at") {
		t.Fatal("legacy readiness was not disabled")
	}
	fmt.Println("[native] spawned at early native position")
	state := "ready"
	reason := ""
	if os.Getenv("BBS_GUARDED_REJECT") == "1" {
		state = "error"
		reason = "notfound"
	}
	data, _ := json.Marshal(bcsTripState{Token: selected.Token, ID: selected.ID, State: state, Error: reason,
		Line: selected.Line, Tour: selected.Tour, Trip: 5, TripName: "selected_by_runtime", Departure: float64(selected.Start * 60)})
	path := strings.TrimSuffix(paths[0], ".lua") + ".data"
	if err = os.WriteFile(filepath.Join(path, "status.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if state == "ready" {
		fmt.Println("[plugin] [BBS trip confirmed] spawned at selected BCS duty ready")
	}
	time.Sleep(time.Second)
}

func TestGuardedNativeProcessConfirmsBeforeReleasingBCS(t *testing.T) {
	for _, reject := range []bool{false, true} {
		base := t.TempDir()
		root := filepath.Join(base, "OMSI 2")
		client := filepath.Join(base, "client")
		dir := filepath.Join(base, "OpenOMSI-BBS")
		for _, folder := range []string{filepath.Join(root, "Busbetrieb-Simulator"), client, filepath.Join(dir, "compat")} {
			if err := os.MkdirAll(folder, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := ensureLayout(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Busbetrieb-Simulator", "log.txt"), []byte(selectionFixture), 0600); err != nil {
			t.Fatal(err)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		game := filepath.Join(client, "openomsi.exe")
		if err = os.WriteFile(game, data, 0755); err != nil {
			t.Fatal(err)
		}
		helper := []byte("MZ fixture helper")
		if err = os.WriteFile(filepath.Join(dir, "compat", "omsi-plugin-host32.exe"), helper, 0600); err != nil {
			t.Fatal(err)
		}
		previousHash := bundledPluginHostSHA256
		bundledPluginHostSHA256 = fmt.Sprintf("%x", sha256.Sum256(helper))
		var output bytes.Buffer
		previousOutput, previousLocator := bcsLaunchOutput, bcsGameLogPath
		bcsLaunchOutput = &output
		bcsGameLogPath = func() (string, error) { return filepath.Join(base, "game.log"), nil }
		t.Setenv("BBS_GUARDED_LAUNCH_TEST", "1")
		flag := "0"
		if reject {
			flag = "1"
		}
		t.Setenv("BBS_GUARDED_REJECT", flag)
		args := []string{"-test.run=^TestGuardedBCSLaunchChild$", "--", "--schedule", "--line", "130 & N30", "--tour", "Mo-Fr 3", "--trip", "4", "--map", "maps/Any map/global.cfg", "--bus", "Vehicles/Any/bus.bus"}
		process, startErr := launchBCSNative(context.Background(), localConfig{Root: root, Game: game}, args, dir)
		var waitErr error
		if startErr == nil {
			waitErr = process.Wait()
		}
		bundledPluginHostSHA256 = previousHash
		bcsLaunchOutput, bcsGameLogPath = previousOutput, previousLocator
		if startErr != nil {
			t.Fatal(startErr)
		}
		if reject {
			if waitErr == nil || strings.Contains(output.String(), "] spawned at") {
				t.Fatal("unconfirmed trip was released", waitErr, output.String())
			}
		} else if waitErr != nil || strings.Count(output.String(), "] spawned at") != 1 {
			t.Fatal("confirmed trip was not released", waitErr, output.String())
		}
		if paths, _ := filepath.Glob(filepath.Join(client, "Plugins", "bbs_next_trip_*.lua")); len(paths) != 0 {
			t.Fatal("private verifier was not cleaned up", paths)
		}
	}
}
