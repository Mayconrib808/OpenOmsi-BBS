package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coopFixture(t *testing.T) ([]byte, coopDistribution) {
	t.Helper()
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	spec := coopDistribution{Prefix: "portable/", Files: map[string]string{}}
	for name, content := range map[string]string{"programa/openomsi.exe": "MZ fixture cooperative game", "programa/assets/sounds/coop_turnstile_coin.wav": "RIFF fixture audio", "LEIA-ME.txt": "instructions"} {
		w, err := z.Create(spec.Prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		spec.Files[bundledCoopFolder+"/"+name] = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	spec.Size, spec.SHA256 = int64(data.Len()), fmt.Sprintf("%x", sha256.Sum256(data.Bytes()))
	cut := data.Len() / 2
	for _, part := range [][]byte{data.Bytes()[:cut], data.Bytes()[cut:]} {
		spec.Parts = append(spec.Parts, coopDistributionPart{fmt.Sprintf("%x", sha256.Sum256(part)), int64(len(part))})
	}
	return data.Bytes(), spec
}

func TestCumulativeBundleInstallsManagedServerAndCoopAtExactPaths(t *testing.T) {
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	spec := coopDistribution{Prefix: "Bridge-2203/", FullPaths: true, Files: map[string]string{}}
	want := map[string][]byte{bundledCoopFolder + "/programa/openomsi.exe": []byte("MZ game"), "app/server-dinamico-p14/openomsi.exe": []byte("MZ server")}
	for name, content := range want {
		f, err := z.Create(spec.Prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(content); err != nil {
			t.Fatal(err)
		}
		spec.Files[name] = fmt.Sprintf("%x", sha256.Sum256(content))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	spec.Size = int64(data.Len())
	spec.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data.Bytes()))
	files, err := readCoopDistribution(data.Bytes(), spec)
	if err != nil || len(files) != 2 {
		t.Fatalf("cumulative read: %v", err)
	}
	dir := t.TempDir()
	for name, content := range files {
		if !bytes.Equal(content, want[name]) {
			t.Fatal("wrong destination", name)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if !existingCoopBundle(dir, spec) {
		t.Fatal("installed cumulative bundle not recognized")
	}
	if err := os.WriteFile(filepath.Join(dir, "app/server-dinamico-p14/openomsi.exe"), []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if existingCoopBundle(dir, spec) {
		t.Fatal("damaged managed server accepted")
	}
}

func TestCoopDownloadRechecksCacheAndRejectsChangedPart(t *testing.T) {
	data, spec := coopFixture(t)
	parts := map[string][]byte{}
	var offset int
	for _, part := range spec.Parts {
		parts["/packages/"+part.SHA256+".bin"] = data[offset : offset+int(part.Size)]
		offset += int(part.Size)
	}
	changed := false
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		b, ok := parts[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if changed {
			b = append([]byte(nil), b...)
			b[0] ^= 1
		}
		w.Write(b)
	}))
	defer server.Close()
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, spec.Parts[0].SHA256+".bin"), []byte("invalid cached bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	var reported int64
	actual, err := downloadCoopDistribution(context.Background(), server.Client(), server.URL, spec, cache, func(n, total int64) {
		if total != spec.Size || n < reported {
			t.Error("invalid progress")
		}
		reported = n
	})
	if err != nil || !bytes.Equal(actual, data) || reported != spec.Size || requests != 2 {
		t.Fatalf("download: %v, progress=%d, requests=%d", err, reported, requests)
	}
	requests = 0
	if _, err := downloadCoopDistribution(context.Background(), server.Client(), server.URL, spec, cache, nil); err != nil || requests != 0 {
		t.Fatalf("verified cache: %v %d", err, requests)
	}
	changed = true
	if _, err := downloadCoopDistribution(context.Background(), server.Client(), server.URL, spec, t.TempDir(), nil); err == nil {
		t.Fatal("changed download accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := downloadCoopDistribution(ctx, server.Client(), server.URL, spec, t.TempDir(), nil); err == nil {
		t.Fatal("cancelled operation continued")
	}
}

func TestCoop2215DownloadSupportsMoreThanEightVerifiedChunks(t *testing.T) {
	data, spec := coopFixture(t)
	spec.Parts = nil
	parts := map[string][]byte{}
	size := (len(data) + 9) / 10
	for start := 0; start < len(data); start += size {
		end := start + size
		if end > len(data) {
			end = len(data)
		}
		part := data[start:end]
		hash := fmt.Sprintf("%x", sha256.Sum256(part))
		spec.Parts = append(spec.Parts, coopDistributionPart{hash, int64(len(part))})
		parts["/packages/"+hash+".bin"] = part
	}
	if len(spec.Parts) <= 8 {
		t.Fatal("fixture does not exercise larger 0.3.43 package")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := parts[r.URL.Path]; ok {
			w.Write(p)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	actual, err := downloadCoopDistribution(context.Background(), server.Client(), server.URL, spec, t.TempDir(), nil)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("new package chunk layout rejected: %v", err)
	}
	if _, err = readCoopDistribution(actual, spec); err != nil {
		t.Fatal(err)
	}
	spec.Size = 256<<20 + 1
	if _, err = downloadCoopDistribution(context.Background(), server.Client(), server.URL, spec, t.TempDir(), nil); err == nil {
		t.Fatal("unbounded archive accepted")
	}
}

func TestCoopArchiveRejectsMissingForeignAndCorruptFiles(t *testing.T) {
	data, spec := coopFixture(t)
	if files, err := readCoopDistribution(data, spec); err != nil || len(files) != 3 {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), data...)
	corrupt[len(corrupt)/2] ^= 1
	if _, err := readCoopDistribution(corrupt, spec); err == nil {
		t.Fatal("corrupted archive accepted")
	}
	for _, name := range []string{"portable/../escape.exe", "portable/programa/unknown.exe", "portable/programa\\openomsi.exe", "other/programa/openomsi.exe"} {
		var changed bytes.Buffer
		z := zip.NewWriter(&changed)
		w, _ := z.Create(name)
		w.Write([]byte("MZ bad"))
		z.Close()
		candidate := spec
		candidate.Size = int64(changed.Len())
		candidate.SHA256 = fmt.Sprintf("%x", sha256.Sum256(changed.Bytes()))
		// Keep the file count plausible so path/content checks are exercised.
		candidate.Files = map[string]string{bundledCoopFolder + "/programa/openomsi.exe": spec.Files[bundledCoopFolder+"/programa/openomsi.exe"]}
		if _, err := readCoopDistribution(changed.Bytes(), candidate); err == nil {
			t.Fatal("unsafe archive accepted", name)
		}
	}
}

func TestCoopInstallerAddsFolderAndKeepsExistingPreferencesAndExtraFiles(t *testing.T) {
	data, spec := coopFixture(t)
	old := bundledCoopDistribution
	bundledCoopDistribution = spec
	t.Cleanup(func() { bundledCoopDistribution = old })
	files, err := readCoopDistribution(data, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Setup.exe", "HostAgent.exe", "CompanyHost.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"} {
		files[name] = []byte("MZ updated " + name)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	preferences := filepath.Join(dir, "app", "bridge.ini")
	os.WriteFile(preferences, []byte("existing player settings"), 0600)
	extra := filepath.Join(dir, bundledCoopFolder, "meu-leia-me.txt")
	os.MkdirAll(filepath.Dir(extra), 0755)
	os.WriteFile(extra, []byte("user file"), 0600)
	if _, err := installAutomaticBridgeUpdate(dir, Config{}, files, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !existingCoopBundle(dir, spec) {
		t.Fatal("Coop folder was not fully installed")
	}
	if b, _ := os.ReadFile(preferences); string(b) != "existing player settings" {
		t.Fatal("preferences overwritten")
	}
	if b, _ := os.ReadFile(extra); string(b) != "user file" {
		t.Fatal("extra user file changed")
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "docs", "SHA256.txt"))
	if err != nil || !strings.Contains(string(manifest), bundledCoopFolder+"/programa/openomsi.exe") {
		t.Fatal("incomplete installed inventory", err)
	}
	if _, err := verifyPackage(dir); err != nil {
		t.Fatal("new Setup must accept the installed core and Coop inventory", err)
	}
	files[bundledCoopFolder+"/programa/openomsi.exe"] = []byte("MZ changed bytes")
	if _, err := installAutomaticBridgeUpdate(dir, Config{}, files, func(string) error { return nil }); err == nil {
		t.Fatal("changed Coop runtime accepted")
	}
}

func TestBundledCoopSelectionNeedsGameOnlyAndDoesNotDependOnReadme(t *testing.T) {
	dir := t.TempDir()
	if bundledCoopExecutable(dir) != "" {
		t.Fatal("missing executable accepted")
	}
	path := filepath.Join(dir, bundledCoopFolder, "programa", "openomsi.exe")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("MZ fixture"), 0600)
	if bundledCoopExecutable(dir) != path {
		t.Fatal("bundled selection failed")
	}
}

func TestCoopBackupRestoresModifiedBuildAndRefusesStockOrTampering(t *testing.T) {
	dir := t.TempDir()
	name := bundledCoopFolder + "/programa/openomsi.exe"
	original := []byte("MZ custom cooperative build")
	spec := coopDistribution{FullPaths: true, Files: map[string]string{name: fmt.Sprintf("%x", sha256.Sum256(original))}}
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := writeSetupAtomic(p, original); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "coop.zip")
	if err := createCoopBackup(dir, backup, spec); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("MZ upstream stock updater"), 0600)
	if err := createCoopBackup(dir, backup, spec); err == nil {
		t.Fatal("stock build replaced good backup")
	}
	if err := restoreCoopBackup(dir, backup, spec); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !bytes.Equal(b, original) {
		t.Fatal("custom build not restored")
	}
	var z bytes.Buffer
	w := zip.NewWriter(&z)
	entry, _ := w.Create(name)
	entry.Write([]byte("tampered"))
	w.Close()
	os.WriteFile(backup, z.Bytes(), 0600)
	os.WriteFile(p, []byte("current"), 0600)
	if err := restoreCoopBackup(dir, backup, spec); err == nil {
		t.Fatal("tampered backup accepted")
	}
	b, _ = os.ReadFile(p)
	if string(b) != "current" {
		t.Fatal("changed installation before validating backup")
	}
}
