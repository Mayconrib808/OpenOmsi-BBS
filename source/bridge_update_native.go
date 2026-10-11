package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var errNoNativeUpdateTarget = errors.New("a configuração nativa do BCS não foi encontrada")

// The inspected BCS system.m stores the OMSI root at index 0, the selected
// openOMSI folder at index 49 and the native-mode choice at index 50. Read its
// actual selection; the old Omsi.exe registry interception is not required.
func nativeBCSUpdateSelection(data []byte) (root, adapter string, enabled bool) {
	lines := strings.Split(strings.ReplaceAll(decodeBCSUpdateConfig(data), "\r\n", "\n"), "\n")
	if len(lines) < 51 {
		return "", "", false
	}
	root = strings.Trim(strings.TrimSpace(lines[0]), `"`)
	adapter = strings.Trim(strings.TrimSpace(lines[49]), `"`)
	return root, adapter, strings.EqualFold(strings.TrimSpace(lines[50]), "true")
}

// BCS reads and writes config.cfg and gad.txt as ISO-8859-15. Keep accented
// Windows paths intact; also accept BOM-marked Unicode settings.
func decodeBCSUpdateConfig(data []byte) string {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return decodeText(data)
	}
	var out strings.Builder
	for _, b := range data {
		r := rune(b)
		switch b {
		case 0xa4:
			r = '€'
		case 0xa6:
			r = 'Š'
		case 0xa8:
			r = 'š'
		case 0xb4:
			r = 'Ž'
		case 0xb8:
			r = 'ž'
		case 0xbc:
			r = 'Œ'
		case 0xbd:
			r = 'œ'
		case 0xbe:
			r = 'Ÿ'
		}
		out.WriteRune(r)
	}
	return out.String()
}

func readBCSUpdateSettings(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (128<<10)+1))
	if err == nil && len(data) > 128<<10 {
		err = fmt.Errorf("a configuração do BCS excede o limite de leitura")
	}
	return data, err
}

func nativeBCSSettingsDir(jar string) (string, error) {
	dir := filepath.Dir(jar)
	data, err := readBCSUpdateSettings(filepath.Join(dir, "gad.txt"))
	if os.IsNotExist(err) {
		return dir, nil
	}
	if err != nil {
		return "", err
	}
	selected := strings.TrimSpace(strings.SplitN(decodeBCSUpdateConfig(data), "\n", 2)[0])
	if !filepath.IsAbs(selected) {
		return "", fmt.Errorf("o diretório de configuração do BCS não é absoluto")
	}
	return filepath.Clean(selected), nil
}

func bridgeUpdateTargetAtAdapter(folder, expectedRoot string) (string, Config, error) {
	var empty Config
	folder = filepath.Clean(folder)
	if !filepath.IsAbs(folder) || !strings.EqualFold(filepath.Base(folder), "PeDePeAdapter") {
		return "", empty, errNoNativeUpdateTarget
	}
	dir := filepath.Dir(folder)
	if !fileExists(filepath.Join(folder, "openomsi.exe")) || !fileExists(filepath.Join(dir, "Setup.exe")) || !fileExists(configPath(dir)) {
		return "", empty, fmt.Errorf("o BCS aponta para uma pasta da Bridge incompleta")
	}
	c := readConfig(configPath(dir))
	if !filepath.IsAbs(c.Root) || !samePath(c.Root, expectedRoot) {
		return "", empty, fmt.Errorf("a pasta usada pelo BCS não corresponde à configuração da Bridge; nada foi alterado")
	}
	return dir, c, nil
}

func nativeBCSLaunchFolder(text string) string {
	marker := "OpenOMSI gestartet:"
	start := strings.LastIndex(text, marker)
	if start < 0 {
		return ""
	}
	command := strings.TrimSpace(text[start+len(marker):])
	if end := strings.IndexAny(command, "\r\n"); end >= 0 {
		command = command[:end]
	}
	end := strings.Index(strings.ToLower(command), "openomsi.exe --root ")
	if end < 0 {
		return ""
	}
	executable := strings.Trim(strings.TrimSpace(command[:end+len("openomsi.exe")]), `"`)
	return filepath.Dir(executable)
}

func readNativeDiscoveryTail(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("log do BCS não é arquivo regular")
	}
	var bom [2]byte
	f.ReadAt(bom[:], 0)
	unicode := bytes.Equal(bom[:], []byte{0xff, 0xfe}) || bytes.Equal(bom[:], []byte{0xfe, 0xff})
	start := int64(0)
	if st.Size() > 2<<20 {
		start = st.Size() - (2 << 20)
		if unicode && start%2 != 0 {
			start++
		}
	}
	if _, err = f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(f, 2<<20))
	if err != nil {
		return "", err
	}
	if start > 0 && unicode {
		data = append(bom[:], data...)
	}
	return decodeText(data), nil
}

func nativeBridgeUpdateTarget(roots []string) (string, Config, error) {
	var found string
	var config Config
	seen := map[string]bool{}
	for _, root := range roots {
		if !filepath.IsAbs(root) {
			continue
		}
		key := strings.ToLower(filepath.Clean(root))
		if seen[key] {
			continue
		}
		seen[key] = true
		jar, err := installedBCSJar(root)
		if err != nil {
			return "", config, err
		}
		if jar == "" {
			continue
		}
		settings, err := nativeBCSSettingsDir(jar)
		if err != nil {
			return "", config, err
		}
		cfg := filepath.Join(settings, "config.cfg")
		var folder string
		if data, e := readBCSUpdateSettings(cfg); e == nil {
			cfgRoot, selected, enabled := nativeBCSUpdateSelection(data)
			if !enabled || !samePath(cfgRoot, root) {
				continue
			}
			folder = selected
		} else if os.IsNotExist(e) {
			// A real native launch is a useful fallback for installations whose
			// settings have not yet been persisted. Never override saved settings.
			if text, e := readNativeDiscoveryTail(filepath.Join(settings, "log.txt")); e == nil {
				folder = nativeBCSLaunchFolder(text)
			}
		} else {
			return "", config, e
		}
		if folder == "" {
			continue
		}
		dir, c, err := bridgeUpdateTargetAtAdapter(folder, root)
		if errors.Is(err, errNoNativeUpdateTarget) {
			continue
		}
		if err != nil {
			return "", config, err
		}
		if found != "" && !samePath(found, dir) {
			return "", config, fmt.Errorf("há mais de uma instalação nativa da Bridge; nada foi alterado")
		}
		found, config = dir, c
	}
	if found == "" {
		return "", config, errNoNativeUpdateTarget
	}
	return found, config, nil
}

func automaticBridgeUpdateTarget(r registryAPI, preferredRoot string, steamRoots []string) (string, Config, error) {
	if filepath.IsAbs(preferredRoot) {
		dir, c, err := nativeBridgeUpdateTarget([]string{preferredRoot})
		if !errors.Is(err, errNoNativeUpdateTarget) {
			return dir, c, err
		}
	}
	dir, c, err := nativeBridgeUpdateTarget(steamRoots)
	if !errors.Is(err, errNoNativeUpdateTarget) {
		return dir, c, err
	}
	// Retain compatibility with old packages, without creating or requiring
	// their registration when the BCS native selection identifies the package.
	return registeredBridgeUpdateTarget(r)
}

func steamLibraryUpdateRoots(steamDir string) []string {
	if !filepath.IsAbs(steamDir) {
		return nil
	}
	roots := []string{filepath.Join(steamDir, "steamapps", "common", "OMSI 2")}
	pattern := regexp.MustCompile(`"(?:path|[0-9]+)"\s*"([^"\r\n]+)"`)
	for _, rel := range []string{"steamapps/libraryfolders.vdf", "config/libraryfolders.vdf"} {
		f, err := os.Open(filepath.Join(steamDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, 1<<20))
		f.Close()
		if err != nil {
			continue
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			path := strings.ReplaceAll(match[1], `\\`, `\`)
			if filepath.IsAbs(path) {
				roots = append(roots, filepath.Join(path, "steamapps", "common", "OMSI 2"))
			}
		}
	}
	return roots
}
