package main

import (
	"testing"
	"time"
)

func TestIndependentHostKeepsPeersAndStopsOnlyAfterIdleGrace(t *testing.T) {
	start := time.Now()
	idle := hostIdle{Started: start}
	if idle.shouldStop(start.Add(5*time.Minute), 0) {
		t.Fatal("slow first game launch stopped its host")
	}
	if idle.shouldStop(start.Add(5*time.Minute), 2) {
		t.Fatal("active host stopped")
	}
	if idle.shouldStop(start.Add(6*time.Minute), 1) {
		t.Fatal("host player's exit stopped remaining peer")
	}
	if idle.shouldStop(start.Add(6*time.Minute+30*time.Second), 0) {
		t.Fatal("idle grace skipped")
	}
	if !idle.shouldStop(start.Add(8*time.Minute), 0) {
		t.Fatal("empty host was not stopped")
	}
}
func TestHostHandshakeAcceptsOnlyItsLocalListener(t *testing.T) {
	if !localHostAddress("http://127.0.0.1:12345") {
		t.Fatal("local host rejected")
	}
	for _, raw := range []string{"https://foreign.example", "http://127.0.0.1:0", "http://user@127.0.0.1:12345", "http://127.0.0.1:12345/?key=secret"} {
		if localHostAddress(raw) {
			t.Fatal("invalid host listener accepted", raw)
		}
	}
}

func TestLargeMapStartupDoesNotTimeoutAfterFifteenMinutes(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	idle := hostIdle{Started: start}
	if hostStartupTimeout <= 25*time.Minute {
		t.Fatal("startup timeout must accommodate 25-minute maps")
	}
	if idle.shouldStop(start.Add(25*time.Minute), 0) {
		t.Fatal("large map was stopped while still loading")
	}
	if !idle.shouldStop(start.Add(hostStartupTimeout+time.Second), 0) {
		t.Fatal("stalled server was never stopped")
	}
}

func TestLargeMapReuseButMemoryEventuallyReleased(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	ready := start.Add(25*time.Minute)
	idle := hostIdle{Started: start, ReadyAt: ready}
	if idle.shouldStop(ready, 1) {
		t.Fatal("active player must keep loaded world")
	}
	if idle.shouldStop(ready.Add(5*time.Minute), 0) {
		t.Fatal("long-loaded map should remain available for the next BCS trip")
	}
	if !idle.shouldStop(ready.Add(hostHeavyMapIdle+time.Second), 0) {
		t.Fatal("empty heavy server must eventually release memory")
	}
}

func TestSmallMapIsNotKeptInMemoryForTenMinutes(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	ready := start.Add(30*time.Second)
	idle := hostIdle{Started: start, ReadyAt: ready}
	idle.shouldStop(ready, 1)
	if idle.shouldStop(ready.Add(30*time.Second), 0) {
		t.Fatal("short map lost its existing grace period")
	}
	if !idle.shouldStop(ready.Add(2*time.Minute), 0) {
		t.Fatal("small empty server should not needlessly hold RAM")
	}
}

func TestUnjoinedLoadedMapExpiresAfterShortGrace(t *testing.T) {
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	ready := start.Add(25*time.Minute)
	idle := hostIdle{Started: start, ReadyAt: ready}
	if idle.shouldStop(ready.Add(time.Minute), 0) {
		t.Fatal("players must have time to join a ready map")
	}
	if !idle.shouldStop(ready.Add(hostNeverJoinedIdle+time.Second), 0) {
		t.Fatal("unjoined loaded map must release RAM")
	}
}
