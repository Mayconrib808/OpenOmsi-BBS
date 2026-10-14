package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordSetupErrorCreatesLogsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "OpenOMSI-BBS")
	recordSetupError(dir, "configurebcs", localConfig{Root: filepath.Join(t.TempDir(), "OMSI 2")}, os.ErrPermission)
	path := filepath.Join(dir, "logs", "setup.log")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		t.Fatalf("setup log was not created: %v", err)
	}
}
