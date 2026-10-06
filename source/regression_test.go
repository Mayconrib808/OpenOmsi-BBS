package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A Chrono can replace just a trip profile, leaving the line's TTL in base
// TTData. The game loads the higher-priority TTP first regardless of TTL origin.
func TestV110ChronoTTPPriorityIndependentOfTTLLocation(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	base := filepath.Join(root, "maps", "Test Map", "TTData")
	chrono := filepath.Join(root, "maps", "Test Map", "Chrono", "2026 service")
	writeTestFile(t, filepath.Join(base, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n360.000\n[addtrip]\nBACK\n0\n390.000\n")
	writeTestFile(t, filepath.Join(base, "OUT.ttp"), "[trip]\nTrackOut\nAlpha\n10\n[station]\n1\n0\nDepot\n")
	writeTestFile(t, filepath.Join(base, "BACK.ttp"), "[trip]\nTrackBack\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	writeTestFile(t, filepath.Join(chrono, "Chrono.cfg"), "[startdate]\n20260101\n[enddate]\n20261231\n")
	writeTestFile(t, filepath.Join(chrono, "TTData", "OUT.ttp"), "[trip]\nTrackOutChanged\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	writeTestFile(t, filepath.Join(chrono, "TTData", "BACK.ttp"), "[trip]\nTrackBackChanged\nAlpha\n10\n[station]\n1\n0\nDepot\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot", StartPoint: "Alpha"})
	defer removeTimetableOverlay(res)
	if !res.Applied || res.TripName != "OUT" || res.TripIndex != 1 {
		t.Fatalf("game uses Chrono OUT (Alpha -> Depot), bridge must select the same trip; got %+v", res)
	}
}

// No start/destination metadata agrees with BCS. A single matching line field
// does not establish the selected trip's route or time.
func TestV110LineMatchAloneDoesNotIdentifyTheBCSRoute(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n360.000\n[addtrip]\nBACK\n0\n390.000\n")
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), "[trip]\nTrackOut\nOther terminal\n10\n[station]\n1\n0\nOther start\n")
	writeTestFile(t, filepath.Join(tt, "BACK.ttp"), "[trip]\nTrackBack\nAnother terminal\n20\n[station]\n1\n0\nAnother start\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot", StartPoint: "Alpha"})
	defer removeTimetableOverlay(res)
	if res.Applied || res.TripIndex != 0 {
		t.Fatalf("no trip matches BCS route, so do not publish an assumed trip; got %+v", res)
	}
}

// Stable first-launch marker evidence from user's 1.0.2 logs: the new wait
// returns immediately when the marker was already present.
func TestV110ExistingMarkerDoesNotDelayOrValidateBCSConnection(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "bbs.start")
	writeTestFile(t, p, "existing BCS marker")
	present, elapsed := waitForBCSStartupMarker(root, 2*time.Second)
	if !present || elapsed > 200*time.Millisecond {
		t.Fatalf("existing marker should immediately satisfy the wait; present=%v elapsed=%s", present, elapsed)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "existing BCS marker" {
		t.Fatalf("marker changed: %q %v", b, err)
	}
	t.Logf("marker wait returned in %s; it contains no plugin connection or overlay visibility check", elapsed)
}

// An overlay with an invalid departure must never be emitted as valid TTData.
func TestV110InvalidNonFiniteDepartureDoesNotProduceOverlay(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\nNaN\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00"})
	defer removeTimetableOverlay(res)
	if res.Applied || res.OverrideZIP != "" {
		t.Fatalf("non-finite departure should be rejected; got %+v", res)
	}
}

// With repeated trips in the same direction there is no temporal tie breaker.
func TestV110RepeatedDirectionReportsAmbiguity(t *testing.T) {
	root, pkg, mapRel := basicMap(t)
	tt := filepath.Join(root, "maps", "Test Map", "TTData")
	writeTestFile(t, filepath.Join(tt, "10.ttl"), "[newtour]\nA\nDepot\n1023\n[addtrip]\nOUT\n0\n360.000\n[addtrip]\nOUT\n0\n420.000\n")
	writeTestFile(t, filepath.Join(tt, "OUT.ttp"), "[trip]\nTrackOut\nDepot\n10\n[station]\n1\n0\nAlpha\n")
	res := prepareTimetableSync(root, pkg, mapRel, "2026-10-06", TripInfo{Line: "10", Tour: "A", TripStart: "10:00", RouteText: "Alpha - Depot", StartPoint: "Alpha"})
	defer removeTimetableOverlay(res)
	if res.Ready || res.Applied || res.TripIndex != 0 || !strings.Contains(res.Reason, "ambiguous") {
		t.Fatalf("repeated route requires explicit ambiguity, got %+v", res)
	}
	t.Logf("original timetable is retained; launch must be blocked: %s", res.Reason)
}
