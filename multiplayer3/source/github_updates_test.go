package main

import (
	"testing"
)

func TestGitHubUpdatesNeverOffer2xAndCompareDevVersions(t *testing.T) {
	releases := []githubRelease{{Tag: "v2.99.99", Page: githubReleaseRoot + "tag/v2.99.99"}, {Tag: "v3.0.2-dev.10", Page: githubReleaseRoot + "tag/v3.0.2-dev.10"}, {Tag: "v3.0.2-dev.3", Page: githubReleaseRoot + "tag/v3.0.2-dev.3"}}
	r := selectGitHubV3(releases, "3.0.2-dev.4")
	if r == nil || r.Version != "3.0.2-dev.10" || r.Installer.URL != "" {
		t.Fatal(r)
	}
	if selectGitHubV3(releases[:1], "3.0.2-dev.4") != nil {
		t.Fatal("2.x offered")
	}
	if !newerV3("3.0.2", "3.0.2-dev.100") || newerV3("3.0.2-dev.100", "3.0.2") {
		t.Fatal("stable ordering")
	}
	if approvedGitHubAsset(githubReleaseRoot + "download/v2.1.5/Atualizar-Multiplayer-2.1.5.exe") {
		t.Fatal("2.x download accepted")
	}
	if !approvedGitHubAsset(githubReleaseRoot + "download/v3.0.2-dev.10/Atualizar-Multiplayer-3.0.2-dev.10.exe") {
		t.Fatal("3.x updater rejected")
	}
}
