package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func recordSetupError(dir, action string, c localConfig, err error) {
	if err == nil {
		return
	}
	// Paths and underlying local setup error only. Never log room invitations or admin keys.
	f, e := os.OpenFile(filepath.Join(dir, "logs", "setup.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return
	}
	defer f.Close()
	clean := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ") }
	fmt.Fprintf(f, "%s [%s] OMSI=%q openOMSI=%q: %s\n", time.Now().Format(time.RFC3339), action, clean(c.Root), clean(c.Game), clean(setupErrorDetail(c, err)))
}

func setupErrorDetail(c localConfig, err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if c.Invitation != "" {
		s = strings.ReplaceAll(s, c.Invitation, "[invitation]")
	}
	return s
}
