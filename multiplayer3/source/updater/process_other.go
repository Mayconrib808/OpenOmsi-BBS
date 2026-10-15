//go:build !windows

package main

import "fmt"

func waitForSetup(pid int) error         { return fmt.Errorf("Windows updater") }
func showFailure(root string, err error) { fmt.Println(err) }

func chooseUpdateTarget() (string, error) { return "", fmt.Errorf("specify --target for this platform") }
