//go:build windows

package main

import (
	"os/exec"
	"os"
	"time"
	"path/filepath"
	"strings"
	"syscall"
)

// The updater is a background helper, but Setup is a GUI application that must
// become visible after installation. Windows uses STARTUPINFO.ShowWindow for the
// first ShowWindow call in the child: passing SW_HIDE can silently suppress
// Setup's own ShowWindow(SW_SHOWNORMAL) and therefore its release notes.
func bridgeUpdateCommand(path, directory string, args ...string) *exec.Cmd {
	cmd := exec.Command(path, args...)
	cmd.Dir = directory
	// Keep the updater helper's previous behavior while allowing the re-opened
	// Setup.exe to display its window (including the 2.2.2 changelog).
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: !strings.EqualFold(filepath.Base(path), "Setup.exe"),
	}
	return cmd
}

// A Start() success only means Windows created a child process. It does NOT
// mean the updater parsed the arguments, passed preflight or will install.
// Keep the old Setup visible until the new updater explicitly acknowledges.
func startBridgeUpdateWithAcknowledgment(path,directory string,args ...string) error {
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{return err}
    defer os.Remove(ready)
    args=append(args,"--handoff-ready",ready)
    cmd:=bridgeUpdateCommand(path,directory,args...)
    if err:=cmd.Start();err!=nil{return err}
    defer cmd.Process.Release()
    return awaitBridgeUpdateHandoff(ready,8*time.Second)
}

func startBridgeUpdateProcess(path, directory string, args ...string) error {
	cmd := bridgeUpdateCommand(path, directory, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
