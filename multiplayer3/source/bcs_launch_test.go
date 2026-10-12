package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func prepareBCSTest(t *testing.T, args []string, root string) []string {
	t.Helper()
	out, err := prepareBCSLaunch(args, root, root)
	if err != nil {
		t.Fatal(err)
	}
	return out
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
	got := prepareBCSTest(t, fresh, root)
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
	again := prepareBCSTest(t, got, root)
	if !reflect.DeepEqual(got, again) {
		t.Fatal("autostart duplicated")
	}
	negative := strings.ReplaceAll(strings.ReplaceAll(saved, "\n2394\n", "\n-2394\n"), "\n11288\n", "\n-11288\n")
	if err := os.WriteFile(situation, []byte(negative), 0644); err != nil {
		t.Fatal(err)
	}
	negativeArgs, err := peDePeFreshTripArgs(original, root)
	if err != nil {
		t.Fatal(err)
	}
	spawn, _ = pedepeArgValue(negativeArgs, "--spawn")
	if !strings.HasPrefix(spawn, "-") {
		t.Fatal("test must exercise a negative map coordinate", spawn)
	}
	attached := false
	for _, arg := range negativeArgs {
		if arg == "--spawn" {
			t.Fatal("negative coordinate remains a separate option-like token")
		}
		if arg == "--spawn="+spawn {
			attached = true
		}
	}
	if !attached {
		t.Fatal("spawn value was not attached to its option")
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
