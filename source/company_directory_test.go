package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// A cold host already received the wake request. A temporary poll failure must
// not throw that startup away or make the player fall back to an old tunnel.
func TestDirectoryColdStartupSurvivesFailedPollAndProfileRefresh(t *testing.T) {
	_, p, _, _ := companyFixture(t)
	wakes, polls, profiles := 0, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/wake"):
			wakes++
			_ = json.NewEncoder(w).Encode(companyDirectoryReply{true, companyDirectoryState{SessionID: p.Sessions[0].ID, State: "starting"}})
		case strings.Contains(r.URL.Path, "/sessions/"):
			polls++
			if polls == 1 {
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(companyDirectoryReply{true, companyDirectoryState{SessionID: p.Sessions[0].ID, State: "online", Address: "https://fresh-host.trycloudflare.com"}})
		case strings.HasSuffix(r.URL.Path, "/profile"):
			profiles++
			if profiles == 1 {
				http.Error(w, "upstream restarted", http.StatusBadGateway)
				return
			}
			updated := p
			updated.Sessions = append([]CompanySession(nil), p.Sessions...)
			updated.Sessions[0].ServerURL = "https://fresh-host.trycloudflare.com"
			_ = json.NewEncoder(w).Encode(updated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p.DirectoryURL = server.URL + "/rooms/0123456789abcdef0123456789abcdef"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session, err := waitCompanyDirectorySession(ctx, server.Client(), p, p.Sessions[0].ID)
	if err != nil || session.ServerURL != "https://fresh-host.trycloudflare.com" || wakes != 1 || polls != 2 || profiles != 2 {
		t.Fatal(session, err, wakes, polls, profiles)
	}
}

func TestDirectoryRetriesTransportFailuresWithinBudget(t *testing.T) {
	for _, first := range []error{io.EOF, context.DeadlineExceeded} {
		calls := 0
		err := retryCompanyDirectoryRequestWithin(context.Background(), time.Second, time.Millisecond, func(context.Context) error {
			calls++
			if calls == 1 {
				return companyDirectoryTransportError(&url.Error{Op: "Get", URL: "https://example.test/rooms/private-room/profile", Err: first})
			}
			return nil
		})
		if err != nil || calls != 2 {
			t.Fatal(err, calls)
		}
	}

	last := &companyDirectoryRequestError{status: http.StatusServiceUnavailable}
	start := time.Now()
	err := retryCompanyDirectoryRequestWithin(context.Background(), 35*time.Millisecond, 5*time.Millisecond, func(context.Context) error { return last })
	if !errors.Is(err, last) || time.Since(start) > time.Second {
		t.Fatal("retries lost the actual cause or exceeded budget", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = retryCompanyDirectoryRequestWithin(ctx, time.Second, time.Hour, func(context.Context) error { cancel(); return last })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("caller cancellation was ignored", err)
	}
}

func TestDirectoryDoesNotRetryPermanentFailures(t *testing.T) {
	for _, failure := range []error{&companyDirectoryRequestError{status: 401}, &companyDirectoryRequestError{status: 403}, &companyDirectoryRequestError{status: 404}, fmt.Errorf("invalid directory JSON"), fmt.Errorf("o diretório mudou de empresa ou endereço")} {
		calls := 0
		err := retryCompanyDirectoryRequestWithin(context.Background(), time.Second, time.Millisecond, func(context.Context) error { calls++; return failure })
		if err != failure || calls != 1 {
			t.Fatal("a permanent error was retried", failure, err, calls)
		}
	}
}

func TestDirectoryNetworkDiagnosticsKeepCauseWithoutInvitation(t *testing.T) {
	err := companyDirectoryTransportError(&url.Error{Op: "Post", URL: "https://example.test/rooms/private-invitation/wake?secret=private-key", Err: io.ErrUnexpectedEOF})
	for language, label := range map[string]string{"pt": "diretório da empresa", "en": "company directory", "de": "Firmenverzeichnis"} {
		text := companyDirectoryErrorText(err, language)
		if !strings.Contains(text, label) || !strings.Contains(text, "unexpected EOF") || strings.Contains(text, "private-") || strings.Contains(text, "example.test") {
			t.Fatal(language, text)
		}
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
	requests := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	err := retryCompanyDirectoryRequest(context.Background(), func(ctx context.Context) error {
		return companyDirectoryHTTP(ctx, source.Client(), "POST", source.URL, "private-host-key", map[string]bool{"ok": true}, nil)
	})
	if err == nil || leaked || requests != 1 {
		t.Fatal("private redirect followed", err, leaked)
	}
}

func TestDirectoryProfileReadRecoversWithoutUsingStaleCachedTunnel(t *testing.T) {
	c, p, _, dir := companyFixture(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(p)
	}))
	defer server.Close()
	p.DirectoryURL = server.URL + "/rooms/0123456789abcdef0123456789abcdef"
	p.Sessions[0].ServerURL = "https://fresh-profile.trycloudflare.com"
	c.CompanyProfileSource = companyDirectoryProfileLink(p)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := loadInstalledCompanyProfile(ctx, c, dir, server.Client())
	if err != nil || requests != 2 || got.Sessions[0].ServerURL != p.Sessions[0].ServerURL {
		t.Fatal(got, err, requests)
	}
}
