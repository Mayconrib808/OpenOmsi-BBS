package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var clientVersionPattern = regexp.MustCompile(`\b0\.([0-9]+)\.([0-9]+)\b`)

func protocol7EraVersion(v string) bool {
	m := clientVersionPattern.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	minor, _ := strconv.Atoi(m[1])
	patch, _ := strconv.Atoi(m[2])
	return minor > 2 || minor == 2 && patch >= 48
}

// A manual executable may be an older protocol-6 client even when the company
// server is current. Probe the real executable before reserving a host session.
func validateMultiplayerClient(ctx context.Context, game string) error {
	probe, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probe, game, "--version")
	prepareVersionProbe(cmd)
	text, err := cmd.CombinedOutput()
	if err != nil || !protocol7EraVersion(string(text)) {
		return fmt.Errorf("protocolo 7: cliente openOMSI 0.2.48+ necessário; selecione Automático no Setup (%s)", strings.TrimSpace(string(text)))
	}
	return nil
}
