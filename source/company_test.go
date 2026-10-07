package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
		{"another game", func(p *CompanyProfile) { p.OpenOMSIVersion = "0.1.7" }},
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

func TestCompanyExtraRepaintCannotChangeRemotePaintIndicesUnnoticed(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/Texture/extra.cti"), []byte("extra repaint"), 0644); err != nil {
		t.Fatal(err)
	}
	got := checkCompanyPackages(c.Root, p, p.Sessions[0])
	if len(got) != 1 || !strings.Contains(got[0].Detail, "extra.cti") || got[0].DownloadURL != "https://example.invalid/bus-a" {
		t.Fatal(got)
	}
}

func TestCompanyInventoryKeepsChosenRootWhenRootIsAnAlias(t *testing.T) {
	c, p, _, _ := companyFixture(t)
	alias := filepath.Join(t.TempDir(), "OMSI-alias")
	if err := os.Symlink(c.Root, alias); err != nil {
		t.Skip("root aliases unavailable", err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, "Vehicles/A/Texture/extra.cti"), []byte("extra repaint"), 0644); err != nil {
		t.Fatal(err)
	}
	got := checkCompanyPackages(alias, p, p.Sessions[0])
	if len(got) != 1 || !strings.Contains(got[0].Detail, "extra.cti") {
		t.Fatal("extra repaint hidden by canonical-root spelling", got)
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
	if !got.Multiplayer || saved.CompanyID != p.CompanyID || saved.PlayerName != "Maycon" || saved.CompanyProfile != profile {
		t.Fatal(saved)
	}
	u.input = bufio.NewScanner(strings.NewReader("2\n"))
	got, err = u.configureCompany(saved)
	if err != nil || got.Multiplayer || readConfig(configPath(dir)).Multiplayer {
		t.Fatal(got, err)
	}
	if got.Root != c.Root || got.CompanyProfile != profile {
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
