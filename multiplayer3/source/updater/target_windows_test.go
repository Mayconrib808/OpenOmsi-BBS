//go:build windows

package main

import "testing"

func TestManualUpdaterDoesNotRequireSetupProcessID(t *testing.T) {
	if err := waitForSetup(0); err != nil { t.Fatal("double-click launch rejected",err) }
	if err := waitForSetup(-1); err == nil { t.Fatal("invalid process ID accepted") }
}
