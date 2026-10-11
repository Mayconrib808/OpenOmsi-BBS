package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
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

func nativeAutomaticUpdateFixture(t *testing.T) (string, Config, string) {
	t.Helper()
	dir, c, _ := automaticUpdateFixture(t, false)
	jar := filepath.Join(c.Root, "Busbetrieb-Simulator", "busbetrieb-simulator.jar")
	if err := os.MkdirAll(filepath.Dir(jar), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("discovery fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 51)
	lines[0], lines[49], lines[50] = c.Root, filepath.Join(dir, "PeDePeAdapter")+string(filepath.Separator), "true"
	settings := filepath.Join(filepath.Dir(jar), "config.cfg")
	if err := os.WriteFile(settings, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir, c, settings
}

func TestAutomaticUpdateNativeBCSNeedsNoLegacyActivation(t *testing.T) {
	dir, c, _ := nativeAutomaticUpdateFixture(t)
	r := newMemoryRegistry(false)
	for _, stale := range []bool{false, true} {
		if stale {
			r.Write(64, ifeoBridge, "Debugger", stringValue(`"`+filepath.Join(t.TempDir(), "unrelated.exe")+`"`))
		}
		before := cloneValues(r)
		got, cfg, err := automaticBridgeUpdateTarget(r, "", []string{c.Root})
		if err != nil || got != dir || cfg.Root != c.Root || cfg.CompanyID != c.CompanyID {
			t.Fatal(got, cfg, err)
		}
		if fmt.Sprint(before) != fmt.Sprint(cloneValues(r)) {
			t.Fatal("discovery changed registry")
		}
	}
}

func TestAutomaticUpdateNativePrefersExistingUserConfiguration(t *testing.T) {
	dir, c, _ := nativeAutomaticUpdateFixture(t)
	_, other, _ := nativeAutomaticUpdateFixture(t)
	got, _, err := automaticBridgeUpdateTarget(newMemoryRegistry(false), c.Root, []string{other.Root})
	if err != nil || got != dir {
		t.Fatal(got, err)
	}
	if _, _, err := nativeBridgeUpdateTarget([]string{c.Root, other.Root}); err == nil {
		t.Fatal("ambiguous native packages accepted")
	}
}

func TestAutomaticUpdateNativeSavedSettingsOutrankOldLaunchLog(t *testing.T) {
	dir, c, settings := nativeAutomaticUpdateFixture(t)
	log := "OpenOMSI gestartet: " + filepath.Join(dir, "PeDePeAdapter", "openomsi.exe") + " --root " + c.Root + " --schedule\r\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(settings), "log.txt"), []byte(log), 0644); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []string{"false", "true"} {
		lines := make([]string, 51)
		lines[0], lines[49], lines[50] = c.Root, filepath.Join(dir, "PeDePeAdapter"), enabled
		if enabled == "true" {
			lines[0] = t.TempDir()
		}
		if err := os.WriteFile(settings, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := nativeBridgeUpdateTarget([]string{c.Root}); !errors.Is(err, errNoNativeUpdateTarget) {
			t.Fatal("old log overrode explicit settings", err)
		}
	}
}

func TestAutomaticUpdateNativeReadsBCSLegacyAccentedPaths(t *testing.T) {
	data := []byte(strings.Join(append([]string{"C:\\J\xe3o\\OMSI 2"}, make([]string, 50)...), "\r\n"))
	lines := strings.Split(string(data), "\r\n")
	lines[49], lines[50] = "D:\\Atualiza\xe7\xe3o \\PeDePeAdapter\\", "true"
	root, adapter, enabled := nativeBCSUpdateSelection([]byte(strings.Join(lines, "\r\n")))
	if !enabled || root != `C:\Jão\OMSI 2` || adapter != `D:\Atualização \PeDePeAdapter\` {
		t.Fatal(root, adapter, enabled)
	}
	if got := decodeBCSUpdateConfig([]byte{0xa4, 0xa6, 0xa8, 0xb4, 0xb8, 0xbc, 0xbd, 0xbe}); got != "€ŠšŽžŒœŸ" {
		t.Fatal(got)
	}
}

func TestAutomaticUpdateNativeFindsRedirectedBCSSettings(t *testing.T) {
	dir, c, settings := nativeAutomaticUpdateFixture(t)
	redirect := t.TempDir()
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(redirect, "config.cfg"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(settings), "gad.txt"), []byte(redirect+"\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _, err := automaticBridgeUpdateTarget(newMemoryRegistry(false), "", []string{c.Root})
	if err != nil || got != dir {
		t.Fatal(got, err)
	}
}

func TestAutomaticUpdateNativeFindsSteamSecondaryLibrary(t *testing.T) {
	dir, c, settings := nativeAutomaticUpdateFixture(t)
	steam, library := t.TempDir(), t.TempDir()
	root := filepath.Join(library, "steamapps", "common", "OMSI 2")
	if err := os.MkdirAll(filepath.Dir(root), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(c.Root, root); err != nil {
		t.Fatal(err)
	}
	c.Root = root
	if err := saveInstalledConfig(dir, c); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 51)
	lines[0], lines[49], lines[50] = root, filepath.Join(dir, "PeDePeAdapter"), "true"
	if err := os.WriteFile(filepath.Join(root, "Busbetrieb-Simulator", filepath.Base(settings)), []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(steam, "steamapps"), 0755); err != nil {
		t.Fatal(err)
	}
	vdf := fmt.Sprintf("\"libraryfolders\" { \"0\" { \"path\" \"%s\" } }", library)
	if err := os.WriteFile(filepath.Join(steam, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0644); err != nil {
		t.Fatal(err)
	}
	got, _, err := automaticBridgeUpdateTarget(newMemoryRegistry(false), "", steamLibraryUpdateRoots(steam))
	if err != nil || got != dir {
		t.Fatal(got, err)
	}
}

func TestAutomaticUpdateNativeReadsLastLaunchFromLargeUnicodeLog(t *testing.T) {
	for _, encoding := range []string{"utf8", "le", "be"} {
		t.Run(encoding, func(t *testing.T) {
			dir, c, settings := nativeAutomaticUpdateFixture(t)
			if err := os.Remove(settings); err != nil {
				t.Fatal(err)
			}
			text := "OpenOMSI gestartet: " + filepath.Join(t.TempDir(), "PeDePeAdapter", "openomsi.exe") + " --root " + c.Root + "\r\n"
			text += strings.Repeat("older log line\r\n", 160000)
			text += "OpenOMSI gestartet: " + filepath.Join(dir, "PeDePeAdapter", "openomsi.exe") + " --root " + c.Root + " --schedule\r\n"
			if err := os.WriteFile(filepath.Join(filepath.Dir(settings), "log.txt"), encodedLog(text, encoding), 0644); err != nil {
				t.Fatal(err)
			}
			got, _, err := automaticBridgeUpdateTarget(newMemoryRegistry(false), "", []string{c.Root})
			if err != nil || got != dir {
				t.Fatal(got, err)
			}
		})
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
