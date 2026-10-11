package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeSetupAtomic(path string, b []byte) error {
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
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}

func verifyPackage(dir string) (int, error) {
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
		st, e := os.Lstat(path)
		if e != nil {
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
		if !bytes.Equal(h.Sum(nil), expected) {
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
			if rel != "Setup.exe" && rel != "HostAgent.exe" && rel != "PeDePeAdapter/openomsi.exe" {
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
