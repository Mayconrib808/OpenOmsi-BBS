//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// Wait for the specific Setup handle, rather than killing it or guessing a
// delay. No game or server process is terminated by the updater.
func waitForSetupUpdate(pid uint32, dir string) error {
	if pid == 0 {
		return nil
	}
	h, _, e := setupKernel.NewProc("OpenProcess").Call(0x00100000|0x1000, 0, uintptr(pid))
	if h == 0 {
		// The Setup can finish between launching the updater and OpenProcess.
		// In that case there is nothing left to wait for.
		if e == syscall.Errno(87) {
			return nil
		}
		return fmt.Errorf("não foi possível aguardar o Setup: %v", e)
	}
	defer syscall.CloseHandle(syscall.Handle(h))

	expected := filepath.Join(dir, "Setup.exe")
	actual := processExecutableForUpdate(pid)
	if !sameExecutableForUpdate(actual, expected) {
		// --wait-setup is only a handoff hint from the Setup that launched this
		// updater. The process may already have exited and its PID may even have
		// been recycled before this process queries it. Do not block the update
		// solely on that stale hint: bridge_update_main performs a fresh scan of
		// the real target Setup after this function and refuses all writes while
		// that executable is still running.
		return nil
	}

	result, err := syscall.WaitForSingleObject(syscall.Handle(h), 45000)
	if err != nil {
		return err
	}
	if result != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("feche o Setup antes de atualizar")
	}
	return nil
}
