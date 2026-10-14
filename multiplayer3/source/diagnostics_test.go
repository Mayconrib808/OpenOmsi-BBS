package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsIncludesOwnedLogsAndRedactsInvitationAndSecrets(t *testing.T) {
	dir := t.TempDir()
	_ = ensureLayout(dir)
	cfg := localConfig{Root: "C:/Users/Player/OMSI", Game: "D:/openOMSI/openomsi.exe", Player: "PlayerSecret", Invitation: "https://company.example/rooms/" + strings.Repeat("a", 32)}
	log := cfg.Root + " " + cfg.Player + " " + cfg.Invitation + " Bearer secret-token " + strings.Repeat("b", 64)
	_ = os.WriteFile(filepath.Join(dir, "logs", "multiplayer.log"), []byte(log), 0600)
	_ = os.WriteFile(filepath.Join(dir, "logs", "private.json"), []byte("admin-token"), 0600)
	_ = os.WriteFile(adminPath(dir), []byte("admin-token"), 0600)
	data, err := diagnosticZIP(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name != "logs/multiplayer.log" && f.Name != "diagnostics.json" {
			t.Fatal("private file exported", f.Name)
		}
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		for _, secret := range []string{cfg.Root, cfg.Invitation, cfg.Player, "secret-token", strings.Repeat("b", 64), "admin-token"} {
			if strings.Contains(string(b), secret) {
				t.Fatal("diagnostics leaked private data", f.Name)
			}
		}
	}
}
