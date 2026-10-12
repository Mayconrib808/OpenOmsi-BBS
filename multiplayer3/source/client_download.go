package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func cachedClient(dir, version string) (string, error) {
	folder := filepath.Join(dir, "client", version)
	b, e := os.ReadFile(filepath.Join(folder, "verified.json"))
	if e != nil {
		return "", e
	}
	var r serverReceipt
	if json.Unmarshal(b, &r) != nil || r.Version != version {
		return "", fmt.Errorf("cache do cliente inválido")
	}
	if e = verifyClient(folder, r); e != nil {
		return "", e
	}
	return filepath.Join(folder, "openomsi.exe"), nil
}
func safeArchivePath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && !strings.ContainsAny(p, "\\:") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func verifyClient(dir string, r serverReceipt) error {
	if r.Files["openomsi.exe"] == "" || r.Files["steam_api64.dll"] == "" {
		return fmt.Errorf("download do cliente incompleto")
	}
	for name, want := range r.Files {
		if !safeArchivePath(name) {
			return fmt.Errorf("cache do cliente inválido")
		}
		got, e := hashFile(filepath.Join(dir, filepath.FromSlash(name)))
		if e != nil || got != want {
			return fmt.Errorf("cache do cliente alterado: %s", name)
		}
	}
	return nil
}
func unpackClient(archive, dest, version, digest string) (serverReceipt, error) {
	r := serverReceipt{Version: version, ArchiveSHA256: digest, Files: map[string]string{}}
	z, e := zip.OpenReader(archive)
	if e != nil {
		return r, e
	}
	defer z.Close()
	prefix := ""
	found := false
	for _, f := range z.File {
		if path.Base(f.Name) == "openomsi.exe" {
			if found {
				return r, fmt.Errorf("download com mais de um cliente")
			}
			prefix = strings.TrimSuffix(f.Name, "openomsi.exe")
			found = true
		}
	}
	if !found {
		return r, fmt.Errorf("download sem cliente")
	}
	var total uint64
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safeArchivePath(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return r, fmt.Errorf("caminho inválido no ZIP")
		}
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		name := strings.TrimPrefix(f.Name, prefix)
		if !safeArchivePath(name) {
			return r, fmt.Errorf("caminho inválido no ZIP")
		}
		if strings.EqualFold(name, "plugins/bbs.lua") {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := r.Files[key]; ok {
			return r, fmt.Errorf("arquivo duplicado no ZIP")
		}
		total += f.UncompressedSize64
		if total > 1<<30 {
			return r, fmt.Errorf("ZIP do cliente excede o limite")
		}
		// Windows treats case variants as the same path; store one normalized path.
		target := filepath.Join(dest, filepath.FromSlash(key))
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return r, e
		}
		in, e := f.Open()
		if e != nil {
			return r, e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return r, e
		}
		h := sha256.New()
		_, e = io.Copy(io.MultiWriter(out, h), io.LimitReader(in, int64(f.UncompressedSize64)+1))
		ce := out.Close()
		in.Close()
		if e != nil {
			return r, e
		}
		if ce != nil {
			return r, ce
		}
		r.Files[key] = hex.EncodeToString(h.Sum(nil))
	}
	return r, verifyClient(dest, r)
}
