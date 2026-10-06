package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

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
