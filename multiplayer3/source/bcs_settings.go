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

func bcsSettingsPath(root string) (string, error) {
	jar, err := installedBCSJar(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(jar)
	b, err := readSmallFile(filepath.Join(dir, "gad.txt"), 128<<10)
	if err == nil {
		dir = strings.TrimSpace(strings.SplitN(decodeBCSConfig(b), "\n", 2)[0])
		if !filepath.IsAbs(dir) || strings.ContainsAny(dir, "\x00\r\n\"") {
			return "", fmt.Errorf("bcs settings: diretório inválido")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return filepath.Join(dir, "config.cfg"), nil
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
	if len(lines) < 51 || (strings.TrimSpace(lines[50]) != "true" && strings.TrimSpace(lines[50]) != "false") {
		return nil, fmt.Errorf("bcs settings unsupported")
	}
	selectedRoot := strings.Trim(strings.TrimSpace(lines[0]), "\"")
	if !samePath(root, selectedRoot) {
		return nil, fmt.Errorf("bcs settings belong to another OMSI installation")
	}
	if !filepath.IsAbs(adapter) || strings.ContainsAny(adapter, "\r\n\x00\"") {
		return nil, fmt.Errorf("pasta do adaptador inválida")
	}
	lines[49], lines[50] = filepath.Clean(adapter)+string(filepath.Separator), "true"
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
	if len(lines) < 51 {
		return false, fmt.Errorf("bcs settings unsupported")
	}
	return samePath(strings.Trim(strings.TrimSpace(lines[0]), "\""), root) && samePath(strings.Trim(strings.TrimSpace(lines[49]), "\""), adapterDirectory(dir)) && strings.TrimSpace(lines[50]) == "true", nil
}
