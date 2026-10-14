//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showError(m string)             { fmt.Fprintln(os.Stderr, m) }
func openSetup(address string) error { fmt.Println(address); return nil }
