//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func waitForSetup(pid int) error {
	// A direct double click has no parent Setup to wait for. The selected
	// installation is still validated by apply before any file is replaced.
	if pid == 0 { return nil }
	if pid < 0 { return fmt.Errorf("invalid setup process ID") }
	h, err := syscall.OpenProcess(0x00100000, false, uint32(pid))
	if err != nil {
		if err == syscall.Errno(87) {
			return nil
		}
		return err
	}
	defer syscall.CloseHandle(h)
	result, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("WaitForSingleObject").Call(uintptr(h), 20000)
	if result != 0 {
		return fmt.Errorf("setup did not close: %v", err)
	}
	return nil
}
func showFailure(root string, err error) {
	if filepath.IsAbs(root) {
		_ = os.WriteFile(filepath.Join(root, "OpenOMSI-BBS", "logs", "update-error.txt"), []byte(err.Error()), 0600)
	}
	message, _ := syscall.UTF16PtrFromString("Atualização não concluída / Update could not finish / Update fehlgeschlagen.\n\n" + err.Error() + "\n\nDetalhes / Details: OpenOMSI-BBS/logs/update-error.txt")
	title, _ := syscall.UTF16PtrFromString("BBS+OpenOmsi Multiplayer")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
