package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCarraoRepeatedDutyAfterMidnight(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	var ttl strings.Builder
	ttl.WriteString("[newtour]\n01\nDepot\n1023\n")
	// The diagnostic duty contains 64 alternating trips from 04:00 to 25:00.
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&ttl, "[addtrip]\n2201rota%d\n0\n%.3f\n", 1+i%2, float64(240+20*i))
	}
	path := filepath.Join(tt, "2201.ttl")
	writeTestFile(t, path, ttl.String())
	for _, p := range []struct{ name, first, last string }{
		{"2201rota1", "CPTM Guaianazes 2201", "Divisa de Feraz 2201"},
		{"2201rota2", "Divisa de Feraz 2201", "CPTM Guaianazes 2201"},
	} {
		profile := strings.ReplaceAll(endpointProfile(p.last, p.first, "Middle", p.last), "\n10\n", "\n2201\n")
		writeTestFile(t, filepath.Join(tt, p.name+".ttp"), profile)
	}
	info := TripInfo{Line: "2201", Tour: "01", TripStart: "00:20", RouteText: "Divisa de Feraz 2201 - CPTM Guaianazes 2201", StartPoint: "Divisa de Ferraz 2201"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	defer removeTimetableOverlay(res)
	if !res.Ready || res.Applied || res.TripIndex != 62 || res.TripName != "2201rota2" || res.OriginalDeparture != 1460 || res.BCSDeparture != 1460 || res.BCSCivilDeparture != 20 || res.OffsetMinutes != 0 || res.OverrideZIP != "" {
		t.Fatalf("00:20 must select the unique 24:20 trip with no logical offset: %+v", res)
	}
	original, err := os.ReadFile(path)
	if err != nil || string(original) != ttl.String() {
		t.Fatal("installed TTL must stay byte-identical")
	}
	info.TripStart = "00:30"
	blocked := prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	if blocked.Ready || !strings.Contains(blocked.Reason, "ambiguous") {
		t.Fatalf("must not guess the nearest overnight trip: %+v", blocked)
	}
	info.TripStart = "00:20"
	info.RouteText = "CPTM Guaianazes 2201 - Divisa de Feraz 2201"
	info.StartPoint = "CPTM Guaianazes 2201"
	blocked = prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	if blocked.Ready {
		t.Fatalf("00:20 must not select the opposite direction: %+v", blocked)
	}
}

func TestOvernightDuplicateCivilDeparturesStayAmbiguous(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n20.000\n[addtrip]\nOUT\n0\n1460.000\n")
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), endpointProfile("Beta", "Alpha", "Middle", "Beta"))
	info := TripInfo{Line: "10", Tour: "A", TripStart: "00:20", RouteText: "Alpha - Beta"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	if res.Ready || res.OverrideZIP != "" || !strings.Contains(res.Reason, "ambiguous") {
		t.Fatalf("00:20 and 24:20 cannot be silently distinguished: %+v", res)
	}
	info.TripStart = "24:20"
	res = prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	if !res.Ready || res.Applied || res.TripIndex != 2 {
		t.Fatalf("explicit extended departure must remain exact: %+v", res)
	}
}

func TestBCSDepartureCivilAndExtendedHours(t *testing.T) {
	for _, tc := range []struct {
		departure, bcs float64
		want           bool
	}{
		{1440, 0, true}, {1460, 20, true}, {1500, 60, true},
		{1460, 1460, true}, {20, 1460, false}, {1460, 21, false},
		{2900, 20, true}, {1440, 1439, false}, {1439, 0, false},
	} {
		if got := sameBCSDeparture(tc.departure, tc.bcs); got != tc.want {
			t.Fatalf("departure %v BCS %v: %t, want %t", tc.departure, tc.bcs, got, tc.want)
		}
	}
}

func endpointProfile(display, first, middle, last string) string {
	return fmt.Sprintf("[trip]\nLoopTrack\n%s\n10\n[station]\n1\n0\n%s\n0\n0\n0\n0\n0\n[station]\n2\n1\n%s\n0\n0\n0\n0\n0\n[station]\n3\n2\n%s\n0\n0\n0\n0\n0\n[profile]\nstandard\n20.000\n",
		display, first, middle, last)
}

func TestFinalStopDifferentFromDisplaySelectsExactRepeatedDeparture(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\n02\nDepot\n1023\n[addtrip]\nLOOP\n0\n250.000\n[addtrip]\nLOOP\n0\n280.000\n[addtrip]\nLOOP\n0\n310.000\n")
	writeTestFile(t, filepath.Join(tt, "LOOP.ttp"), endpointProfile("Terminal Alpha 10", "Terminal Alpha 10", "Middle", "Terminal Alpha 10 p1"))
	info := TripInfo{Line: "10", Tour: "02", TripStart: "04:40", RouteText: "Terminal Alpha 10 - Terminal Alpha 10 p1", StartPoint: "Different spawn point"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", info)
	defer removeTimetableOverlay(res)
	if !res.Ready || res.Applied || res.TripIndex != 2 || res.OriginalDeparture != 280 || res.BCSDeparture != 280 {
		t.Fatalf("exact BCS departure must select #2 without shifting: %+v", res)
	}
	if !strings.Contains(res.Reason, "last stop matches") || !strings.Contains(res.CandidateDetails, "last=\"Terminal Alpha 10 p1\"") {
		t.Fatalf("endpoint evidence missing: %+v", res)
	}
}

func TestPhysicalEndpointsPreventWrongRouteSelection(t *testing.T) {
	for _, tc := range []struct{ name, display, first, middle, last string }{
		{"other platform", "Terminal Alpha 10 p1", "Terminal Alpha 10", "Middle", "Terminal Alpha 10 p2"},
		{"other terminal", "Terminal Alpha 10 p1", "Terminal Alpha 10", "Middle", "Other Terminal Alpha 10 p1"},
		{"intermediate stop", "Other destination", "Terminal Alpha 10", "Terminal Alpha 10 p1", "Other destination"},
		{"wrong origin", "Terminal Alpha 10 p1", "Other origin", "Middle", "Terminal Alpha 10 p1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, pkg, mapRel := basicMap(t)
			tt := filepath.Join(root, "maps", "Test Map", "TTData")
			writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\n02\nDepot\n1023\n[addtrip]\nLOOP\n0\n280.000\n")
			writeTestFile(t, filepath.Join(tt, "LOOP.ttp"), endpointProfile(tc.display, tc.first, tc.middle, tc.last))
			res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "02", TripStart: "04:40", RouteText: "Terminal Alpha 10 - Terminal Alpha 10 p1"})
			defer removeTimetableOverlay(res)
			if res.Ready || res.Applied || res.TripIndex != 0 {
				t.Fatalf("contradictory route must remain blocked: %+v", res)
			}
		})
	}
}

func TestTripMetadataPreservesUnnamedEndpointsAndType2Precedence(t *testing.T) {
	for _, tc := range []struct{ name, first, last, extra, wantFirst, wantLast string }{
		{"named", "Start", "End", "", "Start", "End"},
		{"unnamed first", "", "End", "", "", "End"},
		{"unnamed last", "Start", "", "", "Start", ""},
		{"type2 precedence", "Old start", "Old end", "[station_typ2]\n7\n[station_typ2]\n8\n", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trip.ttp")
			writeTestFile(t, path, endpointProfile("Display", tc.first, "Interior", tc.last)+tc.extra)
			meta := readTripMeta(path)
			if !meta.Valid || meta.FirstStop != tc.wantFirst || meta.LastStop != tc.wantLast {
				t.Fatalf("actual endpoint positions must be preserved: %+v", meta)
			}
		})
	}
}

func TestType2TripRetainsDisplayDestinationFallback(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n600.000\n")
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), "[trip]\n\nDepot\n10\n[station_typ2]\n123\n[station_typ2]\n456\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot"})
	if !res.Ready || res.Applied || res.TripIndex != 1 {
		t.Fatalf("display fallback should preserve supported type-2 behavior: %+v", res)
	}
}

func TestSzczecin522OperationalDepartureWithoutOffset(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	var ttl strings.Builder
	ttl.WriteString("[newtour]\n1 (ni-sr)\nDepot\n1023\n")
	for _, v := range []float64{140, 340, 540, 740, 940, 1540, 1740, 1940, 2140, 2340, 2540} {
		fmt.Fprintf(&ttl, "[addtrip]\n522_Kor-Koll\n0\n%.3f\n", v)
	}
	path := filepath.Join(tt, "522.ttl")
	writeTestFile(t, path, ttl.String())
	writeTestFile(t, filepath.Join(tt, "522_Kor-Koll.ttp"), "[trip]\n\nKollataja\n522\n[station_typ2]\n123\n[station_typ2]\n456\n")
	info := TripInfo{Line: "522", Tour: "1 (ni-sr)", TripStart: "01:40", TripEnd: "01:48", RouteText: "Kormoranow - Kollataja", ShiftID: "4930868"}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-07", info)
	defer removeTimetableOverlay(res)
	if !res.Ready || res.Applied || res.TripIndex != 6 || res.TripName != "522_Kor-Koll" || res.BCSDeparture != 1540 || res.BCSCivilDeparture != 100 || res.OffsetMinutes != 0 || res.OverrideZIP != "" {
		t.Fatalf("522 must select #6 25:40 with offset zero: %+v", res)
	}
	original, err := os.ReadFile(path)
	if err != nil || string(original) != ttl.String() {
		t.Fatal("installed timetable changed")
	}
	if !strings.Contains(timetableSyncDiagnostic(res), "25:40") {
		t.Fatal("operational time missing from diagnostic")
	}
}

func TestExtended49HourRuntimePreservesLogicalOffsetAndInventory(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	path := filepath.Join(tt, "10.ttl")
	original := "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n600.000\n[addtrip]\nOUT\n0\n2980.000\n"
	writeTestFile(t, path, original)
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), endpointProfile("Beta", "Alpha", "Middle", "Beta"))
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-07", TripInfo{Line: "10", Tour: "A", TripStart: "01:40", RouteText: "Alpha - Beta"})
	defer removeTimetableOverlay(res)
	if !res.Ready || !res.Applied || res.TripIndex != 2 || res.BCSDeparture != 2980 || res.OffsetMinutes != 0 || res.RuntimeDayAdjustmentMinutes != -1440 {
		t.Fatalf("49-hour alignment: %+v", res)
	}
	_, body := readOnlyZipEntry(t, res.OverrideZIP)
	tours := parseTTL(body)
	if len(tours) != 1 || len(tours[0].Trips) != 2 || tours[0].Trips[0].Departure != 600 || tours[0].Trips[1].Departure != 1540 {
		t.Fatal("unexpected runtime overlay", body)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != original {
		t.Fatal("installed TTL changed")
	}
}

func writeHafenCityType2Fixture(t *testing.T, tt, line, ttl string) {
	t.Helper()
	writeTestFile(t, filepath.Join(tt, line+".ttl"), ttl)
	writeTestFile(t, filepath.Join(tt, "109_UAL_RAM.ttp"), "[trip]\n\nRathausmarkt\n109\n[station_typ2]\n4266642\n[station_typ2]\n5735767\n")
}

func TestHafenCityType2DecoratedTerminusUsesUniqueBCSMinute(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	line := "Addon Tag und Nacht Li. 109"
	writeHafenCityType2Fixture(t, tt, line, "[newtour]\n55120\nDepot\n1023\n[addtrip]\n109_UAL_RAM\n2\n1300.000\n[addtrip]\n109_UAL_RAM\n2\n1334.343\n[addtrip]\n109_UAL_RAM\n2\n1400.000\n")

	info := TripInfo{
		Line:       line,
		Tour:       "55120",
		TripStart:  "22:14",
		RouteText:  "U Alsterdorf (Ankunft) - Rathausmarkt (Terminus 109)",
		StartPoint: "U Alsterdorf (Ankunft)",
	}
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-08", info)
	defer removeTimetableOverlay(res)

	if !res.Ready || res.Applied || res.TripIndex != 2 || res.TripName != "109_UAL_RAM" {
		t.Fatalf("HafenCity type-2 trip should be selected safely: %+v", res)
	}
	if res.OriginalDeparture != 1334.343 || res.BCSDeparture != 1334.343 || res.OffsetMinutes != 0 || res.OverrideZIP != "" {
		t.Fatalf("minute-only BCS time must preserve the timetable seconds: %+v", res)
	}
	if !strings.Contains(res.Reason, "type-2 display destination") || !strings.Contains(res.Reason, "unique BCS display minute") {
		t.Fatalf("expected explicit fallback evidence in diagnostic: %+v", res)
	}
}

func TestHafenCityType2SameMinuteStillAmbiguous(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	line := "Addon Tag und Nacht Li. 109"
	writeHafenCityType2Fixture(t, tt, line, "[newtour]\n55120\nDepot\n1023\n[addtrip]\n109_UAL_RAM\n2\n1334.100\n[addtrip]\n109_UAL_RAM\n2\n1334.700\n")

	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-08", TripInfo{
		Line:      line,
		Tour:      "55120",
		TripStart: "22:14",
		RouteText: "U Alsterdorf (Ankunft) - Rathausmarkt (Terminus 109)",
	})
	if res.Ready || res.Applied || res.TripIndex != 0 || !strings.Contains(res.Reason, "ambiguous") {
		t.Fatalf("two trips inside the same BCS display minute must stay blocked: %+v", res)
	}
}
