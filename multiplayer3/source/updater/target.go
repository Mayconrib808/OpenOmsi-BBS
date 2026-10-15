package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var errUpdateCancelled = errors.New("update cancelled")

// A direct launch uses its own configured installation if it is beside Setup;
// otherwise the user selects the existing installation. Never guess another
// installation from Downloads, registry history, or a similarly named folder.
func updateTarget(requested, executable string, pick func() (string, error)) (string, error) {
	if requested != "" { return requested, nil }
	root := filepath.Dir(executable)
	if configuredUpdateRoot(root) { return root, nil }
	return pick()
}

func configuredUpdateRoot(root string) bool {
	if !filepath.IsAbs(root) { return false }
	st, err := os.Stat(filepath.Join(root, "Setup.exe"))
	if err != nil || !st.Mode().IsRegular() { return false }
	b, err := os.ReadFile(filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json"))
	if err != nil || len(b) > 256<<10 { return false }
	var config struct { Version int `json:"version"` }
	return json.Unmarshal(b, &config) == nil && config.Version == 3
}
