package main

import (
	"path/filepath"
	"strings"
	"testing"
)

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
