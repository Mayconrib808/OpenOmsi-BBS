package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testBCSJar(t *testing.T) (string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "busbetrieb-simulator.jar")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for _, file := range []struct{ name, data string }{{bcsLoggerEntry, "original logger"}, {"logic/unchanged.class", "unchanged trip/authentication logic"}, {"res/openomsi/bbs.lua", "unchanged telemetry"}} {
		w, e := z.Create(file.name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(file.data)); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	h, e := bcsJarSHA(p)
	if e != nil {
		t.Fatal(e)
	}
	return p, h
}

func TestBCSLogPatchPreservesBackupAndOtherEntries(t *testing.T) {
	p, h := testBCSJar(t)
	original, _ := os.ReadFile(p)
	changed, e := patchBCSJar(p, h, []byte("observational logger"))
	if e != nil || !changed {
		t.Fatal(changed, e)
	}
	backup, e := os.ReadFile(p + ".antes-proxima-viagem.bak")
	if e != nil || !bytes.Equal(original, backup) {
		t.Fatal("backup did not preserve the original", e)
	}
	r, e := zip.OpenReader(p + ".antes-proxima-viagem.bak")
	if e != nil {
		t.Fatal(e)
	}
	if e = verifyPatchedBCSJar(p, r, []byte("observational logger")); e != nil {
		t.Fatal(e)
	}
	r.Close()
	changed, e = patchBCSJar(p, h, []byte("observational logger"))
	if e != nil || changed {
		t.Fatal("installed patch was applied twice", changed, e)
	}
}

func TestBCSLogPatchRejectsUnknownBuild(t *testing.T) {
	p, _ := testBCSJar(t)
	before, _ := os.ReadFile(p)
	if _, e := patchBCSJar(p, "another vendor build", []byte("observer")); e == nil {
		t.Fatal("unknown build accepted")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("unknown build modified")
	}
	if _, e := os.Stat(p + ".antes-proxima-viagem.bak"); !os.IsNotExist(e) {
		t.Fatal("unknown build generated a backup")
	}
}

func TestBCSLogPatchKeepsBackupFromPreviousVendorBuild(t *testing.T) {
	p, h := testBCSJar(t)
	original, _ := os.ReadFile(p)
	legacy := p + ".antes-proxima-viagem.bak"
	if e := os.WriteFile(legacy, []byte("older BCS vendor backup"), 0600); e != nil {
		t.Fatal(e)
	}
	changed, e := patchBCSJar(p, h, []byte("observer"))
	if e != nil || !changed {
		t.Fatal(changed, e)
	}
	old, _ := os.ReadFile(legacy)
	if string(old) != "older BCS vendor backup" {
		t.Fatal("previous vendor backup was overwritten")
	}
	prefix := h
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	versioned := p + ".antes-proxima-viagem." + prefix + ".bak"
	backup, e := os.ReadFile(versioned)
	if e != nil || !bytes.Equal(original, backup) {
		t.Fatal("current vendor build did not get its own backup", e)
	}
}

func TestBCSLogPatchRejectsConflictingBuildSpecificBackup(t *testing.T) {
	p, h := testBCSJar(t)
	before, _ := os.ReadFile(p)
	if e := os.WriteFile(p+".antes-proxima-viagem.bak", []byte("older BCS vendor backup"), 0600); e != nil {
		t.Fatal(e)
	}
	prefix := h
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	versioned := p + ".antes-proxima-viagem." + prefix + ".bak"
	if e := os.WriteFile(versioned, []byte("conflicting backup"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := patchBCSJar(p, h, []byte("observer")); e == nil {
		t.Fatal("conflicting build-specific backup accepted")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("original replaced despite conflicting build-specific backup")
	}
}

func TestBCSSupportedVendorBuilds(t *testing.T) {
	if !supportedBCSJarSHA(bcsInspectedJarSHA) {
		t.Fatal("current BCS vendor build rejected")
	}
	if !supportedBCSJarSHA(bcsPreviousInspectedJarSHA) {
		t.Fatal("previous BCS vendor build rejected")
	}
	if !supportedBCSJarSHA(bcs503InspectedJarSHA) {
		t.Fatal("BCS 5.0.3.0 vendor build rejected")
	}
	if supportedBCSJarSHA("unknown") {
		t.Fatal("unknown BCS vendor build accepted")
	}
}
