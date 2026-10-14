//go:build !windows

package main

import "fmt"

func checkBCSClosed() error { return fmt.Errorf("bcs process check requires Windows") }
