//go:build !windows || !386

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Mayconrib808
package main

import "fmt"

func loadPlugin(string) (plugin, error) {
	return nil, fmt.Errorf("the BCS DLL host must be built for Windows x86")
}
func pumpMessages() bool { return true }
