//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func checkBCSClosed() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	// Constant script: user paths and credentials never enter PowerShell code.
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; Get-CimInstance Win32_Process | Where-Object { $_.Name -match '^(javaw?|busbetrieb-simulator|bbs)\\.exe$' -and ($_.CommandLine -match 'busbetrieb-simulator|Busbetrieb[s]?-Simulator' -or $_.Name -match '^(busbetrieb-simulator|bbs)\\.exe$') } | ForEach-Object { 'BCS_RUNNING' }")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("bcs process check failed: %w", err)
	}
	if strings.Contains(string(b), "BCS_RUNNING") {
		return fmt.Errorf("bcs running")
	}
	return nil
}
