//go:build !windows

package main

import "os/exec"

func prepareCompanyHostProcess(child *exec.Cmd) {}
func ownCompanyHostProcess(child *exec.Cmd) (func(), error) {
	return func() { _ = child.Process.Kill() }, nil
}
