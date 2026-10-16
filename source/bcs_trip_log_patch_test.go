package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
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

func TestBCSLogPatchRejectsConcurrentArchiveChange(t *testing.T) {
	p, _ := testBCSJar(t)
	before, _ := os.ReadFile(p)
	if _, e := patchBCSJar(p, "outdated snapshot", []byte("observer")); e == nil {
		t.Fatal("concurrently changed archive accepted")
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

func TestBCSAutomaticUpdateAcceptsChangedCommunication(t *testing.T) {
	for _, lang := range []string{"pt", "en", "de"} {
		t.Run(lang, func(t *testing.T) {
			jar, _ := testBCSJar(t)
			before, _ := os.ReadFile(jar)
			closed := false
			state, err := prepareBCSNextTrip(Config{Root: filepath.Dir(jar), Language: lang}, func(path string) error { closed = path == jar; return nil })
			if err != nil || !closed || state.Path != jar || state.Changed || state.Note != bcsObserverNativeNote(lang) {
				t.Fatalf("BCS update blocked activation: %+v %v", state, err)
			}
			after, _ := os.ReadFile(jar)
			if !bytes.Equal(before, after) {
				t.Fatal("unrecognized logger was overwritten")
			}
		})
	}
}

// Compiled from our synthetic system.J fixture with the four static methods
// used by ObserverTemplate.java; contains no PeDePe code.
const bcsStateClassFixture = "yv66vgAAADQAFwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBQAAAAAAAAB7BwAKAQAIc3lzdGVtL0oBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABaAEAAygpQgEAAUkBAAMoKUoBAAFpAQAHKClMci9uOwEAAWoBAAcoKUxyL0U7AQAKU291cmNlRmlsZQEABkouamF2YQAhAAkAAgAAAAAABQABAAUABgABAAsAAAAdAAEAAQAAAAUqtwABsQAAAAEADAAAAAYAAQAAAAIACQANAA4AAQALAAAAGgABAAAAAAACBawAAAABAAwAAAAGAAEAAAADAAkADwAQAAEACwAAABwAAgAAAAAABBQAB60AAAABAAwAAAAGAAEAAAAEAAkAEQASAAEACwAAABoAAQAAAAAAAgGwAAAAAQAMAAAABgABAAAABQAJABMAFAABAAsAAAAaAAEAAAAAAAIBsAAAAAEADAAAAAYAAQAAAAYAAQAVAAAAAgAW"

func TestBCSStateOwnerFollowsInstalledInterfaceWithoutJarHash(t *testing.T) {
	class, err := base64.StdEncoding.DecodeString(bcsStateClassFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"system/I", "system/J", "system/AnotherName"} {
		t.Run(owner, func(t *testing.T) {
			reference := append([]byte{0, byte(len(owner))}, []byte(owner)...)
			data := bytes.Replace(class, []byte("\x00\x08system/J"), reference, 1)
			jar := filepath.Join(t.TempDir(), "busbetrieb-simulator.jar")
			var archive bytes.Buffer
			z := zip.NewWriter(&archive)
			w, _ := z.Create(owner + ".class")
			w.Write(data)
			w, _ = z.Create("unrelated/new-update.txt")
			w.Write([]byte("resource changed by the automatic BCS update"))
			z.Close()
			os.WriteFile(jar, archive.Bytes(), 0600)
			actual, err := bcsTripStateOwner(jar)
			if err != nil || actual != owner {
				t.Fatal(actual, err)
			}
		})
	}
	// Every truncated prefix is invalid; malformed classes must never panic.
	for i := 0; i < len(class)-10; i++ {
		_, _ = bcsClassMethods(class[:i])
	}
	bad := append([]byte(nil), class...)
	binary.BigEndian.PutUint16(bad[8:10], 65535)
	if _, ok := bcsClassMethods(bad); ok {
		t.Fatal("malformed constant pool accepted")
	}
}
