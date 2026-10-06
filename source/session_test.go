package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticSessionClosesWithoutExactHistoricalPrefix(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "bcs-synthetic-session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	trip, err := parseBCSLog(filepath.Join("testdata", "bcs-synthetic-session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if trip.ShiftID != "1000123" || trip.Tour != "04" || trip.TripStart != "22:45" {
		t.Fatalf("wrong current trip: %+v", trip)
	}
	cut := strings.Index(text, "Schicht abschließen: 1000123")
	if cut < 0 {
		t.Fatal("missing actual finish marker")
	}
	baseline := "historical lines removed by BCS\n" + text[:cut]
	g := newCompletionGate(trip.ShiftID, baseline)
	if strings.HasPrefix(text, baseline) {
		t.Fatal("regression needs changed historical prefix")
	}
	if !g.accepts(text) {
		t.Fatal("actual completion discarded after a changed log prefix")
	}
	if g.accepts(text[:cut]) {
		t.Fatal("closed before current shift completion")
	}
}

func TestCompletionGateRejectsOtherOldAndPartialShifts(t *testing.T) {
	cases := []struct {
		name, id, baseline, current string
		want                        bool
	}{
		{"current shift", "1000123", "Schicht ID: 1000123\n", "Schicht abschließen: 1000123\n", true},
		{"older shift", "1000123", "", "Schicht abschließen: 1000042\n", false},
		{"partial identifier", "1000123", "", "Schicht abschließen: 100012\n", false},
		{"identifier is not substring", "100012", "", "Schicht abschließen: 1000123\n", false},
		{"already completed at launch", "1000123", "Schicht abschließen: 1000123\n", "Schicht abschließen: 1000123\n", false},
		{"missing shift", "", "", "Schicht abschließen: 1000123\n", false},
		{"ASCII spelling", "1000123", "", "Schicht abschliessen: 1000123\n", true},
		{"no marker", "1000123", "", "Auswertungsdaten: 1015####20\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newCompletionGate(tc.id, tc.baseline).accepts(tc.current); got != tc.want {
				t.Fatalf("gate=%v, wanted %v", got, tc.want)
			}
		})
	}
	// A subsequent free-game block must not inherit the multiplayer shift ID.
	log := "INFO Spielmodus: MULTIPLAYER\nINFO Schicht ID: 1000123\nINFO Karte: Old\nINFO Tour: 22:45 - 22:53 (Linie: 2002)\nA - B (Umlauf: 04)\nINFO Spielmodus: FREIES_SPIEL\nINFO Karte: New\nINFO Tour: 17:00 - 17:38 (Linie: 137)\nA - B (Umlauf: Mo-Fr 5)\n"
	p := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(p, []byte(log), 0644); err != nil {
		t.Fatal(err)
	}
	trip, err := parseBCSLog(p)
	if err != nil || trip.ShiftID != "" || trip.MapName != "New" {
		t.Fatalf("inherited old shift: %+v %v", trip, err)
	}
}
