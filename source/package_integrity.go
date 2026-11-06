package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func writeSetupAtomic(path string, b []byte) error {
	// Recovery updates are allowed to recreate missing package subdirectories.
	// The destination name itself still comes from the updater's fixed allowlist.
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".bridge-setup-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return replaceSetupAtomicFile(name, path)
}

func replaceSetupAtomicFile(name, path string) error {
	var e error
	// Windows may retain the executable mapping briefly after Setup exits,
	// or a scanner may hold the verified replacement. Retry only sharing/lock
	// and access errors; keep the old file intact until an atomic rename works.
	deadline := time.Now().Add(8 * time.Second)
	for {
		e = os.Rename(name, path)
		if e == nil || runtime.GOOS != "windows" ||
			!(errors.Is(e, syscall.Errno(5)) || errors.Is(e, syscall.Errno(32)) || errors.Is(e, syscall.Errno(33))) ||
			time.Now().After(deadline) {
			return e
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func verifyPackage(dir string) (int, error) {
	return verifyPackageWithRepairableBinaries(dir, nil)
}

// The automatic updater carries its replacement executables inside the signed,
// SHA-256-verified installer. A stale, damaged OR MISSING executable that is
// about to be replaced must not make recovery impossible. Everything outside
// that exact replacement set remains under the normal integrity check.
func verifyPackageForUpdate(dir string, payload map[string][]byte) (int, error) {
	repairable := map[string]bool{}
	for name := range payload {
		if !bridgeUpdateBinaryAllowed(name) {
			return 0, fmt.Errorf("unsupported update component: %s", name)
		}
		repairable[strings.ToLower(name)] = true
	}
	return verifyPackageWithRepairableBinaries(dir, repairable)
}

// Setup activation needs a working Bridge, not a byte-for-byte copy of the
// entire distributable. Custom CompanyHost/HostAgent builds are independent
// applications; their hashes remain auditable with Setup.exe --verify.
// The 2.2.3.1 cumulative updater authenticates its complete Coop archive through
// the signed update manifest. The original release accidentally omitted
// docs/SHA256.txt from that archive; an activator must not reject its own
// verified install merely because that *legacy* file was not shipped.
//
// Never accept arbitrary hashes taken from the installed directory. The
// replacement Setup embeds expected hashes for the Bridge executables at
// build time, plus the signed Coop manifest for game/Launcher/P17 binaries.
var activationPinnedRuntimeSHA256 string

func verifyPackageForActivation(dir string) (int, error) {
	// The original P17 release and its later signed updaters use a pinned,
	// cumulative manifest for the actual game, launcher, server and Bridge
	// runtime. A legacy docs/SHA256.txt may remain in an upgraded directory
	// and still attest the old Setup.exe. That file must NOT override the
	// embedded signed-runtime pins when a newer Setup is installed.
	//
	// This does not waive executable integrity: the P17 gate below hashes
	// four Bridge runtime binaries and four native/host binaries against
	// build-time trusted SHA-256 values. The updater authenticates Setup.exe
	// itself using the separately signed manifest before installation.
	if bundledCoopDistribution.FullPaths &&
		strings.HasPrefix(bundledCoopDistribution.Prefix, "Bridge-2.2.3.1/") {
		return verifySignedCumulativeActivation(dir)
	}
	// Older, non-P17 releases still use the legacy full manifest policy.
	return verifyPackageWithPolicy(dir, nil, true)
}

func verifySignedCumulativeActivation(dir string) (int, error) {
	if !bundledCoopDistribution.FullPaths ||
		!strings.HasPrefix(bundledCoopDistribution.Prefix, "Bridge-2.2.3.1/") {
		return 0, fmt.Errorf("legacy package manifest missing; this Setup has no 2.2.3.1 cumulative integrity proof")
	}
	const expectedPins = 4
	pins := strings.Split(activationPinnedRuntimeSHA256, ":")
	if len(pins) != expectedPins {
		return 0, fmt.Errorf("Setup has no pinned Bridge executable hashes; repair with a signed installer")
	}
	names := []string{
		"PeDePeAdapter/openomsi.exe",
		"app/OpenOMSI_BCS_Bridge.exe",
		"app/compat/Omsi.exe",
		"app/compat/omsi-plugin-host32.exe",
	}
	digests := make([]string, 0, 8)
	digests = append(digests, pins...)
	coopNames := []string{
		"OpenOmsi Coop 0.3.41/programa/openomsi.exe",
		"OpenOmsi Coop 0.3.41/programa/openomsi-launcher-app.exe",
		"app/server-dinamico-p17/openomsi.exe",
		"app/hosts-p17/HostAgent.exe",
	}
	for _, name := range coopNames {
		digests = append(digests, bundledCoopDistribution.Files[name])
	}
	for i, name := range append(names, coopNames...) {
		expected := digests[i]
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != sha256.Size || expected != strings.ToLower(expected) {
			return i, fmt.Errorf("unsigned or malformed pinned SHA-256 for %s", name)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil {
			return i, fmt.Errorf("missing or inaccessible signed runtime %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return i, fmt.Errorf("runtime is not a regular file: %s", name)
		}
		base, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return i, err
		}
		full, err := filepath.EvalSymlinks(path)
		if err != nil {
			return i, err
		}
		inside, err := filepath.Rel(base, full)
		if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
			return i, fmt.Errorf("runtime file outside installed package: %s", name)
		}
		in, err := os.Open(path)
		if err != nil {
			return i, err
		}
		h := sha256.New()
		_, readErr := io.Copy(h, in)
		closeErr := in.Close()
		if readErr != nil || closeErr != nil {
			return i, fmt.Errorf("cannot verify signed runtime %s: read=%v close=%v", name, readErr, closeErr)
		}
		if !bytes.Equal(h.Sum(nil), decoded) {
			return i, fmt.Errorf("SHA256 mismatch in signed runtime: %s", name)
		}
	}
	return len(digests), nil
}

func packageActivationRuntimeBinary(rel string) bool {
	switch strings.ToLower(rel) {
	case "setup.exe", "app/openomsi_bcs_bridge.exe",
		"app/compat/omsi.exe", "app/compat/omsi-plugin-host32.exe",
		"pedepeadapter/openomsi.exe":
		return true
	}
	return false
}

func verifyPackageWithRepairableBinaries(dir string, repairable map[string]bool) (int, error) {
	return verifyPackageWithPolicy(dir, repairable, false)
}

func verifyPackageWithPolicy(dir string, repairable map[string]bool, activationOnly bool) (int, error) {
	patches, e := packagePatchHashes(dir)
	if e != nil {
		return 0, e
	}
	f, e := os.Open(filepath.Join(dir, "docs", "SHA256.txt"))
	if e != nil {
		return 0, e
	}
	defer f.Close()
	count := 0
	seen := map[string]bool{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 67 || line[64:66] != "  " {
			return count, fmt.Errorf("invalid SHA256 manifest")
		}
		expected, e := hex.DecodeString(line[:64])
		if e != nil || len(expected) != 32 {
			return count, fmt.Errorf("invalid SHA256 hash")
		}
		rel := line[66:]
		if replacement, ok := patches[rel]; ok {
			expected = replacement
			delete(patches, rel)
		}
		if strings.Contains(rel, `\`) || strings.HasPrefix(rel, "/") || strings.Contains(rel, ":") {
			return count, fmt.Errorf("unsafe manifest path")
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		r, e := filepath.Rel(dir, path)
		if e != nil || r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) || seen[strings.ToLower(rel)] || strings.EqualFold(rel, "docs/SHA256.txt") {
			return count, fmt.Errorf("unsafe or duplicate manifest path")
		}
		seen[strings.ToLower(rel)] = true
		// Activation verifies only the executable chain it actually installs.
		// A separate host build, optional assets or edited source files must
		// not disable the user's existing BCS integration.
		if activationOnly && !packageActivationRuntimeBinary(rel) {
			continue
		}
		// Documentation is optional and never participates in runtime integrity.
		if packageOptionalDocumentation(rel) {
			continue
		}
		st, e := os.Lstat(path)
		if e != nil {
			// A signed updater may restore a component it explicitly carries.
			if os.IsNotExist(e) && repairable[strings.ToLower(rel)] {
				count++
				continue
			}
			return count, e
		}
		if !st.Mode().IsRegular() {
			return count, fmt.Errorf("not a regular file: %s", rel)
		}
		full, e := filepath.EvalSymlinks(path)
		if e != nil {
			return count, e
		}
		base, e := filepath.EvalSymlinks(dir)
		if e != nil {
			return count, e
		}
		inside, e := filepath.Rel(base, full)
		if e != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
			return count, fmt.Errorf("file outside the package: %s", rel)
		}
		input, e := os.Open(path)
		if e != nil {
			return count, e
		}
		h := sha256.New()
		_, e = io.Copy(h, input)
		input.Close()
		if e != nil {
			return count, e
		}
		if !bytes.Equal(h.Sum(nil), expected) && !repairable[strings.ToLower(rel)] &&
			!approvedPassengerVoiceV2Digest(rel, fmt.Sprintf("%x", h.Sum(nil))) {
			// Only the eight known, user-approved Brazilian V2 WAV hashes
			// can differ from the original signed package manifest. All
			// executables, other assets, and arbitrary WAV edits stay strict.
			return count, fmt.Errorf("SHA256 mismatch: %s", rel)
		}
		count++
	}
	if e = scan.Err(); e != nil {
		return count, e
	}
	if len(patches) != 0 {
		return count, fmt.Errorf("update manifest references a component absent from the base package")
	}
	// Keep the legacy minimum list so old unit-test fixtures remain valid. In a
	// real 2.1 package PeDePeAdapter/openomsi.exe is still part of SHA256.txt, so
	// deleting or changing it is detected by the normal manifest verification.
	for _, required := range []string{"setup.exe", "app/openomsi_bcs_bridge.exe", "app/compat/omsi.exe", "app/compat/omsi-plugin-host32.exe"} {
		if !seen[required] {
			return count, fmt.Errorf("manifest lacks required component: %s", required)
		}
	}
	if count < 4 {
		return count, fmt.Errorf("empty or incomplete manifest")
	}
	return count, nil
}

// Small host/player updates attest only their replaced components. Keep the
// base manifest and all its checks, allowing both updates in either order.
func packagePatchHashes(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, name := range []string{"PATCH-PLAYER-SHA256.txt", "PATCH-HOST-SHA256.txt"} {
		p := filepath.Join(dir, "docs", name)
		b, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if len(b) > 16<<10 {
			return nil, fmt.Errorf("update manifest too large")
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSuffix(line, "\r")
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(line) < 67 || line[64:66] != "  " {
				return nil, fmt.Errorf("invalid update manifest")
			}
			h, e := hex.DecodeString(line[:64])
			if e != nil || len(h) != 32 {
				return nil, fmt.Errorf("invalid update checksum")
			}
			rel := line[66:]
			// Only executable components may be replaced by these binary patches.
			if !bridgeUpdateBinaryAllowed(rel) {
				return nil, fmt.Errorf("unsupported update component: %s", rel)
			}
			if old, ok := out[rel]; ok && !bytes.Equal(old, h) {
				return nil, fmt.Errorf("host/player updates have conflicting checksums for %s", rel)
			}
			out[rel] = h
		}
	}
	return out, nil
}

// Only human-readable documentation is optional. Executables, scripts, runtime
// configuration and integrity/control manifests retain their existing checks.
func packageOptionalDocumentation(rel string) bool {
	p := strings.ToLower(rel)
	name := strings.ToLower(filepath.Base(filepath.FromSlash(rel)))
	if !strings.Contains(p, "/") {
		return ((name == "leia-me.txt" || strings.HasPrefix(name, "leia-me-")) && strings.HasSuffix(name, ".txt")) ||
			((name == "readme.md" || strings.HasPrefix(name, "readme.")) && strings.HasSuffix(name, ".md")) || name == "tutorial.html"
	}
	if strings.HasPrefix(p, "docs/") {
		if strings.Contains(name, "sha256") || strings.HasPrefix(name, "patch-") || name == "package_files.txt" {
			return false
		}
		return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".txt")
	}
	return false
}
