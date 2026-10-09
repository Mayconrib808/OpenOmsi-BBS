package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func hostConfigFixture(t *testing.T) hostAgentConfig {
	t.Helper()
	c, p, _, _ := companyFixture(t)
	config := defaultHostAgentConfig()
	config.Root = c.Root
	config.Server = filepath.Join(t.TempDir(), "openomsi.exe")
	for _, path := range []string{filepath.Join(config.Root, "Omsi.exe"), config.Server} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config.Company = p
	config.Company.Clock = &CompanyClock{TimeZone: "Etc/UTC"}
	config.Company.Sessions[0].Date = "company"
	config.Company.Sessions[0].Fleet = []string{"Vehicles/A/a.bus"}
	config.Company.DirectoryURL = "https://directory.example/rooms/0123456789abcdef0123456789abcdef"
	config.Maps[config.Company.Sessions[0].ID] = defaultHostMapOptions(config)
	return config
}
func TestHostMapDiscoveryPortsAndFleetAreIndependent(t *testing.T) {
	c := hostConfigFixture(t)
	maps, err := hostDiscoverMaps(c.Root)
	if err != nil || len(maps) != 1 || maps[0].Name != "Sample" {
		t.Fatal(maps, err)
	}
	for _, path := range []string{"maps/Other/global.cfg", "Vehicles/A/Vip 5 AI.bus"} {
		full := filepath.Join(c.Root, filepath.FromSlash(path))
		_ = os.MkdirAll(filepath.Dir(full), 0700)
		_ = os.WriteFile(full, []byte("[name]\nOther\n"), 0600)
	}
	maps, err = hostDiscoverMaps(c.Root)
	if err != nil || len(maps) != 2 {
		t.Fatal(maps, err)
	}
	index, err := hostAddMap(&c, hostInstalledMap{"Other", "maps/Other/global.cfg"})
	if err != nil {
		t.Fatal(err)
	}
	first := c.Maps[c.Company.Sessions[0].ID]
	second := c.Maps[c.Company.Sessions[index].ID]
	if first.Port == second.Port || first.WebPort == second.WebPort || len(c.Company.Sessions[index].Fleet) != 0 || len(c.Company.Sessions[0].Fleet) != 1 {
		t.Fatal(first, second, c.Company.Sessions)
	}
	buses, err := hostDiscoverFleet(c.Root)
	if err != nil || len(buses) != 3 {
		t.Fatal(buses, err)
	}
	if hostSuggestedBus("Vehicles/A/Vip 5 AI.bus") || !hostSuggestedBus("Vehicles/A/Vip 5 Manual.bus") || !hostSuggestedBus("Vehicles/A/Auto.bus") {
		t.Fatal("AI suggestion policy")
	}
}
func TestHostConfigRejectsPortConflictsAndNeverExportsPrivateSettings(t *testing.T) {
	c := hostConfigFixture(t)
	if err := validateHostAgentConfig(c, true); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(c.Company)
	for _, secret := range []string{c.HostKey, c.ControlKey, c.Server, c.Root} {
		if strings.Contains(string(b), secret) {
			t.Fatal("private host data in public profile")
		}
	}
	id := c.Company.Sessions[0].ID
	m := c.Maps[id]
	m.WebPort = c.ControlPort
	c.Maps[id] = m
	if err := validateHostAgentConfig(c, true); err == nil {
		t.Fatal("control port conflict accepted")
	}
}
func TestHostConcurrentPlayersStartOneMapAndIdleStopAllowsNewSession(t *testing.T) {
	c := hostConfigFixture(t)
	id := c.Company.Sessions[0].ID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 2)
	var starts atomic.Int32
	a := &hostAgent{config: c, maps: map[string]*hostAgentMap{id: {state: companyDirectoryState{SessionID: id, State: "offline"}}}, output: io.Discard}
	a.runServer = func(ctx context.Context, o companyHostOptions, _ io.Writer) error {
		starts.Add(1)
		o.onPublicReady("https://new-host.trycloudflare.com")
		o.onStatus(companyHostStatus{Players: 0}, true)
		started <- struct{}{}
		<-ctx.Done()
		return nil
	}
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	a.demand(ctx, id, 1, dir)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("map did not start")
	}
	a.demand(ctx, id, 2, dir)
	a.demand(ctx, id, 2, dir)
	if starts.Load() != 1 {
		t.Fatal("duplicate startup", starts.Load())
	}
	snapshot := a.snapshot()
	if len(snapshot.Sessions) != 1 || snapshot.Sessions[0].State != "online" {
		t.Fatal(snapshot)
	}
	a.stopIdle(time.Now().Add(16 * time.Minute))
	a.wg.Wait()
	if a.snapshot().Sessions[0].State != "offline" {
		t.Fatal(a.snapshot())
	}
	a.demand(ctx, id, 3, dir)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("map did not restart")
	}
	if starts.Load() != 2 {
		t.Fatal(starts.Load())
	}
	cancel()
	a.wg.Wait()
}
func TestHostBusyMapDoesNotStopDuringPlayerTrip(t *testing.T) {
	c := hostConfigFixture(t)
	id := c.Company.Sessions[0].ID
	cancelled := false
	a := &hostAgent{config: c, maps: map[string]*hostAgentMap{id: {state: companyDirectoryState{SessionID: id, State: "online", Players: 2}, idleSince: time.Now().Add(-time.Hour), cancel: func() { cancelled = true }}}}
	a.stopIdle(time.Now())
	if cancelled {
		t.Fatal("stopped occupied map")
	}
}

func TestHostFirstPlayerWeatherDoesNotChangeAnOccupiedWorld(t *testing.T) {
	c := hostConfigFixture(t)
	id := c.Company.Sessions[0].ID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	rain, err := bridgeWeatherFromOWT([]byte(rainyBCSWeather))
	if err != nil {
		t.Fatal(err)
	}
	sunny := strings.Replace(rain, "pt=1", "pt=0", 1)
	started := make(chan string, 1)
	a := &hostAgent{config: c, maps: map[string]*hostAgentMap{id: {state: companyDirectoryState{SessionID: id, State: "offline"}}}, output: io.Discard}
	a.runServer = func(ctx context.Context, o companyHostOptions, _ io.Writer) error {
		b, e := os.ReadFile(o.Config)
		if e != nil {
			return e
		}
		started <- string(b)
		o.onPublicReady("https://weather.trycloudflare.com")
		o.onStatus(companyHostStatus{Players: 2}, true)
		<-ctx.Done()
		return nil
	}
	a.demand(ctx, id, 1, dir, rain)
	select {
	case text := <-started:
		if !strings.Contains(text, "weather = "+rain) {
			t.Fatal(text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("host did not consume BCS weather")
	}
	a.demand(ctx, id, 2, dir, sunny)
	path := filepath.Join(companyHostDataDir(dir), "HostedMaps", c.Company.CompanyID, id, "server.cfg")
	b, e := os.ReadFile(path)
	if e != nil || !strings.Contains(string(b), "weather = "+rain) {
		t.Fatal("later player changed shared weather", e)
	}
	cancel()
	a.wg.Wait()
}
func TestHostAgentImportsExistingFleetAndWritesSeparateConfigs(t *testing.T) {
	c := hostConfigFixture(t)
	p := c.Company
	p.Sessions[0].Fleet = nil
	dir := t.TempDir()
	path := saveTestCompany(t, dir, p)
	if err := hostImportProfile(&c, path); err != nil {
		t.Fatal(err)
	}
	fleet := c.Company.Sessions[0].Fleet
	if len(fleet) != 2 {
		t.Fatal(fleet)
	}
	session := c.Company.Sessions[0]
	text := hostMapConfigText(c.Company.CompanyName, c.Maps[session.ID])
	if strings.Contains(string(text), c.HostKey) || !strings.Contains(string(text), "tunnel = 1") || !strings.Contains(string(text), "web_port = 27025") {
		t.Fatal(string(text))
	}
}

func TestHostMapCanRunWithoutAnyRegisteredBus(t *testing.T) {
	c := hostConfigFixture(t)
	c.Company.Sessions[0].Fleet = nil
	if err := validateHostAgentConfig(c, true); err != nil {
		t.Fatal("enabled map required bus authorization", err)
	}
	config := string(hostMapConfigText(c.Company.CompanyName, c.Maps[c.Company.Sessions[0].ID]))
	if !strings.Contains(config, "passengers = 1") || !strings.Contains(config, "timetable = 1") {
		t.Fatal("map simulation options changed", config)
	}
}
func TestMigratedHostAutomaticallyUsesBundledFreeBusServer(t *testing.T) {
	c := hostConfigFixture(t)
	old := c.Server
	dir := t.TempDir()
	hostUseBundledServer(&c, dir)
	if c.Server != old {
		t.Fatal("missing bundled binary replaced working path")
	}
	server := filepath.Join(dir, "app", "server", "openomsi.exe")
	if err := os.MkdirAll(filepath.Dir(server), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server, []byte("free bus server fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	hostUseBundledServer(&c, dir)
	if c.Server != server {
		t.Fatal("old restricted server was retained", c.Server)
	}
}

func TestSelectedServerSurvivesSavingAndPackageUpgrade(t *testing.T) {
	c := hostConfigFixture(t)
	selected := c.Server
	key, room, directory := c.HostKey, c.RoomID, c.Company.DirectoryURL
	mapID := c.Company.Sessions[0].ID
	mapOptions := c.Maps[mapID]
	dir := t.TempDir()
	bundled := filepath.Join(dir, "app", "server", "openomsi.exe")
	if err := os.MkdirAll(filepath.Dir(bundled), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundled, []byte("bundled fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	hostSelectServer(&c, dir, `"`+selected+`"`)
	settings := filepath.Join(t.TempDir(), "HostAgent.local.json")
	if err := saveHostAgentConfig(settings, c); err != nil {
		t.Fatal(err)
	}
	reopened, err := loadHostAgentConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	hostUseBundledServer(&reopened, dir)
	if reopened.Server != selected || !reopened.UseCustomServer {
		t.Fatal("saved server selection was replaced by the new package", reopened.Server)
	}
	if reopened.HostKey != key || reopened.RoomID != room || reopened.Company.DirectoryURL != directory || reopened.Maps[mapID] != mapOptions {
		t.Fatal("server selection changed company credentials or map settings")
	}
	// Missing custom binaries must produce a clear error, not start another server.
	if err := os.Remove(selected); err != nil {
		t.Fatal(err)
	}
	hostUseBundledServer(&reopened, dir)
	if reopened.Server != selected || validateHostAgentConfig(reopened, false) == nil {
		t.Fatal("missing selected executable silently fell back to the bundled server")
	}
}

func TestSelectingIncludedServerRestoresAutomaticPackageUpdates(t *testing.T) {
	c := hostConfigFixture(t)
	c.UseCustomServer = true
	dir := t.TempDir()
	bundled := filepath.Join(dir, "app", "server", "openomsi.exe")
	hostSelectServer(&c, dir, bundled)
	if c.UseCustomServer || c.Server != bundled {
		t.Fatal("selecting the included server did not restore automatic updates")
	}
	newDir := t.TempDir()
	newBundled := filepath.Join(newDir, "app", "server", "openomsi.exe")
	if err := os.MkdirAll(filepath.Dir(newBundled), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBundled, []byte("updated bundled fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	hostUseBundledServer(&c, newDir)
	if c.Server != newBundled {
		t.Fatal("included server did not follow the new package")
	}
}
