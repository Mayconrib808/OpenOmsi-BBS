//go:build windows

package main

import (
	"fmt"
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

func checkBridgeSetupClosedForUpdate(dir string) error {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = syscall.Process32First(snapshot, &entry)
	for err == nil {
		if strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), "Setup.exe") && samePath(processExecutableForUpdate(entry.ProcessID), filepath.Join(dir, "Setup.exe")) {
			return fmt.Errorf("feche o Setup da Bridge antes de atualizar")
		}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err != syscall.Errno(18) {
		return err
	}
	return nil
}
