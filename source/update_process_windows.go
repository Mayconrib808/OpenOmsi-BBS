//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// exec.Command supplies valid NUL handles for GUI processes without a console.
// Direct os.StartProcess with an empty Files slice returns EINVAL on Windows.
func startBridgeUpdateProcess(path, directory string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
