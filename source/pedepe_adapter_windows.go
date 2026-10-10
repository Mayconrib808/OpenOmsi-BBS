//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func peDePeAdapterShowError(message string) {
	title, _ := syscall.UTF16PtrFromString("OpenOMSI + BBS 2.1 - PeDePe native adapter")
	text, _ := syscall.UTF16PtrFromString(message)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
