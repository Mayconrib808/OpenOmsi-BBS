package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestAutoDetectionReadsLargeBCSLogsInBothUTF16Orders(t *testing.T) {
	text := strings.Repeat("old log entry\r\n", 80000) + "INFO Spielmodus: MULTIPLAYER\r\nINFO Karte: Sample\r\nINFO Tour: 12:00 - 12:30 (Linie: 1)\r\nA - B (Umlauf: 01)\r\n"
	for _, big := range []bool{false, true} {
		var order binary.ByteOrder = binary.LittleEndian
		mark := []byte{0xff, 0xfe}
		if big {
			order = binary.BigEndian
			mark = []byte{0xfe, 0xff}
		}
		u := utf16.Encode([]rune(text))
		b := make([]byte, 2+len(u)*2)
		copy(b, mark)
		for i, v := range u {
			order.PutUint16(b[2+2*i:], v)
		}
		p := filepath.Join(t.TempDir(), "log.txt")
		if e := os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
		if !looksLikeBCSLog(p) {
			t.Fatal("large UTF-16 BCS log not detected", big)
		}
		tail, e := readLogTail(p, 1024*1024-1)
		if e != nil || !strings.Contains(tail, "Karte: Sample") {
			t.Fatal("unaligned suffix lost UTF-16 text", e)
		}
	}
}

type memoryRegistry struct {
	values         map[string]registryValue
	shared         bool
	failAt, writes int
}

func (m *memoryRegistry) k(view int, key, name string) string {
	if m.shared {
		view = 64
	}
	return fmt.Sprintf("%d|%s|%s", view, key, name)
}
func (m *memoryRegistry) Read(v int, k, n string) (registryValue, error) {
	return m.values[m.k(v, k, n)], nil
}
func (m *memoryRegistry) Write(v int, k, n string, x registryValue) error {
	m.writes++
	if m.failAt > 0 && m.writes == m.failAt {
		return fmt.Errorf("simulated access denied")
	}
	m.values[m.k(v, k, n)] = x
	return nil
}
func (m *memoryRegistry) DeleteValue(v int, k, n string) error {
	delete(m.values, m.k(v, k, n))
	return nil
}
func (m *memoryRegistry) DeleteKey(v int, k string) error {
	prefix := strings.TrimSuffix(m.k(v, k, ""), "|") + "|"
	for key := range m.values {
		if strings.HasPrefix(key, prefix) {
			delete(m.values, key)
		}
	}
	return nil
}
func (m *memoryRegistry) Keys(v int, k string) ([]string, error) {
	prefix := strings.TrimSuffix(m.k(v, k, ""), "|") + `\`
	set := map[string]bool{}
	for key := range m.values {
		if strings.HasPrefix(key, prefix) {
			name := strings.SplitN(strings.TrimPrefix(key, prefix), "|", 2)[0]
			name = strings.SplitN(name, `\`, 2)[0]
			set[name] = true
		}
	}
	var out []string
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}
func newMemoryRegistry(shared bool) *memoryRegistry {
	return &memoryRegistry{values: map[string]registryValue{}, shared: shared}
}
func cloneValues(m *memoryRegistry) map[string]registryValue {
	v := map[string]registryValue{}
	for k, x := range m.values {
		v[k] = x
	}
	return v
}

func TestRegistryActivationAndRemovalRestorePreviousState(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, initial := range []int{-1, 0, 1} {
			t.Run(fmt.Sprintf("shared=%t,initial=%d", shared, initial), func(t *testing.T) {
				m := newMemoryRegistry(shared)
				if initial >= 0 {
					for _, v := range []int{64, 32} {
						m.Write(v, ifeoParent, "UseFilter", dwordValue(uint32(initial)))
					}
				}
				before := cloneValues(m)
				original := `C:\Games\OMSI 2\Omsi.exe`
				bridge := `D:\Tools\Maycon's bridge & ônibus !\OpenOMSI_BCS_Bridge.exe`
				if e := activateRegistry(m, original, bridge); e != nil {
					t.Fatal(e)
				}
				for _, v := range []int{64, 32} {
					d, _ := m.Read(v, ifeoBridge, "Debugger")
					p, _ := m.Read(v, ifeoBridge, "FilterFullPath")
					if d.text() != `"`+bridge+`"` || p.text() != original {
						t.Fatal("paths were not preserved")
					}
				}
				if e := activateRegistry(m, original, bridge); e != nil {
					t.Fatal("repeated activation", e)
				}
				if e := deactivateRegistry(m, bridge); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(before, m.values) {
					t.Fatal("deactivation did not restore values", before, m.values)
				}
			})
		}
	}
}
func TestRegistryFailureRollsBackBothViews(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for fail := 1; fail <= 14; fail++ {
			t.Run(fmt.Sprintf("shared=%t,fail=%d", shared, fail), func(t *testing.T) {
				m := newMemoryRegistry(shared)
				m.failAt = fail
				before := cloneValues(m)
				e := activateRegistry(m, `C:\OMSI\Omsi.exe`, `C:\Bridge\OpenOMSI_BCS_Bridge.exe`)
				if m.writes >= fail && e == nil {
					t.Fatal("failure not returned")
				}
				if e != nil && !reflect.DeepEqual(before, m.values) {
					t.Fatal("partial activation left after rollback", m.values)
				}
			})
		}
	}
}
func TestRegistryPreservesOtherToolsAndRejectsConflicts(t *testing.T) {
	original := `C:\OMSI\Omsi.exe`
	bridge := `C:\Bridge\OpenOMSI_BCS_Bridge.exe`
	for _, kind := range []string{"global debugger", "same folder filter", "foreign owner", "different OMSI", "invalid flag"} {
		t.Run(kind, func(t *testing.T) {
			m := newMemoryRegistry(false)
			switch kind {
			case "global debugger":
				m.Write(64, ifeoParent, "Debugger", stringValue(`other.exe`))
			case "same folder filter":
				m.Write(32, ifeoParent+`\OtherTool`, "FilterFullPath", stringValue(original))
			case "foreign owner":
				m.Write(64, ifeoBridge, "Debugger", stringValue(`"C:\Other\tool.exe"`))
			case "different OMSI":
				m.Write(64, ifeoBridge, "Debugger", stringValue(`"C:\Old\OpenOMSI_BCS_Bridge.exe"`))
				m.Write(64, ifeoBridge, "FilterFullPath", stringValue(`D:\OMSI\Omsi.exe`))
			case "invalid flag":
				m.Write(64, ifeoParent, "UseFilter", dwordValue(7))
			}
			before := cloneValues(m)
			if e := activateRegistry(m, original, bridge); e == nil {
				t.Fatal("conflict accepted")
			}
			if !reflect.DeepEqual(before, m.values) {
				t.Fatal("conflict changed registry")
			}
		})
	}
	m := newMemoryRegistry(false)
	for _, view := range []int{64, 32} {
		m.Write(view, ifeoParent+`\OtherTool`, "FilterFullPath", stringValue(`D:\Another\Omsi.exe`))
	}
	if e := activateRegistry(m, original, bridge); e != nil {
		t.Fatal(e)
	}
	if e := deactivateRegistry(m, bridge); e != nil {
		t.Fatal(e)
	}
	for _, v := range []int{64, 32} {
		other, _ := m.Read(v, ifeoParent+`\OtherTool`, "FilterFullPath")
		flag, _ := m.Read(v, ifeoParent, "UseFilter")
		if other.text() == "" {
			t.Fatal("deleted another filter")
		}
		if n, _ := flag.number(); n != 1 {
			t.Fatal("disabled another filter")
		}
	}
}
func TestRegistryUpgradeAndRemovalOwnership(t *testing.T) {
	m := newMemoryRegistry(false)
	original := `C:\OMSI\Omsi.exe`
	old := `D:\Old\OpenOMSI_BCS_Bridge.exe`
	next := `D:\New\OpenOMSI_BCS_Bridge.exe`
	for _, v := range []int{64, 32} {
		m.Write(v, ifeoParent, "UseFilter", dwordValue(1))
		m.Write(v, ifeoBridge, "Debugger", stringValue(`"`+old+`"`))
		m.Write(v, ifeoBridge, "FilterFullPath", stringValue(original))
	}
	if e := activateRegistry(m, original, next); e != nil {
		t.Fatal("legacy upgrade", e)
	}
	before := cloneValues(m)
	if e := deactivateRegistry(m, old); e == nil {
		t.Fatal("old folder removed new activation")
	}
	if !reflect.DeepEqual(before, m.values) {
		t.Fatal("ownership failure changed registry")
	}
	if e := deactivateRegistry(m, next); e != nil {
		t.Fatal(e)
	}
}

func fakePE(t *testing.T, path string, machine uint16) {
	t.Helper()
	b := make([]byte, 256)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[60:], 128)
	copy(b[128:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[132:], machine)
	if e := os.WriteFile(path, b, 0644); e != nil {
		t.Fatal(e)
	}
}
func TestConfigUnicodePathsRoundTripAndValidation(t *testing.T) {
	t.Setenv("OMSI_PLUGIN_HOST32", "")
	dir := t.TempDir()
	root := filepath.Join(dir, "OMSI 2 - ônibus & Maycon's !")
	open := filepath.Join(dir, "open folder")
	for _, p := range []string{filepath.Join(root, "maps"), filepath.Join(root, "vehicles"), filepath.Join(root, "Drivers"), open, filepath.Join(appDir(dir), "compat")} {
		if e := os.MkdirAll(p, 0755); e != nil {
			t.Fatal(e)
		}
	}
	fakePE(t, filepath.Join(root, "Omsi.exe"), 0x14c)
	fakePE(t, filepath.Join(open, "openomsi.exe"), 0x8664)
	// Match the official 0.2.0 archive: no host alongside openomsi.exe.
	fakePE(t, filepath.Join(appDir(dir), "compat", "omsi-plugin-host32.exe"), 0x14c)
	c := defaultConfig()
	c.Root = root
	c.OpenOMSI = filepath.Join(open, "openomsi.exe")
	c.Language = "en"
	if e := validateConfig(c, dir); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(filepath.Join(root, installedPluginHostName)); !os.IsNotExist(e) {
		t.Fatal("folder validation must not write the runtime helper", e)
	}
	p, e := cleanInputPath(`"` + root + `"`)
	if e != nil || p != root {
		t.Fatal("quoted path was corrupted", p, e)
	}
	cfgPath := filepath.Join(dir, "bridge.ini")
	if e = os.WriteFile(cfgPath, append([]byte{0xef, 0xbb, 0xbf}, encodeConfig(c)...), 0644); e != nil {
		t.Fatal(e)
	}
	if got := readConfig(cfgPath); !reflect.DeepEqual(got, c) {
		t.Fatal("configuration round trip", got, c)
	}
	if got := readConfig(filepath.Join(dir, "missing.ini")); got.Root != "" || got.OpenOMSI != "" {
		t.Fatal("personal path default leaked")
	}
	bad := c
	bad.Root = "relative"
	if validateConfig(bad, dir) == nil {
		t.Fatal("relative installation accepted")
	}
	fakePE(t, c.OpenOMSI, 0x14c)
	if validateConfig(c, dir) == nil {
		t.Fatal("32-bit openOMSI accepted")
	}
	for _, p := range []string{"", "C:\\Games\nroot=evil", "nul\x00path"} {
		if _, e := cleanInputPath(p); e == nil {
			t.Fatal("unsafe path accepted")
		}
	}
}

func TestPluginHelperBundledPathAndExplicitOverride(t *testing.T) {
	t.Setenv("OMSI_PLUGIN_HOST32", "")
	dir := t.TempDir()
	if e := os.MkdirAll(filepath.Join(appDir(dir), "compat"), 0755); e != nil {
		t.Fatal(e)
	}
	host := filepath.Join(appDir(dir), "compat", "omsi-plugin-host32.exe")
	c := defaultConfig()
	c.Root = filepath.Join(dir, "original OMSI root")
	if e := os.Mkdir(c.Root, 0755); e != nil {
		t.Fatal(e)
	}
	c.OpenOMSI = filepath.Join(dir, "separate openOMSI 0.2.0 folder", "openomsi.exe")
	c.Language = "pt"
	if _, e := pluginHostSource(c, dir); e == nil || !strings.Contains(e.Error(), "Extraia o ZIP completo da bridge") {
		t.Fatal("missing helper was not diagnosed in Portuguese", e)
	}
	fakePE(t, host, 0x8664)
	if _, e := pluginHostSource(c, dir); e == nil {
		t.Fatal("64-bit helper accepted for 32-bit plugins")
	}
	fakePE(t, host, 0x14c)
	if got, e := pluginHostSource(c, dir); e != nil || got != host {
		t.Fatal("bundled helper not selected independently of openOMSI folder", got, e)
	}
	t.Setenv("OMSI_PLUGIN_HOST32", "relative.exe")
	if _, e := pluginHostSource(c, dir); e == nil {
		t.Fatal("invalid explicit override silently replaced with bundled helper")
	}
	override := filepath.Join(dir, "explicit helper.exe")
	fakePE(t, override, 0x14c)
	t.Setenv("OMSI_PLUGIN_HOST32", override)
	if got, e := pluginHostSource(c, dir); e != nil || got != override {
		t.Fatal("valid explicit override was not preserved", got, e)
	}
	d, e := preparePluginHost(c, dir)
	if e != nil || filepath.Dir(d.Path) != c.Root || d.Path == override {
		t.Fatal("override must also run from the OMSI marker directory", d, e)
	}
	expected, e := os.ReadFile(override)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(d.Path); e != nil || !reflect.DeepEqual(b, expected) {
		t.Fatal("explicit override payload was not preserved", e)
	}
}

func pluginHostFixture(t *testing.T) (Config, string, []byte) {
	t.Helper()
	t.Setenv("OMSI_PLUGIN_HOST32", "")
	dir := t.TempDir()
	packageDir := filepath.Join(dir, "bridge - Maycon's & ônibus !")
	c := defaultConfig()
	c.Root = filepath.Join(dir, "OMSI 2 - São Paulo")
	c.OpenOMSI = filepath.Join(dir, "separate openOMSI 0.2.0", "openomsi.exe")
	c.Language = "en"
	for _, p := range []string{c.Root, filepath.Join(appDir(packageDir), "compat")} {
		if e := os.MkdirAll(p, 0755); e != nil {
			t.Fatal(e)
		}
	}
	// A synthetic PE header exercises staging, ownership and rollback without
	// distributing or executing the historical helper of unresolved provenance.
	host := filepath.Join(appDir(packageDir), "compat", "omsi-plugin-host32.exe")
	fakePE(t, host, 0x14c)
	payload, e := os.ReadFile(host)
	if e != nil {
		t.Fatal(e)
	}
	return c, packageDir, payload
}

func TestPluginHelperRunsFromOMSIStartupMarkerDirectory(t *testing.T) {
	c, packageDir, payload := pluginHostFixture(t)
	marker := filepath.Join(c.Root, "bbs.start")
	markerBytes := []byte("BCS-owned startup marker")
	if e := os.WriteFile(marker, markerBytes, 0644); e != nil {
		t.Fatal(e)
	}
	d, e := preparePluginHost(c, packageDir)
	if e != nil || !d.Created || filepath.Dir(d.Path) != c.Root || filepath.Base(d.Path) != installedPluginHostName {
		t.Fatal("helper must run from the BCS startup marker directory", d, e)
	}
	if b, e := os.ReadFile(d.Path); e != nil || !reflect.DeepEqual(b, payload) {
		t.Fatal("helper payload changed", e)
	}
	if filepath.Join(filepath.Dir(d.Path), "bbs.start") != marker {
		t.Fatal("BCS plugin would look for bbs.start in the wrong directory")
	}
	second, e := preparePluginHost(c, packageDir)
	if e != nil || second.Created || second.Path != d.Path {
		t.Fatal("matching helper was not reused", second, e)
	}
	if b, e := os.ReadFile(marker); e != nil || !reflect.DeepEqual(b, markerBytes) {
		t.Fatal("BCS marker was modified", e)
	}
}

func TestPluginHelperPreservesUnknownFilesAndDoesNotCreateMarker(t *testing.T) {
	for _, kind := range []string{"different file", "directory", "symbolic link"} {
		t.Run(kind, func(t *testing.T) {
			c, packageDir, _ := pluginHostFixture(t)
			target, _ := pluginHostPath(c)
			original := []byte("belongs to someone else")
			switch kind {
			case "different file":
				if e := os.WriteFile(target, original, 0644); e != nil {
					t.Fatal(e)
				}
			case "directory":
				if e := os.Mkdir(target, 0755); e != nil {
					t.Fatal(e)
				}
			case "symbolic link":
				other := filepath.Join(packageDir, "other.exe")
				if e := os.WriteFile(other, original, 0644); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(other, target); e != nil {
					t.Skip("symlinks unavailable", e)
				}
			}
			before, e := os.Lstat(target)
			if e != nil {
				t.Fatal(e)
			}
			if _, e := preparePluginHost(c, packageDir); e == nil {
				t.Fatal("unknown target was accepted")
			}
			if state, e := removeInstalledPluginHost(c.Root); e != nil || state != "preserved" {
				t.Fatal("unknown target was not preserved during removal", state, e)
			}
			after, e := os.Lstat(target)
			if e != nil || !os.SameFile(before, after) {
				t.Fatal("unknown target was replaced", e)
			}
			if kind != "directory" {
				if b, e := os.ReadFile(target); e != nil || !reflect.DeepEqual(b, original) {
					t.Fatal("unknown content changed", e)
				}
			}
			if _, e := os.Stat(filepath.Join(c.Root, "bbs.start")); !os.IsNotExist(e) {
				t.Fatal("bridge created the BCS marker", e)
			}
		})
	}
}

func TestPluginHelperActivationFailureOnlyRollsBackNewCopy(t *testing.T) {
	for _, reused := range []bool{false, true} {
		for _, failure := range []string{"registry conflict", "partial registry write"} {
			t.Run(fmt.Sprintf("reused=%t,%s", reused, failure), func(t *testing.T) {
				c, packageDir, payload := pluginHostFixture(t)
				target, _ := pluginHostPath(c)
				if reused {
					if e := os.WriteFile(target, payload, 0755); e != nil {
						t.Fatal(e)
					}
				}
				m := newMemoryRegistry(false)
				if failure == "registry conflict" {
					m.Write(64, ifeoParent, "Debugger", stringValue("other tool"))
				} else {
					m.failAt = 8
				}
				before := cloneValues(m)
				if _, e := activateRegistryWithHost(m, c, packageDir, filepath.Join(packageDir, "OpenOMSI_BCS_Bridge.exe")); e == nil {
					t.Fatal("activation failure was not returned")
				}
				if !reflect.DeepEqual(before, m.values) {
					t.Fatal("registry changed after failed activation")
				}
				b, e := os.ReadFile(target)
				if reused {
					if e != nil || !reflect.DeepEqual(b, payload) {
						t.Fatal("existing matching helper removed", e)
					}
				} else if !os.IsNotExist(e) {
					t.Fatal("new helper was not rolled back", e)
				}
			})
		}
	}
}

func TestPluginHelperDeactivationChecksOwnerAndPreservesUnknownPayload(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			c, packageDir, payload := pluginHostFixture(t)
			marker := filepath.Join(c.Root, "bbs.start")
			originalHelper := filepath.Join(c.Root, "omsi-plugin-host32.exe")
			for _, p := range []string{marker, originalHelper} {
				if e := os.WriteFile(p, []byte("original"), 0644); e != nil {
					t.Fatal(e)
				}
			}
			m := newMemoryRegistry(shared)
			bridge := filepath.Join(packageDir, "OpenOMSI_BCS_Bridge.exe")
			before := cloneValues(m)
			d, e := activateRegistryWithHost(m, c, packageDir, bridge)
			if e != nil {
				t.Fatal(e)
			}
			if e := deactivateRegistryWithHost(m, filepath.Join(packageDir, "wrong", "OpenOMSI_BCS_Bridge.exe")); e == nil {
				t.Fatal("foreign package deactivated this installation")
			}
			if b, e := os.ReadFile(d.Path); e != nil || !reflect.DeepEqual(b, payload) {
				t.Fatal("ownership failure removed helper", e)
			}
			if e := deactivateRegistryWithHost(m, bridge); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(before, m.values) {
				t.Fatal("deactivation did not restore registry")
			}
			if b, e := os.ReadFile(d.Path); e != nil || !reflect.DeepEqual(b, payload) {
				t.Fatal("unrecognised helper must remain after Registry deactivation", e)
			}
			for _, p := range []string{marker, originalHelper} {
				if b, e := os.ReadFile(p); e != nil || string(b) != "original" {
					t.Fatal("original file was changed", p, e)
				}
			}
		})
	}
}

func TestHistoricalPluginHelperDeactivationRemovesRecognisedCopy(t *testing.T) {
	// This optional local check requires exact historical bytes, which are not
	// included in this repository. Generic helper behaviour is tested above.
	payload, e := os.ReadFile(filepath.Join("..", "local-only", "omsi-plugin-host32.exe"))
	if os.IsNotExist(e) {
		t.Skip("historical helper is excluded; exact-hash removal needs a separately verified local copy")
	}
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(payload)) != historicalPluginHostSHA256 {
		t.Fatal("local helper does not match the recorded historical bytes", e)
	}
	c, packageDir, _ := pluginHostFixture(t)
	host := filepath.Join(appDir(packageDir), "compat", "omsi-plugin-host32.exe")
	if e := os.WriteFile(host, payload, 0755); e != nil {
		t.Fatal(e)
	}
	m := newMemoryRegistry(false)
	bridge := filepath.Join(packageDir, "OpenOMSI_BCS_Bridge.exe")
	before := cloneValues(m)
	d, e := activateRegistryWithHost(m, c, packageDir, bridge)
	if e != nil {
		t.Fatal(e)
	}
	if e := deactivateRegistryWithHost(m, bridge); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, m.values) {
		t.Fatal("deactivation did not restore Registry state")
	}
	if _, e := os.Stat(d.Path); !os.IsNotExist(e) {
		t.Fatal("recognised historical helper was not removed", e)
	}
}

func TestPluginHelperRollbackPreservesExternallyModifiedCopy(t *testing.T) {
	c, packageDir, _ := pluginHostFixture(t)
	d, e := preparePluginHost(c, packageDir)
	if e != nil {
		t.Fatal(e)
	}
	changed := []byte("external change")
	if e := os.WriteFile(d.Path, changed, 0644); e != nil {
		t.Fatal(e)
	}
	if e := rollbackPluginHost(d); e == nil {
		t.Fatal("changed helper rollback did not report preservation")
	}
	if state, e := removeInstalledPluginHost(c.Root); e != nil || state != "preserved" {
		t.Fatal(state, e)
	}
	if b, e := os.ReadFile(d.Path); e != nil || !reflect.DeepEqual(b, changed) {
		t.Fatal("external change was deleted", e)
	}
}

func TestRecognisedHelperUpgradeAndRollback(t *testing.T) {
	oldDigest := bundledPluginHostSHA256
	t.Cleanup(func() { bundledPluginHostSHA256 = oldDigest })
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("registryFailure=%t", failure), func(t *testing.T) {
			c, dir, old := pluginHostFixture(t)
			target, _ := pluginHostPath(c)
			if e := os.WriteFile(target, old, 0755); e != nil {
				t.Fatal(e)
			}
			bundledPluginHostSHA256 = fmt.Sprintf("%x", sha256.Sum256(old))
			updated := append(append([]byte{}, old...), []byte("new release fixture")...)
			if e := os.WriteFile(filepath.Join(appDir(dir), "compat", "omsi-plugin-host32.exe"), updated, 0755); e != nil {
				t.Fatal(e)
			}
			m := newMemoryRegistry(false)
			if failure {
				m.failAt = 8
			}
			before := cloneValues(m)
			bridge := bridgePath(dir)
			d, e := activateRegistryWithHost(m, c, dir, bridge)
			if (e != nil) != failure {
				t.Fatal(e)
			}
			want := updated
			if failure {
				want = old
			}
			if got, e := os.ReadFile(target); e != nil || !bytes.Equal(got, want) {
				t.Fatal("incorrect update/rollback bytes", e)
			}
			if failure && !reflect.DeepEqual(before, m.values) {
				t.Fatal("Registry rollback incomplete")
			}
			if !failure {
				if d.previous == nil || d.Created {
					t.Fatal("upgrade ownership state", d)
				}
				bundledPluginHostSHA256 = fmt.Sprintf("%x", sha256.Sum256(updated))
				if e := deactivateRegistryWithHost(m, bridge); e != nil {
					t.Fatal(e)
				}
				if _, e := os.Stat(target); !os.IsNotExist(e) {
					t.Fatal("new recognised helper not removed", e)
				}
			}
		})
	}
}

func TestBuiltRuntimePackage(t *testing.T) {
	dir := os.Getenv("BRIDGE_BUILT_PACKAGE")
	if dir == "" {
		t.Skip("runs after the complete package is assembled by scripts/build.py")
	}
	if count, e := verifyPackage(dir); e != nil || count < 10 {
		t.Fatal("complete package integrity", count, e)
	}
	actualHost := filepath.Join(appDir(dir), "compat", "omsi-plugin-host32.exe")
	payload, e := os.ReadFile(actualHost)
	if e != nil {
		t.Fatal(e)
	}
	if bundledPluginHostSHA256 == "" || fmt.Sprintf("%x", sha256.Sum256(payload)) != bundledPluginHostSHA256 {
		t.Fatal("host digest was not injected into the consuming programs")
	}
	c, fixtureDir, _ := pluginHostFixture(t)
	if e := os.WriteFile(filepath.Join(appDir(fixtureDir), "compat", "omsi-plugin-host32.exe"), payload, 0755); e != nil {
		t.Fatal(e)
	}
	m := newMemoryRegistry(false)
	bridge := bridgePath(fixtureDir)
	if _, e := activateRegistryWithHost(m, c, fixtureDir, bridge); e != nil {
		t.Fatal(e)
	}
	if e := deactivateRegistryWithHost(m, bridge); e != nil {
		t.Fatal(e)
	}
	target, _ := pluginHostPath(c)
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal("built helper not removed", e)
	}
	previousPath := os.Getenv("BRIDGE_PREVIOUS_HOST")
	if previousPath == "" {
		t.Fatal("complete package test requires the exact previous dev.1 helper")
	}
	previous, e := os.ReadFile(previousPath)
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(previous)) != previousGUIPluginHostSHA256 {
		t.Fatal("incorrect previous release helper", e)
	}
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("dev1Upgrade/registryFailure=%t", failure), func(t *testing.T) {
			c, fixtureDir, _ := pluginHostFixture(t)
			target, _ := pluginHostPath(c)
			if e := os.WriteFile(target, previous, 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(appDir(fixtureDir), "compat", "omsi-plugin-host32.exe"), payload, 0755); e != nil {
				t.Fatal(e)
			}
			m := newMemoryRegistry(false)
			before := cloneValues(m)
			if failure {
				m.failAt = 8
			}
			bridge := bridgePath(fixtureDir)
			_, e := activateRegistryWithHost(m, c, fixtureDir, bridge)
			if (e != nil) != failure {
				t.Fatal("dev.1 upgrade activation", e)
			}
			want := payload
			if failure {
				want = previous
				if !reflect.DeepEqual(before, m.values) {
					t.Fatal("dev.1 upgrade Registry rollback incomplete")
				}
			}
			if actual, e := os.ReadFile(target); e != nil || !bytes.Equal(actual, want) {
				t.Fatal("dev.1 upgrade did not preserve exact update/rollback bytes", e)
			}
			if !failure {
				if e := deactivateRegistryWithHost(m, bridge); e != nil {
					t.Fatal(e)
				}
				if _, e := os.Stat(target); !os.IsNotExist(e) {
					t.Fatal("upgraded helper not removed", e)
				}
			}
		})
	}
}

func TestPluginHostDiagnosticsContainHashesWithoutMarkerContents(t *testing.T) {
	c, packageDir, payload := pluginHostFixture(t)
	if _, e := preparePluginHost(c, packageDir); e != nil {
		t.Fatal(e)
	}
	secret := "private marker content must stay local"
	if e := os.WriteFile(filepath.Join(c.Root, "bbs.start"), []byte(secret), 0644); e != nil {
		t.Fatal(e)
	}
	p, e := collectLogs(packageDir, c)
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(p)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	found := false
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(r)
		r.Close()
		if e != nil || strings.Contains(string(b), secret) {
			t.Fatal("diagnostics exposed marker contents", e)
		}
		if f.Name == "PLUGIN_HOST.txt" {
			found = true
			if !strings.Contains(string(b), filepath.Join(c.Root, installedPluginHostName)) || !strings.Contains(string(b), fmt.Sprintf("%x", sha256.Sum256(payload))) || !strings.Contains(string(b), "marker present at collection: true") {
				t.Fatal("helper diagnostics lack runtime path, hash or marker status", string(b))
			}
		}
	}
	if !found {
		t.Fatal("helper diagnostic entry not collected")
	}
}

func TestPluginEnvironmentOnlyOverridesTheSimulatorChild(t *testing.T) {
	t.Setenv("OMSI_PLUGIN_HOST32", "unchanged parent value")
	inherited := []string{"PATH=keep path", "omsi_plugin_host32=old one", "SteamAppId=252530", "OMSI_PLUGIN_HOST32=old two", "OTHER=a=b"}
	original := append([]string(nil), inherited...)
	host := filepath.Join(t.TempDir(), "bridge ônibus & Maycon's !", "compat", "omsi-plugin-host32.exe")
	child := pluginEnvironment(inherited, host)
	expected := []string{"PATH=keep path", "SteamAppId=252530", "OTHER=a=b", "OMSI_PLUGIN_HOST32=" + host}
	if !reflect.DeepEqual(child, expected) {
		t.Fatal("child environment has stale or corrupted values", child)
	}
	if !reflect.DeepEqual(inherited, original) || os.Getenv("OMSI_PLUGIN_HOST32") != "unchanged parent value" {
		t.Fatal("parent environment was changed")
	}
}

func manifest(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	var s strings.Builder
	for path, b := range files {
		p := filepath.Join(dir, filepath.FromSlash(path))
		os.MkdirAll(filepath.Dir(p), 0755)
		if e := os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
		fmt.Fprintf(&s, "%x  %s\n", sha256.Sum256(b), path)
	}
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	if e := os.WriteFile(filepath.Join(dir, "docs", "SHA256.txt"), []byte(s.String()), 0644); e != nil {
		t.Fatal(e)
	}
}
func TestPackageVerificationDetectsTamperingAndTraversal(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"Setup.exe": []byte("setup"), "app/OpenOMSI_BCS_Bridge.exe": []byte("bridge"), "app/compat/Omsi.exe": []byte("facade"), "app/compat/omsi-plugin-host32.exe": []byte("helper")}
	manifest(t, dir, files)
	if n, e := verifyPackage(dir); e != nil || n != 4 {
		t.Fatal(n, e)
	}
	os.WriteFile(filepath.Join(dir, "Setup.exe"), []byte("modified"), 0644)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("tampering accepted")
	}
	os.WriteFile(filepath.Join(dir, "docs", "SHA256.txt"), []byte(fmt.Sprintf("%x  ../outside.exe\n", sha256.Sum256([]byte("outside")))), 0644)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("traversal accepted")
	}
	for _, rel := range []string{"app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"} {
		manifest(t, dir, files)
		os.Remove(filepath.Join(dir, filepath.FromSlash(rel)))
		if _, e := verifyPackage(dir); e == nil {
			t.Fatal("missing component accepted", rel)
		}
	}
	delete(files, "app/compat/omsi-plugin-host32.exe")
	manifest(t, dir, files)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("manifest omitting the required plugin helper was accepted")
	}
}
func TestDiagnosticCollectionOnlyIncludesExpectedLocalFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(appDir(dir), 0755)
	os.WriteFile(filepath.Join(appDir(dir), "bridge-v1.1.3.log"), []byte("saved trip"), 0644)
	os.WriteFile(filepath.Join(dir, "private-passwords.txt"), []byte("do not collect"), 0644)
	p, e := collectLogs(dir, defaultConfig())
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(p)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	found := false
	for _, f := range z.File {
		if f.Name == "private-passwords.txt" {
			t.Fatal("unrelated file collected")
		}
		if f.Name == "bridge-v1.1.3.log" {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			b, e := io.ReadAll(r)
			r.Close()
			if e != nil || string(b) != "saved trip" {
				t.Fatal("log bytes not preserved")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("trip log missing")
	}
}
