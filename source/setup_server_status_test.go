package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type setupStatusTransport func(*http.Request) (*http.Response, error)

func (f setupStatusTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSetupListsEveryOnlineMapAndCopiesOnlyTheCurrentJoinAddress(t *testing.T) {
	states := map[string]companyDirectoryState{
		"map-a": {SessionID: "map-a", State: "online", Address: "https://carrao.trycloudflare.com", Players: 3},
		"map-b": {SessionID: "map-b", State: "offline", Address: "https://expired.trycloudflare.com"},
		"map-c": {SessionID: "map-c", State: "online", Address: "https://szczecin.trycloudflare.com", Players: 2},
		"map-d": {SessionID: "map-d", State: "online", Address: "https://admin:secret@example.com"},
	}
	calls := 0
	client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.Header.Get("Authorization") != "" {
			t.Fatal("status changed a server or leaked authentication")
		}
		id := filepath.Base(r.URL.Path)
		data, _ := json.Marshal(companyDirectoryReply{HostOnline: true, Session: states[id]})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})}
	ids := []string{"map-a", "map-b", "map-c", "map-d", "map-a"}
	got := nameSetupServers(checkSetupDirectoryStatus(context.Background(), client, "https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef", ids), CompanyProfile{Sessions: []CompanySession{{ID: "map-a", Name: "Carrão City"}, {ID: "map-c", Name: "Fikcyjny Szczecin"}}})
	if !got.Ready || got.Key != "server_online" || calls != 4 || len(got.Maps) != 2 {
		t.Fatal(got, calls)
	}
	if got.Maps[0].Name != "Carrão City" || got.Maps[0].Address != states["map-a"].Address || got.Maps[1].Name != "Fikcyjny Szczecin" {
		t.Fatal(got.Maps)
	}
	states["map-a"] = companyDirectoryState{SessionID: "map-a", State: "offline"}
	states["map-c"] = companyDirectoryState{SessionID: "map-c", State: "starting"}
	got = checkSetupDirectoryStatus(context.Background(), client, "https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef", ids[:3])
	if len(got.Maps) != 0 || got.Key != "server_starting" {
		t.Fatal("stale copy link survived a stopped server", got)
	}
}

func TestSetupServerReadinessRequiresLocalAgentAndFreshDirectory(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		running                    bool
		directoryError, localState string
		online                     bool
		remoteState, want          string
		ready                      bool
	}{
		{"standby", true, "", "offline", true, "offline", "server_standby", true},
		{"online", true, "", "online", true, "online", "server_online", true},
		{"starting", true, "", "starting", true, "starting", "server_starting", true},
		{"local stopped", false, "", "offline", true, "offline", "server_agent_offline", false},
		{"broken heartbeat", true, "network error", "offline", true, "offline", "server_unavailable", false},
		{"expired heartbeat", true, "", "offline", false, "offline", "server_agent_offline", false},
		{"disabled map", true, "", "disabled", true, "offline", "server_no_maps", false},
		{"failed map", true, "", "error", true, "offline", "server_no_maps", false},
		{"directory disabled", true, "", "offline", true, "disabled", "server_no_maps", false},
		{"unexpected state", true, "", "offline", true, "unknown", "server_unavailable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", t.TempDir())
			dir := t.TempDir()
			path := setupHostSettingsPath(dir)
			os.MkdirAll(filepath.Dir(path), 0700)
			original := []byte(`{"control_port":27199,"control_key":"0123456789abcdef0123456789abcdef","company":{"directory_url":"https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef"},"maps":{"map-a":{"voice":true,"voice_radio":true,"admin_password":"preserve-me"}}}`)
			os.WriteFile(path, original, 0600)
			calls := 0
			client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || strings.Contains(r.URL.Path, "wake") {
					t.Fatal("status attempted to start or change the server")
				}
				var body any
				if r.URL.Host == "127.0.0.1:27199" {
					if r.Header.Get("Authorization") != "Bearer 0123456789abcdef0123456789abcdef" {
						t.Fatal("missing local authentication")
					}
					body = setupAgentStatus{Running: tc.running, DirectoryError: tc.directoryError, Sessions: []companyDirectoryState{{SessionID: "map-a", State: tc.localState}}}
				} else {
					if r.Header.Get("Authorization") != "" || r.URL.Path != "/rooms/0123456789abcdef0123456789abcdef/sessions/map-a" {
						t.Fatal("credentials leaked or wrong session")
					}
					body = companyDirectoryReply{HostOnline: tc.online, Session: companyDirectoryState{SessionID: "map-a", State: tc.remoteState}}
				}
				b, _ := json.Marshal(body)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header)}, nil
			})}
			got := checkSetupServerStatus(context.Background(), client, dir, Config{})
			if got.Ready != tc.ready || got.Key != tc.want {
				t.Fatal(got)
			}
			if (!tc.running || tc.directoryError != "" || tc.localState == "disabled" || tc.localState == "error") && calls != 1 {
				t.Fatal("offline local host trusted cached remote state")
			}
			after, _ := os.ReadFile(path)
			if string(original) != string(after) {
				t.Fatal("read-only status modified P15 preferences")
			}
		})
	}
}

func TestSetupServerPollCannotWakeRedirectOrStayGreenAfterFailure(t *testing.T) {
	directory := "https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef"
	for _, code := range []int{302, 401, 404, 503} {
		client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "directory.workers.dev" {
				t.Fatal("followed a redirect")
			}
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{"Location": []string{"https://other.workers.dev"}}}, nil
		})}
		if got := checkSetupDirectoryStatus(context.Background(), client, directory, []string{"map-a"}); got.Ready || got.Key != "server_unavailable" {
			t.Fatal(code, got)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	start := time.Now()
	if got := checkSetupDirectoryStatus(ctx, client, directory, []string{"map-a"}); got.Ready {
		t.Fatal("cancelled check is green")
	}
	if time.Since(start) > time.Second {
		t.Fatal("status blocked GUI beyond timeout")
	}
	if setupServerPollInterval != 5*time.Second || setupServerCheckTimeout >= setupServerPollInterval {
		t.Fatal("checks can overlap")
	}
}

func TestSetupServerPlayerUsesSavedProfileAndSkipsDisabledMaps(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "company.json")
	os.WriteFile(path, []byte(`{"company_id":"demo","directory_url":"https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef","sessions":[{"id":"map-a"},{"id":"map-b"}]}`), 0600)
	calls := 0
	client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		state := "disabled"
		if calls == 2 {
			state = "offline"
		}
		id := filepath.Base(r.URL.Path)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"host_online":true,"session":{"session_id":%q,"state":%q}}`, id, state))), Header: make(http.Header)}, nil
	})}
	if got := checkSetupServerStatus(context.Background(), client, dir, Config{CompanyID: "demo", CompanyProfile: path}); !got.Ready || got.Key != "server_standby" || calls != 2 {
		t.Fatal(got, calls)
	}
	if got := checkSetupServerStatus(context.Background(), client, dir, Config{CompanyID: "other", CompanyProfile: path}); got.Ready || got.Key != "server_unconfigured" {
		t.Fatal("used another company", got)
	}
}


func TestSetupClientWithStoppedLocalAgentCanSeeRemoteCompany(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	dir := t.TempDir()
	localPath := setupHostSettingsPath(dir)
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil { t.Fatal(err) }
	local := []byte(`{"control_port":27199,"control_key":"0123456789abcdef0123456789abcdef"}`)
	if err := os.WriteFile(localPath, local, 0600); err != nil { t.Fatal(err) }
	profile := filepath.Join(dir, "company.json")
	if err := os.WriteFile(profile, []byte(`{"company_id":"demo","directory_url":"https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef","sessions":[{"id":"map-a","name":"Carrão City"}]}`), 0600); err != nil { t.Fatal(err) }
	localCalls, remoteCalls := 0, 0
	client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" { t.Fatal("read-only status changed settings") }
		var body string
		if r.URL.Host == "127.0.0.1:27199" {
			localCalls++
			if r.Header.Get("Authorization") != "Bearer 0123456789abcdef0123456789abcdef" { t.Fatal("local auth missing") }
			body = `{"running":false}`
		} else {
			remoteCalls++
			if r.Header.Get("Authorization") != "" { t.Fatal("leaked local key to directory") }
			body = `{"host_online":true,"session":{"session_id":"map-a","state":"online","address":"https://carrao.trycloudflare.com"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	got := checkSetupServerStatus(context.Background(), client, dir, Config{CompanyID:"demo",CompanyProfile:profile})
	if !got.Ready || got.Key != "server_online" || len(got.Maps) != 1 || localCalls != 1 || remoteCalls != 1 {
		t.Fatal("remote company incorrectly shown offline on player PC",got,localCalls,remoteCalls)
	}
	b,_ := os.ReadFile(localPath)
	if string(b) != string(local) { t.Fatal("local host settings changed") }
}

func TestSetupMixedMapStatesDoNotHideAnOnlineSession(t *testing.T) {
	directory := "https://directory.workers.dev/rooms/0123456789abcdef0123456789abcdef"
	client := &http.Client{Transport: setupStatusTransport(func(r *http.Request) (*http.Response, error) {
		id := filepath.Base(r.URL.Path)
		body := fmt.Sprintf(`{"host_online":false,"session":{"session_id":%q,"state":"offline"}}`,id)
		if id == "map-b" {
			body = fmt.Sprintf(`{"host_online":true,"session":{"session_id":%q,"state":"online","address":"https://map-b.trycloudflare.com"}}`,id)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	got := checkSetupDirectoryStatus(context.Background(),client,directory,[]string{"map-a","map-b"})
	if !got.Ready || got.Key != "server_online" || len(got.Maps)!=1 || got.Maps[0].ID!="map-b" {
		t.Fatal("offline map suppressed online map",got)
	}
}
