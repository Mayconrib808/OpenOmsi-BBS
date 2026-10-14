//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func prepareVersionProbe(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
