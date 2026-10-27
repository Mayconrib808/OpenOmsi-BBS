package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCoop15MigrationKeepsAllHostPreferencesAndChoosesMatchingServer(t *testing.T) {
	if got := defaultHostAgentConfig().Company.OpenOMSIVersion; got != "0.3.45" {
		t.Fatalf("new host advertises %s instead of the shipped version", got)
	}
	dir := t.TempDir()
	for _, custom := range []bool{false, true} {
		input := map[string]any{
			"server":            filepath.Join(dir, "app", "server-dinamico-p14", "openomsi.exe"),
			"use_custom_server": custom,
			"company":           map[string]any{"company_id": "transfort-br", "protocol": 14, "openomsi_version": "0.3.38", "opaque": "preserved"},
			"maps":              map[string]any{"carrao": map[string]any{"voice": true, "voice_radio": true, "admin_password": "local-only", "max_people": 1000}},
			"control_key":       "preserve-control-key", "future": []any{1, "keep"},
		}
		before, _ := json.Marshal(input)
		after, id, changed, err := migrateManagedCoop15Host(before, dir)
		if err != nil || !changed || id != "transfort-br" {
			t.Fatal(id, changed, err)
		}
		var got map[string]json.RawMessage
		if json.Unmarshal(after, &got) != nil {
			t.Fatal("invalid output")
		}
		var original map[string]json.RawMessage
		json.Unmarshal(before, &original)
		for _, key := range []string{"maps", "control_key", "future"} {
			var a, b any
			json.Unmarshal(original[key], &a)
			json.Unmarshal(got[key], &b)
			x, _ := json.Marshal(a)
			y, _ := json.Marshal(b)
			if !bytes.Equal(x, y) {
				t.Fatalf("lost opaque field %s", key)
			}
		}
		var company map[string]json.RawMessage
		json.Unmarshal(got["company"], &company)
		if string(company["protocol"]) != "15" || hostJSONString(company, "openomsi_version") != "0.3.45" || hostJSONString(company, "opaque") != "preserved" {
			t.Fatal(string(got["company"]))
		}
		if hostJSONString(got, "server") != filepath.Join(dir, filepath.FromSlash(managedDynamicP15Server)) {
			t.Fatal(string(got["server"]))
		}
	}
}

func TestCoop15MigrationPreservesAnIndependentlySelectedCustomServer(t *testing.T) {
	before := []byte(`{"server":"independent.exe","use_custom_server":true,"company":{"company_id":"demo","protocol":14},"maps":{"a":{"voice":true}}}`)
	after, id, changed, err := migrateManagedCoop15Host(before, t.TempDir())
	if err != nil || changed || id != "" || !bytes.Equal(before, after) {
		t.Fatal(changed, id, err)
	}
}

func TestCoop15InstallationMigratesHostClientAndCachedProfileTogether(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	hostPath := filepath.Join(managedDataDir(dir), "HostAgent.local.json")
	profilePath := filepath.Join(managedCompanyCacheDir(dir), "transfort-br.json")
	for _, path := range []string{hostPath, profilePath, configPath(dir)} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	host := fmt.Sprintf(`{"server":%q,"use_custom_server":true,"company":{"company_id":"transfort-br","protocol":14,"openomsi_version":"0.3.38"},"maps":{"a":{"voice":true,"voice_radio":true,"admin_password":"keep"}},"future":[1,2]}`, filepath.Join(dir, "app", "server-dinamico-p14", "openomsi.exe"))
	profile := `{"company_id":"transfort-br","protocol":14,"openomsi_version":"0.3.38","clock":{"time_zone":"Europe/Berlin","shift_minutes":-480},"future":"keep"}`
	ini := fmt.Sprintf("# keep this comment\r\ncompany_id=transfort-br\r\ncompany_profile=%s\r\nopenomsi=%s\r\nautostart=false\r\nunknown_option=preserved\r\n", profilePath, filepath.Join(dir, "OpenOmsi Coop 0.3.38", "programa", "openomsi.exe"))
	for path, data := range map[string]string{hostPath: host, profilePath: profile, configPath(dir): ini} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	server := []byte("MZverified-p15-runtime")
	marker, _ := json.Marshal(map[string]any{"protocol": 15, "native_sha256": fmt.Sprintf("%x", sha256.Sum256(server))})
	payload := map[string][]byte{managedDynamicP15Server: server, "app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json": marker, bundledCoopFolder + "/programa/openomsi.exe": []byte("MZverified-coop12")}
	if err := activateUpdatedDynamicHost(dir, payload); err != nil {
		t.Fatal(err)
	}
	updatedHost, _ := os.ReadFile(hostPath)
	var doc map[string]json.RawMessage
	json.Unmarshal(updatedHost, &doc)
	var hostCompany map[string]json.RawMessage
	json.Unmarshal(doc["company"], &hostCompany)
	if hostJSONString(hostCompany, "openomsi_version") != "0.3.45" || hostJSONString(doc, "server") != filepath.Join(dir, filepath.FromSlash(managedDynamicP15Server)) || !bytes.Contains(doc["maps"], []byte("voice_radio")) || !bytes.Contains(doc["maps"], []byte("keep")) {
		t.Fatal(string(updatedHost))
	}
	updatedProfile, _ := os.ReadFile(profilePath)
	json.Unmarshal(updatedProfile, &doc)
	if string(doc["protocol"]) != "15" || hostJSONString(doc, "openomsi_version") != "0.3.45" || hostJSONString(doc, "future") != "keep" {
		t.Fatal(string(updatedProfile))
	}
	updatedINI, _ := os.ReadFile(configPath(dir))
	if !bytes.Contains(updatedINI, []byte("unknown_option=preserved\r\n")) || !bytes.Contains(updatedINI, []byte("autostart=false\r\n")) || readConfig(configPath(dir)).OpenOMSI != filepath.Join(dir, bundledCoopFolder, "programa", "openomsi.exe") {
		t.Fatal(string(updatedINI))
	}
	if err := activateUpdatedDynamicHost(dir, payload); err != nil {
		t.Fatal(err)
	}
	backup, _ := os.ReadFile(profilePath + ".before-dynamic-update")
	if string(backup) != profile {
		t.Fatal("second installation replaced the original profile backup")
	}
}

func TestDynamicHostMigrationPreservesP15AndCustom(t *testing.T) {
	dir := t.TempDir()
	for _, data := range []string{
		`{"server":"custom.exe","company":{"protocol":15},"maps":{"a":{"voice":true,"admin_password":"local"}}}`,
		`{"server":"custom.exe","use_custom_server":true,"company":{"protocol":14}}`,
		`{"server":"custom.exe","company":{"protocol":7}}`,
	} {
		out, changed, e := migrateDynamicHostSettings([]byte(data), dir, false)
		if e != nil || changed || string(out) != data {
			t.Fatal(changed, e, string(out))
		}
	}
}
func TestDynamicHostMigrationActivatesManagedP14WithoutDroppingFields(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"server":"old.exe","company":{"protocol":14,"key":"unchanged"},"maps":{"a":{"voice":true,"voice_range":200,"admin_password":"local"}},"future":[1,2]}`)
	out, changed, e := migrateDynamicHostSettings(data, dir, false)
	if e != nil || !changed {
		t.Fatal(changed, e)
	}
	var doc map[string]json.RawMessage
	json.Unmarshal(out, &doc)
	if hostJSONString(doc, "server") != filepath.Join(dir, filepath.FromSlash(managedDynamicServer)) || string(doc["use_custom_server"]) != "true" {
		t.Fatal(string(out))
	}
	var original map[string]json.RawMessage
	json.Unmarshal(data, &original)
	for _, k := range []string{"company", "maps", "future"} {
		var a, b any
		json.Unmarshal(doc[k], &a)
		json.Unmarshal(original[k], &b)
		aa, _ := json.Marshal(a)
		bb, _ := json.Marshal(b)
		if !bytes.Equal(aa, bb) {
			t.Fatal(k)
		}
	}
}
func TestDynamicHostActivationValidatesBytesAndSettingsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataRoot)
	settings := filepath.Join(managedDataDir(dir), "HostAgent.local.json")
	os.MkdirAll(filepath.Dir(settings), 0700)
	original := []byte(`{"server":"old.exe","company":{"protocol":14},"future":true}`)
	os.WriteFile(settings, original, 0600)
	payload := map[string][]byte{managedDynamicServer: []byte("server")}
	if activateUpdatedDynamicHost(dir, payload) == nil {
		t.Fatal("unverified server accepted")
	}
	got, _ := os.ReadFile(settings)
	if !bytes.Equal(got, original) {
		t.Fatal("settings changed")
	}
	payload["app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json"] = []byte(fmt.Sprintf(`{"native_sha256":"%x"}`, sha256.Sum256(payload[managedDynamicServer])))
	// A second malformed settings file must leave the first untouched.
	os.WriteFile(filepath.Join(dir, "CompanyHost.local.json"), []byte("broken"), 0600)
	if activateUpdatedDynamicHost(dir, payload) == nil {
		t.Fatal("malformed preferences accepted")
	}
	got, _ = os.ReadFile(settings)
	if !bytes.Equal(got, original) {
		t.Fatal("partial activation")
	}
	os.Remove(filepath.Join(dir, "CompanyHost.local.json"))
	if e := activateUpdatedDynamicHost(dir, payload); e != nil {
		t.Fatal(e)
	}
	got, _ = os.ReadFile(settings)
	if bytes.Equal(got, original) {
		t.Fatal("not activated")
	}
	backup, _ := os.ReadFile(settings + ".before-dynamic-update")
	if !bytes.Equal(backup, original) {
		t.Fatal("backup not exact")
	}
	if e := activateUpdatedDynamicHost(dir, payload); e != nil {
		t.Fatal("not idempotent", e)
	}
}

func TestDynamicHostP15RequiresItsOwnVerifiedRuntimeAndRetainsAllOptions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	path := filepath.Join(managedDataDir(dir), "HostAgent.local.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	original := []byte(`{"server":"old.exe","company":{"protocol":15,"clock":{"time_zone":"Europe/Berlin"}},"maps":{"a":{"voice":true,"voice_radio":true,"voice_range":300,"public_server":false,"require_hardware_id":true,"admin_password":"local"}}}`)
	os.WriteFile(path, original, 0600)
	payload := map[string][]byte{managedDynamicP15Server: []byte("p15-server")}
	marker := "app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json"
	payload[marker] = []byte(fmt.Sprintf(`{"protocol":14,"native_sha256":"%x"}`, sha256.Sum256(payload[managedDynamicP15Server])))
	if activateUpdatedDynamicHost(dir, payload) == nil {
		t.Fatal("P14 runtime was allowed to replace P15")
	}
	payload[marker] = []byte(fmt.Sprintf(`{"protocol":15,"native_sha256":"%x"}`, sha256.Sum256(payload[managedDynamicP15Server])))
	if e := activateUpdatedDynamicHost(dir, payload); e != nil {
		t.Fatal(e)
	}
	actual, _ := os.ReadFile(path)
	var before, after map[string]json.RawMessage
	json.Unmarshal(original, &before)
	json.Unmarshal(actual, &after)
	if hostJSONString(after, "server") != filepath.Join(dir, filepath.FromSlash(managedDynamicP15Server)) {
		t.Fatal(string(actual))
	}
	var a, b any
	json.Unmarshal(before["maps"], &a)
	json.Unmarshal(after["maps"], &b)
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if !bytes.Equal(aa, bb) {
		t.Fatal("P15 controls were dropped")
	}
}
