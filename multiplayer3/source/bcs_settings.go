package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
)

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
func splitConfigLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

func installedBCSJar(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("pasta do OMSI inválida")
	}
	var found string
	for _, folder := range []string{"Busbetrieb-Simulator", "Busbetriebs-Simulator", "Bus Company Simulator", "BBS", ""} {
		p := filepath.Join(root, folder, "busbetrieb-simulator.jar")
		s, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !s.Mode().IsRegular() || !insideDirectory(root, p) {
			return "", fmt.Errorf("BCS fora da pasta esperada")
		}
		if found != "" {
			return "", fmt.Errorf("mais de uma instalação BCS; escolha a pasta correta")
		}
		found = p
	}
	if found == "" {
		return "", fmt.Errorf("bcs settings not found")
	}
	return found, nil
}

var latin15 = map[byte]rune{0xa4: '€', 0xa6: 'Š', 0xa8: 'š', 0xb4: 'Ž', 0xb8: 'ž', 0xbc: 'Œ', 0xbd: 'œ', 0xbe: 'Ÿ'}

func decodeBCSConfig(b []byte) string {
	if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) || bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		return decodeText(b)
	}
	var s strings.Builder
	for _, v := range b {
		r := rune(v)
		if replacement, ok := latin15[v]; ok {
			r = replacement
		}
		s.WriteRune(r)
	}
	return s.String()
}

func encodeBCSConfig(text string, original []byte) ([]byte, error) {
	if bytes.HasPrefix(original, []byte{0xef, 0xbb, 0xbf}) {
		return append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), nil
	}
	if bytes.HasPrefix(original, []byte{0xff, 0xfe}) || bytes.HasPrefix(original, []byte{0xfe, 0xff}) {
		var order binary.ByteOrder = binary.LittleEndian
		if original[0] == 0xfe {
			order = binary.BigEndian
		}
		out := append([]byte{}, original[:2]...)
		for _, u := range utf16.Encode([]rune(text)) {
			var v [2]byte
			order.PutUint16(v[:], u)
			out = append(out, v[:]...)
		}
		return out, nil
	}
	var out []byte
	for _, r := range text {
		found := false
		for b, replacement := range latin15 {
			if replacement == r {
				out = append(out, b)
				found = true
				break
			}
		}
		if found {
			continue
		}
		if r > 255 {
			return nil, fmt.Errorf("bcs encoding: caminho não representável")
		}
		if _, replaced := latin15[byte(r)]; replaced {
			return nil, fmt.Errorf("bcs encoding: caractere não representável")
		}
		out = append(out, byte(r))
	}
	return out, nil
}

func readSmallFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !s.Mode().IsRegular() || s.Size() > limit {
		return nil, fmt.Errorf("arquivo inválido ou grande demais: %s", path)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("arquivo mudou durante leitura")
	}
	return b, err
}

func expandBCSPath(raw string) string {
	raw = os.ExpandEnv(raw)
	// Java/Windows applications commonly store %APPDATA%-style variables.
	// Expand only variables that actually exist; leave unknown tokens untouched.
	for start := strings.IndexByte(raw, '%'); start >= 0; {
		rest := raw[start+1:]
		endRel := strings.IndexByte(rest, '%')
		if endRel < 0 {
			break
		}
		end := start + 1 + endRel
		name := raw[start+1 : end]
		value, ok := os.LookupEnv(name)
		if !ok || name == "" {
			next := strings.IndexByte(raw[end+1:], '%')
			if next < 0 {
				break
			}
			start = end + 1 + next
			continue
		}
		raw = raw[:start] + value + raw[end+1:]
		start = strings.IndexByte(raw, '%')
	}
	return raw
}

func firstBCSPathLine(b []byte) string {
	for _, line := range splitConfigLines(decodeBCSConfig(b)) {
		line = strings.Trim(strings.TrimSpace(line), "\"")
		if line != "" {
			return line
		}
	}
	return ""
}

func regularBCSConfig(path string) (bool, error) {
	s, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !s.Mode().IsRegular() {
		return false, fmt.Errorf("bcs settings: config.cfg não é arquivo regular: %s", path)
	}
	if s.Size() > 128<<10 {
		return false, fmt.Errorf("bcs settings: config.cfg grande demais: %s", path)
	}
	return true, nil
}

func bcsSettingsPath(root string) (string, error) {
	jar, err := installedBCSJar(root)
	if err != nil {
		return "", err
	}
	jarDir := filepath.Dir(jar)
	var candidates []string
	add := func(path string) {
		path = filepath.Clean(path)
		for _, current := range candidates {
			if samePath(current, path) {
				return
			}
		}
		candidates = append(candidates, path)
	}

	gadPath := filepath.Join(jarDir, "gad.txt")
	gad, gadErr := readSmallFile(gadPath, 128<<10)
	if gadErr == nil {
		target := expandBCSPath(firstBCSPathLine(gad))
		if target != "" && !strings.ContainsAny(target, "\x00\r\n") {
			if !filepath.IsAbs(target) {
				target = filepath.Join(jarDir, target)
			}
			if strings.EqualFold(filepath.Base(target), "config.cfg") {
				add(target)
			} else {
				add(filepath.Join(target, "config.cfg"))
			}
		}
	} else if !os.IsNotExist(gadErr) {
		return "", fmt.Errorf("não foi possível ler gad.txt do BCS: %w", gadErr)
	}

	// Old and current BCS builds can keep config.cfg beside the JAR. Do not let
	// a stale/quoted gad.txt hide a perfectly valid local configuration.
	add(filepath.Join(jarDir, "config.cfg"))
	for _, path := range candidates {
		ok, err := regularBCSConfig(path)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}
	}
	if gadErr == nil {
		return "", fmt.Errorf("config.cfg do BCS não encontrado; gad.txt=%q; também verificado %s", firstBCSPathLine(gad), filepath.Join(jarDir, "config.cfg"))
	}
	return "", fmt.Errorf("config.cfg do BCS não encontrado em %s", jarDir)
}

func bcsSettingIndexes(lines []string, root string) (rootIndex, pathIndex, boolIndex int, err error) {
	rootIndex = -1
	if len(lines) > 0 && samePath(strings.Trim(strings.TrimSpace(lines[0]), "\""), root) {
		rootIndex = 0
	} else {
		for i, line := range lines {
			selected := strings.Trim(strings.TrimSpace(line), "\"")
			if selected != "" && samePath(selected, root) {
				if rootIndex >= 0 {
					return -1, -1, -1, fmt.Errorf("bcs settings: pasta do OMSI aparece mais de uma vez")
				}
				rootIndex = i
			}
		}
	}
	if rootIndex < 0 {
		return -1, -1, -1, fmt.Errorf("bcs settings belong to another OMSI installation")
	}
	pathIndex, boolIndex = rootIndex+49, rootIndex+50
	if boolIndex >= len(lines) {
		return -1, -1, -1, fmt.Errorf("bcs settings unsupported")
	}
	enabled := strings.TrimSpace(lines[boolIndex])
	if enabled != "true" && enabled != "false" {
		return -1, -1, -1, fmt.Errorf("bcs settings unsupported")
	}
	return rootIndex, pathIndex, boolIndex, nil
}

// This is the user settings layout inspected in the 2.x integration, not a
// PeDePe API. Unknown layouts are refused and use the on-screen folder fallback.
func updatedBCSSettings(original []byte, root, adapter string) ([]byte, error) {
	text := decodeBCSConfig(original)
	if roundTrip, e := encodeBCSConfig(text, original); e != nil || !bytes.Equal(roundTrip, original) {
		return nil, fmt.Errorf("bcs encoding: arquivo incompleto; preservado")
	}
	separator := "\n"
	if strings.Contains(text, "\r\n") {
		separator = "\r\n"
	}
	lines := strings.Split(text, separator)
	_, pathIndex, boolIndex, err := bcsSettingIndexes(lines, root)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(adapter) || strings.ContainsAny(adapter, "\r\n\x00\"") {
		return nil, fmt.Errorf("pasta do adaptador inválida")
	}
	lines[pathIndex], lines[boolIndex] = filepath.Clean(adapter)+string(filepath.Separator), "true"
	return encodeBCSConfig(strings.Join(lines, separator), original)
}

func configureBCS(root, dir string, closed func() error) (string, error) {
	path, err := bcsSettingsPath(root)
	if err != nil {
		return "", err
	}
	if err = closed(); err != nil {
		return "", err
	}
	adapter := adapterDirectory(dir)
	if s, err := os.Stat(filepath.Join(adapter, "openomsi.exe")); err != nil || !s.Mode().IsRegular() {
		return "", fmt.Errorf("arquivo do adaptador não encontrado")
	}
	before, err := readSmallFile(path, 128<<10)
	if err != nil {
		return "", err
	}
	after, err := updatedBCSSettings(before, root, adapter)
	if err != nil {
		return "", err
	}
	if bytes.Equal(before, after) {
		return "", nil
	}
	backup := path + ".multiplayer-" + time.Now().UTC().Format("20060102-150405.000000000") + ".bak"
	f, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, err = f.Write(before)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return "", err
	}
	current, err := readSmallFile(path, 128<<10)
	if err != nil {
		return backup, err
	}
	if !bytes.Equal(current, before) {
		return backup, fmt.Errorf("bcs settings changed; preserved")
	}
	if err = closed(); err != nil {
		return backup, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return backup, err
	}
	if !st.Mode().IsRegular() {
		return backup, fmt.Errorf("bcs settings: arquivo não regular; preservado")
	}
	if err = writeAtomic(path, after, st.Mode().Perm()); err != nil {
		return backup, err
	}
	return backup, nil
}

func inspectBCS(root, dir string) (bool, error) {
	path, err := bcsSettingsPath(root)
	if err != nil {
		return false, err
	}
	b, err := readSmallFile(path, 128<<10)
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.ReplaceAll(decodeBCSConfig(b), "\r\n", "\n"), "\n")
	_, pathIndex, boolIndex, err := bcsSettingIndexes(lines, root)
	if err != nil {
		return false, err
	}
	return samePath(strings.Trim(strings.TrimSpace(lines[pathIndex]), "\""), adapterDirectory(dir)) && strings.TrimSpace(lines[boolIndex]) == "true", nil
}

func deactivateBCS(root, dir string, closed func() error) (string, error) {
	if err := closed(); err != nil {
		return "", err
	}
	path, err := bcsSettingsPath(root)
	if err != nil {
		return "", err
	}
	before, err := readSmallFile(path, 128<<10)
	if err != nil {
		return "", err
	}
	text := decodeBCSConfig(before)
	sep := "\n"
	if strings.Contains(text, "\r\n") {
		sep = "\r\n"
	}
	lines := strings.Split(text, sep)
	_, pathIndex, boolIndex, err := bcsSettingIndexes(lines, root)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(lines[boolIndex]) == "false" {
		return "", nil
	}
	currentAdapter := strings.Trim(strings.TrimSpace(lines[pathIndex]), "\"")
	if !samePath(currentAdapter, adapterDirectory(dir)) && !knownPreviousV3Adapter(currentAdapter) {
		return "", fmt.Errorf("BCS now points to another adapter; preserved")
	}
	lines[boolIndex] = "false"
	after, err := encodeBCSConfig(strings.Join(lines, sep), before)
	if err != nil {
		return "", err
	}
	backup := path + ".multiplayer-disabled-" + time.Now().UTC().Format("20060102-150405.000000000") + ".bak"
	if err = os.WriteFile(backup, before, 0600); err != nil {
		return "", err
	}
	current, err := readSmallFile(path, 128<<10)
	if err != nil {
		return backup, err
	}
	if !bytes.Equal(before, current) {
		return backup, fmt.Errorf("bcs settings changed; preserved")
	}
	if err = closed(); err != nil {
		return backup, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return backup, err
	}
	if !st.Mode().IsRegular() {
		return backup, fmt.Errorf("arquivo não regular; preservado")
	}
	return backup, writeAtomic(path, after, st.Mode().Perm())
}
