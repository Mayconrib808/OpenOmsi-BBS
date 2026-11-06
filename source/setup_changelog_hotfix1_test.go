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
		{"2.2.3-hotfix.2", 2},
		{"2.2.3-hotfix.1", 3},
		{"2.2.3", 4},
		{"2.2.2.4-hotfix.5", 5},
		{"2.2.2.4-hotfix.4", 6},
		{"2.2.2.4-hotfix.3", 7},
	}
	for _, c := range cases {
		sections := selectLogSections(c.from)
		if len(sections) != c.count {
			t.Fatalf("upgrading from %s: got %d chapters, want %d", c.from, len(sections), c.count)
		}
		for _, lang := range []string{"pt", "en", "de"} {
			if !strings.Contains(logL(lang, sections[0].label), "2.2.3.2") {
				t.Fatalf("%s is missing the dedicated 2.2.3.1 chapter", lang)
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
