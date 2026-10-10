package main

import (
	"os"
	"path/filepath"
	"strings"
)

func exeDir() string {
	p, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(p)
}

// Setup is at the package's entrance; the legacy launcher lives in app and the
// PeDePe-native drop-in lives in PeDePeAdapter. Never use the current working
// directory: BBS, UAC and the simulator may start us from another folder.
func packageRootForExecutable(executable string) string {
	dir := filepath.Dir(executable)
	base := filepath.Base(executable)
	if strings.EqualFold(base, "OpenOMSI_BCS_Bridge.exe") && strings.EqualFold(filepath.Base(dir), "app") {
		return filepath.Dir(dir)
	}
	if strings.EqualFold(base, "openomsi.exe") && strings.EqualFold(filepath.Base(dir), "PeDePeAdapter") {
		return filepath.Dir(dir)
	}
	return dir
}

func packageRoot() string {
	p, err := os.Executable()
	if err != nil {
		return "."
	}
	return packageRootForExecutable(p)
}

func appDir(root string) string     { return filepath.Join(root, "app") }
func configPath(root string) string { return filepath.Join(appDir(root), "bridge.ini") }
func bridgePath(root string) string { return filepath.Join(appDir(root), "OpenOMSI_BCS_Bridge.exe") }
func peDePeAdapterPath(root string) string { return filepath.Join(root, "PeDePeAdapter", "openomsi.exe") }

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func fileExists(p string) bool  { fi, e := os.Stat(p); return e == nil && !fi.IsDir() }
