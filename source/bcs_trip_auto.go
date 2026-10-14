package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type bcsNextTripPreparation struct {
	Path    string
	Changed bool
}

// 2.1.9 keeps the previously validated BCS build working while adding the
// official 5.0.2.0 build. Both builds contain the same original bbs/f.class,
// so they use the same narrow logger delta.
const bcsPreviousInspectedJarSHA = "b624b07f436eaa67aee51ea41340529b8f6a6d1d0d29d4c9c199c61d0ca3bbe0"

func supportedBCSJarSHA(sha string) bool {
	return sha == bcsInspectedJarSHA || sha == bcsPreviousInspectedJarSHA
}

// Resolve only the BCS belonging to the configured OMSI installation. Never
// ask a player to select a JAR or scan unrelated downloads/installations.
func installedBCSJar(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("a pasta do OMSI ainda não está configurada")
	}
	var found string
	for _, folder := range []string{"Busbetrieb-Simulator", "Busbetriebs-Simulator", "Bus Company Simulator", "BBS", ""} {
		path := filepath.Join(root, folder, "busbetrieb-simulator.jar")
		st, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("o BCS não é um arquivo regular")
		}
		if found != "" && !samePath(found, path) {
			return "", fmt.Errorf("há mais de uma instalação do BCS nesta pasta; atualização preservada")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		base, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(base, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("o BCS está fora da instalação configurada")
		}
		found = path
	}
	return found, nil
}

func bcsLoggerBytes(path string) ([]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != bcsLoggerEntry {
			continue
		}
		if f.UncompressedSize64 > 256<<10 {
			return nil, fmt.Errorf("logger do BCS inválido")
		}
		input, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(input)
		input.Close()
		return data, err
	}
	return nil, fmt.Errorf("a comunicação desta versão do BCS ainda não foi validada")
}

// The automatic Setup carries copy ranges and our added bytecode, rather than
// a vendor class/JAR. Both input and reconstructed output have exact hashes.
func automaticBCSLogger(original []byte) ([]byte, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(original))
	if hash == bcsAutoLoggerSHA || hash == bcsPreviousLoggerSHA {
		return original, nil
	}
	if hash != bcsOriginalLoggerSHA {
		return nil, fmt.Errorf("a comunicação desta versão do BCS ainda não foi validada")
	}
	delta, err := base64.StdEncoding.DecodeString(bcsAutoLoggerDelta)
	if err != nil || len(delta) < 9 || string(delta[:9]) != "BCSDELTA1" {
		return nil, fmt.Errorf("atualização de comunicação inválida")
	}
	delta = delta[9:]
	var result []byte
	for len(delta) > 0 {
		op := delta[0]
		delta = delta[1:]
		switch op {
		case 0:
			if len(delta) < 8 {
				return nil, fmt.Errorf("delta incompleto")
			}
			start, count := uint64(binary.BigEndian.Uint32(delta)), uint64(binary.BigEndian.Uint32(delta[4:]))
			delta = delta[8:]
			if start+count > uint64(len(original)) {
				return nil, fmt.Errorf("delta fora dos limites")
			}
			result = append(result, original[int(start):int(start+count)]...)
		case 1:
			if len(delta) < 4 {
				return nil, fmt.Errorf("delta incompleto")
			}
			count := uint64(binary.BigEndian.Uint32(delta))
			delta = delta[4:]
			if count > uint64(len(delta)) {
				return nil, fmt.Errorf("delta incompleto")
			}
			result = append(result, delta[:int(count)]...)
			delta = delta[int(count):]
		default:
			return nil, fmt.Errorf("delta inválido")
		}
		if len(result) > 256<<10 {
			return nil, fmt.Errorf("delta muito grande")
		}
	}
	if fmt.Sprintf("%x", sha256.Sum256(result)) != bcsAutoLoggerSHA || bytes.Equal(original, result) {
		return nil, fmt.Errorf("a integridade da comunicação não confere")
	}
	return result, nil
}

func prepareBCSNextTrip(c Config, checkClosed func(string) error) (bcsNextTripPreparation, error) {
	path, err := installedBCSJar(c.Root)
	state := bcsNextTripPreparation{Path: path}
	if err != nil || path == "" {
		return state, err
	}
	if checkClosed != nil {
		if err = checkClosed(path); err != nil {
			return state, err
		}
	}
	original, err := bcsLoggerBytes(path)
	if err != nil {
		return state, err
	}
	logger, err := automaticBCSLogger(original)
	if err != nil {
		return state, err
	}
	// If our logger is already present, do not reject the JAR merely because
	// replacing one entry necessarily changed the whole-archive SHA.
	if bytes.Equal(original, logger) {
		return state, nil
	}
	jarSHA, err := bcsJarSHA(path)
	if err != nil {
		return state, err
	}
	if !supportedBCSJarSHA(jarSHA) {
		return state, fmt.Errorf("a comunicação desta versão do BCS ainda não foi validada")
	}
	state.Changed, err = patchBCSJar(path, jarSHA, logger)
	return state, err
}
