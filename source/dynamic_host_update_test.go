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
