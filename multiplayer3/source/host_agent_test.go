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
