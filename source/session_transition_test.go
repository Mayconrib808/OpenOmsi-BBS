package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func syntheticTransitionTrip(id, mapName, line, tour string) string {
	return "INFO Schicht ID: " + id + "\nINFO Karte: " + mapName + "\nINFO Tour: 00:45 - 01:15 (Linie: " + line + ")\nA - B (Umlauf: " + tour + ")\nINFO Startpunkt: A\n"
}

func TestNextTripSelectedAfterGameExitStillRelaunches(t *testing.T) {
	baseline := syntheticTransitionTrip("1001", "First", "1", "1")
	closed := baseline + "Schicht abschließen: 1001\n"
	g := newNextTripGate("1001", baseline)
	_, _ = g.observe(closed)
	reads := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	trip, ok := waitForNextBCSTrip(ctx, &g, func() (string, error) {
		reads++
		if reads < 3 {
			return closed, nil
		}
		if reads == 3 {
			return closed + "Schicht ID: 1002\nKarte: Next\n", nil
		}
		return closed + syntheticTransitionTrip("1002", "Next", "522", "2"), nil
	}, time.Millisecond)
	if !ok || trip.ShiftID != "1002" || trip.MapName != "Next" || trip.Tour != "2" {
		t.Fatal(trip, ok)
	}
	if _, ok = g.observe(closed + syntheticTransitionTrip("1002", "Next", "522", "2")); ok {
		t.Fatal("handoff delivered twice")
	}
}

func TestNextTripWaitCancelsWithoutReplayingIncompleteTrip(t *testing.T) {
	baseline := syntheticTransitionTrip("1001", "First", "1", "1")
	g := newNextTripGate("1001", baseline)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok := waitForNextBCSTrip(ctx, &g, func() (string, error) {
		return baseline + "Schicht abschließen: 1001\nSchicht ID: 1002\nKarte: Partial\n", nil
	}, time.Millisecond)
	if ok {
		t.Fatal("timeout replayed partial trip")
	}
}

func TestRepeatedCompletionDoesNotSwallowNextTrip(t *testing.T) {
	baseline := syntheticTransitionTrip("1001", "First", "1", "1")
	closed := baseline + "Schicht abschließen: 1001\n"
	next := syntheticTransitionTrip("1002", "Next", "522", "2")
	for _, observed := range []bool{false, true} {
		g := newNextTripGate("1001", baseline)
		if observed {
			_, _ = g.observe(closed)
		}
		trip, ok := g.observe(closed + next + "Schicht abschließen: 1001\n")
		if !ok || trip.ShiftID != "1002" {
			t.Fatal("duplicate closure swallowed fresh trip", trip, ok)
		}
	}
}

func TestNextTripNeedsCompletionAndDistinctCompleteShift(t *testing.T) {
	baseline := syntheticTransitionTrip("1001", "First", "N407", "03")
	current := baseline + "INFO Auswertungsdaten: real saved values\n"
	g := newNextTripGate("1001", baseline)
	if _, ok := g.observe(current + syntheticTransitionTrip("1999", "Premature", "2201", "02")); ok {
		t.Fatal("a new selection before current shift completion stopped the game")
	}
	current += "INFO Schicht abschließen: 1001\n"
	if _, ok := g.observe(current + syntheticTransitionTrip("1001", "First", "N407", "03")); ok {
		t.Fatal("retry of the current shift was treated as a next trip")
	}
	if _, ok := g.observe(current + "INFO Schicht ID: 1002\nINFO Karte: Second\n"); ok {
		t.Fatal("incomplete next trip was accepted")
	}
	next := syntheticTransitionTrip("1002", "Second", "2201", "02")
	trip, ok := g.observe(current + next)
	if !ok || trip.ShiftID != "1002" || trip.MapName != "Second" || trip.Line != "2201" || trip.Tour != "02" {
		t.Fatalf("fresh complete shift not delivered: %+v %v", trip, ok)
	}
	if _, ok := g.observe(current + next); ok {
		t.Fatal("same handoff delivered twice")
	}
}

func TestNextTripIgnoresHistoryAndRequiresEventOrder(t *testing.T) {
	old := syntheticTransitionTrip("99", "Old", "1", "1")
	baseline := old + syntheticTransitionTrip("1001", "First", "N407", "03")
	g := newNextTripGate("1001", baseline)
	wrongOrder := baseline + syntheticTransitionTrip("1002", "Second", "2", "2") + "Schicht abschließen: 1001\n"
	if _, ok := g.observe(wrongOrder); ok {
		t.Fatal("trip selected before completion was used after completion")
	}
	if _, ok := g.observe(old); ok {
		t.Fatal("trimmed history reused an old shift")
	}
	if _, ok := g.observe(syntheticTransitionTrip("1002", "Second", "2", "2")); ok {
		t.Fatal("log replacement replayed a shift selected before completion")
	}
	next := syntheticTransitionTrip("1003", "Fresh", "3", "3")
	if trip, ok := g.observe(next); !ok || trip.ShiftID != "1003" {
		t.Fatalf("completion observed before log replacement was forgotten: %+v %v", trip, ok)
	}
}

func TestNextTripRejectsClosedOrPartialNewestBlock(t *testing.T) {
	baseline := syntheticTransitionTrip("1001", "First", "N407", "03")
	complete := baseline + "Schicht abschliessen: 1001\n"
	next := syntheticTransitionTrip("1002", "Next", "2", "2")
	cases := []string{
		complete + next + "Schicht abschließen: 1002\n",
		complete + next + "Schicht ID: 1003\nKarte: Latest\n",
		complete + "Schicht ID: 1002\nKarte: Next\nTour: 00:45 - 01:15 (Linie: 2)\n",
	}
	for _, text := range cases {
		g := newNextTripGate("1001", baseline)
		if _, ok := g.observe(text); ok {
			t.Fatalf("unsafe incomplete/closed next shift accepted: %q", text)
		}
	}
	for _, id := range []string{"", "100", "1001x"} {
		g := newNextTripGate(id, baseline)
		if _, ok := g.observe(complete + next); ok {
			t.Fatalf("missing/partial current ID %q authorized handoff", id)
		}
	}
	g := newNextTripGate("1001", complete)
	if _, ok := g.observe(complete + next); ok {
		t.Fatal("a current shift already completed before launch authorized handoff")
	}
}

func TestCompletedEvaluationRetainsActualRecordForNextBaseline(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("testdata", "bbs-before.odr"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	nativePath := filepath.Join(dir, "bbs.odr")
	if err := os.WriteFile(nativePath, base, 0644); err != nil {
		t.Fatal(err)
	}
	s, err := prepareDriver(nativePath, dir)
	if err != nil {
		t.Fatal(err)
	}
	actual := s.Last
	if err := s.freezeEvaluation(); err != nil {
		t.Fatal(err)
	}
	later := actual
	later.Stops[0] += 10
	later.Hectom += 25
	later.Tickets += 12
	if err := os.WriteFile(s.OpenPath, later.encode(true), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if changed, err := s.poll(); changed || err != nil {
			t.Fatalf("completed record updated after next-screen save: changed=%v err=%v", changed, err)
		}
	}
	got, err := os.ReadFile(nativePath)
	if err != nil || !bytes.Equal(got, base) || s.Last.evaluation() != actual.evaluation() {
		t.Fatalf("actual BCS evaluation/baseline overwritten after completion: %v", err)
	}
}

func TestEvaluationFreezeFlagMatchesOnlyExactCurrentShift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "freeze.flag")
	if evaluationFreezeRequested(path, "1001") {
		t.Fatal("missing completion flag froze driver data")
	}
	if err := os.WriteFile(path, []byte("1001\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !evaluationFreezeRequested(path, "1001") {
		t.Fatal("exact matching completion flag ignored")
	}
	for _, id := range []string{"", "100", "10012", "01001", "+1001", "1001x"} {
		if evaluationFreezeRequested(path, id) {
			t.Fatalf("completion flag froze a different/invalid shift %q", id)
		}
	}
}
