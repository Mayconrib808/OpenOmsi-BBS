package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func automaticUpdateFixture(t *testing.T, host bool) (string, Config, *memoryRegistry) {
	t.Helper()
	dir := t.TempDir()
	c := defaultConfig()
	c.Root = filepath.Join(dir, "OMSI 2")
	c.OpenOMSI = filepath.Join(dir, "real-openOMSI", "openomsi.exe")
	c.Language = "pt"
	c.CompanyID = "transfort-br"
	c.PlayerName = "Maycon"
	c.Multiplayer = true
	var manifest strings.Builder
	for _, rel := range []string{"Setup.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe", "PeDePeAdapter/openomsi.exe", "HostAgent.exe"} {
		data := []byte("MZ previous " + rel)
		path := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(data), rel)
	}
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	if err := os.WriteFile(filepath.Join(dir, "docs", "SHA256.txt"), []byte(manifest.String()), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveInstalledConfig(dir, c); err != nil {
		t.Fatal(err)
	}
	if host {
		setup, _ := os.ReadFile(filepath.Join(dir, "Setup.exe"))
		agent, _ := os.ReadFile(filepath.Join(dir, "HostAgent.exe"))
		data := []byte(fmt.Sprintf("# Existing host update\n%x  Setup.exe\n%x  HostAgent.exe\n", sha256.Sum256(setup), sha256.Sum256(agent)))
		if err := os.WriteFile(filepath.Join(dir, "docs", "PATCH-HOST-SHA256.txt"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := newMemoryRegistry(false)
	for _, v := range []int{64, 32} {
		r.Write(v, ifeoBridge, "Debugger", stringValue(`"`+bridgePath(dir)+`"`))
		r.Write(v, ifeoBridge, "FilterFullPath", stringValue(filepath.Join(c.Root, "Omsi.exe")))
	}
	return dir, c, r
}

func TestAutomaticUpdateDiscoveryUsesRegisteredInstallation(t *testing.T) {
	dir, c, r := automaticUpdateFixture(t, false)
	got, cfg, err := registeredBridgeUpdateTarget(r)
	if err != nil || got != dir || cfg.Root != c.Root || cfg.CompanyID != c.CompanyID {
		t.Fatal(got, cfg, err)
	}
	r.Write(32, ifeoBridge, "FilterFullPath", stringValue(filepath.Join(t.TempDir(), "Omsi.exe")))
	if _, _, err = registeredBridgeUpdateTarget(r); err == nil {
		t.Fatal("different OMSI accepted")
	}
	if _, _, err = registeredBridgeUpdateTarget(newMemoryRegistry(false)); err == nil {
		t.Fatal("missing registration accepted")
	}
}

func TestAutomaticUpdateRejectsForeignExecutableAndConflictingViews(t *testing.T) {
	_, _, r := automaticUpdateFixture(t, false)
	r.Write(64, ifeoBridge, "Debugger", stringValue(`"`+filepath.Join(t.TempDir(), "foreign.exe")+`"`))
	if _, _, err := registeredBridgeUpdateTarget(r); err == nil {
		t.Fatal("foreign executable accepted")
	}
	_, _, r = automaticUpdateFixture(t, false)
	dir, _, _ := automaticUpdateFixture(t, false)
	r.Write(32, ifeoBridge, "Debugger", stringValue(`"`+bridgePath(dir)+`"`))
	if _, _, err := registeredBridgeUpdateTarget(r); err == nil {
		t.Fatal("conflicting views accepted")
	}
}

func TestAutomaticUpdateRejectsUnsupportedBCSWithoutChangingBridge(t *testing.T) {
	dir, c, _ := automaticUpdateFixture(t, true)
	jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
	os.MkdirAll(filepath.Dir(jar), 0755)
	f, _ := os.Create(jar)
	z := zip.NewWriter(f)
	entry, _ := z.Create(bcsLoggerEntry)
	entry.Write([]byte("unknown vendor class"))
	z.Close()
	f.Close()
	before, _ := os.ReadFile(filepath.Join(dir, "Setup.exe"))
	config, _ := os.ReadFile(configPath(dir))
	_, err := installAutomaticBridgeUpdate(dir, c, map[string][]byte{"Setup.exe": []byte("MZ new setup"), "PeDePeAdapter/openomsi.exe": []byte("MZ new adapter")}, nil)
	after, _ := os.ReadFile(filepath.Join(dir, "Setup.exe"))
	current, _ := os.ReadFile(configPath(dir))
	if err == nil || !bytes.Equal(before, after) || !bytes.Equal(config, current) {
		t.Fatal("unsupported BCS changed installation", err)
	}
	if _, err := verifyPackage(dir); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticUpdateRealBCSBackupIdempotencyAndHostManifest(t *testing.T) {
	source := os.Getenv("BRIDGE_BCS_TEST_JAR")
	if source == "" {
		t.Skip("exact local vendor fixture not available in public CI")
	}
	for _, host := range []bool{false, true} {
		t.Run(fmt.Sprintf("host=%t", host), func(t *testing.T) {
			dir, c, _ := automaticUpdateFixture(t, host)
			jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
			os.MkdirAll(filepath.Dir(jar), 0755)
			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(jar, data, 0644)
			config, _ := os.ReadFile(configPath(dir))
			agent, _ := os.ReadFile(filepath.Join(dir, "HostAgent.exe"))
			payload := map[string][]byte{"Setup.exe": []byte("MZ new setup"), "PeDePeAdapter/openomsi.exe": []byte("MZ new adapter")}
			state, err := installAutomaticBridgeUpdate(dir, c, payload, nil)
			if err != nil || !state.Changed || state.Path != jar {
				t.Fatal(state, err)
			}
			backup, err := os.ReadFile(jar + ".antes-proxima-viagem.bak")
			if err != nil || !bytes.Equal(backup, data) {
				t.Fatal("original backup not preserved", err)
			}
			logger, err := bcsLoggerBytes(jar)
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(logger)) != bcsAutoLoggerSHA {
				t.Fatal("wrong observer", err)
			}
			if _, err = verifyPackage(dir); err != nil {
				t.Fatal(err)
			}
			current, _ := os.ReadFile(configPath(dir))
			newAgent, _ := os.ReadFile(filepath.Join(dir, "HostAgent.exe"))
			if !bytes.Equal(config, current) || !bytes.Equal(agent, newAgent) {
				t.Fatal("configuration or host changed")
			}
			state, err = installAutomaticBridgeUpdate(dir, c, payload, nil)
			if err != nil || state.Changed {
				t.Fatal("second run changed BCS", state, err)
			}
		})
	}
}

func TestAutomaticUpdateRollsBackBridgeWhenCompleteVendorHashDiffers(t *testing.T) {
	source := os.Getenv("BRIDGE_BCS_TEST_JAR")
	if source == "" {
		t.Skip("exact local vendor fixture not available in public CI")
	}
	dir, c, _ := automaticUpdateFixture(t, true)
	jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
	os.MkdirAll(filepath.Dir(jar), 0755)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(jar, append(data, []byte("modified archive")...), 0644)
	before, _ := os.ReadFile(filepath.Join(dir, "Setup.exe"))
	host, _ := os.ReadFile(filepath.Join(dir, "docs", "PATCH-HOST-SHA256.txt"))
	_, err = installAutomaticBridgeUpdate(dir, c, map[string][]byte{"Setup.exe": []byte("MZ new setup"), "PeDePeAdapter/openomsi.exe": []byte("MZ new adapter")}, nil)
	after, _ := os.ReadFile(filepath.Join(dir, "Setup.exe"))
	currentHost, _ := os.ReadFile(filepath.Join(dir, "docs", "PATCH-HOST-SHA256.txt"))
	if err == nil || !bytes.Equal(before, after) || !bytes.Equal(host, currentHost) {
		t.Fatal("failed vendor update left partial Bridge", err)
	}
	if _, err = verifyPackage(dir); err != nil {
		t.Fatal(err)
	}
	if fileExists(jar + ".antes-proxima-viagem.bak") {
		t.Fatal("backup created for unverified vendor archive")
	}
}

func TestAutomaticUpdatePayloadRejectsUnexpectedPaths(t *testing.T) {
	for _, invalid := range []string{"../Setup.exe", "HostAgent.exe", "app/bridge.ini"} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		w, _ := z.Create(invalid)
		w.Write([]byte("MZ"))
		z.Close()
		if _, err := readAutomaticBridgePayload(b.Bytes()); err == nil {
			t.Fatal("unexpected path accepted", invalid)
		}
	}
}

func TestAutomaticUpdatePreviouslyInstalledLoggerIsRecognized(t *testing.T) {
	source := os.Getenv("BRIDGE_BCS_PREVIOUS_JAR")
	if source == "" {
		t.Skip("previous local logger fixture not available in public CI")
	}
	_, c, _ := automaticUpdateFixture(t, false)
	jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
	os.MkdirAll(filepath.Dir(jar), 0755)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(jar, data, 0644)
	state, err := prepareBCSNextTrip(c, nil)
	if err != nil || state.Changed {
		t.Fatal("previous logger patch not recognized", state, err)
	}
	after, _ := os.ReadFile(jar)
	if !bytes.Equal(data, after) {
		t.Fatal("previous patch was replaced")
	}
}

func TestAutomaticUpdateRealInstalledPackages(t *testing.T) {
	source, vendor, payloadPath := os.Getenv("BRIDGE_UPDATE_INSTALLED_PACKAGE"), os.Getenv("BRIDGE_BCS_TEST_JAR"), os.Getenv("BRIDGE_UPDATE_PAYLOAD")
	if source == "" || vendor == "" || payloadPath == "" {
		t.Skip("local release fixtures not available in public CI")
	}
	dir := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Root = filepath.Join(t.TempDir(), "OMSI 2")
	jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
	os.MkdirAll(filepath.Dir(jar), 0755)
	data, err := os.ReadFile(vendor)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(jar, data, 0644)
	payloadBytes, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := readAutomaticBridgePayload(payloadBytes)
	if err != nil {
		t.Fatal(err)
	}
	state, err := installAutomaticBridgeUpdate(dir, c, payload, nil)
	if err != nil || !state.Changed {
		t.Fatal("actual installed package upgrade failed", state, err)
	}
	if _, err = verifyPackage(dir); err != nil {
		t.Fatal(err)
	}
	for name, want := range payload {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("incorrect source-built component installed", name, err)
		}
	}
}
