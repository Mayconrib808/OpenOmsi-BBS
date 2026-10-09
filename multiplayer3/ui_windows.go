//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

func showError(message string) {
	m, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("OpenOmsi + BBS 3.0")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(title)), 0x10)
}
func openSetup(address string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address).Start()
}
