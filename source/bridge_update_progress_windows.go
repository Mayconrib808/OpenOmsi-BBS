//go:build windows

package main

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type coopUpdateWindowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}

type coopUpdateMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

var coopUpdateCancel context.CancelFunc
var coopUpdateUser = syscall.NewLazyDLL("user32.dll")
var coopUpdateProcedure = syscall.NewCallback(func(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if message == 0x0010 {
		if coopUpdateCancel != nil {
			coopUpdateCancel()
		}
		return 0
	}
	r, _, _ := coopUpdateUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
	return r
})

func withCoopUpdateProgress(language string, task func(context.Context, func(int64, int64)) (map[string][]byte, error)) (map[string][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	coopUpdateCancel = cancel
	defer func() { coopUpdateCancel = nil }()
	instance, _, _ := setupKernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := coopUpdateUser.NewProc("LoadCursorW").Call(0, 32512)
	class := coopUpdateWindowClass{Procedure: coopUpdateProcedure, Instance: instance, Cursor: cursor, Background: 6, Name: winPtr("OpenOmsiCoopUpdateProgress")}
	class.Size = uint32(unsafe.Sizeof(class))
	registered, _, err := coopUpdateUser.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 {
		return nil, fmt.Errorf("não foi possível criar a janela de atualização: %v", err)
	}
	defer coopUpdateUser.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class.Name)), instance)
	width, _, _ := coopUpdateUser.NewProc("GetSystemMetrics").Call(0)
	height, _, _ := coopUpdateUser.NewProc("GetSystemMetrics").Call(1)
	x, y := int(width)/2-280, int(height)/2-95
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	window, _, err := coopUpdateUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class.Name)), uintptr(unsafe.Pointer(winPtr("OpenOmsi BBS "+bridgeVersion))), 0x10C80000, uintptr(x), uintptr(y), 560, 190, 0, 0, instance, 0)
	if window == 0 {
		return nil, fmt.Errorf("não foi possível abrir a janela de atualização: %v", err)
	}
	defer coopUpdateUser.NewProc("DestroyWindow").Call(window)
	text := localText(language, "Preparando OpenOmsi Coop 0.3.38...", "Preparing OpenOmsi Coop 0.3.38...", "OpenOmsi Coop 0.3.38 wird vorbereitet...")
	label, _, _ := coopUpdateUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(winPtr("STATIC"))), uintptr(unsafe.Pointer(winPtr(text))), 0x50000000, 24, 24, 500, 55, window, 0, instance, 0)
	font, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	coopUpdateUser.NewProc("SendMessageW").Call(label, 0x0030, font, 1)
	init := struct{ Size, Classes uint32 }{8, 0x20}
	syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&init)))
	bar, _, _ := coopUpdateUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(winPtr("msctls_progress32"))), 0, 0x50000000, 24, 94, 500, 24, window, 0, instance, 0)
	coopUpdateUser.NewProc("SendMessageW").Call(bar, 0x0406, 0, 100)
	coopUpdateUser.NewProc("ShowWindow").Call(window, 1)
	coopUpdateUser.NewProc("UpdateWindow").Call(window)
	type result struct {
		files map[string][]byte
		err   error
	}
	completed := make(chan result, 1)
	var mutex sync.Mutex
	var done, total int64
	go func() {
		files, err := task(ctx, func(d, t int64) { mutex.Lock(); done, total = d, t; mutex.Unlock() })
		completed <- result{files, err}
	}()
	for {
		select {
		case r := <-completed:
			return r.files, r.err
		default:
		}
		var message coopUpdateMessage
		for {
			have, _, _ := coopUpdateUser.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 1)
			if have == 0 {
				break
			}
			coopUpdateUser.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			coopUpdateUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
		mutex.Lock()
		d, t := done, total
		mutex.Unlock()
		if t > 0 {
			percent := d * 100 / t
			text = fmt.Sprintf(localText(language, "Baixando Cooperativo: %.1f / %.1f MB (%d%%)", "Downloading Cooperative: %.1f / %.1f MB (%d%%)", "Coop herunterladen: %.1f / %.1f MB (%d%%)"), float64(d)/(1<<20), float64(t)/(1<<20), percent)
			coopUpdateUser.NewProc("SetWindowTextW").Call(label, uintptr(unsafe.Pointer(winPtr(text))))
			coopUpdateUser.NewProc("SendMessageW").Call(bar, 0x0402, uintptr(percent), 0)
		}
		if ctx.Err() != nil {
			coopUpdateUser.NewProc("SetWindowTextW").Call(label, uintptr(unsafe.Pointer(winPtr(localText(language, "Cancelando...", "Cancelling...", "Abbrechen...")))))
		}
		time.Sleep(30 * time.Millisecond)
	}
}
