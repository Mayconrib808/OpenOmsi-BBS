package main

import "testing"

func TestPublishedPreviewHelpersRemainMigratable(t *testing.T) {
	for _, hash := range []string{
		"7728ef0a6080b91e68f79e75113d1957dd65af76ae4a4a6fcd4a22658924cb96", // dev.7
		"d4850e4785a1d5f8e49a96c1e6b6bbfdca5e984a278ac43f564aaa99648a88fa", // dev.8
		"1c13a1da6bdd3991c447ebab0163a3fc4184ebd44b341149b2ed255db93f6ff9", // dev.9
	} {
		if !previousPluginHosts[hash] {
			t.Fatalf("published preview helper is no longer migratable: %s", hash)
		}
	}
	if previousPluginHosts["ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"] {
		t.Fatal("unknown helper must not be accepted")
	}
}
