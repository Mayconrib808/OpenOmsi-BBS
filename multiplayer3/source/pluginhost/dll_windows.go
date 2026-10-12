//go:build windows && 386

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Mayconrib808
// DLL ABI and frame order adapted from openOMSI, Copyright (c) 2026 usonskyyyy.
package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	setDllDirectory  = kernel32.NewProc("SetDllDirectoryW")
	setErrorMode     = kernel32.NewProc("SetErrorMode")
	user32           = syscall.NewLazyDLL("user32.dll")
	peekMessage      = user32.NewProc("PeekMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
)

type windowsPlugin struct {
	dll                                   *syscall.DLL
	startProc, finalizeProc               *syscall.Proc
	variable, trigger, system, stringProc *syscall.Proc
}

func loadPlugin(path string) (plugin, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dll, root, err := pluginPaths(path, executable)
	if err != nil {
		return nil, err
	}
	// These are process-local settings. Nothing is written into Steam, vendor
	// configuration, the Registry, or the vendor-owned bbs.start marker here.
	if err = os.Chdir(root); err != nil {
		return nil, err
	}
	for _, key := range []string{"SteamAppId", "SteamGameId"} {
		if err = os.Setenv(key, "252530"); err != nil {
			return nil, err
		}
	}
	u, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return nil, err
	}
	if ok, _, e := setDllDirectory.Call(uintptr(unsafe.Pointer(u))); ok == 0 {
		return nil, fmt.Errorf("SetDllDirectory: %w", e)
	}
	setErrorMode.Call(0x0001 | 0x0002 | 0x8000)
	library, err := syscall.LoadDLL(dll)
	if err != nil {
		return nil, err
	}
	p := &windowsPlugin{dll: library}
	if p.startProc, err = library.FindProc("PluginStart"); err != nil {
		library.Release()
		return nil, err
	}
	if p.finalizeProc, err = library.FindProc("PluginFinalize"); err != nil {
		library.Release()
		return nil, err
	}
	p.variable, _ = library.FindProc("AccessVariable")
	p.trigger, _ = library.FindProc("AccessTrigger")
	p.system, _ = library.FindProc("AccessSystemVariable")
	p.stringProc, _ = library.FindProc("AccessStringVariable")
	return p, nil
}
func (p *windowsPlugin) start()    { p.startProc.Call(0) }
func (p *windowsPlugin) finalize() { p.finalizeProc.Call() }
func (p *windowsPlugin) close()    { p.dll.Release() }
func (p *windowsPlugin) flags() byte {
	return boolByte(p.variable != nil) | boolByte(p.trigger != nil)<<1 |
		boolByte(p.system != nil)<<2 | boolByte(p.stringProc != nil)<<3
}
func accessFloats(proc *syscall.Proc, entries []floatEntry) []floatResult {
	results := make([]floatResult, len(entries))
	if proc == nil {
		return results
	}
	for i, entry := range entries {
		value, write := entry.value, byte(0)
		proc.Call(uintptr(entry.index), uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&write)))
		results[i] = floatResult{write != 0, value}
	}
	return results
}
func (p *windowsPlugin) frame(f frame) reply {
	r := reply{
		system: accessFloats(p.system, f.system), variables: accessFloats(p.variable, f.variables),
		strings: make([]stringResult, len(f.strings)), triggers: make([]bool, len(f.triggers)),
	}
	if p.stringProc != nil {
		for i, entry := range f.strings {
			encoded := utf16.Encode([]rune(entry.value))
			size := len(encoded) + 1
			if size < 4096 {
				size = 4096
			}
			buffer := make([]uint16, size)
			copy(buffer, encoded)
			write := byte(0)
			p.stringProc.Call(uintptr(entry.index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&write)))
			end := 0
			for end < len(buffer) && buffer[end] != 0 {
				end++
			}
			r.strings[i] = stringResult{write != 0, string(utf16.Decode(buffer[:end]))}
			runtime.KeepAlive(buffer)
		}
	}
	if p.trigger != nil {
		for i, index := range f.triggers {
			active := byte(0)
			p.trigger.Call(uintptr(index), uintptr(unsafe.Pointer(&active)))
			r.triggers[i] = active != 0
		}
	}
	return r
}

// MSG including the reserved field: 32 bytes on x86. Do not block the pipe reader.
type windowsMessage struct {
	hwnd, message, wparam, lparam, time uint32
	x, y                                int32
	private                             uint32
}

func pumpMessages() bool {
	var message windowsMessage
	for i := 0; i < 64; i++ {
		ok, _, _ := peekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 1)
		if ok == 0 {
			break
		}
		if message.message == 0x0012 { // WM_QUIT
			return false
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&message)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	return true
}
