package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestUpdateClosesItsLaunchingSetupAndWaitsForExit(t *testing.T) {
	var events []string
	open := true
	err := awaitBridgeSetupClosure(time.Second, func() ([]uint32, error) {
		if open {
			events = append(events, "still-open")
			return []uint32{12}, nil
		}
		events = append(events, "closed")
		return nil, nil
	}, func(pid uint32) error {
		if pid != 12 {
			t.Fatal(pid)
		}
		events = append(events, "normal-close")
		open = false
		return nil
	})
	if err != nil || !reflect.DeepEqual(events, []string{"still-open", "normal-close", "closed"}) {
		t.Fatal(events, err)
	}
}

func TestUpdateClosesAdditionalSetupAndRechecksProcessList(t *testing.T) {
	open := map[uint32]bool{12: true, 13: true}
	var closed []uint32
	scans := 0
	err := awaitBridgeSetupClosure(time.Second, func() ([]uint32, error) {
		scans++
		var pids []uint32
		for _, pid := range []uint32{12, 13, 14} {
			if open[pid] {
				pids = append(pids, pid)
			}
		}
		return pids, nil
	}, func(pid uint32) error {
		closed = append(closed, pid)
		delete(open, pid)
		if pid == 13 {
			open[14] = true
		} // A second snapshot must catch a newly opened Setup.
		return nil
	})
	if err != nil || scans != 3 || !reflect.DeepEqual(closed, []uint32{12, 13, 14}) {
		t.Fatal(scans, closed, err)
	}
}

func TestUpdateBusySetupBlocksWritesWithoutRepeatedCloseRequests(t *testing.T) {
	calls := 0
	err := awaitBridgeSetupClosure(time.Millisecond, func() ([]uint32, error) {
		return []uint32{12}, nil
	}, func(uint32) error { calls++; return nil })
	if !errors.Is(err, errBridgeSetupStillOpen) || calls != 1 {
		t.Fatal(calls, err)
	}
}

func TestUpdateAlreadyClosedSetupAndProcessErrors(t *testing.T) {
	if err := awaitBridgeSetupClosure(time.Second, func() ([]uint32, error) { return nil, nil }, func(uint32) error {
		t.Fatal("requested closure after the Setup exited")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := errors.New("process access denied")
	if err := awaitBridgeSetupClosure(time.Second, func() ([]uint32, error) { return nil, want }, func(uint32) error { return nil }); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := awaitBridgeSetupClosure(time.Second, func() ([]uint32, error) { return []uint32{12}, nil }, func(uint32) error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
}
