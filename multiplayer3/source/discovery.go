package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Detect existing installations only. No launch or file mutation on opening
// setup; an empty result leaves the verified official download available.
func detectClient(root, dir string) string {
	paths := []string{filepath.Join(root, "openOMSI", "openomsi.exe"), filepath.Join(dir, "client", "0.2.29", "openomsi.exe")}
	if path, err := bcsSettingsPath(root); err == nil {
		if b, err := readSmallFile(path, 128<<10); err == nil {
			lines := splitConfigLines(decodeBCSConfig(b))
			if len(lines) > 49 {
				paths = append(paths, filepath.Join(lines[49], "openomsi.exe"))

			}
		}
	}
	for _, p := range paths {
		if samePath(filepath.Dir(p), root) || filepath.Base(filepath.Dir(p)) == "PeDePeAdapter" || insideDirectory(adapterDirectory(dir), p) {
			continue
		}
		if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func steamOMSIPaths(steam string) []string {
	if !filepath.IsAbs(steam) {
		return nil
	}
	paths := []string{filepath.Join(steam, "steamapps", "common", "OMSI 2")}
	pattern := regexp.MustCompile(`"path"\s*"([^"\r\n]+)"`)
	for _, name := range []string{"steamapps/libraryfolders.vdf", "config/libraryfolders.vdf"} {
		if b, e := readSmallFile(filepath.Join(steam, filepath.FromSlash(name)), 1<<20); e == nil {
			for _, m := range pattern.FindAllStringSubmatch(string(b), -1) {
				path := strings.ReplaceAll(m[1], `\\`, `\`)
				if filepath.IsAbs(path) {
					paths = append(paths, filepath.Join(path, "steamapps", "common", "OMSI 2"))
				}
			}
		}
	}
	return paths
}
