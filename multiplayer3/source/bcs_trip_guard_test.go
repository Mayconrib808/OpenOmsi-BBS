package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestSavedMapCalendarDoesNotDependOnPCDaylightSaving(t *testing.T) {
	previous := time.Local
	defer func() { time.Local = previous }()
	for _, zone := range []string{"UTC", "America/Sao_Paulo", "Europe/Berlin", "America/Bogota"} {
		location, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = location
		date, _, ok := peDePeSituationTime([]string{"1994", "283", "1", "55", "0"}, 0)
		if !ok || date != "1994-10-10" {
			t.Fatal(zone, date, ok)
		}
	}
}

const selectionFixture = "INFO Schicht ID: 4974938\nINFO Karte: Any map\nINFO Tour: 00:38:00 - 00:56:00 (Linie: 130 & N30)\nNervenklinik - U Rathaus Spandau (Umlauf: Mo-Fr 3)\nINFO Startpunkt: Spandau Nervenklinik\n"

func selectionInvocation() pedepeNativeInvocation {
	return pedepeNativeInvocation{Map: "maps/Any map/global.cfg", Bus: "Vehicles/Any/bus.bus", Line: "130 & N30", Tour: "Mo-Fr 3", Trip: "4", HasSchedule: true}
}

func TestBCSSelectionUsesLatestCompleteShiftAndRealDeparture(t *testing.T) {
	in := selectionInvocation()
	s, err := parseBCSTripSelection("INFO Schicht ID: 123\nold incomplete\n"+selectionFixture, in)
	if err != nil || s.Start != 38 || s.End != 56 || s.Route != "Nervenklinik - U Rathaus Spandau" || s.StartPoint != "Spandau Nervenklinik" {
		t.Fatal(s, err)
	}
	if _, err = parseBCSTripSelection(selectionFixture+"INFO Schicht ID: 999\n", in); err == nil {
		t.Fatal("reused old selection while new block is incomplete")
	}
	in.Tour = "different"
	if _, err = parseBCSTripSelection(selectionFixture, in); err == nil {
		t.Fatal("accepted a different current BCS tour")
	}
}

func TestBCSSelectionLogHandlesUTF16AndBoundedTail(t *testing.T) {
	text := strings.Repeat("older log line\n", 200000) + selectionFixture
	for _, wide := range []bool{false, true} {
		data := []byte(text)
		if wide {
			u := utf16.Encode([]rune(text))
			data = make([]byte, 2+len(u)*2)
			data[0], data[1] = 255, 254
			for i, v := range u {
				binary.LittleEndian.PutUint16(data[2+i*2:], v)
			}
		}
		p := filepath.Join(t.TempDir(), "log.txt")
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := readBCSSelectionLog(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = parseBCSTripSelection(got, selectionInvocation()); err != nil {
			t.Fatal(wide, err)
		}
	}
}

func TestBCSLoadingMarkerWaitsForActualTripConfirmation(t *testing.T) {
	var out bytes.Buffer
	w := &bcsTripLoadingLog{output: &out}
	for _, s := range []string{"[native] spaw", "ned at entry\n[plugin] [BBS bridge] spawned at multiplayer player vehicle ready\n", "[plugin] [BBS trip confirmed] spawned at selected BCS duty ready\n"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(out.String(), "] spawned at") != 1 || !strings.Contains(out.String(), "[BBS trip confirmed] spawned at") {
		t.Fatal(out.String())
	}
}

func TestTripGuardUsesRuntimeAPIWithoutMapTimetableFiles(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "OMSI 2")
	client := filepath.Join(base, "client")
	dir := filepath.Join(base, "OpenOMSI-BBS")
	if err := os.MkdirAll(filepath.Join(root, "Busbetrieb-Simulator"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Busbetrieb-Simulator", "log.txt"), []byte(selectionFixture), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := localConfig{Root: root, Game: filepath.Join(client, "openomsi.exe")}
	args := []string{"--schedule", "--line", "130 & N30", "--tour", "Mo-Fr 3", "--trip", "4", "--map", "maps/Any map/global.cfg", "--bus", "Vehicles/Any/bus.bus", "--time", "00:25:00"}
	g, out, err := prepareBCSTripGuard(cfg, args, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.cleanup()
	if option(out, "--trip") != "1" || !pedepeHasArg(out, "--whole-tour") || option(out, "--time") != "00:25:00" {
		t.Fatal(out)
	}
	script, err := os.ReadFile(g.plugin)
	if err != nil || !bytes.Contains(script, []byte("omsi.duty.trips")) || bytes.Contains(script, []byte("__BCS_SELECTION_JSON__")) {
		t.Fatal(err)
	}
	// There are deliberately no map, TTData, Chrono, TTP or TTL fixture files.
}
