//go:build !windows

package main

// Unit tests only exercise parsing and gate decisions; native window closing
// is compiled for Windows separately and validated in the user's integration.
func postOpenOMSIClose(pid uint32) int { panic("Windows closing must not run in portable unit tests") }

func acquireBridgeLock(root string) (func(), error) { return func() {}, nil }
func showLaunchError(lang, message string)          {}
func openMultiplayerDocument(path string) error     { return nil }
