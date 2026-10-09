package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestNativeTripPreservesBCSArguments(t *testing.T) {
	args := []string{"--root", `C:\OMSI 2`, "--situation", `maps\São Paulo\laststn.osn`, "--driver", "Drivers/bbs.odr", "--keep-time", "--setvar", "engine_damage=1", "--line", "10", "--tour", "3", "--trip", "2", "--lan-join", "old", "--lan-name=old"}
	got := nativeArguments(args, "https://host.trycloudflare.com", "Maycon")
	want := append(append([]string{}, args[:len(args)-3]...), "--lan-join", "https://host.trycloudflare.com", "--lan-name", "Maycon")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BCS arguments changed: %#v", got)
	}
}
func TestUTF16SituationUsesUniversalMapIdentity(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "laststn.osn")
	var b bytes.Buffer
	b.Write([]byte{255, 254})
	for _, u := range utf16.Encode([]rune("[name]\r\nTest\r\n[map]\r\nMaps\\São Paulo\\global.cfg\r\n")) {
		_ = binary.Write(&b, binary.LittleEndian, u)
	}
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	got, e := tripMap([]string{"--situation", p}, root)
	if e != nil || got != "maps/são paulo/global.cfg" {
		t.Fatalf("%s %v", got, e)
	}
	if mapID(got) != mapID("maps/são paulo/global.cfg") {
		t.Fatal("map identity changed")
	}
}
func TestCompanyOffsetsCrossMidnightAndDST(t *testing.T) {
	cases := []struct {
		now   string
		shift int
		want  string
	}{{"2026-10-09T01:00:00Z", -480, "2026-10-08 19:00"}, {"2026-10-09T01:00:00Z", -180, "2026-10-09 00:00"}, {"2026-03-29T01:30:00Z", -180, "2026-03-29 00:30"}}
	for _, c := range cases {
		instant, _ := time.Parse(time.RFC3339, c.now)
		got, e := companyNow(CompanyClock{"Europe/Berlin", c.shift}, instant)
		if e != nil || got.Format("2006-01-02 15:04") != c.want {
			t.Fatalf("%+v: %v %v", c, got, e)
		}
	}
}
func TestForwardNativePluginKeepsGeneratedDamageAndExistingMods(t *testing.T) {
	t.Setenv("OMSI_CONTENT", "")
	dir, gameDir := t.TempDir(), t.TempDir()
	cfg := localConfig{Game: filepath.Join(gameDir, "openomsi.exe")}
	if e := os.MkdirAll(filepath.Join(dir, "plugins"), 0755); e != nil {
		t.Fatal(e)
	}
	plugin := []byte("local SCHAEDEN = { engine_damage = 1 } -- BBS_SCHAEDEN\nomsi.send(4041, 'native')\n")
	_ = os.WriteFile(filepath.Join(dir, "plugins", "bbs.lua"), plugin, 0600)
	_ = os.WriteFile(filepath.Join(gameDir, "installed-mod.txt"), []byte("keep"), 0600)
	if e := forwardPlugin(cfg, dir); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(filepath.Join(gameDir, "Plugins", "bbs.lua"))
	if e != nil || !bytes.Equal(got, plugin) {
		t.Fatalf("plugin changed: %v", e)
	}
	if b, _ := os.ReadFile(filepath.Join(gameDir, "installed-mod.txt")); string(b) != "keep" {
		t.Fatal("mod changed")
	}
	env := nativeEnvironment([]string{"OMSI_CONTENT=old", "OMSI_NO_PLUGINS=1", "CUSTOM=keep"}, gameDir, false)
	if !strings.Contains(strings.Join(env, "\n"), "CUSTOM=keep") || strings.Contains(strings.Join(env, "\n"), "OMSI_NO_PLUGINS=1") {
		t.Fatal("native plugin blocked")
	}
}
func TestReadinessUsesActiveWorldAndChecksFreeBuses(t *testing.T) {
	if activeWorld(nil) || activeWorld([]byte("null")) || !activeWorld([]byte(`{"tiles":10}`)) {
		t.Fatal("world readiness is wrong")
	}
	s := serverStatus{Map: "maps/grundorf/global.cfg", Version: "0.2.20-bbs-free2", Protocol: 6, Free: true, Vehicles: []byte(`""`)}
	if e := validStatus(s, s.Map); e != nil {
		t.Fatal(e)
	}
	s.Free = false
	if validStatus(s, s.Map) == nil {
		t.Fatal("restricted host accepted")
	}
}
