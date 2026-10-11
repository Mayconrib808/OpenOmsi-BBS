//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateProcessChild(t *testing.T) {
	if os.Getenv("BRIDGE_UPDATE_LAUNCH_CHILD") != "1" {
		return
	}
	args := os.Args
	marker := args[len(args)-1]
	if err := os.WriteFile(marker, []byte("started"), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestUpdateProcessStartsWithoutConsoleHandles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder with spaces")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "update child.exe")
	if err = os.WriteFile(executable, data, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "launch result.txt")
	t.Setenv("BRIDGE_UPDATE_LAUNCH_CHILD", "1")
	if err = startBridgeUpdateProcess(executable, dir, "-test.run=^TestUpdateProcessChild$", "--", marker); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if result, err := os.ReadFile(marker); err == nil && string(result) == "started" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("update process did not receive its argument or start")
}
