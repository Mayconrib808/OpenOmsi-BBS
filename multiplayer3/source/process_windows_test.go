//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOwnedWindowsProcessChild(t *testing.T) {
	if os.Getenv("BBS_HOST_PROCESS_CHILD") != "1" { return }
	fmt.Fprintln(os.Stdout, "native-stdout-ready")
	fmt.Fprintln(os.Stderr, "native-stderr-ready")
	for { time.Sleep(time.Second) }
}

func TestOwnedWindowsProcessRunsAndStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.log")
	out, err := os.Create(path)
	if err != nil { t.Fatal(err) }
	defer out.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestOwnedWindowsProcessChild$")
	cmd.Env = append(os.Environ(), "BBS_HOST_PROCESS_CHILD=1")
	cmd.Stdout, cmd.Stderr = out, out
	prepareCompanyHostProcess(cmd)
	if err = cmd.Start(); err != nil { t.Fatal(err) }
	stop, err := ownCompanyHostProcess(cmd)
	if err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
	defer stop()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(10*time.Second)
	for {
		b, _ := os.ReadFile(path)
		if bytes.Contains(b, []byte("native-stdout-ready")) && bytes.Contains(b, []byte("native-stderr-ready")) { break }
		if time.Now().After(deadline) { t.Fatalf("owned Windows child did not run: %s", b) }
		time.Sleep(20*time.Millisecond)
	}
	stop()
	select { case <-done: case <-time.After(5*time.Second): t.Fatal("job close did not stop its child") }
}

// CI supplies the verified, unmodified 0.3.0 executable. This checks its actual
// Windows loader and redirected output independently of proprietary map data.
func TestOfficialWindowsServerStarts(t *testing.T) {
	exe := os.Getenv("OPENOMSI_TEST_EXE")
	if exe == "" { t.Skip("official runtime not supplied") }
	cmd := exec.Command(exe, "--version")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	prepareCompanyHostProcess(cmd)
	if err := cmd.Start(); err != nil { t.Fatal(err) }
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil || !strings.Contains(output.String(), "0.3.0") { t.Fatalf("official 0.3.0 did not answer: %v: %s", err, output.String()) }
		t.Log(output.String())
	case <-time.After(20*time.Second):
		_ = cmd.Process.Kill(); <-done
		t.Fatal("official server 0.3.0 did not run within 20 seconds")
	}
}
