//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode/utf16"
	"unsafe"
)

func (g *setupGUI) checkServerStatus() {
	if g.serverChecking || g.serverClosing {
		return
	}
	g.serverChecking = true
	guiUser.NewProc("EnableWindow").Call(g.serverButton, 0)
	config := g.config
	ctx, cancel := context.WithCancel(context.Background())
	g.serverCancel = cancel
	window := g.window
	go func() {
		client := companyHTTPClient()
		defer client.CloseIdleConnections()
		result := checkSetupServerStatus(ctx, client, g.dir, config)
		select {
		case g.serverResults <- result:
			if ctx.Err() == nil {
				guiUser.NewProc("PostMessageW").Call(window, guiWMServerStatus, 0, 0)
			}
		case <-ctx.Done():
		}
	}()
}

func (g *setupGUI) finishServerStatus() {
	select {
	case g.serverState = <-g.serverResults:
	default:
		return
	}
	g.serverChecking = false
	if g.serverCancel != nil {
		g.serverCancel()
		g.serverCancel = nil
	}
	g.renderServerStatus()
	guiUser.NewProc("EnableWindow").Call(g.serverButton, 1)
}

func (g *setupGUI) renderServerStatus() {
	if g.serverLabel == 0 {
		return
	}
	key := g.serverState.Key
	if key == "" {
		key = "server_checking"
	}
	guiSetText(g.serverLabel, g.text("server_status")+" — "+g.text(key))
	guiUser.NewProc("InvalidateRect").Call(g.serverLamp, 0, 1)
	pages := (len(g.serverState.Maps) + 2) / 3
	if pages < 1 {
		pages = 1
	}
	if g.serverPage >= pages {
		g.serverPage = pages - 1
	}
	if g.serverPage < 0 {
		g.serverPage = 0
	}
	for i, label := range g.serverMapLabels {
		text := ""
		at := g.serverPage*3 + i
		enabled := uintptr(0)
		if at < len(g.serverState.Maps) {
			entry := g.serverState.Maps[at]
			text = fmt.Sprintf("● %s   (%d)", entry.Name, entry.Players)
			enabled = 1
		} else if i == 0 && len(g.serverState.Maps) == 0 {
			text = g.text("server_active_none")
		}
		guiSetText(label, text)
		guiSetText(g.serverCopyButtons[i], g.text("server_copy"))
		guiUser.NewProc("EnableWindow").Call(g.serverCopyButtons[i], enabled)
	}
	guiSetText(g.serverPageLabel, fmt.Sprintf("%d/%d", g.serverPage+1, pages))
	previous, next := uintptr(0), uintptr(0)
	if g.serverPage > 0 {
		previous = 1
	}
	if g.serverPage+1 < pages {
		next = 1
	}
	guiUser.NewProc("EnableWindow").Call(g.serverPrevious, previous)
	guiUser.NewProc("EnableWindow").Call(g.serverNext, next)
}

func (g *setupGUI) copyServerLink(row int) {
	at := g.serverPage*3 + row
	if row < 0 || row >= len(g.serverCopyButtons) || at < 0 || at >= len(g.serverState.Maps) {
		return
	}
	entry := g.serverState.Maps[at]
	if _, err := companyWebURL(entry.Address, false); err != nil {
		return
	}
	if err := setupClipboardText(g.window, entry.Address); err != nil {
		guiMessageBox(g.window, err.Error(), 0x10)
		return
	}
	guiSetText(g.status, g.text("server_copied")+entry.Name)
}

func setupClipboardText(window uintptr, text string) error {
	opened, _, _ := guiUser.NewProc("OpenClipboard").Call(window)
	if opened == 0 {
		return fmt.Errorf("clipboard is busy")
	}
	defer guiUser.NewProc("CloseClipboard").Call()
	buffer := append(utf16.Encode([]rune(text)), 0)
	handle, _, _ := setupKernel.NewProc("GlobalAlloc").Call(2, uintptr(len(buffer)*2))
	if handle == 0 {
		return fmt.Errorf("cannot allocate clipboard")
	}
	ptr, _, _ := setupKernel.NewProc("GlobalLock").Call(handle)
	if ptr == 0 {
		setupKernel.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("cannot lock clipboard")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(buffer)), buffer)
	setupKernel.NewProc("GlobalUnlock").Call(handle)
	cleared, _, _ := guiUser.NewProc("EmptyClipboard").Call()
	if cleared == 0 {
		setupKernel.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("cannot clear clipboard")
	}
	result, _, _ := guiUser.NewProc("SetClipboardData").Call(13, handle)
	if result == 0 {
		setupKernel.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("cannot set clipboard")
	}
	return nil
}

// Exercise the actual native buttons and UTF-16 clipboard in --gui-smoke.
func (g *setupGUI) smokeServerLinks() error {
	if g.scrollMax > 0 {
		var before, after guiRect
		guiUser.NewProc("GetWindowRect").Call(g.serverPrevious, uintptr(unsafe.Pointer(&before)))
		g.scrollTo(g.scrollMax)
		guiUser.NewProc("GetWindowRect").Call(g.serverPrevious, uintptr(unsafe.Pointer(&after)))
		if int(before.Top-after.Top) != g.scrollMax {
			return fmt.Errorf("small-screen scrolling did not move native map controls")
		}
		g.scrollTo(0)
	}
	g.serverState = setupServerStatus{Ready: true, Key: "server_online", Maps: []setupActiveServer{
		{ID: "a", Name: "Carrão City", Address: "https://carrao.trycloudflare.com", Players: 2},
		{ID: "b", Name: "Fikcyjny Szczecin", Address: "https://szczecin.trycloudflare.com", Players: 1},
		{ID: "c", Name: "SP Noroeste V2", Address: "https://noroeste.trycloudflare.com", Players: 3},
		{ID: "d", Name: "Berlin Spandau", Address: "https://berlin.trycloudflare.com", Players: 1},
	}}
	g.renderServerStatus()
	if guiText(g.serverMapLabels[0]) != "● Carrão City   (2)" {
		return fmt.Errorf("native active-map row missing")
	}
	g.command(guiIDServerNext)
	if g.serverPage != 1 || guiText(g.serverMapLabels[0]) != "● Berlin Spandau   (1)" {
		return fmt.Errorf("active-map pagination failed")
	}
	g.command(guiIDServerCopyFirst)
	opened, _, _ := guiUser.NewProc("OpenClipboard").Call(g.window)
	if opened == 0 {
		return fmt.Errorf("native copy button did not open clipboard")
	}
	handle, _, _ := guiUser.NewProc("GetClipboardData").Call(13)
	ptr, _, _ := setupKernel.NewProc("GlobalLock").Call(handle)
	if ptr == 0 {
		guiUser.NewProc("CloseClipboard").Call()
		return fmt.Errorf("native copy button did not copy text")
	}
	buffer := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(g.serverState.Maps[3].Address)+1)
	got := string(utf16.Decode(buffer[:len(buffer)-1]))
	setupKernel.NewProc("GlobalUnlock").Call(handle)
	guiUser.NewProc("CloseClipboard").Call()
	if got != g.serverState.Maps[3].Address {
		return fmt.Errorf("native copy selected the wrong map link")
	}
	if previews := os.Getenv("BRIDGE_SETUP_PREVIEWS"); previews != "" {
		g.serverPage = 0
		g.renderServerStatus()
		g.scrollTo(g.scrollMax)
		guiUser.NewProc("RedrawWindow").Call(g.window, 0, 0, 0x0581)
		if err := captureSetupPreview(g.window, filepath.Join(previews, "setup-active-maps.png")); err != nil {
			return err
		}
		g.scrollTo(0)
	}
	g.serverState = setupServerStatus{Key: "server_agent_offline"}
	g.renderServerStatus()
	for _, button := range g.serverCopyButtons {
		enabled, _, _ := guiUser.NewProc("IsWindowEnabled").Call(button)
		if enabled != 0 {
			return fmt.Errorf("stale copy button remained enabled")
		}
	}
	return nil
}

func (g *setupGUI) drawServerLamp(item *guiDrawItem) {
	guiUser.NewProc("FillRect").Call(item.DC, uintptr(unsafe.Pointer(&item.Rect)), g.background)
	color := guiRGB(255, 210, 60)
	if g.serverState.Ready {
		color = guiRGB(54, 211, 119)
	}
	brush, _, _ := guiGDI.NewProc("CreateSolidBrush").Call(color)
	previous, _, _ := guiGDI.NewProc("SelectObject").Call(item.DC, brush)
	guiGDI.NewProc("Ellipse").Call(item.DC, 4, 6, 20, 22)
	guiGDI.NewProc("SelectObject").Call(item.DC, previous)
	guiGDI.NewProc("DeleteObject").Call(brush)
}

func (g *setupGUI) startServerStatus() {
	guiUser.NewProc("SetTimer").Call(g.window, guiIDServerTimer, uintptr(setupServerPollInterval/time.Millisecond), 0)
	g.checkServerStatus()
}
