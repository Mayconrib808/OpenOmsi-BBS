package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testCompanyStatus() map[string]any {
	return map[string]any{"name": "Company server", "map": "maps/Sample/global.cfg", "version": "0.2.0 (test-build)", "protocol": multiplayerProtocol, "time": "09:20", "players": 1, "max_players": 16, "password": false, "vehicles": `Vehicles/A/a.bus;Vehicles/B/b.bus`}
}

func companyStatusServer(t *testing.T, status map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAutomaticCompanyJoinPreservesOwnTripAndBuildsOnlyJoinFlags(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	server := companyStatusServer(t, testCompanyStatus())
	p.Sessions[0].ServerURL = server.URL
	saveTestCompany(t, dir, p)
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
	if err != nil || len(problems) != 0 || plan == nil {
		t.Fatal(plan, problems, err)
	}
	if plan.CompanyID != c.CompanyID || plan.Trip != trip || plan.Session.ID != "morning" {
		t.Fatal(plan)
	}
	want := []string{"--lan-join", server.URL, "--lan-name", "Maycon"}
	if got := multiplayerArguments(plan); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if err := recheckMultiplayer(context.Background(), plan, companyHTTPClient()); err != nil {
		t.Fatal(err)
	}
	if got := multiplayerArguments(nil); len(got) != 0 {
		t.Fatal("single-player got network arguments")
	}
}

func TestCompanySelectsWorkingRoomWhenAnotherIsOfflineWrongMapOrWrongClock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"wrong map", func(s map[string]any) { s["map"] = "maps/Other/global.cfg" }},
		{"wrong clock", func(s map[string]any) { s["time"] = "18:00" }},
		{"full", func(s map[string]any) { s["players"] = 16 }},
		{"wrong protocol", func(s map[string]any) { s["protocol"] = 5 }},
		{"wrong game", func(s map[string]any) { s["version"] = "0.1.7" }},
		{"bus not offered", func(s map[string]any) { s["vehicles"] = "Vehicles/B/b.bus" }},
		{"undeclared remote bus", func(s map[string]any) { s["vehicles"] = "Vehicles/A/a.bus;Vehicles/B/b.bus;Vehicles/C/c.bus" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, p, trip, dir := companyFixture(t)
			bad := testCompanyStatus()
			tc.change(bad)
			first := companyStatusServer(t, bad)
			good := companyStatusServer(t, testCompanyStatus())
			p.Sessions[0].ServerURL = first.URL
			second := p.Sessions[0]
			second.ID = "second"
			second.Name = "Second"
			second.ServerURL = good.URL
			p.Sessions = append(p.Sessions, second)
			saveTestCompany(t, dir, p)
			plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
			if err != nil || len(problems) != 0 || plan == nil || plan.Session.ID != "second" {
				t.Fatal(plan, problems, err)
			}
		})
	}
	t.Run("offline", func(t *testing.T) {
		c, p, trip, dir := companyFixture(t)
		off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		off.Close()
		good := companyStatusServer(t, testCompanyStatus())
		p.Sessions[0].ServerURL = off.URL
		second := p.Sessions[0]
		second.ID = "working"
		second.ServerURL = good.URL
		p.Sessions = append(p.Sessions, second)
		saveTestCompany(t, dir, p)
		plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
		if err != nil || len(problems) != 0 || plan == nil || plan.Session.ID != "working" {
			t.Fatal(plan, problems, err)
		}
	})
}

func TestCompanyMissingBusReturnsOwnerDownloadLinkEvenIfServerOffline(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	p.Sessions[0].ServerURL = server.URL
	os.Remove(filepath.Join(c.Root, "Vehicles/B/b.bus"))
	saveTestCompany(t, dir, p)
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
	if err != nil || plan != nil || len(problems) == 0 || problems[0].DownloadURL != "https://example.invalid/bus-b" {
		t.Fatal(plan, problems, err)
	}
}

func TestCompanyIdentityDateAndOptionalModeCannotSilentlySwitchRooms(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	saveTestCompany(t, dir, p)
	c.Multiplayer = false
	if plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, nil); plan != nil || problems != nil || err != nil {
		t.Fatal(plan, problems, err)
	}
	c.Multiplayer = true
	c.CompanyID = "another-company"
	if _, _, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient()); err == nil {
		t.Fatal("changed company identity accepted")
	}
	c.CompanyID = p.CompanyID
	trip.Date = "2026-10-07"
	if plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient()); err != nil || plan != nil || len(problems) == 0 {
		t.Fatal(plan, problems, err)
	}
}

func TestCompanyDoesNotSubstituteAnotherMapWithTheSameDisplayName(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Sessions[0].ServerURL = companyStatusServer(t, testCompanyStatus()).URL
	saveTestCompany(t, dir, p)
	trip.MapFile = "maps/Other/global.cfg" // same display name; different actual map
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
	if err != nil || plan != nil || len(problems) == 0 {
		t.Fatal(plan, problems, err)
	}
}

func TestMultiplayerRecheckCatchesNewlyOfferedUndeclaredBus(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	var changed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := testCompanyStatus()
		if changed.Load() {
			s["vehicles"] = "Vehicles/A/a.bus;Vehicles/B/b.bus;Vehicles/C/c.bus"
		}
		json.NewEncoder(w).Encode(s)
	}))
	defer server.Close()
	p.Sessions[0].ServerURL = server.URL
	saveTestCompany(t, dir, p)
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, companyHTTPClient())
	if err != nil || plan == nil || len(problems) != 0 {
		t.Fatal(plan, problems, err)
	}
	changed.Store(true)
	if err := recheckMultiplayer(context.Background(), plan, companyHTTPClient()); err == nil {
		t.Fatal("new unchecked fleet accepted")
	}
}

func TestCompanyProfileCanBeReadFromAdminsHTTPSAddress(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	b, _ := json.Marshal(p)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer server.Close()
	got, err := loadCompanyProfile(context.Background(), server.URL, "", server.Client())
	if err != nil || got.CompanyID != c.CompanyID {
		t.Fatal(got, err)
	}
}

func TestServerStatusBodyIsBoundedAndDeadlineCancelsUnavailableRoom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", (128<<10)+1)) }))
	defer server.Close()
	if _, err := readCompanyServer(context.Background(), companyHTTPClient(), CompanySession{ServerURL: server.URL}); err == nil {
		t.Fatal("oversized status accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readCompanyServer(ctx, companyHTTPClient(), CompanySession{ServerURL: server.URL}); err == nil {
		t.Fatal("cancelled request accepted")
	}
}

func TestMultiplayerChildUsesVerifiedAssetsAndDisablesPeerDownloads(t *testing.T) {
	base := []string{"PATH=kept", "OMSI_PLUGIN_HOST32=helper", "omsi_content=other", "OMSI_CONTENT_ZIP=unsafe.zip", "omsi_no_lan_mods=0"}
	got := multiplayerEnvironment(base, "empty-content")
	want := []string{"PATH=kept", "OMSI_PLUGIN_HOST32=helper", "OMSI_CONTENT=empty-content", "OMSI_NO_LAN_MODS=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if base[2] != "omsi_content=other" {
		t.Fatal("mutated parent's environment")
	}
}

func TestMultiplayerGuardRequiresActualJoinedWorldWithCorrectDateAndClock(t *testing.T) {
	_, p, trip, _ := companyFixture(t)
	plan := &MultiplayerPlan{Session: p.Sessions[0], Trip: trip}
	for _, tc := range []struct {
		text          string
		ready, failed bool
	}{
		{"LAN: welcome after 0.05 s\n", false, false},
		{"LAN: taking the host's world: 2026-10-06 09:20:01.20 weather clear season summer\n", true, false},
		{"LAN: taking the host's world: 2026-10-07 09:20:01.20 weather clear season summer\n", false, true},
		{"LAN: taking the host's world: 2026-10-06 18:00:00.00 weather clear season summer\n", false, true},
		{"LAN: cannot reach 'https://server': refused\n", false, true},
		{"LAN: disconnected from the server: version mismatch\n", false, true},
		{"LAN: the session is on the host's map maps/Other/global.cfg\n", false, true},
	} {
		ok, err := checkMultiplayerLog(tc.text, plan)
		if ok != tc.ready || (err != nil) != tc.failed {
			t.Fatal(tc.text, ok, err)
		}
	}
}

func TestMultiplayerWatcherIgnoresPreviousLaunchAndPartialLogLine(t *testing.T) {
	_, p, trip, dir := companyFixture(t)
	path := filepath.Join(dir, "game.log")
	old := "LAN: taking the host's world: 2026-10-07 18:00:00 weather clear season summer\n"
	os.WriteFile(path, []byte(old), 0644)
	done := make(chan struct{})
	defer close(done)
	watch := watchMultiplayerLaunch(path, int64(len(old)), &MultiplayerPlan{Session: p.Sessions[0], Trip: trip}, done)
	partial := "LAN: taking the host's world: 2026-10-06 09:20:02.00 weather clear season summer"
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(partial)
	f.Close()
	select {
	case <-watch.Ready:
		t.Fatal("partial line confirmed")
	case err := <-watch.Errors:
		t.Fatal(err)
	case <-time.After(250 * time.Millisecond):
	}
	f, _ = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("\n")
	f.Close()
	select {
	case <-watch.Ready:
	case err := <-watch.Errors:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("joined world not confirmed")
	}
}

func TestBBSReadyFlagWaitsForMultiplayerConfirmation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bridge.log")
	flag := filepath.Join(dir, "ready.flag")
	os.WriteFile(path, []byte("auto-start finished: ready\n"), 0644)
	network := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	watchOpenOMSIReady(path, 0, flag, network, done)
	time.Sleep(250 * time.Millisecond)
	if fileExists(flag) {
		t.Fatal("BBS-ready flag published before connection confirmation")
	}
	close(network)
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("ready flag never published")
		case <-tick.C:
			if fileExists(flag) {
				return
			}
		}
	}
}
