//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

func acquireBridgeLock(root string) (func(), error) {
	sum := sha256.Sum256([]byte(strings.ToLower(root)))
	name, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`Local\OpenOMSI_BCS_Bridge_%x`, sum[:16]))
	h, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, err
	}
	if err == syscall.Errno(183) {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, errBridgeBusy
	}
	return func() { syscall.CloseHandle(syscall.Handle(h)) }, nil
}

func showLaunchError(lang, message string) {
	title, _ := syscall.UTF16PtrFromString("OpenOMSI BCS Bridge - by " + bridgeAuthor)
	s, _ := syscall.UTF16PtrFromString(message)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(title)), 0x10)
}

func openMultiplayerDocument(path string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r, _, e := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("open requirements page: %v", e)
	}
	return nil
}

// Ask only the render windows of this launch to close, so openOMSI can run its
// normal end-session save. BCS acceptance is checked by the caller first.
func postOpenOMSIClose(pid uint32) int {
	u := syscall.NewLazyDLL("user32.dll")
	enum := u.NewProc("EnumWindows")
	owner := u.NewProc("GetWindowThreadProcessId")
	visible := u.NewProc("IsWindowVisible")
	post := u.NewProc("PostMessageW")
	count := 0
	cb := syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
		var windowPID uint32
		owner.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		v, _, _ := visible.Call(hwnd)
		if windowPID == pid && v != 0 {
			if ok, _, _ := post.Call(hwnd, 0x0010, 0, 0); ok != 0 {
				count++
			}
		}
		return 1
	})
	enum.Call(cb, 0)
	return count
}
