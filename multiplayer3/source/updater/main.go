package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The build generates an embedded payload; no publisher credential is shipped.
var payload []byte

type receipt struct {
	Product string            `json:"product"`
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

var allowed = map[string]bool{
	"Setup.exe": true,
	"OpenOMSI-BBS/PeDePeAdapter/openomsi.exe":    true,
	"OpenOMSI-BBS/compat/omsi-plugin-host32.exe": true,
}

func unpack(data []byte) (map[string][]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	var total uint64
	for _, f := range z.File {
		if (!allowed[f.Name] && f.Name != "package.json") || files[f.Name] != nil || !f.Mode().IsRegular() || f.UncompressedSize64 > 16<<20 {
			return nil, fmt.Errorf("unexpected update component")
		}
		total += f.UncompressedSize64
		if total > 48<<20 {
			return nil, fmt.Errorf("update too large")
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(r, 16<<20+1))
		r.Close()
		if err != nil || len(b) > 16<<20 {
			return nil, fmt.Errorf("incomplete update component")
		}
		files[f.Name] = b
	}
	var meta receipt
	if json.Unmarshal(files["package.json"], &meta) != nil || meta.Product != "bbs-openomsi-multiplayer" || !strings.HasPrefix(meta.Version, "3.") || len(meta.Files) != len(allowed) || len(files) != len(allowed)+1 {
		return nil, fmt.Errorf("invalid update identity")
	}
	for name := range allowed {
		b := files[name]
		if len(b) < 2 || string(b[:2]) != "MZ" || fmt.Sprintf("%x", sha256.Sum256(b)) != meta.Files[name] {
			return nil, fmt.Errorf("update component failed SHA-256 check")
		}
	}
	delete(files, "package.json")
	files["OpenOMSI-BBS/build.json"], _ = json.MarshalIndent(meta, "", "  ")
	return files, nil
}

func atomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".update-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return replacePreparedFile(name, path)
}

func existingParent(path string) string {
	current := path
	var missing []string
	for {
		if real, e := filepath.EvalSymlinks(current); e == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func apply(root string, data []byte) (backup string, err error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("target must be absolute")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(root, "OpenOMSI-BBS", "config", "multiplayer.json"))
	if err != nil {
		return "", fmt.Errorf("configure 3.x before updating")
	}
	var config struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(b, &config) != nil || config.Version != 3 {
		return "", fmt.Errorf("this updater only supports 3.x")
	}
	files, err := unpack(data)
	if err != nil {
		return "", err
	}
	var names []string
	for name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		parent := existingParent(filepath.Dir(path))
		rel, err := filepath.Rel(root, parent)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("update path escapes installation")
		}
		if s, err := os.Lstat(path); err == nil {
			if !s.Mode().IsRegular() {
				return "", fmt.Errorf("component is not a regular file")
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err = os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(name))), 0755); err != nil {
			return "", err
		}
	}
	if err = os.MkdirAll(filepath.Join(root, "OpenOMSI-BBS", "updater"), 0755); err != nil {
		return "", err
	}
	backup, err = os.MkdirTemp(filepath.Join(root, "OpenOMSI-BBS", "updater"), "backup-")
	if err != nil {
		return "", err
	}
	original := map[string][]byte{}
	var written []string
	defer func() {
		if err == nil {
			return
		}
		for i := len(written) - 1; i >= 0; i-- {
			name := written[i]
			path := filepath.Join(root, filepath.FromSlash(name))
			current, re := os.ReadFile(path)
			if re != nil || !bytes.Equal(current, files[name]) {
				err = fmt.Errorf("%w; concurrent change preserved", err)
				continue
			}
			if original[name] == nil {
				re = os.Remove(path)
			} else {
				re = atomicWrite(path, original[name])
			}
			if re != nil {
				err = fmt.Errorf("%w; rollback failed: %v", err, re)
			}
		}
	}()
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		old, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			return backup, e
		}
		original[name] = old
		if old != nil {
			dest := filepath.Join(backup, filepath.FromSlash(name))
			if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
				return backup, e
			}
			if e = os.WriteFile(dest, old, 0600); e != nil {
				return backup, e
			}
		}
	}
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		current, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			return backup, e
		}
		if !bytes.Equal(current, original[name]) {
			return backup, fmt.Errorf("installation changed during update; preserved")
		}
		if err = atomicWrite(path, files[name]); err != nil {
			return backup, err
		}
		written = append(written, name)
	}
	return backup, nil
}

func main() {
	target := flag.String("target", "", "installation folder")
	pid := flag.Int("pid", 0, "setup process")
	flag.Parse()
	exe, err := os.Executable()
	if err == nil {
		*target, err = updateTarget(*target, exe, chooseUpdateTarget)
	}
	if errors.Is(err, errUpdateCancelled) { return }
	if err == nil { err = waitForSetup(*pid) }
	if err == nil {
		_, err = apply(*target, payload)
	}
	if err != nil {
		showFailure(*target, err)
		return
	}
	_ = os.Remove(filepath.Join(*target, "OpenOMSI-BBS", "logs", "update-error.txt"))
	cmd := exec.Command(filepath.Join(*target, "Setup.exe"))
	if err = cmd.Start(); err == nil {
		err = cmd.Process.Release()
	}
	if err != nil {
		showFailure(*target, err)
	}
}
