//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func registryText(hive uintptr, path, name string, view uint32) string {
	api := syscall.NewLazyDLL("advapi32.dll")
	p, _ := syscall.UTF16PtrFromString(path)
	n, _ := syscall.UTF16PtrFromString(name)
	var key uintptr
	status, _, _ := api.NewProc("RegOpenKeyExW").Call(hive, uintptr(unsafe.Pointer(p)), 0, uintptr(1|view), uintptr(unsafe.Pointer(&key)))
	if status != 0 {
		return ""
	}
	defer api.NewProc("RegCloseKey").Call(key)
	var size, kind uint32
	query := api.NewProc("RegQueryValueExW")
	status, _, _ = query.Call(key, uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(&kind)), 0, uintptr(unsafe.Pointer(&size)))
	if status != 0 || kind != 1 || size > 64<<10 || size < 2 {
		return ""
	}
	buffer := make([]uint16, (size+1)/2)
	status, _, _ = query.Call(key, uintptr(unsafe.Pointer(n)), 0, uintptr(unsafe.Pointer(&kind)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer)
}
func detectedSteamRoots() []string {
	var roots []string
	for _, p := range []string{registryText(0x80000001, `Software\Valve\Steam`, "SteamPath", 0), registryText(0x80000002, `Software\Valve\Steam`, "InstallPath", 0x200), registryText(0x80000002, `Software\Valve\Steam`, "InstallPath", 0x100), filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam")} {
		roots = append(roots, steamOMSIPaths(p)...)
	}
	return roots
}
