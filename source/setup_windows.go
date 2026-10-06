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

var setupAdvapi = syscall.NewLazyDLL("advapi32.dll")
var setupKernel = syscall.NewLazyDLL("kernel32.dll")
var setupShell = syscall.NewLazyDLL("shell32.dll")

type windowsRegistry struct{}

func newWindowsRegistry() registryAPI { return windowsRegistry{} }
func regView(v int) uint32 {
	if v == 64 {
		return syscall.KEY_WOW64_64KEY
	}
	return syscall.KEY_WOW64_32KEY
}
func regMissing(e error) bool {
	return e == syscall.ERROR_FILE_NOT_FOUND || e == syscall.ERROR_PATH_NOT_FOUND
}
func winPtr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func (r windowsRegistry) open(view int, key string, access uint32) (syscall.Handle, error) {
	var h syscall.Handle
	e := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, winPtr(key), 0, access|regView(view), &h)
	return h, e
}
func (r windowsRegistry) Read(view int, key, name string) (registryValue, error) {
	h, e := r.open(view, key, syscall.KEY_QUERY_VALUE)
	if regMissing(e) {
		return registryValue{}, nil
	}
	if e != nil {
		return registryValue{}, e
	}
	defer syscall.RegCloseKey(h)
	var kind, n uint32
	e = syscall.RegQueryValueEx(h, winPtr(name), nil, &kind, nil, &n)
	if regMissing(e) {
		return registryValue{}, nil
	}
	if e != nil {
		return registryValue{}, e
	}
	if n > 1<<20 {
		return registryValue{}, fmt.Errorf("registry value exceeds size limit")
	}
	data := make([]byte, n)
	var buf *byte
	if n > 0 {
		buf = &data[0]
	}
	e = syscall.RegQueryValueEx(h, winPtr(name), nil, &kind, buf, &n)
	if e != nil {
		return registryValue{}, e
	}
	return registryValue{true, kind, data[:n]}, nil
}
func (r windowsRegistry) Write(view int, key, name string, v registryValue) error {
	var h syscall.Handle
	var disposition uint32
	result, _, _ := setupAdvapi.NewProc("RegCreateKeyExW").Call(syscall.HKEY_LOCAL_MACHINE, uintptr(unsafe.Pointer(winPtr(key))), 0, 0, 0, uintptr(syscall.KEY_SET_VALUE|regView(view)), 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disposition)))
	if result != 0 {
		return syscall.Errno(result)
	}
	defer syscall.RegCloseKey(h)
	var b uintptr
	if len(v.Data) > 0 {
		b = uintptr(unsafe.Pointer(&v.Data[0]))
	}
	result, _, _ = setupAdvapi.NewProc("RegSetValueExW").Call(uintptr(h), uintptr(unsafe.Pointer(winPtr(name))), 0, uintptr(v.Kind), b, uintptr(len(v.Data)))
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}
func (r windowsRegistry) DeleteValue(view int, key, name string) error {
	h, e := r.open(view, key, syscall.KEY_SET_VALUE)
	if regMissing(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer syscall.RegCloseKey(h)
	result, _, _ := setupAdvapi.NewProc("RegDeleteValueW").Call(uintptr(h), uintptr(unsafe.Pointer(winPtr(name))))
	if result != 0 && !regMissing(syscall.Errno(result)) {
		return syscall.Errno(result)
	}
	return nil
}
func (r windowsRegistry) DeleteKey(view int, key string) error {
	result, _, _ := setupAdvapi.NewProc("RegDeleteKeyExW").Call(syscall.HKEY_LOCAL_MACHINE, uintptr(unsafe.Pointer(winPtr(key))), uintptr(regView(view)), 0)
	if result != 0 && !regMissing(syscall.Errno(result)) {
		return syscall.Errno(result)
	}
	return nil
}
func (r windowsRegistry) Keys(view int, key string) ([]string, error) {
	h, e := r.open(view, key, syscall.KEY_ENUMERATE_SUB_KEYS)
	if regMissing(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer syscall.RegCloseKey(h)
	var out []string
	for i := uint32(0); ; i++ {
		buf := make([]uint16, 512)
		n := uint32(len(buf))
		e = syscall.RegEnumKeyEx(h, i, &buf[0], &n, nil, nil, nil, nil)
		if e == syscall.Errno(259) {
			break
		}
		if e != nil {
			return nil, e
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
	return out, nil
}
func prepareConsole() {
	setupKernel.NewProc("SetConsoleCP").Call(65001)
	setupKernel.NewProc("SetConsoleOutputCP").Call(65001)
	setupKernel.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(winPtr("OpenOMSI BCS Bridge v" + bridgeVersion + " - by " + bridgeAuthor))))
}
func isElevated() bool {
	p, e := syscall.GetCurrentProcess()
	if e != nil {
		return false
	}
	var token syscall.Token
	if e = syscall.OpenProcessToken(p, syscall.TOKEN_QUERY, &token); e != nil {
		return false
	}
	defer token.Close()
	var elevated, used uint32
	e = syscall.GetTokenInformation(token, 20, (*byte)(unsafe.Pointer(&elevated)), 4, &used)
	return e == nil && elevated != 0
}

type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon, Process                     uintptr
}

func elevateSetup(action, lang string) error {
	if action != "activate" && action != "deactivate" {
		return fmt.Errorf("invalid elevation action")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	// Parameters are fixed tokens. User folder paths are read from bridge.ini after elevation.
	params := "--admin=" + action + " --lang=" + normalizeLanguage(lang)
	info := shellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: winPtr("runas"), File: winPtr(exe), Parameters: winPtr(params), Directory: winPtr(filepath.Dir(exe)), Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	ole := syscall.NewLazyDLL("ole32.dll")
	ok, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(ok) >= 0 {
		defer ole.NewProc("CoUninitialize").Call()
	}
	result, _, err := setupShell.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return err
	}
	if info.Process == 0 {
		return fmt.Errorf("elevated process handle missing")
	}
	defer syscall.CloseHandle(syscall.Handle(info.Process))
	waitResult, e := syscall.WaitForSingleObject(syscall.Handle(info.Process), syscall.INFINITE)
	if e != nil {
		return e
	}
	if waitResult != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("elevated process wait failed")
	}
	var exit uint32
	if e = syscall.GetExitCodeProcess(syscall.Handle(info.Process), &exit); e != nil {
		return e
	}
	if exit != 0 {
		return fmt.Errorf("%s", localText(lang, "A alteração como administrador falhou; veja setup-v1.1.2.log.", "Administrator action failed; see setup-v1.1.2.log.", "Die Änderung mit Administratorrechten ist fehlgeschlagen. Siehe setup-v1.1.2.log."))
	}
	return nil
}
func openDocument(path string) error {
	result, _, err := setupShell.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(winPtr("open"))), uintptr(unsafe.Pointer(winPtr(path))), 0, 0, 1)
	if result <= 32 {
		return fmt.Errorf("open tutorial: %v", err)
	}
	return nil
}
func gamesRunning() (bool, error) {
	h, e := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return false, e
	}
	defer syscall.CloseHandle(h)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	e = syscall.Process32First(h, &entry)
	for e == nil {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "Omsi.exe") || strings.EqualFold(name, "openomsi.exe") {
			return true, nil
		}
		e = syscall.Process32Next(h, &entry)
	}
	if e != syscall.Errno(18) {
		return false, e
	}
	return false, nil
}
