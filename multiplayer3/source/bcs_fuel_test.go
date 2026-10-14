package main

import (
	"encoding/binary"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPeDePeFuelSelectsPlayerForAnyMapAndBus(t *testing.T) {
	for _, field := range []string{"engine_tank_content", "fuel", "ENGINE_TANK_CONTENT"} {
		for _, amount := range []string{"231.0", "0", "17,25"} {
			for _, marker := range []string{"[myvehicle]\n1\n", "[ismyVehicle]\n"} {
				root := t.TempDir()
				saved := "[map]\nmaps/Arbitrary Map/global.cfg\n[vehicle]\nVehicles/AI/ai.bus\n[vars]\n1\nengine_tank_content\n999\n[vehicle]\nVehicles/Ônibus customizado/model.bus\n[vars]\n2\nColorscheme\n4\n" + field + "\n" + amount + "\n" + marker
				want, _ := strconv.ParseFloat(strings.ReplaceAll(amount, ",", "."), 64)
				for _, wide := range []bool{false, true} {
					data := []byte(saved)
					if wide {
						data = []byte{0xff, 0xfe}
						for _, value := range utf16.Encode([]rune(saved)) {
							data = binary.LittleEndian.AppendUint16(data, value)
						}
					}
					fuel, err := parsePeDePeFuelSnapshot(data, root)
					if err != nil || fuel == nil || fuel.Litres != want || fuel.Field != field || !strings.Contains(fuel.Bus, "customizado") {
						t.Fatalf("wrong player fuel for field=%s amount=%s marker=%s UTF16=%t: %+v %v", field, amount, marker, wide, fuel, err)
					}
				}
			}
		}
	}
}

func TestPeDePeFuelMissingAndInvalidValues(t *testing.T) {
	for _, raw := range []string{"-1", "NaN", "Inf", "1e100", "bad"} {
		data := "[vehicle]\nVehicles/Test/test.bus\n[vars]\n1\nengine_tank_content\n" + raw + "\n[ismyvehicle]\n"
		if _, err := parsePeDePeFuelSnapshot([]byte(data), t.TempDir()); err == nil {
			t.Fatal("invalid fuel accepted", raw)
		}
	}
	for _, data := range []string{
		"[vehicle]\nVehicles/Test/test.bus\n[vars]\n0\n",
		"[vehicle]\nVehicles/Test/test.bus\n[vars]\n1\nColorscheme\n3\n",
	} {
		if fuel, err := parsePeDePeFuelSnapshot([]byte(data), t.TempDir()); err != nil || fuel != nil {
			t.Fatalf("missing fuel invented: %+v %v", fuel, err)
		}
	}
	if _, err := parsePeDePeFuelSnapshot([]byte("[vehicle]\nAI/a.bus\n[vehicle]\nAI/b.bus\n[myvehicle]\nbad\n"), t.TempDir()); err == nil {
		t.Fatal("ambiguous player accepted")
	}
}

func TestPeDePeFuelMergesAssignmentsWithoutChangingOtherState(t *testing.T) {
	for _, option := range [][]string{{"--setvar", "door_0=0,engine_tank_content=999,IBIS_Ziel=42"}, {"--setvar=door_0=0,engine_tank_content=999,IBIS_Ziel=42"}} {
		args := append([]string{"--bus", "Vehicles/Any/model.bus", "--map", "maps/Any/global.cfg", "--autostart"}, option...)
		saved := &pedepeFuelSnapshot{Bus: "Vehicles/Any/model.bus", Field: "engine_tank_content", Litres: 231}
		out, err := peDePeFuelArgs(args, saved, "")
		if err != nil {
			t.Fatal(err)
		}
		value, _ := pedepeArgValue(out, "--setvar")
		if value != "door_0=0,IBIS_Ziel=42,engine_tank_content=231" {
			t.Fatal("assignments lost", value)
		}
		if again, err := peDePeFuelArgs(out, saved, ""); err != nil || !reflect.DeepEqual(out, again) {
			t.Fatal("fuel assignment duplicated", again, err)
		}
		if _, err := peDePeFuelArgs([]string{"--bus", "Vehicles/Other/model.bus"}, saved, ""); err == nil {
			t.Fatal("fuel applied to a different bus")
		}
		if out, err := peDePeFuelArgs(args, nil, ""); err != nil || !reflect.DeepEqual(out, args) {
			t.Fatal("missing saved fuel changed arguments")
		}
	}
}
