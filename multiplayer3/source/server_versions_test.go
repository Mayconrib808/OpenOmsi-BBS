package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerSelectionModes(t *testing.T) {
	p, e := normalizeServer(serverPreference{})
	if e != nil || p.Mode != "latest" {
		t.Fatal(p, e)
	}
	p, e = normalizeServer(serverPreference{"manual", " v0.2.50 "})
	if e != nil || p.Version != "0.2.50" {
		t.Fatal(p, e)
	}
	for _, v := range []string{"../0.2.50", "latest", "0.2.50;command", "https://example.test"} {
		if _, e := normalizeServer(serverPreference{"manual", v}); e == nil {
			t.Fatal("invalid version accepted", v)
		}
	}
	r := serverRelease{Tag: "v0.2.50", Assets: []releaseAsset{{Name: "openOMSI-0.2.50-server-windows-x64.zip", URL: "https://github.com/openOMSI-org/openOMSI/releases/download/v0.2.50/server.zip", Digest: "sha256:" + strings.Repeat("a", 64), Size: 100}}}
	if v, _, e := releaseSelection(r, ""); e != nil || v != "0.2.50" {
		t.Fatal(v, e)
	}
	if _, _, e := releaseSelection(r, "0.2.49"); e == nil {
		t.Fatal("version substitution accepted")
	}
	r.Assets[0].Digest = ""
	if _, _, e := releaseSelection(r, ""); e == nil {
		t.Fatal("unverified archive accepted")
	}
}
func makeRuntimeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "server.zip")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for n, data := range entries {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = w.Write([]byte(data))
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	_ = f.Close()
	return p
}
func TestServerCacheOfflineAndTampering(t *testing.T) {
	dir := t.TempDir()
	entries := map[string]string{}
	for _, name := range []string{"openomsi.exe", "steam_api64.dll", "vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll"} {
		entries["package/"+name] = "binary " + name
	}
	archive := makeRuntimeZip(t, entries)
	dest := filepath.Join(dir, "server-cache", "0.2.50")
	receipt, e := unpackServer(archive, dest, "0.2.50", strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(receipt)
	_ = os.WriteFile(filepath.Join(dest, "verified.json"), b, 0600)
	exe, e := resolveServer(context.Background(), dir, serverPreference{"manual", "0.2.50"})
	if e != nil || exe != filepath.Join(dest, "openomsi.exe") {
		t.Fatal(exe, e)
	}
	_ = os.WriteFile(exe, []byte("altered"), 0600)
	if _, e = cachedServer(dir, "0.2.50"); e == nil {
		t.Fatal("corrupt cached server accepted")
	}
}
func TestServerArchiveCannotEscapeCache(t *testing.T) {
	for _, bad := range []string{"../escape.dll", "/escape.dll", "package/../escape.dll", `package\escape.dll`} {
		p := makeRuntimeZip(t, map[string]string{"openomsi.exe": "binary", bad: "escape"})
		if _, e := unpackServer(p, filepath.Join(t.TempDir(), "runtime"), "0.2.50", ""); e == nil {
			t.Fatal("unsafe archive accepted", bad)
		}
	}
}
func TestSeparateActivationPreservesOriginalOMSIPlugins(t *testing.T) {
	root, launcher := t.TempDir(), t.TempDir()
	dir := appDirectory(launcher)
	t.Setenv("OMSI_CONTENT", root)
	_ = os.MkdirAll(filepath.Join(launcher, "plugins"), 0755)
	_ = os.WriteFile(filepath.Join(launcher, "plugins", "bbs.lua"), []byte("native plugin"), 0600)
	if e := forwardPlugin(localConfig{Root: root, Game: filepath.Join(t.TempDir(), "openomsi.exe")}, dir); e != nil {
		t.Fatal("inherited content should be ignored", e)
	}
	if _, e := os.Stat(filepath.Join(root, "Plugins")); !os.IsNotExist(e) {
		t.Fatal("OMSI changed")
	}
	if !insideDirectory(root, filepath.Join(root, "3.0.1")) || insideDirectory(root, root+"-separate") {
		t.Fatal("directory isolation wrong")
	}
}

func TestSeparateActivationResolvesJunctionParent(t *testing.T) {
	root, outer := t.TempDir(), t.TempDir()
	link := filepath.Join(outer, "content")
	if e := os.Symlink(root, link); e != nil {
		t.Skip(e)
	}
	if !insideDirectory(root, filepath.Join(link, "Plugins")) {
		t.Fatal("write through alias was not blocked")
	}
}

func TestOldPreviewRecommendedServerUpgradesWithoutReinstallation(t *testing.T) {
	dir := t.TempDir()
	entries := map[string]string{}
	for _, name := range []string{"openomsi.exe", "steam_api64.dll", "vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll"} {
		entries["pkg/"+name] = "binary " + name
	}
	zip := makeRuntimeZip(t, entries)
	old, err := unpackServer(zip, filepath.Join(dir, "server"), "0.2.20-bbs-free2", "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(old)
	_ = os.WriteFile(filepath.Join(dir, "server", "bbs-server.json"), b, 0600)
	current, err := unpackServer(zip, filepath.Join(dir, "server-cache", "0.2.50"), "0.2.50", "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(current)
	_ = os.WriteFile(filepath.Join(dir, "server-cache", "0.2.50", "verified.json"), b, 0600)
	exe, err := resolveServer(context.Background(), dir, serverPreference{Mode: "recommended"})
	if err != nil || exe != filepath.Join(dir, "server-cache", "0.2.50", "openomsi.exe") {
		t.Fatal("old preview demanded reinstall or started protocol 6", exe, err)
	}
	retained, _ := os.ReadFile(filepath.Join(dir, "server", "bbs-server.json"))
	if !strings.Contains(string(retained), "0.2.20-bbs-free2") {
		t.Fatal("old bundle unexpectedly overwritten")
	}
}
