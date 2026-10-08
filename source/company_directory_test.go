package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDirectoryProfileRefreshKeepsIdentityAndRotatesTunnel(t *testing.T) {
	c, p, _, dir := companyFixture(t)
	var response CompanyProfile
	var keySeen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keySeen = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	p.DirectoryURL = server.URL + "/rooms/0123456789abcdef0123456789abcdef"
	response = p
	response.Sessions = append([]CompanySession(nil), p.Sessions...)
	response.Sessions[0].ServerURL = "https://new-host.trycloudflare.com"
	saveTestCompany(t, dir, p)
	got, err := loadInstalledCompanyProfile(context.Background(), c, dir, server.Client())
	if err != nil || got.Sessions[0].ServerURL != response.Sessions[0].ServerURL || keySeen != "" {
		t.Fatal(got, err, keySeen)
	}
	response.CompanyID = "another-company"
	if _, err := loadInstalledCompanyProfile(context.Background(), c, dir, server.Client()); err == nil {
		t.Fatal("directory changed company identity")
	}
}
func TestDirectoryWakesMapAndUsesOnlyPublishedAddress(t *testing.T) {
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Etc/UTC"}
	p.Sessions[0].Date = "company"
	civil := time.Now().UTC()
	trip.Date = civil.Format("2006-01-02")
	trip.Start = civil.Format("15:04")
	trip.Weather, _ = bridgeWeatherFromOWT([]byte(rainyBCSWeather))
	wakeCount := 0
	var online CompanyProfile
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/profile"):
			_ = json.NewEncoder(w).Encode(online)
		case strings.HasSuffix(r.URL.Path, "/wake"):
			var d companyDirectoryDemand
			_ = json.NewDecoder(r.Body).Decode(&d)
			if d.SessionID != p.Sessions[0].ID || d.Weather != trip.Weather {
				t.Error(d)
			}
			wakeCount++
			_ = json.NewEncoder(w).Encode(companyDirectoryReply{true, companyDirectoryState{SessionID: d.SessionID, State: "online", Address: online.Sessions[0].ServerURL}})
		case r.URL.Path == "/status":
			_ = json.NewEncoder(w).Encode(companyServerStatus{Name: "Transfort", Map: p.Sessions[0].MapFile, Version: "0.2.11", Protocol: 6, Time: time.Now().UTC().Format("15:04:05"), MaxPlayers: 16, Vehicles: json.RawMessage(`["Vehicles/A/a.bus"]`), World: json.RawMessage(`{"cars":10}`)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p.DirectoryURL = server.URL + "/rooms/0123456789abcdef0123456789abcdef"
	online = p
	online.Sessions = append([]CompanySession(nil), p.Sessions...)
	online.Sessions[0].ServerURL = server.URL
	online.Sessions[0].Fleet = []string{"Vehicles/A/a.bus"}
	saveTestCompany(t, dir, p)
	plan, problems, err := prepareMultiplayer(context.Background(), c, dir, trip, server.Client())
	if err != nil || len(problems) > 0 || plan == nil || plan.Session.ServerURL != server.URL || wakeCount != 1 {
		t.Fatal(plan, problems, err, wakeCount)
	}
}
func TestDirectoryOfflineAndDisabledHostReturnActionableError(t *testing.T) {
	_, p, _, _ := companyFixture(t)
	for _, state := range []string{"offline", "disabled", "error"} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(companyDirectoryReply{HostOnline: state != "offline", Session: companyDirectoryState{SessionID: p.Sessions[0].ID, State: state, Error: "server failed to load"}})
			}))
			defer server.Close()
			p.DirectoryURL = server.URL + "/rooms/0123456789abcdef0123456789abcdef"
			start := time.Now()
			if _, err := waitCompanyDirectorySession(context.Background(), server.Client(), p, p.Sessions[0].ID); err == nil || time.Since(start) > time.Second {
				t.Fatal(err)
			}
		})
	}
}
func TestDirectoryDoesNotRedirectPrivateHostKey(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	if err := companyDirectoryHTTP(context.Background(), source.Client(), "POST", source.URL, "private-host-key", map[string]bool{"ok": true}, nil); err == nil || leaked {
		t.Fatal("private redirect followed", err, leaked)
	}
}
