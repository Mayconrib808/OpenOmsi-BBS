package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Exercise two real child environments concurrently: the native Settings::path
// lookup must reach each map's settings even if the parent already has HOME set.
func TestCompanyHostPopulationIsIndependentAcrossConcurrentProcesses(t *testing.T) {
	user := t.TempDir()
	original := []byte("version=2\r\npax_density=2.0\r\nfullscreen=0\r\nai_max_humans=200\r\n")
	originalPath := filepath.Join(user, ".openomsi", "settings.cfg")
	if err := writeCompanyHostFileAtomic(originalPath, original); err != nil {
		t.Fatal(err)
	}
	parent := []string{"HOME=" + user, "USERPROFILE=" + t.TempDir(), "OMSI_CONTENT=unverified", "PATH=" + os.Getenv("PATH")}
	before := append([]string(nil), parent...)
	var wg sync.WaitGroup
	root := t.TempDir()
	for _, item := range []struct {
		mapID, budget string
		limit         int
	}{{"berlin", "700", 700}, {"carrao", "1800", 1800}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			config := filepath.Join(root, item.mapID, "server.cfg")
			profile, err := prepareCompanyHostPopulation(parent, config, item.limit)
			if err != nil {
				t.Error(err)
				return
			}
			child := exec.Command(os.Args[0], "-test.run=^TestCompanyHostPopulationProcessFixture$")
			child.Env = companyHostPopulationEnvironment(companyHostEnvironment(parent, "verified-content"), profile)
			child.Env = append(child.Env, "OPENOMSI_POPULATION_FIXTURE="+item.budget, "OPENOMSI_POPULATION_EXPECTED_PROFILE="+profile)
			if output, err := child.CombinedOutput(); err != nil {
				t.Errorf("%s native settings lookup: %v: %s", item.mapID, err, output)
			}
		}()
	}
	wg.Wait()
	actual, err := os.ReadFile(originalPath)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("the user's settings changed", err)
	}
	if strings.Join(parent, "\n") != strings.Join(before, "\n") {
		t.Fatal("the agent's environment was changed")
	}
}

func TestCompanyHostPopulationProcessFixture(t *testing.T) {
	limit := os.Getenv("OPENOMSI_POPULATION_FIXTURE")
	if limit == "" {
		t.Skip("child process fixture")
	}
	if os.Getenv("HOME") != "" || os.Getenv("USERPROFILE") != os.Getenv("OPENOMSI_POPULATION_EXPECTED_PROFILE") {
		t.Fatal("the native profile lookup would use another map or the user's profile")
	}
	text, err := os.ReadFile(filepath.Join(os.Getenv("USERPROFILE"), ".openomsi", "settings.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(text, []byte("\nai_max_humans="+limit+"\n")) || !bytes.Contains(text, []byte("pax_density=2.0\r\nfullscreen=0")) {
		t.Fatal("the map's budget or the existing density/window setting was lost")
	}
	if os.Getenv("OMSI_CONTENT") != "verified-content" || os.Getenv("OMSI_NO_LAN_MODS") != "1" {
		t.Fatal("content isolation was lost")
	}
}

func TestCompanyHostPopulationMissingSettingsUseNativeDefaults(t *testing.T) {
	profile, err := prepareCompanyHostPopulation([]string{"USERPROFILE=" + t.TempDir()}, filepath.Join(t.TempDir(), "server.cfg"), 1000)
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(profile, ".openomsi", "settings.cfg"))
	if err != nil || string(text) != "\nai_max_humans=1000\n" {
		t.Fatal("defaults were not retained", err)
	}
}
