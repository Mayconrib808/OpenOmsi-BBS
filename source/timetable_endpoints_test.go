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
	if !res.Ready || !res.Applied || res.TripIndex != 62 || res.TripName != "2201rota2" || res.OriginalDeparture != 1460 || res.BCSDeparture != 20 || res.OffsetMinutes != -1440 {
		t.Fatalf("00:20 must select and align the unique 24:20 trip: %+v", res)
	}
	_, body := readOnlyZipEntry(t, res.OverrideZIP)
	originalTours, updatedTours := parseTTL(ttl.String()), parseTTL(body)
	if len(updatedTours) != 1 || len(updatedTours[0].Trips) != 64 {
		t.Fatalf("duty structure changed: %s", body)
	}
	for i, tr := range updatedTours[0].Trips {
		want := originalTours[0].Trips[i].Departure
		if i == 61 {
			want = 20
		}
		if tr.Departure != want || tr.Name != originalTours[0].Trips[i].Name || tr.Profile != originalTours[0].Trips[i].Profile {
			t.Fatalf("record %d changed unexpectedly: %+v", i+1, tr)
		}
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
		{2900, 20, false}, {1440, 1439, false}, {1439, 0, false},
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
