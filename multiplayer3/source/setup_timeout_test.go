package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupExpiryDoesNotReportManualWindowAsFailedTrip(t *testing.T) {
	defer setLanguage("pt-BR")
	for _, launching := range []bool{false, true} {
		dir := appDirectory(t.TempDir())
		off := false
		if err := writePreferences(dir, "en", &off); err != nil {
			t.Fatal(err)
		}
		err := setupWithOptions(dir, "", setupOptions{
			Open: func(string) error { return nil },
			Launching: launching,
			Lifetime: 25 * time.Millisecond,
		})
		if launching {
			if !errors.Is(err, errSetupExpired) || friendlyError(err) != tr("errorsetupexpired") {
				t.Fatalf("launching setup hid the actual expiry: %v", err)
			}
		} else if err != nil {
			t.Fatalf("manual setup expiry produced a false trip failure: %v", err)
		}
		log, readErr := os.ReadFile(filepath.Join(dir, "logs", "setup.log"))
		if readErr != nil || !strings.Contains(string(log), "[setup_timeout]") {
			t.Fatalf("missing expiry diagnostic: %v", readErr)
		}
	}
}
