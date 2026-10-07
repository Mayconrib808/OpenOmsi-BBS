package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func companyFixture(t *testing.T) (Config, CompanyProfile, MultiplayerTrip, string) {
	t.Helper()
	dir, root := t.TempDir(), t.TempDir()
	assets := map[string]string{
		"maps/Sample/global.cfg":         "[name]\nSample\n",
		"maps/Sample/TTData/1.ttl":       "test timetable",
		"Vehicles/A/a.bus":               "test bus A",
		"Vehicles/A/script/main.osc":     "test script A",
		"Vehicles/A/Texture/Company.dds": "test repaint A",
		"Vehicles/B/b.bus":               "test bus B",
	}
	for path, text := range assets {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := CompanyProfile{SchemaVersion: 1, CompanyID: "company-a", CompanyName: "Company A", OpenOMSIVersion: multiplayerGameVersion, Protocol: multiplayerProtocol}
	for i, folder := range []string{"maps/Sample", "Vehicles/A", "Vehicles/B"} {
		files, err := snapshotCompanyFolder(root, folder)
		if err != nil {
			t.Fatal(err)
		}
		p.Packages = append(p.Packages, CompanyPackage{ID: []string{"map", "bus-a", "bus-b"}[i], Name: folder, Version: "1.0", DownloadURL: "https://example.invalid/" + []string{"map", "bus-a", "bus-b"}[i], Files: files, Folders: []string{folder}})
	}
	p.Sessions = []CompanySession{{ID: "morning", Name: "Morning", MapName: "Sample", MapFile: "maps/Sample/global.cfg", Date: "2026-10-06", ServerURL: "http://127.0.0.1:27025", RequiredPackages: []string{"map", "bus-a", "bus-b"}, ClockToleranceSec: 180}}
	c := defaultConfig()
	c.Root, c.Multiplayer, c.CompanyID, c.CompanyProfile, c.PlayerName = root, true, p.CompanyID, "company.json", "Maycon"
	trip := MultiplayerTrip{"Sample", "maps/Sample/global.cfg", "Vehicles/A/a.bus", "2026-10-06", "09:20"}
	return c, p, trip, dir
}

func saveTestCompany(t *testing.T, dir string, p CompanyProfile) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "company.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompanyProfileRejectsUnsafeOrInconsistentRequirements(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*CompanyProfile)
	}{
		{"path traversal", func(p *CompanyProfile) { p.Packages[0].Files[0].Path = "maps/Sample/../../secret.txt" }},
		{"absolute path", func(p *CompanyProfile) { p.Packages[0].Files[0].Path = "C:/Users/secret.txt" }},
		{"nonasset folder", func(p *CompanyProfile) { p.Packages[0].Files[0].Path = "Drivers/bbs.odr" }},
		{"executable", func(p *CompanyProfile) { p.Packages[0].Files[0].Path = "Vehicles/A/run.exe" }},
		{"script download URL", func(p *CompanyProfile) { p.Packages[0].DownloadURL = "javascript:alert(1)" }},
		{"unencrypted remote link", func(p *CompanyProfile) { p.Packages[0].DownloadURL = "http://example.invalid/file" }},
		{"credentials in link", func(p *CompanyProfile) { p.Packages[0].DownloadURL = "https://user:password@example.invalid/file" }},
		{"invalid digest", func(p *CompanyProfile) { p.Packages[0].Files[0].SHA256 = "guess" }},
		{"duplicate case-insensitive file", func(p *CompanyProfile) {
			f := p.Packages[0].Files[0]
			f.Path = strings.ToUpper(f.Path)
			p.Packages[0].Files = append(p.Packages[0].Files, f)
		}},
		{"uncovered map", func(p *CompanyProfile) { p.Sessions[0].RequiredPackages = []string{"bus-a"} }},
		{"unknown package", func(p *CompanyProfile) {
			p.Sessions[0].RequiredPackages = append(p.Sessions[0].RequiredPackages, "other")
		}},
		{"another protocol", func(p *CompanyProfile) { p.Protocol = 5 }},
		{"malformed release label", func(p *CompanyProfile) { p.OpenOMSIVersion = "not-openomsi" }},
		{"ambiguous date", func(p *CompanyProfile) { p.Sessions[0].Date = "auto" }},
		{"tolerance hides wrong hour", func(p *CompanyProfile) { p.Sessions[0].ClockToleranceSec = 3600 }},
		{"server query", func(p *CompanyProfile) { p.Sessions[0].ServerURL = "https://example.invalid/?target=another" }},
		{"server path", func(p *CompanyProfile) { p.Sessions[0].ServerURL = "https://example.invalid/status" }},
		{"unknown schema", func(p *CompanyProfile) { p.SchemaVersion = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, p, _, _ := companyFixture(t)
			tc.change(&p)
			if err := validateCompanyProfile(p); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestCompanyPackagesIdentifyMissingBusScriptAndRepaintVersion(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal(got)
	}
	if err := os.Remove(filepath.Join(c.Root, "Vehicles/B/b.bus")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/script/main.osc"), []byte("another version"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/Texture/Company.dds"), []byte("another repaint"), 0644); err != nil {
		t.Fatal(err)
	}
	problems := checkCompanyPackages(c.Root, p, p.Sessions[0])
	if len(problems) != 2 {
		t.Fatal(problems)
	}
	if problems[0].DownloadURL != "https://example.invalid/bus-a" || !strings.Contains(problems[0].Detail, "main.osc") || !strings.Contains(problems[0].Detail, "Company.dds") {
		t.Fatal(problems[0])
	}
	if problems[1].DownloadURL != "https://example.invalid/bus-b" || !strings.Contains(problems[1].Detail, "b.bus") {
		t.Fatal(problems[1])
	}
}

func TestCompanyAssetsResolveWindowsCaseAndStayInsideRoot(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	for i := range p.Packages {
		for j := range p.Packages[i].Files {
			p.Packages[i].Files[j].Path = strings.ToUpper(strings.ReplaceAll(p.Packages[i].Files[j].Path, "/", `\`))
		}
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal(got)
	}
	outside := filepath.Join(t.TempDir(), "external.bus")
	os.WriteFile(outside, []byte("external"), 0644)
	link := filepath.Join(c.Root, "Vehicles/A/linked.bus")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	if _, err := companyAssetPath(c.Root, "Vehicles/A/linked.bus"); err == nil {
		t.Fatal("escaped the game root")
	}
}

func TestCompanyProfileReadRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	_, p, _, dir := companyFixture(t)
	b, _ := json.Marshal(p)
	for _, bad := range []string{string(b) + "{}", strings.Replace(string(b), `"schema_version":1`, `"schema_version":1,"typo":true`, 1)} {
		os.WriteFile(filepath.Join(dir, "company.json"), []byte(bad), 0644)
		if _, err := loadCompanyProfile(context.Background(), "company.json", dir, companyHTTPClient()); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	b = append([]byte{0xef, 0xbb, 0xbf}, b...)
	os.WriteFile(filepath.Join(dir, "company.json"), b, 0644)
	if _, err := loadCompanyProfile(context.Background(), "company.json", dir, companyHTTPClient()); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyWizardExcludesMutableHOFAndExecutables(t *testing.T) {
	c, _, _, _ := companyFixture(t)
	for _, name := range []string{"selected.hof", "laststn.osn", "run.exe", "account.log"} {
		os.WriteFile(filepath.Join(c.Root, "Vehicles/A", name), []byte("not a packaged requirement"), 0644)
	}
	files, err := snapshotCompanyFolder(c.Root, "Vehicles/A")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatal(files)
	}
}

func TestCompanySavedMapSituationIsNotAStaticFleetRequirement(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	if err := os.WriteFile(filepath.Join(c.Root, "maps/Sample/laststn.osn"), []byte("this player's saved bus and paint"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestCompanyExtraFilesDoNotBlockMultiplayer(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	for path, content := range map[string]string{
		"Vehicles/A/Texture/extra.cti":        "extra repaint",
		"Vehicles/A/script/IBIS_constfile.txt": "local script companion",
		"Vehicles/A/Vip 5 AI.bus":              "extra AI bus variant",
	} {
		full := filepath.Join(c.Root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal("undeclared local files blocked multiplayer", got)
	}
}

func TestCompanyExtraFilesStayIgnoredThroughRootAlias(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	alias := filepath.Join(t.TempDir(), "OMSI-alias")
	if err := os.Symlink(c.Root, alias); err != nil {
		t.Skip("root aliases unavailable", err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/Texture/extra.cti"), []byte("extra repaint"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := checkCompanyPackages(alias, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal("extra file blocked through canonical-root spelling", got)
	}
}

func TestCompanyWindowsMetadataIsNotPackageIdentity(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	path := filepath.Join(c.Root, "Vehicles/A/Texture/Thumbs.db")
	if err := os.WriteFile(path, []byte("machine-local thumbnail cache"), 0644); err != nil {
		t.Fatal(err)
	}
	digest, err := companyFileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Packages[1].Files = append(p.Packages[1].Files, CompanyFile{Path: "Vehicles/A/Texture/Thumbs.db", SHA256: digest})
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal("legacy profile treated Thumbs.db as required content", got)
	}
	files, err := snapshotCompanyFolder(c.Root, "Vehicles/A")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.EqualFold(filepath.Base(file.Path), "Thumbs.db") {
			t.Fatal("new inventory included Windows metadata")
		}
	}
}

func TestCompanyTimezoneDoubleExtensionCompatibilityAlias(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	canonical := filepath.Join(c.Root, "maps/Sample/timezone.txt")
	if err := os.WriteFile(canonical, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	digest, err := companyFileHash(canonical)
	if err != nil {
		t.Fatal(err)
	}
	p.Packages[0].Files = append(p.Packages[0].Files, CompanyFile{Path: "maps/Sample/timezone.txt", SHA256: digest})
	if err := os.Rename(canonical, canonical+".txt"); err != nil {
		t.Fatal(err)
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal("timezone.txt.txt with identical bytes was rejected", got)
	}
	if err := os.WriteFile(canonical+".txt", []byte("different timezone\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 1 || !strings.Contains(got[0].Detail, "timezone.txt") {
		t.Fatal("changed timezone alias was silently accepted", got)
	}
}

func TestCompanyDownloadReportEscapesOwnerTextAndUsesConfiguredLink(t *testing.T) {
	dir := t.TempDir()
	path, err := writeCompanyReport(dir, "pt", `<script>company</script>`, []CompanyProblem{{`<img onerror=alert(1)>`, "missing a.bus", `https://example.invalid/download?a=1&b=2`}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if strings.Contains(s, "<script>") || strings.Contains(s, "<img onerror") || !strings.Contains(s, "https://example.invalid/download?a=1&amp;b=2") {
		t.Fatal("report did not escape text or retain the owner's link")
	}
	if !strings.Contains(s, "Content-Security-Policy") || !strings.Contains(s, "A viagem não foi aberta") {
		t.Fatal("incomplete report")
	}
}

func TestCompanyConfigurationIsOneTimeAndOptional(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	c, p, _, dir := companyFixture(t)
	os.MkdirAll(appDir(dir), 0755)
	profile := saveTestCompany(t, dir, p)
	c.Multiplayer = false
	c.CompanyID = ""
	c.CompanyProfile = ""
	c.PlayerName = ""
	u := setupUI{bufio.NewScanner(strings.NewReader("1\n" + profile + "\nMaycon\ns\n")), "pt", dir}
	got, err := u.configureCompany(c)
	if err != nil {
		t.Fatal(err)
	}
	saved := readConfig(configPath(dir))
	if !got.Multiplayer || saved.CompanyID != p.CompanyID || saved.PlayerName != "Maycon" || saved.CompanyProfileSource != profile || saved.CompanyProfile == profile {
		t.Fatal(saved)
	}
	u.input = bufio.NewScanner(strings.NewReader("2\n"))
	got, err = u.configureCompany(saved)
	if err != nil || got.Multiplayer || readConfig(configPath(dir)).Multiplayer {
		t.Fatal(got, err)
	}
	if got.Root != c.Root || got.CompanyProfile != saved.CompanyProfile || got.CompanyProfileSource != profile {
		t.Fatal("disable discarded unrelated settings")
	}
}

func TestCompanyOwnerWizardWritesReusableProfileWithoutMods(t *testing.T) {
	c, _, _, dir := companyFixture(t)
	input := "my-company\nMinha Empresa\nManhã\nmaps/Sample/global.cfg\nSample\n2026-10-06\nhttp://127.0.0.1:27025\n1.0\nhttps://example.invalid/map\nVehicles/A/a.bus\nApache\n1.2\nhttps://example.invalid/bus\n0\n0\nn\n"
	u := setupUI{bufio.NewScanner(strings.NewReader(input)), "pt", dir}
	path, err := u.createCompany(c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCompanyProfile(context.Background(), path, dir, companyHTTPClient())
	if err != nil || p.CompanyID != "my-company" || len(p.Packages) != 2 || p.Packages[1].DownloadURL != "https://example.invalid/bus" {
		t.Fatal(p, err)
	}
	if p.Clock != nil || p.Sessions[0].Date != "2026-10-06" {
		t.Fatal("fixed-date wizard flow unexpectedly enabled the company clock", p)
	}
	if got := checkCompanyPackages(c.Root, p, p.Sessions[0]); len(got) != 0 {
		t.Fatal(got)
	}
	if len(p.Packages[1].Files) != 3 {
		t.Fatal("wizard did not include script and repaint")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("wizard wrote mod files")
	}
}

func TestCompanyOwnerWizardRetriesInvalidInputAndReusesCompanyClock(t *testing.T) {
	c, _, _, dir := companyFixture(t)
	input := strings.Join([]string{
		"Transfort - BR", "transfort-br", "Transfort - BR",
		"Manhã", "maps/Sample/global.cfg", "Sample", "2026-02-30", "company",
		"-25", "texto", "-8", "http://127.0.0.1:27025",
		"1.0", "", "Vehicles/A/a.bus", "Apache", "1.2", "", "0", "0", "s",
		"Noite", "maps/Sample/global.cfg", "Sample", "company", "http://127.0.0.1:27026",
		"1.0", "", "Vehicles/B/b.bus", "Outro ônibus", "1.0", "", "0", "0", "n", "",
	}, "\n")
	u := setupUI{bufio.NewScanner(strings.NewReader(input)), "pt", dir}
	path, err := u.createCompany(c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCompanyProfile(context.Background(), path, dir, companyHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	if p.CompanyID != "transfort-br" || p.CompanyName != "Transfort - BR" || p.Clock == nil || p.Clock.TimeZone != "Europe/Berlin" || p.Clock.ShiftMinutes != -480 {
		t.Fatal("wizard lost the display name or the configured BCS shift", p)
	}
	if len(p.Sessions) != 2 || p.Sessions[0].Date != "company" || p.Sessions[1].Date != "company" || p.Sessions[1].ServerURL != "http://127.0.0.1:27026" {
		t.Fatal("company clock was not reused for the next session", p.Sessions)
	}
	for _, session := range p.Sessions {
		if got := checkCompanyPackages(c.Root, p, session); len(got) != 0 {
			t.Fatal(got)
		}
	}
}

func TestCompanyServerUsesSupportedProtocolInsteadOfReleaseAllowlist(t *testing.T) {
	for _, version := range []string{"0.2.0", "0.2.11", "0.2.9", "1.0.0-beta.1", "0.2.0 (test-build)", "0.1.0", "0.1.0 (0874dac 2026-10-07 04:13)", "0.1.0 (538ad31 2026-10-05 23:39)", "0.1.0 (538ad31)", "0.1.0 (538ad31b2a2c664cb0726db2547bf6238411eedf)"} {
		if !compatibleCompanyServer(version, 6) {
			t.Errorf("supported protocol rejected because of release label: %s", version)
		}
		if compatibleCompanyServer(version, 7) {
			t.Errorf("wrong protocol accepted: %s", version)
		}
	}
	for _, version := range []string{"", "not openOMSI", "0.2", "0.1.0 (538ad31) extra", "0.1.0 (bad\nlabel)", "0.1.0 (bad\x00label)", "0.1.0 (nested (label))"} {
		if compatibleCompanyServer(version, 6) {
			t.Errorf("invalid server description accepted: %s", version)
		}
	}
	_, profile, trip, _ := companyFixture(t)
	status := companyHostStatus{Name: "Official server", Map: trip.MapFile, Version: "0.1.0 (538ad31 2026-10-05 23:39)", Protocol: 6, Time: trip.Start, MaxPlayers: 16, Vehicles: []byte(`"Vehicles/A/a.bus;Vehicles/B/b.bus"`)}
	if err := validateCompanyHostStatus(status, profile.Sessions[0], []string{"Vehicles/A/a.bus", "Vehicles/B/b.bus"}); err != nil {
		t.Fatal(err)
	}
	joined := companyServerStatus{Name: status.Name, Map: status.Map, Version: status.Version, Protocol: status.Protocol, Time: status.Time, MaxPlayers: status.MaxPlayers, Vehicles: status.Vehicles}
	if err := validateCompanyServer(joined, profile.Sessions[0], trip); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyOwnerRefreshUpdatesOnlyDeclaredHashesAndPreservesMetadata(t *testing.T) {
	c, original, _, _ := companyFixture(t)
	assets := map[string]string{
		"maps/Sample/Holidays.txt": "2026-10-06\n",
		"Vehicles/A/script/cockpit_varlist.txt": "existing_var\n",
	}
	for path, content := range assets {
		if err := os.WriteFile(filepath.Join(c.Root, filepath.FromSlash(path)), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for i, folder := range []string{"maps/Sample", "Vehicles/A"} {
		files, err := snapshotCompanyFolder(c.Root, folder)
		if err != nil {
			t.Fatal(err)
		}
		original.Packages[i].Files = files
	}
	original.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}
	original.Sessions[0].Date = "company"
	before, _ := json.Marshal(original)
	for _, path := range []string{"maps/Sample/Holidays.txt", "Vehicles/A/script/main.osc", "Vehicles/A/script/cockpit_varlist.txt"} {
		if err := os.WriteFile(filepath.Join(c.Root, filepath.FromSlash(path)), []byte("changed local content\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if problems := checkCompanyPackages(c.Root, original, original.Sessions[0]); len(problems) != 2 {
		t.Fatal("joining must still refuse unapproved changed scripts and calendar", problems)
	}
	updated, changed, err := refreshCompanyHashes(c.Root, original)
	if err != nil || len(changed) != 3 {
		t.Fatal(changed, err)
	}
	if problems := checkCompanyPackages(c.Root, updated, updated.Sessions[0]); len(problems) != 0 {
		t.Fatal(problems)
	}
	after, _ := json.Marshal(original)
	if !bytes.Equal(before, after) {
		t.Fatal("refresh mutated the original profile before administrator approval")
	}
	if updated.CompanyID != original.CompanyID || updated.CompanyName != original.CompanyName ||
		!reflect.DeepEqual(updated.Clock, original.Clock) || !reflect.DeepEqual(updated.Sessions, original.Sessions) {
		t.Fatal("refresh changed company, clock or session configuration")
	}
	for i, pkg := range updated.Packages {
		if len(pkg.Files) != len(original.Packages[i].Files) {
			t.Fatal("changed the declared inventory")
		}
		for j, file := range pkg.Files {
			if file.Path != original.Packages[i].Files[j].Path {
				t.Fatal("changed a declared path")
			}
		}
		pkg.Files = original.Packages[i].Files
		if !reflect.DeepEqual(pkg, original.Packages[i]) {
			t.Fatal("changed package metadata or download links")
		}
	}
	// A subsequent content change must still fail; refreshing is not an ignore rule.
	os.WriteFile(filepath.Join(c.Root, "Vehicles/A/script/main.osc"), []byte("another modification"), 0644)
	if problems := checkCompanyPackages(c.Root, updated, updated.Sessions[0]); len(problems) != 1 {
		t.Fatal("new script modifications were silently trusted", problems)
	}
}

func TestCompanyOwnerRefreshRefusesMissingButAllowsUndeclaredFiles(t *testing.T) {
	c, original, _, _ := companyFixture(t)
	before, _ := json.Marshal(original)
	os.Remove(filepath.Join(c.Root, "Vehicles/A/script/main.osc"))
	if _, _, err := refreshCompanyHashes(c.Root, original); err == nil {
		t.Fatal("refresh accepted a missing declared file")
	}
	after, _ := json.Marshal(original)
	if !bytes.Equal(before, after) {
		t.Fatal("failed refresh changed the original profile")
	}

	c, original, _, _ = companyFixture(t)
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/script/unregistered.osc"), []byte("new script"), 0644); err != nil {
		t.Fatal(err)
	}
	updated, _, err := refreshCompanyHashes(c.Root, original)
	if err != nil {
		t.Fatal("refresh rejected an unrelated extra file", err)
	}
	if len(updated.Packages[1].Files) != len(original.Packages[1].Files) {
		t.Fatal("hash-only refresh silently added undeclared files")
	}
}

func TestCompanyOwnerRefreshKeepsSharedPackageHashesConsistent(t *testing.T) {
	c, original, _, _ := companyFixture(t)
	original.Packages[1].Folders = append(original.Packages[1].Folders, original.Packages[0].Folders...)
	original.Packages[1].Files = append(original.Packages[1].Files, original.Packages[0].Files...)
	path := "maps/Sample/global.cfg"
	os.WriteFile(filepath.Join(c.Root, filepath.FromSlash(path)), []byte("[name]\nSample updated\n"), 0644)
	updated, changed, err := refreshCompanyHashes(c.Root, original)
	if err != nil || len(changed) != 1 || changed[0] != path {
		t.Fatal(changed, err)
	}
	if err := validateCompanyProfile(updated); err != nil {
		t.Fatal("shared hashes diverged", err)
	}
}

func TestCompanyProfileRefreshWritesExactBackupAndRejectsConcurrentProfileChanges(t *testing.T) {
	c, original, _, dir := companyFixture(t)
	path := saveTestCompany(t, dir, original)
	raw, _ := os.ReadFile(path)
	raw = append([]byte{0xef, 0xbb, 0xbf}, raw...)
	os.WriteFile(path, raw, 0644)
	os.WriteFile(filepath.Join(c.Root, "Vehicles/A/script/main.osc"), []byte("owner's new reference"), 0644)
	updated, _, err := refreshCompanyHashes(c.Root, original)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := saveCompanyProfileRefresh(path, raw, updated)
	if err != nil {
		t.Fatal(err)
	}
	savedBackup, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(savedBackup, raw) {
		t.Fatal("previous JSON was not preserved byte for byte", err)
	}
	got, err := loadCompanyProfile(context.Background(), path, dir, nil)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatal("saved reference differs from the reviewed profile", got, err)
	}
	expectedCurrent, _ := os.ReadFile(path)
	changedProfile := append(append([]byte(nil), expectedCurrent...), ' ')
	os.WriteFile(path, changedProfile, 0644)
	if _, err := saveCompanyProfileRefresh(path, expectedCurrent, updated); err == nil {
		t.Fatal("concurrently changed profile was overwritten")
	}
	gotBytes, _ := os.ReadFile(path)
	if !bytes.Equal(gotBytes, changedProfile) {
		t.Fatal("failed update changed the JSON")
	}
	backups, _ := filepath.Glob(path + ".backup-*")
	if len(backups) != 1 {
		t.Fatal("failed update wrote another backup", backups)
	}
}

func TestCompanyRefreshWizardCancellationLeavesProfileAndConfigurationUntouched(t *testing.T) {
	c, original, _, dir := companyFixture(t)
	profile := saveTestCompany(t, dir, original)
	c.CompanyProfile = profile
	before, _ := os.ReadFile(profile)
	os.WriteFile(filepath.Join(c.Root, "Vehicles/A/script/main.osc"), []byte("changed script"), 0644)
	u := setupUI{bufio.NewScanner(strings.NewReader("\nn\n")), "pt", dir}
	if path, err := u.refreshCompanyProfile(c); err != nil || path != "" {
		t.Fatal(path, err)
	}
	after, _ := os.ReadFile(profile)
	backups, _ := filepath.Glob(profile + ".backup-*")
	if !bytes.Equal(before, after) || len(backups) != 0 || fileExists(configPath(dir)) {
		t.Fatal("cancelled update wrote profile, backup or configuration")
	}
	u.input = bufio.NewScanner(strings.NewReader("https://example.invalid/profile.json\n"))
	if _, err := u.refreshCompanyProfile(c); err == nil {
		t.Fatal("refresh tried to overwrite a remotely hosted profile")
	}
}
