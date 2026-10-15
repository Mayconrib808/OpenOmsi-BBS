package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsKeepsNewestEvidenceFromLargeHostLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host.log")
	text := strings.Repeat("old line\n", 1000) + "native server failed: useful last line\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil { t.Fatal(err) }
	tail, err := readDiagnosticTail(path, 256)
	if err != nil { t.Fatal(err) }
	if len(tail) > 256 || !strings.Contains(string(tail), "useful last line") { t.Fatal("newest host failure was lost") }
}

func TestDetachedHostOutputSurvivesWithoutConsole(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "host.log"))
	if err != nil { t.Fatal(err) }
	defer file.Close()
	previous := os.Stderr
	os.Stderr = file
	defer func() { os.Stderr = previous }()
	out := &tunnelLog{}
	_, _ = out.Write([]byte("native server output\ntunnel: the session is reachable at https://host-test.trycloudflare.com\n"))
	b, err := os.ReadFile(file.Name())
	if err != nil || !strings.Contains(string(b), "native server output") { t.Fatal("detached native output was lost", err) }
	if out.get() != "https://host-test.trycloudflare.com" { t.Fatal("tunnel discovery was lost") }
}
