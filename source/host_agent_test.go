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
