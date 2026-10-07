package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compatibleOpenOMSIHelp(dedicated, multiplayer bool) string {
	var help strings.Builder
	help.WriteString("openOMSI\nUsage: openomsi [OPTIONS]\nOptions:\n")
	for _, flag := range requiredOpenOMSIFlags(dedicated, multiplayer) {
		help.WriteString("      " + flag + " <VALUE>\n          Description mentions --other-option\n")
	}
	return help.String()
}

func TestOpenOMSIOptionsCheckedByUseRatherThanRelease(t *testing.T) {
	for _, dedicated := range []bool{false, true} {
		for _, multiplayer := range []bool{false, true} {
			help := compatibleOpenOMSIHelp(dedicated, multiplayer)
			if err := validateOpenOMSIHelp(help, dedicated, multiplayer); err != nil {
				t.Fatal(err)
			}
			for _, flag := range requiredOpenOMSIFlags(dedicated, multiplayer) {
				// Merely mentioning an option in prose must not count as a CLI
				// option declaration, nor may --root-old count as --root.
				missing := strings.Replace(help, "      "+flag+" <VALUE>", "      "+flag+"-old <VALUE>\n          Missing option was "+flag, 1)
				if err := validateOpenOMSIHelp(missing, dedicated, multiplayer); err == nil || !strings.Contains(err.Error(), flag) {
					t.Fatalf("missing %s was accepted: %v", flag, err)
				}
			}
		}
	}
	if err := validateOpenOMSIHelp(compatibleOpenOMSIHelp(false, false), false, true); err == nil {
		t.Fatal("single-player CLI incorrectly passed the company multiplayer check")
	}
}

func TestOpenOMSIPluginHostMarkerAcrossReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openomsi.exe")
	// Split the marker across the streamed read boundary, like a large PE.
	data := append(bytes.Repeat([]byte{'x'}, (32<<10)-5), []byte("OMSI_PLUGIN_HOST32")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if found, err := openOMSIPluginHostOverride(path); err != nil || !found {
		t.Fatal("plugin host support was missed at a read boundary", found, err)
	}
	if err := os.WriteFile(path, []byte("OMSI_PLUGIN_HOST64"), 0600); err != nil {
		t.Fatal(err)
	}
	if found, err := openOMSIPluginHostOverride(path); err != nil || found {
		t.Fatal("unsupported plugin host override was accepted", found, err)
	}
}

func TestOpenOMSIProbeOutputIsBounded(t *testing.T) {
	var output openOMSIProbeOutput
	if n, err := output.Write(bytes.Repeat([]byte{'x'}, 128<<10)); n != 128<<10 || err != nil {
		t.Fatal(n, err)
	}
	if n, err := output.Write([]byte("x")); n != 0 || err == nil || !output.tooLarge || output.Len() != 128<<10 {
		t.Fatal("oversized executable output was not bounded", n, err, output.Len())
	}
}

func TestCompanyProfilesKeepReleaseHintWithoutRestrictingPlayers(t *testing.T) {
	_, profile, _, _ := companyFixture(t)
	for _, version := range []string{"0.2.0", "0.2.11", "0.2.42", "1.0.0-beta.1"} {
		profile.OpenOMSIVersion = version
		if err := validateCompanyProfile(profile); err != nil {
			t.Fatalf("valid descriptive release %s rejected: %v", version, err)
		}
	}
	profile.Protocol = multiplayerProtocol + 1
	if err := validateCompanyProfile(profile); err == nil {
		t.Fatal("an unsupported company protocol was accepted")
	}
	profile.Protocol = multiplayerProtocol
	profile.OpenOMSIVersion = "anything"
	if err := validateCompanyProfile(profile); err == nil {
		t.Fatal("malformed descriptive version was accepted")
	}
}
