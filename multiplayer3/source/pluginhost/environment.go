// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Mayconrib808
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This BCS-specific host deliberately refuses other plugins. The installed
// vendor DLL is never copied, patched or included in the bridge package.
func pluginPaths(path, executable string) (dll, root string, err error) {
	if !strings.EqualFold(filepath.Base(path), "bbs.dll") {
		return "", "", fmt.Errorf("this bridge host only accepts the installed bbs.dll")
	}
	dll, err = filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	dll, err = filepath.EvalSymlinks(dll)
	if err != nil {
		return "", "", err
	}
	if st, e := os.Stat(dll); e != nil || !st.Mode().IsRegular() {
		return "", "", fmt.Errorf("plugin is not a regular file")
	}
	for dir := filepath.Dir(dll); ; dir = filepath.Dir(dir) {
		if strings.EqualFold(filepath.Base(dir), "plugins") {
			root = filepath.Dir(dir)
			break
		}
		if filepath.Dir(dir) == dir {
			return "", "", fmt.Errorf("bbs.dll must be inside the original OMSI Plugins folder")
		}
	}
	if st, e := os.Stat(filepath.Join(root, "Omsi.exe")); e != nil || !st.Mode().IsRegular() {
		return "", "", fmt.Errorf("the original OMSI root is missing Omsi.exe")
	}
	exe, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(exe)), filepath.Clean(root)) {
		return "", "", fmt.Errorf("activate the bridge with Setup.exe: this host must run from the original OMSI root")
	}
	return dll, root, nil
}
