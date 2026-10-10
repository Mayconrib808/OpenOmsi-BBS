package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

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
		Map: filepath.Join(root, "maps", "Ruhrau V2", "global.cfg"),
		Bus: filepath.Join(root, "Vehicles", "MAN", "MAN.bus"),
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
