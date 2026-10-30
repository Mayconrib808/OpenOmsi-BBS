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

// The fixture is compiled from our synthetic logger in
// scripts/test_bcs_next_trip_observer.py; it contains no vendor code.
func observerV1Fixture(t *testing.T) []byte {
	t.Helper()
	text, err := os.ReadFile("testdata/bcs-observer-v1.class.base64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(text)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBCSObserverHotfixOnlyRepairsOurGuardAndMarker(t *testing.T) {
	before := observerV1Fixture(t)
	fixed, err := repairBCSNextTripObserver(before)
	if err != nil || bytes.Equal(fixed, before) || !bcsLoggerInstalled(fixed) {
		t.Fatal("v1 observer was not upgraded", err)
	}
	start := 0
	_, ok := inspectBCSClass(before, func(method string, offset, size int) {
		if method == "bridgeNextTripDetails(Ljava/lang/String;)Ljava/lang/String;" {
			start = offset
		}
	})
	marker := bytes.Index(before, []byte(bcsObserverV1)) + len(bcsObserverV1) - 2
	if !ok || len(fixed) != len(before) || !bytes.Equal(fixed[start+24:start+27], []byte{4, 0, 0}) {
		t.Fatal("instruction offsets or constant-pool layout changed")
	}
	for i := range before {
		if before[i] != fixed[i] && !(i >= start+24 && i < start+27) && i != marker {
			t.Fatalf("changed data outside our helper guard/marker at %d", i)
		}
	}
	again, err := repairBCSNextTripObserver(fixed)
	if err != nil || !bytes.Equal(again, fixed) {
		t.Fatal("second installation changed the observer", err)
	}
	bad := append([]byte(nil), before...)
	bad[start+30] = 0 // A differently shaped helper is not a supported v1.
	if _, err := repairBCSNextTripObserver(bad); err == nil {
		t.Fatal("unrecognized helper was rewritten")
	}
	for i := range before {
		_, _ = repairBCSNextTripObserver(before[:i])
	}
}

func TestBCSObserverHotfixUpgradesAlreadyInstalledJarAndPreservesBackup(t *testing.T) {
	logger := observerV1Fixture(t)
	jar := filepath.Join(t.TempDir(), "busbetrieb-simulator.jar")
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	for _, entry := range []struct {
		name string
		data []byte
	}{{bcsLoggerEntry, logger}, {"res/openomsi/bbs.lua", []byte("original telemetry")}, {"logic/unchanged.class", []byte("original trip logic")}} {
		w, _ := z.Create(entry.name)
		_, _ = w.Write(entry.data)
	}
	z.Close()
	before := append([]byte(nil), data.Bytes()...)
	os.WriteFile(jar, before, 0600)
	state, err := prepareBCSNextTrip(Config{Root: filepath.Dir(jar)}, nil)
	if err != nil || !state.Changed || state.Note != "" {
		t.Fatal("installed v1 was incorrectly treated as current", state, err)
	}
	backup, err := os.ReadFile(jar + ".antes-proxima-viagem.bak")
	if err != nil || !bytes.Equal(before, backup) {
		t.Fatal("exact pre-hotfix archive was not preserved", err)
	}
	old, err := zip.OpenReader(jar + ".antes-proxima-viagem.bak")
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	fixed, _ := bcsLoggerBytes(jar)
	entries,err:=bcsCaptureEntries(fixed)
 if err!=nil{t.Fatal(err)}
 if err := verifyPatchedBCSEntries(jar, old, entries); err != nil {
		t.Fatal("vendor entries changed", err)
	}
	after, _ := os.ReadFile(jar)
	state, err = prepareBCSNextTrip(Config{Root: filepath.Dir(jar)}, nil)
	repeated, _ := os.ReadFile(jar)
	if err != nil || state.Changed || !bytes.Equal(after, repeated) {
		t.Fatal("repeat installation changed the archive", state, err)
	}
}
