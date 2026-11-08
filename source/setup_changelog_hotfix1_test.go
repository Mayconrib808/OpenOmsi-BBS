//go:build windows

package main

import (
	"strings"
	"testing"
)

func Test2231ChangelogIncludesOnlyUnseenReleases(t *testing.T) {
	cases := []struct {
		from  string
		count int
	}{
		{"2.2.3.2-hotfix.1", 1},
		{"2.2.3.2", 2},
		{"2.2.3.1-hotfix.2", 3},
		{"2.2.3-hotfix.2", 4},
		{"2.2.3-hotfix.1", 5},
		{"2.2.3", 6},
		{"2.2.2.4-hotfix.5", 7},
		{"2.2.2.4-hotfix.4", 8},
		{"2.2.2.4-hotfix.3", 9},
	}
	for _, c := range cases {
		sections := selectLogSections(c.from)
		if len(sections) != c.count {
			t.Fatalf("upgrading from %s: got %d chapters, want %d", c.from, len(sections), c.count)
		}
		for _, lang := range []string{"pt", "en", "de"} {
			if !strings.Contains(logL(lang, sections[0].label), "2.2.3.3") {
				t.Fatalf("%s is missing the dedicated 2.2.3.3 chapter", lang)
			}
		}
	}
	if len(selectLogSections("2.2.2")) <= 4 {
		t.Fatal("an older installation lost its cumulative history")
	}
	if changelogSeenVersion != bridgeVersion {
		t.Fatal("the new Setup would suppress its automatic changelog")
	}
}
