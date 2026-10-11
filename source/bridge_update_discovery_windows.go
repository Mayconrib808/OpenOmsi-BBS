//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

func readSteamUpdatePath(hive syscall.Handle, key, name string, view int) string {
	var h syscall.Handle
	access := uint32(syscall.KEY_QUERY_VALUE)
	if view != 0 {
		access |= regView(view)
	}
	if err := syscall.RegOpenKeyEx(hive, winPtr(key), 0, access, &h); err != nil {
		return ""
	}
	defer syscall.RegCloseKey(h)
	var kind, size uint32
	if err := syscall.RegQueryValueEx(h, winPtr(name), nil, &kind, nil, &size); err != nil || size < 2 || size > 65536 || (kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ) {
		return ""
	}
	data := make([]byte, size)
	if err := syscall.RegQueryValueEx(h, winPtr(name), nil, &kind, &data[0], &size); err != nil {
		return ""
	}
	return (registryValue{Present: true, Kind: kind, Data: data[:size]}).text()
}

// Steam's existing library list locates OMSI even on another drive. All
// registry accesses here are read-only; no legacy Bridge registration is used.
func steamRootsForBridgeUpdate() []string {
	var roots []string
	steam := readSteamUpdatePath(syscall.HKEY_CURRENT_USER, `Software\Valve\Steam`, "SteamPath", 0)
	roots = append(roots, steamLibraryUpdateRoots(steam)...)
	for _, view := range []int{64, 32} {
		steam = readSteamUpdatePath(syscall.HKEY_LOCAL_MACHINE, `Software\Valve\Steam`, "InstallPath", view)
		roots = append(roots, steamLibraryUpdateRoots(steam)...)
		game := readSteamUpdatePath(syscall.HKEY_LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 252530`, "InstallLocation", view)
		if filepath.IsAbs(game) {
			roots = append(roots, game)
		}
	}
	for _, key := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if base := os.Getenv(key); filepath.IsAbs(base) {
			roots = append(roots, steamLibraryUpdateRoots(filepath.Join(base, "Steam"))...)
		}
	}
	return roots
}
