//go:build windows

package main

import (
	"context"
	"fmt"
	"path/filepath"
)

func setupNativeSelected(dir string, c Config) bool {
	jar, err := installedBCSJar(c.Root)
	if err != nil || jar == "" {
		return false
	}
	settings, err := nativeBCSSettingsDir(jar)
	if err != nil {
		return false
	}
	data, err := readBCSUpdateSettings(filepath.Join(settings, "config.cfg"))
	if err != nil {
		return false
	}
	root, adapter, enabled := nativeBCSUpdateSelection(data)
	return enabled && samePath(root, c.Root) && samePath(adapter, filepath.Join(dir, "PeDePeAdapter"))
}

func prepareSetupNativeTrip(dir string, c Config) error {
	if _, err := verifyPackage(dir); err != nil {
		return err
	}
	state, err := prepareBCSNextTrip(c, checkBCSClosedForUpdate)
	logSetup(dir, fmt.Sprintf("native-next-trip BCS=%s changed=%t", state.Path, state.Changed), err)
	return err
}

func (g *setupGUI) prepareNativeNextTripIfConfigured() {
	c := g.config
	if !setupNativeSelected(g.dir, c) {
		return
	}
	if err := setupChangesAllowed(c.Language); err != nil {
		return
	}
	jar, err := installedBCSJar(c.Root)
	if err != nil || checkBCSClosedForUpdate(jar) != nil {
		return
	}
	g.work(g.text("ready"), func(context.Context) guiResult {
		err := prepareSetupNativeTrip(g.dir, c)
		return guiResult{status: g.text("ready"), err: err}
	})
}
