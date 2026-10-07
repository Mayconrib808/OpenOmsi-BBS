package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestCompanySituationCompanionsAreIgnoredWithoutIgnoringMapContent(t *testing.T) {
	c, profile, _, _ := companyFixture(t)
	generated := []string{"laststn.osn.owt", "laststn.osn_0_0.dds", "laststn.osn_-2_12.dds", "Trip A.osn.owt", "timezone.txt.backup.txt"}
	for _, name := range generated {
		path := "maps/Sample/" + name
		writeTestFile(t, filepath.Join(c.Root, filepath.FromSlash(path)), "local situation")
		// Simulate a dev.1 profile that mistakenly recorded the file.
		digest, _ := companyFileHash(filepath.Join(c.Root, filepath.FromSlash(path)))
		profile.Packages[0].Files = append(profile.Packages[0].Files, CompanyFile{path, digest})
	}
	if err := validateCompanyProfile(profile); err != nil {
		t.Fatal("old generated-file profiles must remain readable", err)
	}
	for _, name := range generated {
		if err := os.Remove(filepath.Join(c.Root, "maps", "Sample", name)); err != nil {
			t.Fatal(err)
		}
	}
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 0 {
		t.Fatal("deleted personal situations blocked joining", got)
	}
	clean, changed, err := refreshCompanyHashes(c.Root, profile)
	if err != nil || len(changed) != len(generated) {
		t.Fatal(changed, err)
	}
	for _, name := range generated {
		writeTestFile(t, filepath.Join(c.Root, "maps", "Sample", name), "another save")
	}
	if got := checkCompanyPackages(c.Root, clean, clean.Sessions[0]); len(got) != 0 {
		t.Fatal("new personal situations blocked joining", got)
	}
	files, err := snapshotCompanyFolder(c.Root, "maps/Sample")
	if err != nil || len(files) != len(clean.Packages[0].Files) {
		t.Fatal("snapshot registered a personal situation", files, err)
	}
	for _, path := range []string{"maps/Sample/timezone.txt", "maps/Sample/real.dds", "maps/Sample/laststn.osn_anything.dds", "Vehicles/A/Texture/laststn.osn_0_0.dds"} {
		if companyRuntimeArtifact(path) {
			t.Fatal("real content classified as a runtime artifact", path)
		}
	}
	writeTestFile(t, filepath.Join(c.Root, "maps/Sample/timezone.txt"), "[timezone]\n1\n")
	if got := checkCompanyPackages(c.Root, clean, clean.Sessions[0]); len(got) != 1 || !strings.Contains(got[0].Detail, "timezone.txt") {
		t.Fatal("timezone must still require registration", got)
	}
}

func TestCompanyBBSChangesRequireRecordedMatchingOriginals(t *testing.T) {
	c, profile, _, _ := companyFixture(t)
	calendar := "maps/Sample/Holidays.txt"
	script := "Vehicles/A/script/main.osc"
	variables := "Vehicles/A/script/cockpit_varlist.txt"
	writeTestFile(t, filepath.Join(c.Root, calendar), "[holiday]\n20150101\nOriginal calendar\n")
	writeTestFile(t, filepath.Join(c.Root, variables), "original_variable\n")
	for i, folder := range []string{"maps/Sample", "Vehicles/A"} {
		files, err := snapshotCompanyFolder(c.Root, folder)
		if err != nil {
			t.Fatal(err)
		}
		profile.Packages[i].Files = files
	}
	before, _ := json.Marshal(profile)
	var list strings.Builder
	for _, path := range []string{calendar, script, variables} {
		live := filepath.Join(c.Root, filepath.FromSlash(path))
		original, err := os.ReadFile(live)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, live+".backup", string(original))
		writeTestFile(t, live, "BBS session-specific content\n")
		list.WriteString(strings.ReplaceAll(live, "/", `\`) + "\r\n")
	}
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 2 {
		t.Fatal("a sibling backup alone must not waive validation", got)
	}
	writeTestFile(t, filepath.Join(c.Root, "Busbetrieb-Simulator/BBS_Backups.txt"), "\xef\xbb\xbf"+list.String())
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 0 {
		t.Fatal("registered BBS changes with exact original backups rejected", got)
	}
	if got := checkCompanyPackagesWithOriginals(c.Root, profile, profile.Sessions[0], nil); len(got) != 2 {
		t.Fatal("BBS backup hid live changes during administrator review", got)
	}
	after, _ := json.Marshal(profile)
	if !bytes.Equal(before, after) {
		t.Fatal("joining changed the company reference")
	}
	current, _ := os.ReadFile(filepath.Join(c.Root, script))
	if string(current) != "BBS session-specific content\n" {
		t.Fatal("overwrote the BBS session script")
	}
	writeTestFile(t, filepath.Join(c.Root, script)+".backup", "another bus version")
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 1 || !strings.Contains(got[0].Detail, script) {
		t.Fatal("wrong original backup accepted", got)
	}
	if err := os.Remove(filepath.Join(c.Root, calendar)); err != nil {
		t.Fatal(err)
	}
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 2 {
		t.Fatal("backup hid a missing live file", got)
	}
}

func TestCompanyBBSBackupListSupportsConfiguredPathsAndUTF16(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "BBS_Backups.txt")
	asset := "Vehicles/Ônibus/Script/cockpit.osc"
	line := "\"" + strings.ReplaceAll(filepath.Join(root, filepath.FromSlash(asset)), "/", `\`) + "\"\r\n"
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		data := make([]byte, 2)
		order.PutUint16(data, 0xfeff)
		for _, unit := range utf16.Encode([]rune(line)) {
			var encoded [2]byte
			order.PutUint16(encoded[:], unit)
			data = append(data, encoded[:]...)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if got := companyBBSBackupInventory(root, path); !got[companyAssetKey(asset)] || len(got) != 1 {
			t.Fatal("configured Unicode BBS inventory was not recognized", got)
		}
	}
}

func TestCompanyBBSBackupCannotWaiveTexturesOrTimezone(t *testing.T) {
	c, profile, _, _ := companyFixture(t)
	texture := "Vehicles/A/Texture/Company.dds"
	live := filepath.Join(c.Root, filepath.FromSlash(texture))
	original, _ := os.ReadFile(live)
	writeTestFile(t, live+".backup", string(original))
	writeTestFile(t, live, "another repaint")
	writeTestFile(t, filepath.Join(c.Root, "BBS_Backups.txt"), live+"\n../outside/script/test.osc\n")
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 1 {
		t.Fatal("backup waived a changed repaint", got)
	}
	for _, path := range []string{"maps/Sample/timezone.txt", "maps/Sample/TTData/1.ttl", "Vehicles/A/model.o3d", "Vehicles/A/Script/engine_constfile.txt"} {
		if companyBBSManagedAsset(path) {
			t.Fatal("unrelated asset eligible for BBS fallback", path)
		}
	}
}

func TestCompanyInventoryRefreshRegistersTimezoneAndDeduplicatesReports(t *testing.T) {
	c, profile, _, _ := companyFixture(t)
	duplicate := profile.Packages[1]
	duplicate.ID = "bus-a-second-model"
	profile.Packages = append(profile.Packages, duplicate)
	profile.Sessions[0].RequiredPackages = append(profile.Sessions[0].RequiredPackages, duplicate.ID)
	writeTestFile(t, filepath.Join(c.Root, "Vehicles/A/script/main.osc"), "new script version")
	if got := checkCompanyPackages(c.Root, profile, profile.Sessions[0]); len(got) != 1 {
		t.Fatal("same folder reported once per selected bus", got)
	}
	writeTestFile(t, filepath.Join(c.Root, "maps/Sample/timezone.txt"), "[timezone]\n1\n")
	before, _ := json.Marshal(profile)
	updated, changes, err := refreshCompanyInventory(c.Root, profile)
	if err != nil || len(changes) != 2 {
		t.Fatal(changes, err)
	}
	if got := checkCompanyPackages(c.Root, updated, updated.Sessions[0]); len(got) != 0 {
		t.Fatal(got)
	}
	after, _ := json.Marshal(profile)
	if !bytes.Equal(before, after) {
		t.Fatal("inventory review changed the original profile")
	}
	if updated.CompanyID != profile.CompanyID || updated.Sessions[0].ServerURL != profile.Sessions[0].ServerURL {
		t.Fatal("inventory refresh changed routing/identity")
	}
	writeTestFile(t, filepath.Join(c.Root, "maps/Sample/timezone.txt"), "[timezone]\n5\n")
	if got := checkCompanyPackages(c.Root, updated, updated.Sessions[0]); len(got) != 1 {
		t.Fatal("registered timezone stopped being checked", got)
	}
}
