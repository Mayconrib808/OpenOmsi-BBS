package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDedicatedServerCopySurvivesBBSImageNameSelectionAndUpdates(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "openomsi.exe")
	first := []byte("official server version one")
	if err := os.WriteFile(original, first, 0600); err != nil {
		t.Fatal(err)
	}
	copyPath, err := isolatedCompanyHostExecutable(original)
	if err != nil {
		t.Fatal(err)
	}
	// BBS uses taskkill /F /IM openomsi.exe and a fallback path suffix match.
	if strings.EqualFold(filepath.Base(copyPath), "openomsi.exe") || strings.HasSuffix(strings.ToLower(copyPath), "\\openomsi.exe") {
		t.Fatal("BBS would still select the dedicated server for termination")
	}
	if filepath.Dir(copyPath) != dir {
		t.Fatal("official runtime resources are no longer next to the executable")
	}
	for _, path := range []string{original, copyPath} {
		b, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(b, first) {
			t.Fatal("official bytes changed", path, err)
		}
	}
	reused, err := isolatedCompanyHostExecutable(original)
	if err != nil || reused != copyPath {
		t.Fatal("unchanged server copy was not reused", err)
	}
	// Repair a damaged unused copy, and preserve an older running version when
	// the administrator selects an updated official server in the same folder.
	if err := os.WriteFile(copyPath, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := isolatedCompanyHostExecutable(original); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(copyPath); !bytes.Equal(b, first) {
		t.Fatal("damaged copy was reused")
	}
	second := []byte("official server version two")
	if err := os.WriteFile(original, second, 0600); err != nil {
		t.Fatal(err)
	}
	updated, err := isolatedCompanyHostExecutable(original)
	if err != nil || updated == copyPath {
		t.Fatal("updated server replaced the old running image", err)
	}
	if b, _ := os.ReadFile(copyPath); !bytes.Equal(b, first) {
		t.Fatal("previous server version changed")
	}
	if b, _ := os.ReadFile(updated); !bytes.Equal(b, second) {
		t.Fatal("updated server copy differs from the official version")
	}
}

func TestCompanyHostCreatesMissingConfigAndPreservesExistingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.cfg")
	session, fleet := hostTestSession(), hostTestFleet()
	profile := CompanyProfile{CompanyName: "Transfort - BR"}
	civil := time.Date(2026, 10, 7, 13, 18, 20, 0, time.UTC)
	cfg, created, err := loadCompanyHostConfig(path, profile, session, fleet, civil)
	if err != nil || !created || cfg.Port != 27015 || cfg.WebPort != 27025 {
		t.Fatal(cfg, created, err)
	}
	for _, line := range []string{"name = Transfort - BR", "map = " + session.MapFile, "date = 2026-10-07", "time = 13:18:20", "real_time = 0", "vehicles = \n", "free_player_vehicles = 1", "fallback_vehicles = " + strings.Join(fleet, ";"), "admin_password = \n"} {
		if !strings.Contains(string(cfg.Text), line) {
			t.Fatal("missing initial setting", line)
		}
	}
	custom := []byte("# owner's settings\r\nport = 27115\r\nweb_port = 27125\r\nmax_players = 10\r\ntunnel = 0\r\n")
	if err := os.WriteFile(path, custom, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, created, err = loadCompanyHostConfig(path, profile, session, fleet, civil.Add(time.Hour))
	if err != nil || created || cfg.Port != 27115 || cfg.WebPort != 27125 {
		t.Fatal(cfg, created, err)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(custom, current) {
		t.Fatal("overwrote existing user settings")
	}
	if err := os.WriteFile(path, []byte("port = invalid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, created, err := loadCompanyHostConfig(path, profile, session, fleet, civil); err == nil || created {
		t.Fatal("invalid existing config replaced with defaults")
	}
	directory := t.TempDir()
	if _, created, err := loadCompanyHostConfig(directory, profile, session, fleet, civil); err == nil || created {
		t.Fatal("a directory/read error treated as a missing configuration")
	}
}

func hostTestSession() CompanySession {
	return CompanySession{ID: "carrao", Name: "Carrão", MapName: "SP_Carrao City", MapFile: "maps/Map Carrao City/global.cfg", Date: "company", RequiredPackages: []string{"map", "bus"}}
}

func hostTestFleet() []string {
	return []string{"Vehicles/Caio Apache Vip I OF 1721 4 portas by Victor/Vip 1 OF 1721 Auto.bus", "Vehicles/Caio Apache Vip I OF 1721 4 portas by Victor/Vip 1 OF 1721 Manual.bus"}
}

func hostTestStatus() companyHostStatus {
	return companyHostStatus{Name: "Transfort", Map: hostTestSession().MapFile, Version: "0.2.11", Protocol: 6, Time: "21:14", Players: 1, MaxPlayers: 16, FreePlayerVehicles: true, Vehicles: json.RawMessage(`""`), World: json.RawMessage(`{"cars":30}`)}
}

func TestCompanyHostConfigPreservesOwnerSettingsAndComments(t *testing.T) {
	original := []byte("\xef\xbb\xbf# my server\r\nmap = maps/Old/global.cfg\r\ndate = 1989-01-01\r\nreal_time = 1\r\nreal_time = 1\r\nweather = Weather/fog.owt\r\nport = 27016\r\nweb_port = 27026\r\ntunnel = 1\r\nadmin_password = oldsecret\r\n")
	copyBefore := append([]byte(nil), original...)
	cfg, err := parseCompanyHostConfig(original)
	if err != nil || cfg.WebPort != 27026 || cfg.Port != 27016 {
		t.Fatalf("config: %+v %v", cfg, err)
	}
	now := time.Date(2026, 10, 6, 21, 14, 37, 0, time.UTC)
	rendered := renderCompanyHostConfig(cfg, hostTestSession(), hostTestFleet(), now, "newsecret")
	for _, expected := range []string{"# my server\r\n", "map = maps/Map Carrao City/global.cfg\r\n", "date = 2026-10-06\r\n", "time = 21:14:37\r\n", "weather = Weather/fog.owt\r\n", "tunnel = 1\r\n", "time_speed = 1\r\n", "admin_password = newsecret\r\n"} {
		if !bytes.Contains(rendered, []byte(expected)) {
			t.Fatalf("missing %q in %s", expected, rendered)
		}
	}
	if bytes.Contains(rendered, []byte("oldsecret")) || bytes.Contains(rendered, []byte("real_time = 1")) || bytes.HasPrefix(rendered, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("unsafe configuration carried into generated host config")
	}
	if strings.Count(string(rendered), "real_time = 0") != 2 || !bytes.Equal(original, copyBefore) {
		t.Fatal("duplicate override or original preservation failed")
	}
	if !bytes.Contains(rendered, []byte("fallback_vehicles = "+strings.Join(hostTestFleet(), ";"))) || !bytes.Contains(rendered, []byte("\r\nvehicles = \r\n")) || !bytes.Contains(rendered, []byte("free_player_vehicles = 1")) {
		t.Fatal("substitute preferences became an authorization list")
	}
}

func TestCompanyHostRejectsInvalidPorts(t *testing.T) {
	for _, text := range []string{"web_port = 0", "web_port = 65536", "web_port = https://evil", "port = -1", "port = 27015\nport = broken", "web_port = 27025 # comment", "name = \x00"} {
		if _, err := parseCompanyHostConfig([]byte(text)); err == nil {
			t.Fatalf("accepted invalid config %q", text)
		}
	}
	cfg, err := parseCompanyHostConfig([]byte("# default ports\n"))
	if err != nil || cfg.WebPort != 27025 || cfg.Port != 27015 {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
}

func TestCompanyHostFleetIsRestrictedToSessionPackages(t *testing.T) {
	p := CompanyProfile{Packages: []CompanyPackage{
		{ID: "bus", Files: []CompanyFile{{Path: hostTestFleet()[1]}, {Path: hostTestFleet()[0]}}},
		{ID: "unused", Files: []CompanyFile{{Path: "Vehicles/Other/other.bus"}}},
	}}
	fleet, err := companyHostFleet(p, hostTestSession())
	if err != nil || strings.Join(fleet, "|") != strings.Join(hostTestFleet(), "|") {
		t.Fatalf("fleet: %v %v", fleet, err)
	}
	p.Packages[0].Files[0].Path = "Vehicles/Evil/path;extra.bus"
	if _, err := companyHostFleet(p, hostTestSession()); err == nil {
		t.Fatal("accepted config fleet separator in a bus filename")
	}
	if fleet, err := companyHostFleet(CompanyProfile{}, hostTestSession()); err != nil || len(fleet) != 0 {
		t.Fatal("empty optional substitutes blocked the host", fleet, err)
	}
}

func TestCompanyHostStatusRejectsWrongServerAndFleet(t *testing.T) {
	if err := validateCompanyHostStatus(hostTestStatus(), hostTestSession(), hostTestFleet()); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*companyHostStatus)
	}{
		{"protocol", func(s *companyHostStatus) { s.Protocol++ }},
		{"version", func(s *companyHostStatus) { s.Version = "not-openomsi" }},
		{"map", func(s *companyHostStatus) { s.Map = "maps/Other/global.cfg" }},
		{"clock", func(s *companyHostStatus) { s.Time = "24:00" }},
		{"legacy server without free mode", func(s *companyHostStatus) { s.FreePlayerVehicles = false }},
		{"extrabus", func(s *companyHostStatus) { s.Vehicles = json.RawMessage(`"Vehicles/Other/bus.bus"`) }},
		{"invalidplayers", func(s *companyHostStatus) { s.Players = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := hostTestStatus()
			tc.edit(&status)
			if err := validateCompanyHostStatus(status, hostTestSession(), hostTestFleet()); err == nil {
				t.Fatal("accepted incompatible local status")
			}
		})
	}
}

func TestCompanyHostUpdatedServerPreservesPrivatePlayerBusAdmission(t *testing.T) {
	status := hostTestStatus()
	status.Version = "0.2.20-bbs-free2"
	status.Vehicles = json.RawMessage(`[]`)
	if err := validateCompanyHostStatus(status, hostTestSession(), []string{"Vehicles/Private/Player.bus"}); err != nil {
		t.Fatal("updated free-bus server rejected a private player bus", err)
	}
	status.Version = "0.2.20"
	status.FreePlayerVehicles = false
	err := validateCompanyHostStatus(status, hostTestSession(), nil)
	if err == nil || !strings.Contains(err.Error(), "0.2.20") || !strings.Contains(err.Error(), "adaptação") {
		t.Fatal("unadapted raw server status did not explain the required host adaptation", err)
	}
}

func TestCompanyHostAdminIsLocalAuthenticatedAndVerified(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			_ = json.NewEncoder(w).Encode(hostTestStatus())
			return
		}
		posts++
		if r.URL.Path != "/admin" || r.Method != http.MethodPost || r.Header.Get("X-Admin-Password") != "local-secret" {
			t.Errorf("incorrect admin request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Forwarded-For") != "" {
			t.Error("unexpected proxy header")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "speed 1\nclock 76477.250\n" {
			t.Errorf("incorrect clock request %q", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client := companyHostClient()
	defer client.CloseIdleConnections()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	status, err := readCompanyHostStatus(context.Background(), client, server.URL)
	if err != nil || status.Time != "21:14" {
		t.Fatalf("status: %+v %v", status, err)
	}
	target := time.Date(2026, 10, 6, 21, 14, 37, 250000000, time.UTC)
	if err := postCompanyHostClock(context.Background(), client, server.URL, "local-secret", target); err != nil || posts != 1 {
		t.Fatalf("admin: %v, %d", err, posts)
	}
	// A 202 response alone is not considered synchronized by the monitor.
	monitor := companyHostMonitor{serverCivil: target.Add(-time.Hour), sampledAt: target, pending: true}
	if synced, err := monitor.observe(target.Add(5*time.Second), target.Add(5*time.Second), "20:14"); synced || err != nil {
		t.Fatalf("first unexecuted command should wait: %t %v", synced, err)
	}
	if _, err := monitor.observe(target.Add(10*time.Second), target.Add(10*time.Second), "20:14"); err == nil {
		t.Fatal("accepted queued clock commands that were never executed")
	}
}

func TestCompanyHostNeverRedirectsAdministrationPassword(t *testing.T) {
	reached := false
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", remote.URL+"/admin")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer local.Close()
	client := companyHostClient()
	defer client.CloseIdleConnections()
	if err := postCompanyHostClock(context.Background(), client, local.URL, "secret", time.Now()); err == nil || reached {
		t.Fatal("followed admin redirect or treated it as accepted")
	}
	if _, err := readCompanyHostStatus(context.Background(), client, local.URL); err == nil || reached {
		t.Fatal("followed status redirect")
	}
}

func TestCompanyHostAdminRejectsAuthenticationAndHTTPFailures(t *testing.T) {
	for _, code := range []int{200, 401, 403, 404, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		client := companyHostClient()
		if err := postCompanyHostClock(context.Background(), client, server.URL, "secret", time.Now()); err == nil {
			t.Errorf("accepted HTTP %d", code)
		}
		client.CloseIdleConnections()
		server.Close()
	}
}

func TestCompanyHostMonitorTracksMidnightAndCalendarBoundaries(t *testing.T) {
	for _, start := range []time.Time{
		time.Date(2026, 10, 6, 23, 59, 57, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 57, 0, time.UTC),
		time.Date(2028, 2, 28, 23, 59, 57, 0, time.UTC),
	} {
		monitor := companyHostMonitor{serverCivil: start, sampledAt: start, pending: true}
		sampled := start.Add(5 * time.Second)
		if synced, err := monitor.observe(sampled, sampled, "00:00"); !synced || err != nil {
			t.Fatalf("midnight %s: %t %v", start, synced, err)
		}
		if monitor.serverCivil.Format("2006-01-02") != sampled.Format("2006-01-02") {
			t.Fatalf("wrong unwrapped date: %s", monitor.serverCivil)
		}
	}
}

func TestCompanyHostMonitorClockChangesAndSafetyGap(t *testing.T) {
	start := time.Date(2026, 10, 25, 1, 59, 57, 0, time.UTC)
	for _, change := range []time.Duration{-time.Hour, time.Hour} {
		monitor := companyHostMonitor{serverCivil: start, sampledAt: start}
		if synced, err := monitor.observe(start.Add(5*time.Second), start.Add(5*time.Second+change), "02:00"); synced || err != nil {
			t.Fatalf("one hour civil change must allow correction: %t %v", synced, err)
		}
	}
	monitor := companyHostMonitor{serverCivil: start, sampledAt: start}
	if _, err := monitor.observe(start.Add(12*time.Hour), start.Add(12*time.Hour), "14:00"); err == nil {
		t.Fatal("accepted ambiguous twelve-hour interruption")
	}
	monitor = companyHostMonitor{serverCivil: start, sampledAt: start}
	if _, err := monitor.observe(start.Add(5*time.Second), start.Add(24*time.Hour), "02:00"); err == nil {
		t.Fatal("accepted unknown calendar jump")
	}
}

func TestCompanyHostMidnightCorrectionUsesSignedWorldSeconds(t *testing.T) {
	// Match the upstream dedicated server's handler, which adjusts a running
	// seconds accumulator and advances the original calendar by that amount.
	start := time.Date(2026, 12, 31, 23, 59, 57, 0, time.UTC)
	current := start.Add(5 * time.Second)
	target := current.Add(120 * time.Second)
	secs := func(t time.Time) float64 { return float64(t.Hour()*3600 + t.Minute()*60 + t.Second()) }
	want, have := secs(target), secs(current)
	delta := math.Mod(want-have+43200+86400, 86400) - 43200
	corrected := start.Add(5*time.Second + time.Duration(delta)*time.Second)
	if !corrected.Equal(target) || corrected.Format("2006-01-02") != "2027-01-01" {
		t.Fatalf("midnight correction changed the wrong calendar: %s", corrected)
	}
	monitor := companyHostMonitor{serverCivil: current, sampledAt: current}
	if synced, err := monitor.observe(current.Add(5*time.Second), target.Add(5*time.Second), current.Format("15:04:05")); synced || err != nil {
		t.Fatalf("must allow startup lag correction across midnight: %t %v", synced, err)
	}
	monitor.pending = true
	if synced, err := monitor.observe(current.Add(10*time.Second), target.Add(10*time.Second), target.Add(10*time.Second).Format("15:04")); !synced || err != nil {
		t.Fatalf("corrected calendar must synchronize: %t %v", synced, err)
	}
}

func TestCompanyHostEnvironmentIsolatesVerifiedContent(t *testing.T) {
	parent := []string{"PATH=untouched", "omsi_content=old", "OMSI_CONTENT_ZIP=foreign.zip", "OMSI_NO_LAN_MODS=0", "OTHER=value"}
	before := append([]string(nil), parent...)
	env := companyHostEnvironment(parent, "private-empty")
	text := strings.Join(env, "\n")
	if !strings.Contains(text, "OMSI_CONTENT=private-empty") || !strings.Contains(text, "OMSI_NO_LAN_MODS=1") || strings.Contains(text, "foreign.zip") || strings.Contains(text, "omsi_content=old") {
		t.Fatalf("unsafe server content environment: %q", text)
	}
	if strings.Join(parent, "\n") != strings.Join(before, "\n") || !strings.Contains(text, "OTHER=value") {
		t.Fatal("changed unrelated or parent environment")
	}
}

func TestCompanyHostPortPrecheckDoesNotReuseRunningServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := ensureCompanyHostPortsFree(companyHostConfig{WebPort: listener.Addr().(*net.TCPAddr).Port, Port: 27015}); err == nil {
		t.Fatal("accepted a occupied web port")
	}
}
