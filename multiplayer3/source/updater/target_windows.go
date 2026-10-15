//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type updateBrowseInfo struct {
	Owner, Root, DisplayName, Title uintptr
	Flags uint32
	Callback, Parameter uintptr
	Image int32
}

func chooseUpdateTarget() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	code, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(code) < 0 { return "", fmt.Errorf("cannot open installation selector: 0x%x", uint32(code)) }
	defer ole.NewProc("CoUninitialize").Call()
	shell := syscall.NewLazyDLL("shell32.dll")
	language, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	titleText := "Close BCS and Bridge. Select the existing installation folder containing Setup.exe."
	invalidText := "Select the configured 3.x installation containing Setup.exe and OpenOMSI-BBS."
	switch uint16(language) & 0x03ff {
	case 0x16:
		titleText = "Feche o BCS e a Bridge. Selecione a pasta da instalacao existente que contem Setup.exe."
		invalidText = "Selecione a instalacao 3.x configurada que contem Setup.exe e OpenOMSI-BBS."
	case 0x07:
		titleText = "BCS und Bridge schliessen. Bestehenden Installationsordner mit Setup.exe waehlen."
		invalidText = "Den eingerichteten 3.x-Ordner mit Setup.exe und OpenOMSI-BBS waehlen."
	}
	title, _ := syscall.UTF16PtrFromString(titleText)
	var display [260]uint16
	info := updateBrowseInfo{DisplayName: uintptr(unsafe.Pointer(&display[0])), Title: uintptr(unsafe.Pointer(title)), Flags: 0x0001 | 0x0040}
	id, _, _ := shell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(title)
	runtime.KeepAlive(display)
	if id == 0 { return "", errUpdateCancelled }
	defer ole.NewProc("CoTaskMemFree").Call(id)
	var path [260]uint16
	ok, _, err := shell.NewProc("SHGetPathFromIDListW").Call(id, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 { return "", fmt.Errorf("cannot read selected installation folder: %w", err) }
	root := syscall.UTF16ToString(path[:])
	if !configuredUpdateRoot(root) { return "", fmt.Errorf("%s", invalidText) }
	return root, nil
}
