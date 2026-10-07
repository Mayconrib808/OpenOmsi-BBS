package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupFormPreservesOtherSettingsAndAcceptsExecutableFolder(t *testing.T) {
	root := t.TempDir()
	executableFolder := t.TempDir()
	previous := defaultConfig()
	previous.Date, previous.BCSMarkerWaitMS, previous.Traffic = "2026-10-07", 3456, 41
	previous.CompanyID, previous.CompanyProfileSource = "transfort-br", "https://example.com/company.json"
	c, err := setupFormConfig(previous, "\""+root+"\"", executableFolder, " MayconRib808 ", true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != root || c.OpenOMSI != filepath.Join(executableFolder, "openomsi.exe") || c.PlayerName != "MayconRib808" {
		t.Fatalf("unexpected form conversion: %+v", c)
	}
	if c.Date != previous.Date || c.BCSMarkerWaitMS != 3456 || c.Traffic != 41 || c.CompanyID != previous.CompanyID {
		t.Fatal("form reset unrelated configuration")
	}
	if setupFormProfileSource(c) != previous.CompanyProfileSource {
		t.Fatal("form exposes imported cache instead of administrator source")
	}
}
func TestSetupFormRejectsInvalidMultiplayerNameWithoutChangingPreviousConfig(t *testing.T) {
	previous := defaultConfig()
	previous.Root = t.TempDir()
	previous.OpenOMSI = filepath.Join(t.TempDir(), "openomsi.exe")
	for _, player := range []string{"", "bad|name", "bad\nname", "012345678901234567890123456789012"} {
		c, err := setupFormConfig(previous, previous.Root, previous.OpenOMSI, player, true)
		if err == nil {
			t.Fatalf("accepted invalid player %q", player)
		}
		if c != previous {
			t.Fatal("invalid form modified previous configuration")
		}
	}
	if _, err := setupFormConfig(previous, previous.Root, previous.OpenOMSI, "", false); err != nil {
		t.Fatal("single player requires a multiplayer name", err)
	}
}
func TestSetupRegistryStateDistinguishesCurrentOldAndPartialActivation(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	c := defaultConfig()
	c.Root = root
	c.Multiplayer, c.CompanyID, c.PlayerName = true, "transfort-br", "MayconRib808"
	r := &memoryRegistry{values: map[string]registryValue{}}
	set := func(view int, debugger string) {
		r.values[r.k(view, ifeoBridge, "Debugger")] = stringValue(debugger)
		r.values[r.k(view, ifeoBridge, "FilterFullPath")] = stringValue(filepath.Join(root, "Omsi.exe"))
		r.values[r.k(view, ifeoParent, "UseFilter")] = dwordValue(1)
	}
	set(64, "\""+bridgePath(dir)+"\"")
	if state, err := setupRegistryState(dir, c, r); err != nil || !strings.HasPrefix(state, "Ativação incompleta") {
		t.Fatalf("partial: %q %v", state, err)
	}
	set(32, "\""+bridgePath(dir)+"\"")
	state, err := setupRegistryState(dir, c, r)
	if err != nil || !strings.HasPrefix(state, "Ponte ativada.") {
		t.Fatalf("active: %q %v", state, err)
	}
	old := filepath.Join(t.TempDir(), "app", "OpenOMSI_BCS_Bridge.exe")
	set(64, "\""+old+"\"")
	set(32, "\""+old+"\"")
	state, err = setupRegistryState(dir, c, r)
	if err != nil || !strings.HasPrefix(state, "Uma versão") {
		t.Fatalf("old: %q %v", state, err)
	}
}
func TestSetupFormProfileSourceFallsBackToImportedProfile(t *testing.T) {
	c := defaultConfig()
	c.CompanyProfile = filepath.Join(t.TempDir(), "company.json")
	if err := os.WriteFile(c.CompanyProfile, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if setupFormProfileSource(c) != c.CompanyProfile {
		t.Fatal("legacy profile was lost")
	}
}

func TestSetupLanguageAndOptionalMultiplayerSurvivePackageUpdate(t *testing.T) {
	for _, lang := range []string{"pt", "en", "de"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", t.TempDir())
			previous := defaultConfig()
			previous.Language = lang
			previous.CompanyID, previous.CompanyProfileSource = "saved-company", "https://example.invalid/profile.json"
			c, err := setupFormConfig(previous, t.TempDir(), filepath.Join(t.TempDir(), "openomsi.exe"), "", false)
			if err != nil {
				t.Fatal("multiplayer-off form rejected empty name", err)
			}
			if err = saveInstalledConfig(t.TempDir(), c); err != nil {
				t.Fatal(err)
			}
			saved := readInstalledConfig(t.TempDir())
			if saved.Language != lang || saved.Multiplayer || saved.PlayerName != "" || saved.CompanyProfileSource != previous.CompanyProfileSource {
				t.Fatalf("settings lost across update: %+v", saved)
			}
			r := &memoryRegistry{values: map[string]registryValue{}}
			state, err := setupRegistryState(t.TempDir(), saved, r)
			prefix := localText(lang, "Ponte desativada.", "Bridge inactive.", "Bridge deaktiviert.")
			if err != nil || !strings.HasPrefix(state, prefix) {
				t.Fatalf("status did not use saved language: %q, %v", state, err)
			}
			_, err = setupFormConfig(saved, "", saved.OpenOMSI, "", false)
			prefix = localText(lang, "Escolha a pasta", "Choose the OMSI", "Wähle den OMSI")
			if err == nil || !strings.HasPrefix(err.Error(), prefix) {
				t.Fatalf("form error did not use selected language: %v", err)
			}
		})
	}
}
