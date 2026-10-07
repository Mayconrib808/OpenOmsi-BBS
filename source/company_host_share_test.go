package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestCompanyHostPreferencesSurvivePackageChangesAndImportLegacy(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	oldPackage, nextPackage := t.TempDir(), t.TempDir()
	options := companyHostOptions{Server: filepath.Join(t.TempDir(), "openomsi.exe"), Root: t.TempDir(), Profile: filepath.Join(t.TempDir(), "company.json"), Session: "morning"}
	legacy := filepath.Join(oldPackage, "CompanyHost.local.json")
	if err := saveCompanyHostLocalOptions(legacy, options); err != nil {
		t.Fatal(err)
	}
	previous, found := loadPreviousCompanyHostOptions(oldPackage)
	if !found || !reflect.DeepEqual(previous, options) {
		t.Fatalf("legacy options: %#v %t", previous, found)
	}
	managed := filepath.Join(companyHostDataDir(oldPackage), "CompanyHost.local.json")
	if err := saveCompanyHostLocalOptions(managed, previous); err != nil {
		t.Fatal(err)
	}
	if companyHostDataDir(oldPackage) != companyHostDataDir(nextPackage) {
		t.Fatal("preferences still depend on the extracted bridge folder")
	}
	previous, found = loadPreviousCompanyHostOptions(nextPackage)
	if !found || !reflect.DeepEqual(previous, options) {
		t.Fatal("the updated package lost the previous host paths")
	}
	text, _ := os.ReadFile(managed)
	if bytes.Contains(text, []byte("onReady")) || bytes.Contains(text, []byte("admin_password")) {
		t.Fatal("saved a runtime callback or private administration credential")
	}
	if err := os.WriteFile(managed, []byte(`{"Server":"relative.exe"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, found := loadCompanyHostLocalOptions(managed); found {
		t.Fatal("accepted incomplete/relative host paths")
	}
}

func TestCompanyHostTunnelOnlyUsesCanonicalOwnedQuickTunnelLines(t *testing.T) {
	want := "https://control-seal-vertex-newcastle.trycloudflare.com"
	for _, line := range []string{
		"  Server address for the players: " + want + "\r",
		"[INFO omsi_net::tunnel] tunnel: the session is reachable at " + want,
	} {
		if got := companyHostTunnelURL(line); got != want {
			t.Fatalf("valid quick tunnel = %q", got)
		}
	}
	for _, address := range []string{
		"http://company.trycloudflare.com", "https://company.trycloudflare.com:443",
		"https://company.trycloudflare.com/status", "https://company.trycloudflare.com/?secret=value",
		"https://user:password@company.trycloudflare.com", "https://company.trycloudflare.com/#section",
		"https://company.trycloudflare.com.evil.invalid", "https://evil.company.trycloudflare.com",
		"https://trycloudflare.com", "https://-company.trycloudflare.com", "https://company-.trycloudflare.com",
		"https://127.0.0.1", "https://localhost", "https://company.invalid", "https://company.trycloudflare.com trailing text",
	} {
		if companyHostTunnelURL("Server address for the players: "+address) != "" {
			t.Fatalf("accepted unsafe/unrelated tunnel %q", address)
		}
	}
	if companyHostTunnelURL("missing texture https://company.trycloudflare.com") != "" {
		t.Fatal("an arbitrary object/error URL became a session address")
	}
}

func TestCompanyHostTunnelOutputHandlesChunksLongLinesAndConcurrentLogs(t *testing.T) {
	var visible bytes.Buffer
	w := newCompanyHostTunnelOutput(&visible)
	first := "https://first-session.trycloudflare.com"
	for _, chunk := range []string{"Server address for the pla", "yers: https://first-session.try", "cloudflare.com\n"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if got := <-w.urls; got != first {
		t.Fatalf("split tunnel = %q", got)
	}
	_, _ = w.Write([]byte(strings.Repeat("x", 5000) + "Server address for the players: " + first + "\n"))
	select {
	case <-w.urls:
		t.Fatal("accepted an overlong log line")
	default:
	}
	var logs sync.WaitGroup
	for i := 0; i < 4; i++ {
		logs.Add(1)
		go func() {
			defer logs.Done()
			_, _ = w.Write([]byte("ordinary server log\n"))
		}()
	}
	logs.Wait()
	second := "https://new-session.trycloudflare.com"
	_, _ = w.Write([]byte("Server address for the players: " + first + "\nServer address for the players: " + second + "\n"))
	if got := <-w.urls; got != second {
		t.Fatalf("latest tunnel = %q", got)
	}
	if strings.Count(visible.String(), "ordinary server log") != 4 {
		t.Fatal("lost visible logs while capturing tunnel output")
	}
}

func TestCompanyHostPlayerExportPreservesAdminIdentityInventoryAndOtherSessions(t *testing.T) {
	_, original, _, dir := companyFixture(t)
	original.Sessions = append(original.Sessions, CompanySession{ID: "evening", Name: "Evening", MapName: "Sample", MapFile: "maps/Sample/global.cfg", Date: "2026-10-06", ServerURL: "https://other-server.invalid", RequiredPackages: []string{"map", "bus-a"}, ClockToleranceSec: 180})
	before, _ := json.Marshal(original)
	adminPath := saveTestCompany(t, dir, original)
	adminBytes, _ := os.ReadFile(adminPath)
	first := "https://first-session.trycloudflare.com"
	path := filepath.Join(t.TempDir(), "Companies", original.CompanyID+".players.json")
	if err := exportCompanyHostPlayerProfile(path, original, "morning", first); err != nil {
		t.Fatal(err)
	}
	var exported CompanyProfile
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &exported) != nil {
		t.Fatal("export is not a complete readable JSON profile")
	}
	if exported.Sessions[0].ServerURL != first || !reflect.DeepEqual(exported.Sessions[1], original.Sessions[1]) || !reflect.DeepEqual(exported.Packages, original.Packages) || exported.CompanyID != original.CompanyID || exported.OpenOMSIVersion != original.OpenOMSIVersion || exported.Protocol != original.Protocol {
		t.Fatal("player export changed profile identity, hashes, metadata or another session")
	}
	after, _ := json.Marshal(original)
	if !bytes.Equal(before, after) {
		t.Fatal("generating the shared profile modified the live administrator object")
	}
	second := "https://next-session.trycloudflare.com"
	if err := exportCompanyHostPlayerProfile(path, original, "morning", second); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !bytes.Contains(data, []byte(second)) || bytes.Contains(data, []byte(first)) {
		t.Fatal("server restart did not replace the stable player export")
	}
	if err := exportCompanyHostPlayerProfile(path, original, "missing-session", first); err == nil {
		t.Fatal("exported a nonexistent hosted session")
	}
	afterAdmin, _ := os.ReadFile(adminPath)
	if !bytes.Equal(adminBytes, afterAdmin) {
		t.Fatal("the administrator's localhost source JSON was overwritten")
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".company-host-*.tmp"))
	if len(leftovers) != 0 {
		t.Fatal("atomic export left temporary files behind")
	}
}

type companyHostTestTransport func(*http.Request) (*http.Response, error)

func (f companyHostTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCompanyHostTunnelVerifiesActiveServerBeforeSharing(t *testing.T) {
	local := hostTestStatus()
	address := "https://company-session.trycloudflare.com"
	remote := local
	client := &http.Client{Transport: companyHostTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != address+"/status" || r.Method != http.MethodGet || r.Header.Get("X-Admin-Password") != "" {
			t.Error("the public check sent an unexpected request or administration secret")
		}
		data, _ := json.Marshal(remote)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}
	if err := verifyCompanyHostTunnel(context.Background(), client, address, local, hostTestSession(), hostTestFleet()); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*companyHostStatus){
		func(s *companyHostStatus) { s.Name = "different server" },
		func(s *companyHostStatus) { s.Protocol++ },
		func(s *companyHostStatus) { s.Map = "maps/Elsewhere/global.cfg" },
		func(s *companyHostStatus) { s.World = json.RawMessage("null") },
		func(s *companyHostStatus) { s.Vehicles = json.RawMessage(`"Vehicles/Elsewhere/other.bus"`) },
	} {
		remote = local
		mutate(&remote)
		if err := verifyCompanyHostTunnel(context.Background(), client, address, local, hostTestSession(), hostTestFleet()); err == nil {
			t.Fatal("shared an address for a different or not yet active company server")
		}
	}
}

func TestCompanyHostShareDestinationCannotOverwriteAdminProfile(t *testing.T) {
	dir := t.TempDir()
	admin := filepath.Join(dir, "company.json")
	if err := os.WriteFile(admin, []byte("administrator source"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := companyHostSharePath(admin, admin, "company", dir); err == nil {
		t.Fatal("accepted the administrator profile as the export destination")
	}
	if _, err := companyHostSharePath(admin, "relative.json", "company", dir); err == nil {
		t.Fatal("accepted relative share destination")
	}
	alias := filepath.Join(dir, "alias.json")
	if err := os.Link(admin, alias); err == nil {
		if _, err := companyHostSharePath(admin, alias, "company", dir); err == nil {
			t.Fatal("accepted a hard-linked administrator profile destination")
		}
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	path, err := companyHostSharePath(admin, "", "company", dir)
	if err != nil || filepath.Base(path) != "company.players.json" || filepath.Dir(path) != filepath.Join(companyHostDataDir(dir), "Companies") {
		t.Fatalf("default export path: %q %v", path, err)
	}
}
