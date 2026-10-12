package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func appDirectory(launcher string) string {
	if filepath.Base(launcher) == "PeDePeAdapter" && filepath.Base(filepath.Dir(launcher)) == "OpenOMSI-BBS" {
		return filepath.Dir(launcher)
	}
	return filepath.Join(launcher, "OpenOMSI-BBS")
}
func adapterDirectory(dir string) string { return filepath.Join(dir, "PeDePeAdapter") }
func configDirectory(dir string) string  { return filepath.Join(dir, "config") }
func adminPath(dir string) string        { return filepath.Join(configDirectory(dir), "company-admin.json") }
func detectRoot(dir string) string {
	root := filepath.Dir(dir)
	if s, e := os.Stat(filepath.Join(root, "Omsi.exe")); e == nil && s.Mode().IsRegular() {
		return root
	}
	var found string
	for _, path := range detectedSteamRoots() {
		if s, e := os.Stat(filepath.Join(path, "Omsi.exe")); e == nil && s.Mode().IsRegular() {
			if found != "" && !samePath(found, path) {
				return ""
			}
			found = path
		}
	}
	return found
}
func ensureLayout(dir string) error {
	for _, name := range []string{"config", "logs", "client", "server-cache", "updater", "PeDePeAdapter"} {
		if e := os.MkdirAll(filepath.Join(dir, name), 0755); e != nil {
			return e
		}
	}
	return nil
}
func saveConfig(dir string, c localConfig) error {
	if e := ensureLayout(dir); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	tmp := configPath(dir) + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, configPath(dir))
}

// Upgrade existing separate-folder users without modifying their old data.
func migrateConfig(launcher, dir string) {
	if _, e := os.Stat(configPath(dir)); e == nil {
		return
	}
	b, e := os.ReadFile(filepath.Join(launcher, "multiplayer.json"))
	if e != nil {
		return
	}
	var c localConfig
	if json.Unmarshal(b, &c) != nil || c.Version != 3 {
		return
	}
	if saveConfig(dir, c) != nil {
		return
	}
	if b, e := os.ReadFile(filepath.Join(launcher, "company-admin.json")); e == nil {
		_ = os.WriteFile(adminPath(dir), b, 0600)
	}
}
