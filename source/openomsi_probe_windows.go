//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func prepareOpenOMSIProbe(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
