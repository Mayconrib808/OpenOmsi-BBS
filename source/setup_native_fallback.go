package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The current BCS/BBS can launch PeDePeAdapter directly. That native mode
// does not require IFEO Registry interception. For machines with protected
// IFEO permissions, use this fallback only after the user explicitly clicked
// Save and activate and the traditional Registry transaction was rolled back.
// Preserve every unrelated BCS preference, including its original byte encoding.
const nativeFallbackBackupName = "config.cfg.bridge-before-native.bak"

func writeNativeBCSConfigAtomically(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bridge-native-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(mode.Perm()); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// A new native BCS selection is safe only when its input follows the exact
// inspected 51-line BCS format. Never guess field offsets or rewrite encodings.
func nativeBCSFallbackBytes(data []byte, root, adapter string) ([]byte, error) {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) ||
		bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return nil, fmt.Errorf("BCS configuration uses an unsupported Unicode BOM; keep native selection unchanged")
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(adapter) || strings.ContainsAny(adapter, "\r\n\x00") {
		return nil, fmt.Errorf("invalid native BCS adapter path")
	}
	// BCS uses ISO-8859-15. Paths that cannot be represented as ASCII are
	// deliberately left for BCS's own UI instead of silently corrupting them.
	for _, r := range adapter {
		if r > 0x7f {
			return nil, fmt.Errorf("native adapter path contains characters requiring the BCS user interface")
		}
	}
	currentRoot, _, _ := nativeBCSUpdateSelection(data)
	if !samePath(currentRoot, root) {
		return nil, fmt.Errorf("native BCS settings point to another OMSI folder")
	}
	end := []byte("\r\n")
	if !bytes.Contains(data, end) {
		end = []byte("\n")
	}
	lines := bytes.Split(data, end)
	if len(lines) < 51 {
		return nil, fmt.Errorf("native BCS settings lack required fields")
	}
	flag := strings.ToLower(strings.TrimSpace(string(lines[50])))
	if flag != "true" && flag != "false" {
		return nil, fmt.Errorf("native BCS mode has an unexpected value")
	}
	lines[49] = []byte(adapter)
	lines[50] = []byte("true")
	return bytes.Join(lines, end), nil
}

func enableNativeBCSFallback(dir string, c Config) (bool, error) {
	adapter := filepath.Join(dir, "PeDePeAdapter")
	if !fileExists(filepath.Join(adapter, "openomsi.exe")) {
		return false, fmt.Errorf("signed PeDePeAdapter is missing")
	}
	jar, err := installedBCSJar(c.Root)
	if err != nil || jar == "" {
		return false, fmt.Errorf("BCS installation not found for native fallback: %v", err)
	}
	if err = checkBCSClosedForUpdate(jar); err != nil {
		return false, err
	}
	folder, err := nativeBCSSettingsDir(jar)
	if err != nil {
		return false, err
	}
	path := filepath.Join(folder, "config.cfg")
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return false, fmt.Errorf("BCS configuration unavailable or not regular: %v", err)
	}
	original, err := readBCSUpdateSettings(path)
	if err != nil {
		return false, err
	}
	updated, err := nativeBCSFallbackBytes(original, c.Root, adapter)
	if err != nil {
		return false, err
	}
	if bytes.Equal(updated, original) {
		return setupNativeSelected(dir, c), nil
	}
	backup := filepath.Join(folder, nativeFallbackBackupName)
	if prior, e := os.ReadFile(backup); e == nil {
		if !bytes.Equal(prior, original) {
			return false, fmt.Errorf("BCS native-mode backup belongs to different settings; refusing overwrite")
		}
	} else if os.IsNotExist(e) {
		f, createErr := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if createErr != nil {
			return false, createErr
		}
		_, writeErr := f.Write(original)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			os.Remove(backup)
			return false, fmt.Errorf("BCS native backup failed: write=%v close=%v", writeErr, closeErr)
		}
	} else {
		return false, e
	}
	if err = writeNativeBCSConfigAtomically(path, updated, st.Mode()); err != nil {
		return false, err
	}
	if !setupNativeSelected(dir, c) {
		if rollbackErr := writeNativeBCSConfigAtomically(path, original, st.Mode()); rollbackErr != nil {
			return false, fmt.Errorf("native BCS selection did not persist; rollback failed: %v", rollbackErr)
		}
		return false, fmt.Errorf("native BCS selection did not persist")
	}
	fmt.Printf("BCS native OpenOMSI integration enabled without IFEO; previous preferences backed up at %s\n", backup)
	return true, nil
}

func restoreNativeBCSFallback(dir string, c Config) (bool, error) {
	jar, err := installedBCSJar(c.Root)
	if err != nil || jar == "" {
		return false, err
	}
	folder, err := nativeBCSSettingsDir(jar)
	if err != nil {
		return false, err
	}
	backup := filepath.Join(folder, nativeFallbackBackupName)
	original, err := os.ReadFile(backup)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !setupNativeSelected(dir, c) {
		return false, fmt.Errorf("BCS native selection was changed externally; preserved its original backup")
	}
	path := filepath.Join(folder, "config.cfg")
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return false, fmt.Errorf("BCS config is not a regular file: %v", err)
	}
	if err = checkBCSClosedForUpdate(jar); err != nil {
		return false, err
	}
	if err = writeNativeBCSConfigAtomically(path, original, st.Mode()); err != nil {
		return false, err
	}
	if err = os.Remove(backup); err != nil {
		return true, fmt.Errorf("previous BCS settings restored, but backup cleanup failed: %w", err)
	}
	return true, nil
}

var _ = sha256.Size // Keep the config fallback tied to SHA-256-gated Setup verification.
