//go:build windows

package main

import (
	"context"
	"time"
	"unsafe"
)

func (g *setupGUI) checkServerStatus() {
	if g.serverChecking || g.serverClosing { return }
	g.serverChecking = true
	guiUser.NewProc("EnableWindow").Call(g.serverButton,0)
	config := g.config
	ctx,cancel := context.WithCancel(context.Background())
	g.serverCancel = cancel
	window := g.window
	go func() {
		client := companyHTTPClient()
		defer client.CloseIdleConnections()
		result := checkSetupServerStatus(ctx,client,g.dir,config)
		select {
		case g.serverResults <- result:
			if ctx.Err() == nil { guiUser.NewProc("PostMessageW").Call(window,guiWMServerStatus,0,0) }
		case <-ctx.Done():
		}
	}()
}

func (g *setupGUI) finishServerStatus() {
	select {
	case g.serverState = <-g.serverResults:
	default: return
	}
	g.serverChecking = false
	if g.serverCancel != nil { g.serverCancel(); g.serverCancel = nil }
	g.renderServerStatus()
	guiUser.NewProc("EnableWindow").Call(g.serverButton,1)
}

func (g *setupGUI) renderServerStatus() {
	if g.serverLabel == 0 { return }
	key := g.serverState.Key
	if key == "" { key = "server_checking" }
	guiSetText(g.serverLabel,g.text("server_status")+" — "+g.text(key))
	guiUser.NewProc("InvalidateRect").Call(g.serverLamp,0,1)
}

func (g *setupGUI) drawServerLamp(item *guiDrawItem) {
	guiUser.NewProc("FillRect").Call(item.DC,uintptr(unsafe.Pointer(&item.Rect)),g.background)
	color := guiRGB(255,210,60)
	if g.serverState.Ready { color = guiRGB(54,211,119) }
	brush,_,_ := guiGDI.NewProc("CreateSolidBrush").Call(color)
	previous,_,_ := guiGDI.NewProc("SelectObject").Call(item.DC,brush)
	guiGDI.NewProc("Ellipse").Call(item.DC,4,6,20,22)
	guiGDI.NewProc("SelectObject").Call(item.DC,previous)
	guiGDI.NewProc("DeleteObject").Call(brush)
}

func (g *setupGUI) startServerStatus() {
	guiUser.NewProc("SetTimer").Call(g.window,guiIDServerTimer,uintptr(setupServerPollInterval/time.Millisecond),0)
	g.checkServerStatus()
}
