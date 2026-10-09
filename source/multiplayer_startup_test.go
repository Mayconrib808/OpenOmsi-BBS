package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise the production launch entry point, including profile refresh and
// the directory's starting -> online transition. The former 12-second context
// would interrupt this before the server announced its public address.
func coldMultiplayerFixture(t *testing.T, loadTime time.Duration) (Config, MultiplayerTrip, string, *httptest.Server) {
	t.Helper()
	c, p, trip, dir := companyFixture(t)
	p.Clock = &CompanyClock{TimeZone: "Etc/UTC"}
	p.Sessions[0].Date = "company"
	trip.Date = time.Now().UTC().Format("2006-01-02")
	trip.Start = time.Now().UTC().Format("15:04")
	var readyAt atomic.Int64
	var address string
	var online CompanyProfile
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/wake"):
			var demand companyDirectoryDemand
			if err := json.NewDecoder(r.Body).Decode(&demand); err != nil || demand.SessionID != p.Sessions[0].ID {
				t.Error("unexpected wake request", demand, err)
				http.Error(w, "invalid wake", http.StatusBadRequest)
				return
			}
			readyAt.CompareAndSwap(0, time.Now().Add(loadTime).UnixNano())
			fallthrough
		case strings.Contains(r.URL.Path, "/sessions/"):
			state := companyDirectoryState{SessionID: p.Sessions[0].ID, State: "starting"}
			if at := readyAt.Load(); at != 0 && time.Now().UnixNano() >= at {
				state.State, state.Address = "online", address
			}
			_ = json.NewEncoder(w).Encode(companyDirectoryReply{HostOnline: true, Session: state})
		case strings.HasSuffix(r.URL.Path, "/profile"):
			_ = json.NewEncoder(w).Encode(online)
		case r.URL.Path == "/status":
			_ = json.NewEncoder(w).Encode(companyServerStatus{Name: "Transfort", Map: p.Sessions[0].MapFile, Version: "0.2.20", Protocol: 6, Time: time.Now().UTC().Format("15:04:05"), MaxPlayers: 16, Vehicles: json.RawMessage(`[]`), World: json.RawMessage(`{"cars":10}`)})
		default:
			http.NotFound(w, r)
		}
	}))
	address = server.URL
	p.DirectoryURL = address + "/rooms/0123456789abcdef0123456789abcdef"
	online = p
	online.Sessions = append([]CompanySession(nil), p.Sessions...)
	online.Sessions[0].ServerURL = address
	saveTestCompany(t, dir, p)
	return c, trip, dir, server
}

func TestMultiplayerLaunchWaitsPastTwelveSecondsForColdServer(t *testing.T) {
	c, trip, dir, server := coldMultiplayerFixture(t, 13*time.Second)
	defer server.Close()
	started := time.Now()
	plan, problems, err := prepareMultiplayerLaunch(context.Background(), c, dir, trip, server.Client())
	if err != nil || len(problems) != 0 || plan == nil || plan.Session.ServerURL != server.URL {
		t.Fatal("cold server launch refused", plan, problems, err)
	}
	if time.Since(started) < 12*time.Second {
		t.Fatal("regression fixture did not cross the former 12-second deadline")
	}
}

func TestMultiplayerLaunchRespectsParentCancellation(t *testing.T) {
	c, trip, dir, server := coldMultiplayerFixture(t, time.Minute)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	plan, _, err := prepareMultiplayerLaunch(ctx, c, dir, trip, server.Client())
	if err == nil || plan != nil || time.Since(started) > time.Second {
		t.Fatal("canceled launch continued waiting", plan, err)
	}
}

func TestMultiplayerLaunchDoesNotDelayAnOnlineServer(t *testing.T) {
	c, trip, dir, server := coldMultiplayerFixture(t, 0)
	defer server.Close()
	started := time.Now()
	plan, problems, err := prepareMultiplayerLaunch(context.Background(), c, dir, trip, server.Client())
	if err != nil || len(problems) != 0 || plan == nil || time.Since(started) > 2*time.Second {
		t.Fatal("online server was unnecessarily delayed", plan, problems, err)
	}
}
