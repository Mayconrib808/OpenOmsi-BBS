//go:build !windows

package main

import "fmt"

func runHostAgentGUI(dir, path string, smoke bool) error {
	return fmt.Errorf("the host configuration window requires Windows")
}
func hostAgentShowError(err error) { fmt.Println(err) }
