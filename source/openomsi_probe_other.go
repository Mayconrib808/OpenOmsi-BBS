//go:build !windows

package main

import "os/exec"

func prepareOpenOMSIProbe(cmd *exec.Cmd) {}
