//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSameExecutableForUpdateAcceptsSameFileAlias(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "Setup.exe")
	alias := filepath.Join(dir, "Setup-alias.exe")
	if err := os.WriteFile(original, []byte("MZ updater path identity test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, alias); err != nil {
		t.Skipf("hard links unavailable on this Windows runner: %v", err)
	}
	if samePath(original, alias) {
		t.Fatal("test requires two different textual paths")
	}
	if !sameExecutableForUpdate(original, alias) {
		t.Fatal("different path spellings for the same file must be accepted")
	}
}

func TestSameExecutableForUpdateRejectsDifferentFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "Setup.exe")
	b := filepath.Join(dir, "Other.exe")
	if err := os.WriteFile(a, []byte("MZ setup"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("MZ other"), 0600); err != nil {
		t.Fatal(err)
	}
	if sameExecutableForUpdate(a, b) {
		t.Fatal("different executables must not compare as the same process image")
	}
}

func TestSameExecutableForUpdateRejectsEmptyPath(t *testing.T) {
	if sameExecutableForUpdate("", `C:\Bridge\Setup.exe`) || sameExecutableForUpdate(`C:\Bridge\Setup.exe`, "") {
		t.Fatal("empty process image paths must never match")
	}
}
