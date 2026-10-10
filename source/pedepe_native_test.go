package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPeDePeDriverUsesBBSExecutableDirectoryInsteadOfOMSIAssetRoot(t *testing.T) {
	root := t.TempDir()
	adapterDir := t.TempDir()
	base := []string{"--root", root, "--schedule", "--line", "137", "--tour", "Sa 2", "--trip", "06:09", "--lan-join", "https://example.invalid"}
	for _, driver := range [][]string{{"--driver", "Drivers/bbs.odr"}, {"--driver=Drivers\\bbs.odr"}} {
		args := append(append([]string(nil), base...), driver...)
		before := append([]string(nil), args...)
		got, err := peDePeDriverArgs(args, adapterDir)
		if err != nil {
			t.Fatal(err)
		}
		path, _ := pedepeArgValue(got, "--driver")
		if path != filepath.Join(adapterDir, "Drivers", "bbs.odr") {
			t.Fatalf("wrong watched file: %q", path)
		}
		if !reflect.DeepEqual(args, before) {
			t.Fatal("input arguments mutated")
		}
		if joined, _ := pedepeArgValue(got, "--lan-join"); joined != "https://example.invalid" {
			t.Fatal("multiplayer changed")
		}
	}
	absolute := filepath.Join(root, "another-driver.odr")
	args := append(append([]string(nil), base...), "--driver", absolute)
	got, err := peDePeDriverArgs(args, "")
	if err != nil || !reflect.DeepEqual(got, args) {
		t.Fatalf("explicit absolute driver changed: %v %v", got, err)
	}
	probe := []string{"--help", "--driver", "Drivers/bbs.odr"}
	got, err = peDePeDriverArgs(probe, "")
	if err != nil || !reflect.DeepEqual(got, probe) {
		t.Fatal("probe changed")
	}
	missingRoot := []string{"--schedule", "--line", "137", "--tour", "Sa 2", "--trip", "06:09", "--driver", "Drivers/bbs.odr"}
	if _, err := peDePeDriverArgs(missingRoot, ""); err == nil {
		t.Fatal("missing root silently accepted")
	}
	got, err = peDePeDriverArgs(missingRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	if path, _ := pedepeArgValue(got, "--driver"); path != filepath.Join(root, "Drivers", "bbs.odr") {
		t.Fatal("configured root ignored")
	}
}

func TestPeDePePublishesSavedStopsToBBSWatchedFile(t *testing.T) {
	assetRoot, adapterDir := t.TempDir(), t.TempDir()
	watched := filepath.Join(adapterDir, "Drivers", "bbs.odr")
	if err := os.MkdirAll(filepath.Dir(watched), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(watched, fixture(t, "bbs-before.odr"), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--root", assetRoot, "--driver", "Drivers/bbs.odr", "--schedule", "--line", "2002", "--tour", "04", "--trip", "08:25"}
	forwarded, err := peDePeDriverArgs(args, adapterDir)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := pedepeArgValue(forwarded, "--driver")
	s, err := prepareDriver(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saved := s.Last
	saved.Stops[0] += 6
	writeSaved(t, s, saved)
	_, _ = s.poll()
	if changed, err := s.poll(); !changed || err != nil {
		t.Fatalf("F9 publication failed: %v %v", changed, err)
	}
	b, err := os.ReadFile(watched)
	if err != nil {
		t.Fatal(err)
	}
	record, err := parseDriver(b, false)
	if err != nil || record.Stops[0] != saved.Stops[0] {
		t.Fatalf("BBS watched file did not receive the six saved stops: %v %v", record.Stops, err)
	}
	if _, err := os.Stat(filepath.Join(assetRoot, "Drivers", "bbs.odr")); !os.IsNotExist(err) {
		t.Fatal("driver was published to the OMSI asset directory")
	}
}

func TestPeDePeNativeInvocationKeepsOfficialTripArguments(t *testing.T) {
	args := []string{
		"--root", `C:\OMSI 2`, "--no-menu",
		"--map", `maps\Ruhrau V2\global.cfg`,
		"--bus", `Vehicles\MAN\MAN.bus`,
		"--paint", "Transfort BR", "--time", "05:42:00",
		"--date=2026-10-10", "--weather", "Clear", "--schedule",
		"--line", "2731", "--tour", "03", "--driver", `Drivers\bbs.odr`,
	}
	in := parsePeDePeNativeInvocation(args)
	if in.Map != `maps\Ruhrau V2\global.cfg` || in.Bus != `Vehicles\MAN\MAN.bus` || in.Clock != "05:42:00" || in.Date != "2026-10-10" || !in.HasSchedule {
		t.Fatalf("unexpected native invocation: %+v", in)
	}
	if in.Line != "2731" || in.Tour != "03" {
		t.Fatalf("schedule metadata lost: %+v", in)
	}
	if in.HasLANJoin || in.Server || in.Probe {
		t.Fatalf("normal BBS trip misclassified: %+v", in)
	}
}

func TestPeDePeCurrentSituationLaunchGetsAutostartWithoutRewritingVendorArgs(t *testing.T) {
	original := []string{
		"--root", `F:\SteamLibrary\steamapps\common\OMSI 2`,
		"--no-menu",
		"--situation", `F:\SteamLibrary\steamapps\common\OMSI 2\maps\Berlin-Spandau\laststn.osn`,
		"--passengers", "--traffic", "30", "--driver", "Drivers/bbs.odr",
		"--schedule", "--line", "130 & N30", "--tour", "Mo-Fr 9", "--trip", "04:05",
		"--keep-time", "--weather", "natural",
	}
	in := parsePeDePeNativeInvocation(original)
	if in.Situation == "" || in.Line != "130 & N30" || in.Tour != "Mo-Fr 9" || in.Trip != "04:05" || !in.HasSchedule {
		t.Fatalf("real PeDePe situation launch parsed incorrectly: %+v", in)
	}
	if !shouldAddPeDePeAutoStart(in) {
		t.Fatal("scheduled PeDePe BBS trip should restore legacy --autostart/IBIS behaviour")
	}
	got := peDePeForwardArgs(original, nil)
	if !reflect.DeepEqual(got[:len(original)], original) {
		t.Fatalf("PeDePe vendor arguments changed: got %#v want prefix %#v", got, original)
	}
	if !reflect.DeepEqual(got[len(original):], []string{"--autostart"}) {
		t.Fatalf("wrong compatibility tail: %#v", got[len(original):])
	}
}

func TestPeDePeSituationLaunchResolvesCompanyMultiplayerTrip(t *testing.T) {
	root := t.TempDir()
	mapDir := filepath.Join(root, "maps", "Berlin-Spandau")
	if err := os.MkdirAll(mapDir, 0700); err != nil {
		t.Fatal(err)
	}
	situation := filepath.Join(mapDir, "laststn.osn")
	text := "[map]\n" + filepath.Join("maps", "Berlin-Spandau", "global.cfg") + "\n" +
		"[time]\n2026\n283\n03\n59\n30\n" +
		"[vehicle]\n" + filepath.Join("Vehicles", "MAN", "MAN.bus") + "\n[ismyvehicle]\n"
	if err := os.WriteFile(situation, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Root = root
	c.Multiplayer = true
	args := []string{
		"--root", root, "--no-menu", "--situation", situation,
		"--schedule", "--line", "130 & N30", "--tour", "Mo-Fr 9", "--trip", "04:05",
		"--keep-time", "--weather", "natural",
	}
	in := parsePeDePeNativeInvocation(args)
	if !shouldInjectPeDePeMultiplayer(in, c) {
		t.Fatal("PeDePe saved-situation trip should receive company multiplayer")
	}
	trip, err := peDePeMultiplayerTrip(c, in, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if trip.MapName != "Berlin-Spandau" || trip.Date != "2026-10-10" || trip.Start != "04:05" || trip.Weather != "natural" {
		t.Fatalf("wrong resolved trip: %+v", trip)
	}
	if trip.MapFile != filepath.Join("maps", "Berlin-Spandau", "global.cfg") {
		t.Fatalf("wrong map file: %q", trip.MapFile)
	}
	if trip.BusFile != filepath.Join("Vehicles", "MAN", "MAN.bus") {
		t.Fatalf("wrong player bus: %q", trip.BusFile)
	}
}

func TestPeDePeSituationClockIsFallbackWhenTripIsNotAClock(t *testing.T) {
	root := t.TempDir()
	mapDir := filepath.Join(root, "maps", "Sample")
	if err := os.MkdirAll(mapDir, 0700); err != nil {
		t.Fatal(err)
	}
	situation := filepath.Join(mapDir, "laststn.osn")
	if err := os.WriteFile(situation, []byte("[map]\n"+filepath.Join("maps", "Sample", "global.cfg")+"\n[time]\n2026\n283\n12\n34\n56\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Root = root
	in := pedepeNativeInvocation{Root: root, Situation: situation, Trip: "Trip A", Weather: "natural"}
	trip, err := peDePeMultiplayerTrip(c, in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if trip.Start != "12:34:56" || trip.Date != "2026-10-10" || trip.MapName != "Sample" {
		t.Fatalf("situation fallback not used: %+v", trip)
	}
}

func TestPeDePeAutoStartIsNeverDuplicatedOrAddedToProbe(t *testing.T) {
	already := []string{"--schedule", "--line", "10", "--tour", "1", "--autostart"}
	if got := peDePeForwardArgs(already, nil); !reflect.DeepEqual(got, already) {
		t.Fatalf("duplicated or rewrote --autostart: %#v", got)
	}
	probe := []string{"--version", "--schedule", "--line", "10", "--tour", "1"}
	if got := peDePeForwardArgs(probe, nil); !reflect.DeepEqual(got, probe) {
		t.Fatalf("probe must remain transparent: %#v", got)
	}
}

func TestPeDePeNativeInjectionStepsAsideForNativeNetworkingAndProbes(t *testing.T) {
	c := defaultConfig()
	c.Multiplayer = true
	base := []string{"--map", `maps\X\global.cfg`, "--bus", `Vehicles\X.bus`, "--time", "12:30", "--schedule"}
	if !shouldInjectPeDePeMultiplayer(parsePeDePeNativeInvocation(base), c) {
		t.Fatal("complete PeDePe trip should receive company multiplayer")
	}
	for _, extra := range [][]string{{"--lan-join", "wss://native.example"}, {"--version"}, {"--server", "server.cfg"}} {
		args := append(append([]string(nil), base...), extra...)
		if shouldInjectPeDePeMultiplayer(parsePeDePeNativeInvocation(args), c) {
			t.Fatalf("adapter must step aside for %v", extra)
		}
	}
}

func TestPeDePeForwardArgsDoesNotRewriteVendorArguments(t *testing.T) {
	original := []string{"--map", "map", "--unknown-future-option", "value", "--driver", "driver.odr"}
	plan := &MultiplayerPlan{PlayerName: "Maycon", Session: CompanySession{ServerURL: "https://company.example/session"}}
	got := peDePeForwardArgs(original, plan)
	wantPrefix := original
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("PeDePe arguments changed: got %#v want prefix %#v", got, wantPrefix)
	}
	wantTail := []string{"--lan-join", "https://company.example/session", "--lan-name", "Maycon"}
	if !reflect.DeepEqual(got[len(wantPrefix):], wantTail) {
		t.Fatalf("wrong multiplayer tail: %#v", got[len(wantPrefix):])
	}
}

func TestPeDePeMultiplayerTripUsesVendorDateAndRelativeAssets(t *testing.T) {
	root := t.TempDir()
	c := defaultConfig()
	c.Root = root
	in := pedepeNativeInvocation{
		Root: root,
		Map:  filepath.Join(root, "maps", "Ruhrau V2", "global.cfg"),
		Bus:  filepath.Join(root, "Vehicles", "MAN", "MAN.bus"),
		Date: "2026-10-10", Clock: "05:42:00", Weather: "Clear",
	}
	trip, err := peDePeMultiplayerTrip(c, in, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if trip.MapName != "Ruhrau V2" || trip.Date != "2026-10-10" || trip.Start != "05:42:00" || trip.Weather != "Clear" {
		t.Fatalf("wrong trip: %+v", trip)
	}
	if filepath.IsAbs(trip.MapFile) || filepath.IsAbs(trip.BusFile) {
		t.Fatalf("assets should be relative to OMSI root: %+v", trip)
	}
}

func TestPackageRootRecognizesPeDePeAdapter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "OpenOmsi BBS")
	exe := filepath.Join(root, "PeDePeAdapter", "openomsi.exe")
	if got := packageRootForExecutable(exe); got != root {
		t.Fatalf("adapter package root = %q, want %q", got, root)
	}
}

func TestPeDePeFreshSituationStartsDutyInsteadOfResuming(t *testing.T) {
	root := t.TempDir()
	mapDir := filepath.Join(root, "maps", "Berlin-Spandau")
	if err := os.MkdirAll(mapDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mapDir, "global.cfg"), []byte("[name]\nBerlin-Spandau\n"), 0644); err != nil {
		t.Fatal(err)
	}
	situation := filepath.Join(mapDir, "laststn.osn")
	saved := "[map]\n" + filepath.Join(mapDir, "global.cfg") + "\n[time]\n1994\n283\n1\n55\n0\n[vehicle]\nVehicles/MAN_SD202/SD92.bus\n230.665\n24.532\n205.29\n0\n0.976\n0\n-0.219\n0\n0\n0\n2394\n11288\n12345\nSpandau 94\n[ismyVehicle]\n[vars]\n2\nColorscheme\n3\nIBIS_Ziel\n0\n[settimetable]\nSTALE\nOLD\n0\n0\n0\n0\n"
	if err := os.WriteFile(situation, []byte(saved), 0644); err != nil {
		t.Fatal(err)
	}
	original := []string{"--root", root, "--no-menu", "--situation", situation, "--passengers", "--traffic", "30", "--driver", "Drivers/bbs.odr", "--schedule", "--line", "130 & N30", "--tour", "Mo-Fr 6", "--trip", "02:05", "--keep-time", "--weather", "natural", "--future-vendor-flag", "value"}
	fresh, err := peDePeFreshTripArgs(original, root)
	if err != nil {
		t.Fatal(err)
	}
	got := peDePeForwardArgs(fresh, nil)
	if pedepeHasArg(got, "--situation") {
		t.Fatal("saved resume still suppresses IBIS initialization")
	}
	for flag, want := range map[string]string{"--line": "130 & N30", "--tour": "Mo-Fr 6", "--trip": "02:05", "--time": "01:55:00", "--date": "1994-10-10", "--hof": "Spandau 94", "--paint": "3", "--bus": "Vehicles/MAN_SD202/SD92.bus", "--future-vendor-flag": "value"} {
		value, ok := pedepeArgValue(got, flag)
		if !ok || value != want {
			t.Errorf("%s = %q want %q", flag, value, want)
		}
	}
	if !pedepeHasArg(got, "--autostart") {
		t.Fatal("fresh duty needs autostart")
	}
	spawn, _ := pedepeArgValue(got, "--spawn")
	values := strings.Split(spawn, ",")
	x, _ := strconv.ParseFloat(values[0], 64)
	y, _ := strconv.ParseFloat(values[1], 64)
	if math.Abs(x-(2394*300+230.665)) > 1e-6 || math.Abs(y-(11288*300+205.29)) > 1e-6 {
		t.Fatal(spawn)
	}
	after, _ := os.ReadFile(situation)
	if string(after) != saved {
		t.Fatal("BBS situation was modified")
	}
	again := peDePeForwardArgs(got, nil)
	if !reflect.DeepEqual(got, again) {
		t.Fatal("autostart duplicated")
	}
}

func TestPeDePeFreshSituationLeavesManualResumesAndRejectsBrokenTemplates(t *testing.T) {
	for _, args := range [][]string{{"--situation", "save.osn"}, {"--help", "--situation", "save.osn", "--schedule", "--line", "1", "--tour", "1", "--trip", "12:00"}} {
		got, err := peDePeFreshTripArgs(args, "")
		if err != nil || !reflect.DeepEqual(got, args) {
			t.Fatalf("manual launch rewritten: %v %v", got, err)
		}
	}
	args := []string{"--root", t.TempDir(), "--situation", "missing.osn", "--schedule", "--line", "1", "--tour", "1", "--trip", "12:00"}
	if _, err := peDePeFreshTripArgs(args, ""); err == nil {
		t.Fatal("broken template must not start another bus/map")
	}
}

func TestPeDePeSavedPositionWorldGrid(t *testing.T) {
	t.Setenv("OMSI_OLD_WORLD_GRID", "1")
	p := filepath.Join(t.TempDir(), "global.cfg")
	os.WriteFile(p, []byte("[worldcoordinates]\n[map]\n2402\n11279\ntile_2402_11279.map\n"), 0644)
	x, y, err := peDePeSavedPosition(p, 2402, 11279, 342.78, 276.13)
	if err != nil || math.Abs(x-(2402*371.9+342.78)) > 1e-6 || math.Abs(y-(11279*371.9+276.13)) > 1e-6 {
		t.Fatalf("legacy grid: %f %f %v", x, y, err)
	}
	os.Unsetenv("OMSI_OLD_WORLD_GRID")
	x, y, err = peDePeSavedPosition(p, 2402, 11279, 342.78, 276.13)
	if err != nil || math.Abs(x-893796.140404) > 0.00001 || math.Abs(y-4195638.526849) > 0.00001 {
		t.Fatalf("world grid: %f %f %v", x, y, err)
	}
}
