package main

import (
	"errors"
	"time"
)

var errBridgeSetupStillOpen = errors.New("the Bridge Setup has not finished closing")
var errBridgeSetupCloseFailed = errors.New("could not request the Bridge Setup to close")

// The Setup that starts an update is expected to be open. Ask matching Setup
// windows to close normally, then poll fresh process snapshots before allowing
// any replacement. Never terminate a process or exempt a still-running PID.
func awaitBridgeSetupClosure(timeout time.Duration, running func() ([]uint32, error), requestClose func(uint32) error) error {
	deadline := time.Now().Add(timeout)
	notified := map[uint32]bool{}
	for {
		pids, err := running()
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		for _, pid := range pids {
			if !notified[pid] {
				if err := requestClose(pid); err != nil {
					return err
				}
				notified[pid] = true
			}
		}
		left := time.Until(deadline)
		if left <= 0 {
			return errBridgeSetupStillOpen
		}
		if left > 100*time.Millisecond {
			left = 100 * time.Millisecond
		}
		time.Sleep(left)
	}
}
