package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Uses the downloaded official executable unchanged. All content below is a
// synthetic map/bus fixture, with no commercial OMSI assets or real BCS session.
func TestCompanyGatewayOfficialRuntime(t *testing.T) {
	exe := os.Getenv("OPENOMSI_GATEWAY_TEST_SERVER")
	if exe == "" {
		t.Skip("set OPENOMSI_GATEWAY_TEST_SERVER to test an official server binary")
	}
	if _, err := checkOpenOMSICompatibility(exe, true, false); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, folder := range []string{"maps/Test", "maps/Grundorf", "maps/Berlin-Spandau", "Vehicles/Test", "Vehicles/MAN_SD200", "Vehicles/MAN_SD202", "Vehicles/MAN_NL_NG", "Sceneryobjects", "Splines", "Texture", "Fonts", "Humans", "Weather", "Inputs"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		"Omsi.exe": []byte("synthetic fixture"), "envir.cfg": {},
		"maps/Grundorf/global.cfg":       []byte("[name]\nSynthetic fixture\n"),
		"maps/Berlin-Spandau/global.cfg": []byte("[name]\nSynthetic fixture\n"),
		"maps/Test/global.cfg":           []byte("[name]\nBBS test\n[map]\n0\n0\ntile_0_0.map\n"),
		"maps/Test/tile_0_0.map":         []byte("[version]\n14\n"),
		"maps/Test/tile_0_0.map.terrain": make([]byte, 3721*4),
		"Vehicles/Test/standin.bus":      []byte("[friendlyname]\nBBS\nStand-in\nTest\n[mass]\n12000\n[boundingbox]\n2.5\n12\n3\n0\n0\n1.5\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := udp.LocalAddr().(*net.UDPAddr).Port
	udp.Close()
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	webPort := tcp.Addr().(*net.TCPAddr).Port
	tcp.Close()
	dir := t.TempDir()
	session := CompanySession{ID: "test-map", Name: "BBS test", MapName: "BBS test", MapFile: "maps/Test/global.cfg", Date: "company", ServerURL: fmt.Sprintf("http://127.0.0.1:%d", webPort), ClockToleranceSec: 180}
	profile := CompanyProfile{SchemaVersion: 1, CompanyID: "runtime-test", CompanyName: "Runtime test", OpenOMSIVersion: multiplayerGameVersion, Protocol: 6, Clock: &CompanyClock{TimeZone: "Etc/UTC"}, Sessions: []CompanySession{session}}
	mapFiles, err := snapshotCompanyFolder(root, "maps/Test")
	if err != nil {
		t.Fatal(err)
	}
	profile.Packages = []CompanyPackage{{ID: "test-content", Name: "Synthetic map", Version: "1.0", DownloadURL: "https://example.invalid/test", Folders: []string{"maps/Test"}, Files: mapFiles}}
	profile.Sessions[0].RequiredPackages = []string{"test-content"}
	profilePath := saveTestCompany(t, dir, profile)
	configPath := filepath.Join(dir, "server.cfg")
	config := fmt.Sprintf("name = Runtime test\nport = %d\nweb_port = %d\ntraffic = 0\ntimetable = 0\npassengers = 0\ntunnel = 0\n", port, webPort)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "runtime.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	t.Setenv("RUST_LOG", "info")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	ready := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- runCompanyHost(ctx, companyHostOptions{Profile: profilePath, Session: session.ID, Server: exe, Root: root, Config: configPath, Share: filepath.Join(dir, "players.json"), onReady: func() { close(ready) }}, log)
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(6 * time.Second):
			t.Error("owned official server did not stop")
		}
	}()
	select {
	case <-ready:
	case err := <-finished:
		cancel()
		finished <- err
		data, _ := os.ReadFile(logPath)
		t.Fatal("official host failed", err, string(data))
	case <-ctx.Done():
		data, _ := os.ReadFile(logPath)
		t.Fatal("official host did not become ready", string(data))
	}
	resp, err := http.Get(session.ServerURL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	var status companyHostStatus
	err = json.NewDecoder(resp.Body).Decode(&status)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || !status.FreePlayerVehicles || string(status.Vehicles) != "[]" || status.Protocol != 6 || !companyActiveWorld(status.World) {
		t.Fatal("official server not freely joinable", status, err)
	}
	clients := make([]*net.UDPConn, 2)
	ids := make([]uint32, 2)
	buses := []string{"Vehicles/PrivateAlice/private.bus", "Vehicles/PrivateBob/another.bus"}
	readUntil := func(c *net.UDPConn, prefix string) []byte {
		_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
		buffer := make([]byte, companyGatewayPacketLimit+1)
		for {
			n, err := c.Read(buffer)
			if err != nil {
				t.Fatal("official datagram not received", prefix, err)
			}
			text := string(buffer[:n])
			if strings.HasPrefix(text, "REJECT|") || strings.HasPrefix(text, "BYE|") {
				t.Fatal("official server rejected a private bus", text)
			}
			if strings.HasPrefix(text, prefix) {
				return append([]byte(nil), buffer[:n]...)
			}
		}
	}
	for i := range clients {
		c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = c
		defer c.Close()
		hello := fmt.Sprintf("HELLO|6|-|Driver%d|%s|%s|2026-10-09|13:15:00||0|nonce%d", i, buses[i], session.MapFile, i)
		_, _ = c.Write([]byte(hello))
		fields := strings.Split(string(readUntil(c, "WELCOME|")), "|")
		ids[i] = companyGatewayID(fields[2])
		if ids[i] == 0 {
			t.Fatal(fields)
		}
		info := fmt.Sprintf("INFO|%d|Driver%d|%s||810|Centro|12|2.5|0|00000000|810/Tour||", ids[i], i, buses[i])
		_, _ = c.Write([]byte(info))
		state := make([]byte, 128)
		state[0], state[1] = 0xb3, 6
		binary.LittleEndian.PutUint16(state[2:4], uint16(ids[i]))
		state[4], state[6] = 1, 1
		_, _ = c.Write(state)
	}
	info := readUntil(clients[0], fmt.Sprintf("INFO|%d|", ids[1]))
	if !strings.Contains(string(info), "|"+buses[1]+"|") {
		t.Fatal("peer did not receive original private filename", string(info))
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), "showing a stand-in") {
			t.Log("official binary loaded the map, accepted two absent private buses, kept their real filenames for peers, and used its built-in stand-in")
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("official stand-in was not instantiated", string(data))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if data, err := os.ReadFile(configPath); err != nil || string(data) != config {
		t.Fatal("user config changed", err, string(data))
	}
}
