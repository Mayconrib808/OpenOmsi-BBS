package main

import (
	"os/exec"
	"strconv"
)

func launchUpdater(path, root string, pid int) error {
	cmd := exec.Command(path, "--target", root, "--pid", strconv.Itoa(pid))
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
