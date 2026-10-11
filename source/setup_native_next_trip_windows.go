//go:build windows

package main

import (
	"context"
	"fmt"
)

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
		return guiResult{status: setupNativeActivationState(c, true), err: err}
	})
}
