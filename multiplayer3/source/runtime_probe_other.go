//go:build !windows

package main

import "os/exec"

func prepareVersionProbe(cmd *exec.Cmd) {}
