//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func processExecutableForUpdate(pid uint32) string {
	h, _, _ := setupKernel.NewProc("OpenProcess").Call(0x1000, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	buffer := make([]uint16, 32768)
	n := uint32(len(buffer))
	ok, _, _ := setupKernel.NewProc("QueryFullProcessImageNameW").Call(h, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:n])
}

// Process image paths are not guaranteed to use the same textual spelling as
// the package path. Windows can report a junction/symlink/alias even when both
// names point at exactly the same executable. Prefer file identity when a
// simple normalized string comparison is not enough.
func sameExecutableForUpdate(actual, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	if samePath(actual, expected) {
		return true
	}
	actualInfo, actualErr := os.Stat(actual)
	expectedInfo, expectedErr := os.Stat(expected)
	if actualErr == nil && expectedErr == nil && os.SameFile(actualInfo, expectedInfo) {
		return true
	}
	actualResolved, actualErr := filepath.EvalSymlinks(actual)
	expectedResolved, expectedErr := filepath.EvalSymlinks(expected)
	return actualErr == nil && expectedErr == nil && samePath(actualResolved, expectedResolved)
}

func checkBCSClosedForUpdate(jar string) error {
	// Java normally retains an open JAR handle. Require exclusive access; a
	// busy/mapped vendor archive must never be replaced during a running trip.
	h, err := syscall.CreateFile(winPtr(jar), syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return fmt.Errorf("feche completamente o BCS antes de atualizar: %w", err)
	}
	syscall.CloseHandle(h)
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = syscall.Process32First(snapshot, &entry)
	base := strings.ToLower(filepath.Clean(filepath.Dir(jar))) + string(filepath.Separator)
	for err == nil {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if name == "java.exe" || name == "javaw.exe" || name == "busbetrieb-simulator.exe" {
			executable := strings.ToLower(filepath.Clean(processExecutableForUpdate(entry.ProcessID)))
			if strings.HasPrefix(executable, base) {
				return fmt.Errorf("feche completamente o BCS antes de atualizar")
			}
		}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err != syscall.Errno(18) {
		return err
	}
	return nil
}

func bridgeSetupsRunningForUpdate(dir string) ([]uint32, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = syscall.Process32First(snapshot, &entry)
	var pids []uint32
	for err == nil {
		if strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), "Setup.exe") && sameExecutableForUpdate(processExecutableForUpdate(entry.ProcessID), filepath.Join(dir, "Setup.exe")) {
			pids = append(pids, entry.ProcessID)
		}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err != syscall.Errno(18) {
		return nil, err
	}
	return pids, nil
}

func checkBridgeSetupClosedForUpdate(dir string) error {
	pids, err := bridgeSetupsRunningForUpdate(dir)
	if err != nil {
		return err
	}
	if len(pids) != 0 {
		return errBridgeSetupStillOpen
	}
	return nil
}

func checkUpdateHostClosed(dir string) error {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = syscall.Process32First(snapshot, &entry)
	for err == nil {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "openomsi.exe") && sameExecutableForUpdate(processExecutableForUpdate(entry.ProcessID), filepath.Join(dir, "app", "server-dinamico-p14", "openomsi.exe")) {
			return fmt.Errorf("feche o servidor dinâmico desta instalação antes de atualizar")
		}
		if (strings.EqualFold(name, "HostAgent.exe") || strings.EqualFold(name, "CompanyHost.exe")) && sameExecutableForUpdate(processExecutableForUpdate(entry.ProcessID), filepath.Join(dir, name)) {
			return fmt.Errorf("feche o HostAgent e o servidor da empresa antes de atualizar")
		}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err != syscall.Errno(18) {
		return err
	}
	return nil
}
