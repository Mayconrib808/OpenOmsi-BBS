//go:build windows

package main

import (
	"fmt"
)

func prepareSetupNativeTrip(dir string, c Config) error {
	if _, err := verifyPackage(dir); err != nil {
		return err
	}
	state, err := prepareBCSNextTrip(c, checkBCSClosedForUpdate)
	logSetup(dir, fmt.Sprintf("native-next-trip BCS=%s changed=%t", state.Path, state.Changed), err)
	if state.Note != "" {
		logSetup(dir, "native-next-trip "+state.Note, nil)
	}
	return err
}
