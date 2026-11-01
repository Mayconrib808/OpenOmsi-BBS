//go:build windows

package main

import (
	"os/exec"
	"os"
	"time"
	"syscall"
)

// Both executables use the GUI subsystem. SW_HIDE can suppress the updater's
// first progress window as well as the reopened Setup/changelog. Let each GUI
// show its own window; this does not create a console for either application.
func bridgeUpdateCommand(path, directory string, args ...string) *exec.Cmd {
	cmd := exec.Command(path, args...)
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false,
	}
	return cmd
}

// A Start() success only means Windows created a child process. It does NOT
// mean the updater parsed the arguments, passed preflight or will install.
// Keep the old Setup visible until the new updater explicitly acknowledges.
func startBridgeUpdateWithAcknowledgment(path,directory string,args ...string) error {
    ready,err:=newBridgeUpdateHandoffPath()
    if err!=nil{return err}
    accepted:=false
    defer func(){
        os.Remove(ready)
        if !accepted{_ = os.WriteFile(ready+".canceled",[]byte("CANCELED\n"),0600)}
    }()
    args=append(args,"--handoff-ready",ready)
    cmd:=bridgeUpdateCommand(path,directory,args...)
    if err:=cmd.Start();err!=nil{return err}
    defer cmd.Process.Release()
    if err:=awaitBridgeUpdateHandoff(ready,30*time.Second);err!=nil{return err}
    if err:=acceptBridgeUpdateHandoff(ready);err!=nil{return err}
    accepted=true
    return nil
}

func startBridgeUpdateProcess(path, directory string, args ...string) error {
	cmd := bridgeUpdateCommand(path, directory, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
