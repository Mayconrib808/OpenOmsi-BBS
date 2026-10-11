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

func TestBCSLogPatchRejectsUnknownBuildAndConflictingBackup(t *testing.T) {
	p, h := testBCSJar(t)
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
	backup := p + ".antes-proxima-viagem.bak"
	os.WriteFile(backup, []byte("different original"), 0600)
	if _, e := patchBCSJar(p, h, []byte("observer")); e == nil {
		t.Fatal("conflicting backup accepted")
	}
	after, _ = os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("original replaced despite conflicting backup")
	}
}
