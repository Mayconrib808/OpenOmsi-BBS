package main

import (
	"archive/zip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictClocksAndNonFiniteTourRecords(t *testing.T) {
	for _, clock := range []string{"01:10:bad", "01:10:20:30", "72:00", "-1:00", "01:60", "01:10:60", "999999999999:10"} {
		if _, e := parseClockMinutes(clock); e == nil {
			t.Fatalf("invalid clock accepted: %s", clock)
		}
	}
	if at, e := parseClockMinutes("25:10:30"); e != nil || at != 1510.5 {
		t.Fatalf("extended valid clock rejected: %v %v", at, e)
	}
	for _, value := range []string{"NaN", "+Inf", "-Inf", "-1", "4321", "n/a"} {
		t.Run(value, func(t *testing.T) {
			root, pkg, mapRel := basicMap(t)
			tt := filepath.Join(root, "maps", "Test Map", "TTData")
			writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nBAD\n0\n"+value+"\n[addtrip]\nGOOD\n0\n600\n")
			writeTestFile(t, filepath.Join(tt, "GOOD.ttp"), "[trip]\nTrack\nDepot\n10\n[station]\n1\n0\nAlpha\n")
			res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot"})
			defer removeTimetableOverlay(res)
			if res.Ready || res.Applied || res.TripIndex != 0 || res.OverrideZIP != "" {
				t.Fatalf("malformed selected tour published: %+v", res)
			}
		})
	}
}

func TestMissingTTPDoesNotShiftTheGamesTripOrdinal(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nMISSING\n0\n100\n[addtrip]\nGOOD\n0\n600\n")
	writeTestFile(t, filepath.Join(tt, "GOOD.ttp"), "[trip]\nTrack\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot"})
	if !res.Ready || res.TripIndex != 1 || res.TripName != "GOOD" || res.Applied {
		t.Fatalf("must index the game's filtered trips: %+v", res)
	}
	writeTestFile(t, filepath.Join(tt, "MISSING.ttp"), "[invalid section]\n")
	res = prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot"})
	if res.Ready || res.TripIndex != 0 {
		t.Fatalf("an unreadable loaded TTP must not silently change the ordinal: %+v", res)
	}
}

func TestEmptyTrackFieldKeepsTTPValuesAndTripOrdinal(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOTHER\n0\n360\n[addtrip]\nGOOD\n0\n600\n")
	writeTestFile(t, filepath.Join(tt, "OTHER.ttp"), "[trip]\n\nOther destination\n10\n[station]\n1\n0\nOther origin\n")
	writeTestFile(t, filepath.Join(tt, "GOOD.ttp"), "[trip]\n\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot"})
	if !res.Ready || res.TripIndex != 2 || res.TripName != "GOOD" || res.Applied {
		t.Fatalf("empty track shifted the trip fields/index: %+v", res)
	}
}

func TestRepeatedRouteUsesOnlyUniqueExactDeparture(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n360\n[addtrip]\nOUT\n0\n420\n")
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), "[trip]\nTrack\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "07:00", RouteText: "Alpha - Depot"})
	if !res.Ready || res.TripIndex != 2 || res.Applied {
		t.Fatalf("unique exact departure not selected: %+v", res)
	}
}

func TestRouteMatchesAreNotSubstrings(t *testing.T) {
	if locationEq("Other Depot", "Depot", "10") || locationEq("Depot 210", "Depot 10", "10") {
		t.Fatal("unrelated location accepted")
	}
	if !locationEq("Terminal Mercado 5109 Final.bug", "Terminal Mercado 5109 Final", "5109") {
		t.Fatal("BCS route decoration was not removed")
	}
	if !locationEq("Alpha 10", "Alpha", "10") {
		t.Fatal("separate line decoration was not removed")
	}
}

func TestRealTimeSettingsAliasesFollowLastValue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.cfg")
	for _, body := range []string{"time_sync=true", "real_time_sync=YES", "time_sync=false\nreal_time_sync=on"} {
		writeTestFile(t, p, body)
		if on, e := readTimeSync(p); e != nil || !on {
			t.Fatalf("missed real-time setting: %q %v %v", body, on, e)
		}
	}
	for _, body := range []string{"# time_sync=true", "time_sync=true\ntime_sync=false", "real_time_sync=true\ntime_sync=0", "time_sync=sim"} {
		writeTestFile(t, p, body)
		if on, e := readTimeSync(p); e != nil || on {
			t.Fatalf("incorrect real-time setting: %q %v %v", body, on, e)
		}
	}
}

func TestHigherPriorityTimetableContentIsPreservedAndBlocked(t *testing.T) {
	for _, archive := range []bool{false, true} {
		t.Run(map[bool]string{false: "loose", true: "archive"}[archive], func(t *testing.T) {
			root := t.TempDir()
			content := filepath.Join(root, "additional content")
			t.Setenv("OMSI_CONTENT", content)
			t.Setenv("OMSI_CONTENT_ZIP", "")
			c := defaultConfig()
			c.Root = filepath.Join(root, "original")
			c.OpenOMSI = filepath.Join(root, "open", "openomsi.exe")
			mapRel := "maps/Test Map/global.cfg"
			if e := ensureOriginalTimetableSources(c, mapRel); e != nil {
				t.Fatal("empty content rejected:", e)
			}
			writeTestFile(t, filepath.Join(content, "maps", "Unrelated", "TTData", "10.ttl"), "untouched")
			if e := ensureOriginalTimetableSources(c, mapRel); e != nil {
				t.Fatal("another map rejected:", e)
			}
			p := filepath.Join(content, "maps", "Test Map", "TTData", "10.ttl")
			if archive {
				p = filepath.Join(content, "Archives", "map.zip")
				os.MkdirAll(filepath.Dir(p), 0755)
				f, e := os.Create(p)
				if e != nil {
					t.Fatal(e)
				}
				z := zip.NewWriter(f)
				w, e := z.Create("maps/Test Map/Chrono/Change/TTData/10.ttl")
				if e != nil {
					t.Fatal(e)
				}
				w.Write([]byte("untouched"))
				z.Close()
				f.Close()
			} else {
				writeTestFile(t, p, "untouched")
			}
			before, _ := os.ReadFile(p)
			if e := ensureOriginalTimetableSources(c, mapRel); e == nil || !strings.Contains(e.Error(), "preservado") {
				t.Fatal("conflicting content not diagnosed:", e)
			}
			after, _ := os.ReadFile(p)
			if string(after) != string(before) {
				t.Fatal("additional content was modified")
			}
		})
	}
}

func TestCleanPackagePathsWorkForSetupAndBCSLauncher(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Bridge - ônibus & Maycon's !")
	for _, exe := range []string{filepath.Join(root, "Setup.exe"), filepath.Join(root, "app", "OpenOMSI_BCS_Bridge.exe")} {
		if got := packageRootForExecutable(exe); got != root {
			t.Fatalf("wrong package root for %s: %s", exe, got)
		}
		if got := configPath(packageRootForExecutable(exe)); got != filepath.Join(root, "app", "bridge.ini") {
			t.Fatalf("setup and launcher disagree: %s", got)
		}
	}
}

func TestBCSCanReadScheduleRetryButtonFromMenuImage(t *testing.T) {
	// Traverse using the reader offsets in the inspected BCS class.
	menu, button := menuMemoryBlocks(0x23456000)
	var found bool
	for slot := 0; slot < 15; slot++ {
		pointer := binary.LittleEndian.Uint32(menu[940+slot*4:])
		if pointer != 0 && binary.LittleEndian.Uint32(button[468:]) == 5 && button[476] != 0 {
			x := binary.LittleEndian.Uint32(button[72:]) + binary.LittleEndian.Uint32(button[80:])/2
			y := binary.LittleEndian.Uint32(button[76:]) + binary.LittleEndian.Uint32(button[84:])/2
			if !isScheduleButtonClick(uintptr(y<<16 | x)) {
				t.Fatal("BCS-generated click misses the retry button")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("BCS cannot find an enabled timetable button")
	}
	if isScheduleButtonClick(0) {
		t.Fatal("unrelated menu click accepted")
	}
}
