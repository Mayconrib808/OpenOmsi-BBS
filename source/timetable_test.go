package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func readOnlyZipEntry(t *testing.T, path string) (string, string) {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if len(z.File) != 1 {
		t.Fatalf("want 1 zip entry, got %d", len(z.File))
	}
	r, err := z.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return z.File[0].Name, decodeText(b)
}

func basicMap(t *testing.T) (root, packageDir, mapRel string) {
	t.Helper()
	root = t.TempDir()
	packageDir = filepath.Join(root, "bridge")
	if err := os.MkdirAll(filepath.Join(packageDir, "compat"), 0755); err != nil {
		t.Fatal(err)
	}
	mapRel = "maps/Test Map/global.cfg"
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(mapRel)), "[name]\r\nTest Map\r\n")
	return
}

func TestTimetableSyncCarraroObservedCase(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "5109.ttl"), `-----------------------
Time Table Line File
-----------------------

[newtour]
07
Depot
1023

Dep.: 4:45:0
[addtrip]
5109ROTA1
0
285.000
`)
	writeTestFile(t, filepath.Join(tt, "5109ROTA1.ttp"), `[trip]
Track 5109 A
Terminal Mercado 5109 Final.bug
5109

[station]
1
0
Terminal Vila Prudente
0
0
0
0
5
`)
	info := TripInfo{Line: "5109", Tour: "07", TripStart: "01:35", RouteText: "Term Vila Prudente 5109.bug - Terminal Mercado 5109 Final.bug", StartPoint: "Terminal Vila Prudente", ShiftID: "4926000"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", info)
	defer removeTimetableOverlay(res)
	if !res.Applied {
		t.Fatalf("sync not applied: %+v", res)
	}
	if res.TripIndex != 1 || res.TripName != "5109ROTA1" {
		t.Fatalf("wrong trip match: %+v", res)
	}
	if res.OffsetMinutes != -190 {
		t.Fatalf("want -190 offset, got %.3f", res.OffsetMinutes)
	}
	entry, body := readOnlyZipEntry(t, res.OverrideZIP)
	if entry != "maps/Test Map/TTData/5109.ttl" {
		t.Fatalf("unexpected zip entry %q", entry)
	}
	if !strings.Contains(body, "95.000") || strings.Contains(body, "285.000") {
		t.Fatalf("departure not shifted in overlay:\n%s", body)
	}
	orig, _ := os.ReadFile(filepath.Join(tt, "5109.ttl"))
	if !strings.Contains(string(orig), "285.000") {
		t.Fatal("original timetable was modified")
	}
}

func TestTimetableSyncMatchesCorrectTripInMultiTripTour(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), `[newtour]
A
Depot
1023
[addtrip]
OUT
0
360.000
[addtrip]
BACK
0
390.000
`)
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), "[trip]\nTrackOut\nAlpha\n10\n[station]\n1\n0\nDepot\n")
	writeTestFile(t, filepath.Join(tt, "BACK.ttp"), "[trip]\nTrackBack\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	info := TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot", StartPoint: "Alpha", ShiftID: "7"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", info)
	defer removeTimetableOverlay(res)
	if !res.Applied || res.TripIndex != 2 || res.TripName != "BACK" {
		t.Fatalf("did not select second route: %+v", res)
	}
	if res.OffsetMinutes != 210 { // 10:00 (600) - original 06:30 (390)
		t.Fatalf("wrong offset %.3f", res.OffsetMinutes)
	}
	_, body := readOnlyZipEntry(t, res.OverrideZIP)
	if !strings.Contains(body, "570.000") || !strings.Contains(body, "600.000") {
		t.Fatalf("whole tour was not shifted consistently:\n%s", body)
	}
}

func TestTimetableSyncUsesActiveChronoOverride(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	base := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(base, "20.ttl"), "[newtour]\n1\nDepot\n1023\n[addtrip]\nBASE\n0\n300.000\n")
	chrono := filepath.Join(root, "maps", "Test Map", "Chrono", "2026 service")
	writeTestFile(t, filepath.Join(chrono, "Chrono.cfg"), "[startdate]\n20260101\n[enddate]\n20261231\n")
	writeTestFile(t, filepath.Join(chrono, "TTData", "20.ttl"), "[newtour]\n1\nDepot\n1023\n[addtrip]\nCHRONO\n0\n420.000\n")
	writeTestFile(t, filepath.Join(chrono, "TTData", "CHRONO.ttp"), "[trip]\nTrackChrono\nB\n20\n[station]\n1\n0\nA\n")
	info := TripInfo{Line: "20", Tour: "1", TripStart: "08:00", RouteText: "A - B", ShiftID: "9"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", info)
	defer removeTimetableOverlay(res)
	if !res.Applied || res.TripName != "CHRONO" || !strings.Contains(res.SourceRelative, "Chrono") {
		t.Fatalf("active chrono was not selected: %+v", res)
	}
	entry, _ := readOnlyZipEntry(t, res.OverrideZIP)
	if !strings.Contains(entry, "Chrono/2026 service/TTData/20.ttl") {
		t.Fatalf("overlay does not replace active chrono file: %q", entry)
	}
}

func TestTimetableSyncAlreadyAlignedStillReturnsCorrectTripOrdinal(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "30.ttl"), "[newtour]\nX\nDepot\n1023\n[addtrip]\nONE\n0\n600.000\n")
	writeTestFile(t, filepath.Join(tt, "ONE.ttp"), "[trip]\nTrack\nB\n30\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "30", Tour: "X", TripStart: "10:00"})
	if res.Applied || res.TripIndex != 1 || res.OverrideZIP != "" {
		t.Fatalf("aligned timetable should need no overlay but retain trip ordinal: %+v", res)
	}
}

func TestTimetableSyncFailsSafeInsteadOfWritingNegativeTimes(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "40.ttl"), "[newtour]\n1\nDepot\n1023\n[addtrip]\nEARLY\n0\n10.000\n[addtrip]\nTARGET\n0\n30.000\n")
	writeTestFile(t, filepath.Join(tt, "EARLY.ttp"), "[trip]\nT\nOther\n40\n")
	writeTestFile(t, filepath.Join(tt, "TARGET.ttp"), "[trip]\nT\nTarget\n40\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "40", Tour: "1", TripStart: "00:00", RouteText: "Start - Target"})
	if res.Applied || res.OverrideZIP != "" || !strings.Contains(res.Reason, "unsupported departure") {
		t.Fatalf("unsafe negative timetable must be skipped: %+v", res)
	}
}
