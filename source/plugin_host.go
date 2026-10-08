package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const installedPluginHostName = "OpenOMSI_BCS_PluginHost32.exe"
const historicalPluginHostSHA256 = "b26af8f2fc55f272ae89f0dcd02c423c26fb758daf24e860960bd2002c82b2d4"

// Source-backed helper shipped in the tested 2.0.1 through 2.0.2-dev.4 packages.
const previousSourcePluginHostSHA256 = "26614aa1acd0a02c9ee272b8bf7fbea075daa3eba488ddfefb2e6c5f388e25f5"

// Source-backed GUI helper shipped in 2.0.3-dev.1. Recognize its exact bytes
// so Setup can replace it with the metadata-bearing dev.2 helper and roll back.
const previousGUIPluginHostSHA256 = "1c7f1f25bd18edb2b010ec42de2e34db5f3fc8ed2b1d948003635528b8a11a1a"

// The build script compiles the source-backed host first, then injects its digest
// into Setup and the launcher. The historical digest is only an upgrade/removal
// identifier; the historical binary is never included in a new distribution.
var bundledPluginHostSHA256 string

func recognisedPluginHostDigest(digest [32]byte) bool {
	hash := fmt.Sprintf("%x", digest)
	return hash == historicalPluginHostSHA256 || hash == previousSourcePluginHostSHA256 || hash == previousGUIPluginHostSHA256 || (bundledPluginHostSHA256 != "" && hash == bundledPluginHostSHA256)
}

type pluginHostDeployment struct {
	Path         string
	Created      bool
	digest       [32]byte
	previous     []byte
	previousMode os.FileMode
}

// waitForBCSStartupMarker gives BCS a short chance to publish its own bbs.start
// marker before openOMSI starts the retained 32-bit plugin host. The bridge never
// creates, reads or deletes marker contents. Presence is a startup prerequisite,
// not evidence of a live plugin connection or a visible panel.
func waitForBCSStartupMarker(root string, timeout time.Duration) (bool, time.Duration) {
	start := time.Now()
	if timeout <= 0 {
		_, err := os.Stat(filepath.Join(root, "bbs.start"))
		return err == nil, time.Since(start)
	}
	deadline := start.Add(timeout)
	for {
		if _, err := os.Stat(filepath.Join(root, "bbs.start")); err == nil {
			return true, time.Since(start)
		}
		if time.Now().After(deadline) {
			return false, time.Since(start)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func pluginHostPath(c Config) (string, error) {
	if !filepath.IsAbs(c.Root) || strings.ContainsAny(c.Root, "\r\n\x00\"") {
		return "", fmt.Errorf("configure the original OMSI folder first")
	}
	return filepath.Join(c.Root, installedPluginHostName), nil
}

func regularFileDigest(path string) ([32]byte, error) {
	var zero [32]byte
	st, err := os.Lstat(path)
	if err != nil {
		return zero, err
	}
	if !st.Mode().IsRegular() {
		return zero, fmt.Errorf("not a regular file: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	return sha256.Sum256(b), nil
}

// bbs.dll checks <directory of ParamStr(0)>/bbs.start during PluginStart.
// Merely changing the helper's working directory is insufficient. Deploy the
// bundled helper beside the original Omsi.exe, under a bridge-specific name.
// Never create, copy or delete the BCS-owned bbs.start marker.
func preparePluginHost(c Config, packageDir string) (pluginHostDeployment, error) {
	var result pluginHostDeployment
	source, err := pluginHostSource(c, packageDir)
	if err != nil {
		return result, err
	}
	result.Path, err = pluginHostPath(c)
	if err != nil {
		return result, err
	}
	b, err := os.ReadFile(source)
	if err != nil {
		return result, err
	}
	result.digest = sha256.Sum256(b)
	checkExisting := func() error {
		digest, err := regularFileDigest(result.Path)
		if err != nil {
			return err
		}
		if digest != result.digest {
			if recognisedPluginHostDigest(digest) {
				st, err := os.Lstat(result.Path)
				if err != nil || !st.Mode().IsRegular() {
					return fmt.Errorf("helper changed during update; preserved: %s", result.Path)
				}
				previous, err := os.ReadFile(result.Path)
				if err != nil {
					return err
				}
				if sha256.Sum256(previous) != digest {
					return fmt.Errorf("helper changed during update; preserved: %s", result.Path)
				}
				if err := replacePluginHost(result.Path, b, st.Mode().Perm()); err != nil {
					return err
				}
				result.previous, result.previousMode = previous, st.Mode().Perm()
				return nil
			}
			return fmt.Errorf("%s: %s", localText(c.Language,
				"Já existe um auxiliar diferente nesse caminho. O arquivo foi preservado; confira a instalação antes de ativar",
				"A different helper already exists at this path. It was preserved; check the installation before activating", "Unter diesem Pfad existiert bereits ein anderes Hilfsprogramm. Es wurde beibehalten. Prüfe die Installation vor der Aktivierung"), result.Path)
		}
		return nil
	}
	if _, err := os.Lstat(result.Path); err == nil {
		err = checkExisting()
		return result, err
	} else if !os.IsNotExist(err) {
		return result, err
	}
	f, err := os.OpenFile(result.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if os.IsExist(err) {
		err = checkExisting()
		return result, err
	}
	if err != nil {
		return result, fmt.Errorf("%s: %w", localText(c.Language,
			"Não foi possível preparar o auxiliar na pasta do OMSI. Feche os jogos e use a opção 2 do Setup.exe (Ativar / atualizar)",
			"Could not prepare the helper in the OMSI folder. Close the games and use Setup.exe option 2 (Activate / update)", "Das Hilfsprogramm konnte im OMSI-Ordner nicht bereitgestellt werden. Schließe die Spiele und wähle in Setup.exe Option 2 (Aktivieren / aktualisieren)"), err)
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		if cleanupErr := os.Remove(result.Path); cleanupErr != nil {
			return result, fmt.Errorf("%w; incomplete helper cleanup failed: %v", err, cleanupErr)
		}
		return result, err
	}
	result.Created = true
	return result, nil
}

// Write in the target directory and atomically replace only a recognised file.
func replacePluginHost(path string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".bridge-helper-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Undo a new copy or restore recognised previous bytes after failed activation.
// If another process changed the target meanwhile, preserve its new content.
func rollbackPluginHost(d pluginHostDeployment) error {
	if !d.Created && d.previous == nil {
		return nil
	}
	digest, err := regularFileDigest(d.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if digest != d.digest {
		return fmt.Errorf("helper changed during activation; preserved: %s", d.Path)
	}
	if d.previous != nil {
		return replacePluginHost(d.Path, d.previous, d.previousMode)
	}
	return os.Remove(d.Path)
}

// Deactivation only removes the bridge-specific filename and a recognised
// payload. Other helpers, a modified copy, original game files and markers remain.
func removeInstalledPluginHost(root string) (string, error) {
	p, err := pluginHostPath(Config{Root: root})
	if err != nil {
		return "", err
	}
	digest, err := regularFileDigest(p)
	if os.IsNotExist(err) {
		return "absent", nil
	}
	if err != nil {
		if st, statErr := os.Lstat(p); statErr == nil && !st.Mode().IsRegular() {
			return "preserved", nil
		}
		return "", err
	}
	if !recognisedPluginHostDigest(digest) {
		return "preserved", nil
	}
	if err := os.Remove(p); err != nil {
		return "", err
	}
	return "removed", nil
}

// Paths, hashes and existence checks only; never read marker contents or copy
// additional game files into a diagnostic archive.
func pluginHostDiagnostics(c Config, packageDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OpenOMSI BCS Bridge v%s - by %s\r\n", bridgeVersion, bridgeAuthor)
	if source, err := pluginHostSource(c, packageDir); err != nil {
		fmt.Fprintf(&b, "Source error: %v\r\n", err)
	} else {
		fmt.Fprintf(&b, "Source: %s\r\n", source)
		if digest, err := regularFileDigest(source); err == nil {
			fmt.Fprintf(&b, "Source SHA-256: %x\r\n", digest)
		} else {
			fmt.Fprintf(&b, "Source hash error: %v\r\n", err)
		}
	}
	if target, err := pluginHostPath(c); err != nil {
		fmt.Fprintf(&b, "Runtime path error: %v\r\n", err)
	} else {
		fmt.Fprintf(&b, "Runtime: %s\r\n", target)
		if digest, err := regularFileDigest(target); err == nil {
			fmt.Fprintf(&b, "Runtime SHA-256: %x\r\n", digest)
		} else {
			fmt.Fprintf(&b, "Runtime hash error: %v\r\n", err)
		}
		_, markerErr := os.Stat(filepath.Join(c.Root, "bbs.start"))
		fmt.Fprintf(&b, "BCS startup marker present at collection: %t\r\n", markerErr == nil)
		fmt.Fprint(&b, "BCS creates the marker and the plugin consumes it. Absence after plugin initialization can be normal.\r\n")
	}
	return b.String()
}
