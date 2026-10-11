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
		if e == syscall.Errno(87) {
			return nil
		}
		return fmt.Errorf("não foi possível aguardar o Setup: %v", e)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	if !samePath(processExecutableForUpdate(pid), filepath.Join(dir, "Setup.exe")) {
		return fmt.Errorf("o processo informado não é o Setup desta instalação")
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
