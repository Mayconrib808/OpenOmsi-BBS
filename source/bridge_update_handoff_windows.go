//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var updateSetupUser32 = syscall.NewLazyDLL("user32.dll")

// A PID supplied by an older Setup is a hint, not the identity of the target.
// Fresh file-identity scans also handle a stale PID, another instance and the
// manual updater. Only this installation's actual Setup windows are closed.
func waitForSetupUpdate(_ uint32, dir string) error {
	return awaitBridgeSetupClosure(45*time.Second, func() ([]uint32, error) {
		return bridgeSetupsRunningForUpdate(dir)
	}, func(pid uint32) error {
		if !sameExecutableForUpdate(processExecutableForUpdate(pid), filepath.Join(dir, "Setup.exe")) {
			return nil
		}
		return requestBridgeSetupClose(pid)
	})
}

func requestBridgeSetupClose(pid uint32) error {
	var closeErr error
	callback := syscall.NewCallback(func(window, _ uintptr) uintptr {
		var owner uint32
		updateSetupUser32.NewProc("GetWindowThreadProcessId").Call(window, uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return 1
		}
		var class [128]uint16
		n, _, _ := updateSetupUser32.NewProc("GetClassNameW").Call(window, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if n == 0 || syscall.UTF16ToString(class[:n]) != "OpenOmsiBBSSetup" {
			return 1
		}
		ok, _, err := updateSetupUser32.NewProc("PostMessageW").Call(window, 0x0010, 0, 0) // WM_CLOSE, normal UI shutdown.
		if ok == 0 && err != syscall.Errno(1400) {
			closeErr = err
		} // A window may have just closed.
		return 1
	})
	if ok, _, err := updateSetupUser32.NewProc("EnumWindows").Call(callback, 0); ok == 0 {
		return fmt.Errorf("%w: %w", errBridgeSetupCloseFailed, err)
	}
	return closeErr
}
