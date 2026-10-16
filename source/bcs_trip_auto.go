package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type bcsNextTripPreparation struct {
	Path    string
	Changed bool
	Note    string
}

// These identify our generated observer, never an allowed BCS release/JAR.
const bcs503AutoLoggerSHA = "1307d92fbdd23a87891f095efabf3abadc59a8e7b48d74f8903c87decbdf2107"

var errBCSObserverUnavailable = errors.New("BCS next-trip observer does not match the installed communication interface")

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
	return nil, errBCSObserverUnavailable
}

func bcsLoggerInstalled(logger []byte) bool {
	hash := fmt.Sprintf("%x", sha256.Sum256(logger))
	if hash == bcsAutoLoggerSHA || hash == bcsPreviousLoggerSHA || hash == bcs503AutoLoggerSHA {
		return true
	}
	methods, ok := bcsClassMethods(logger)
	return ok && methods["bridgeNextTripDetails(Ljava/lang/String;)Ljava/lang/String;"] && bytes.Contains(logger, []byte("[OpenOmsi-BBS next-trip log v1]"))
}

// The automatic Setup carries copy ranges and our added bytecode, rather than
// a vendor class/JAR. Both input and reconstructed output have exact hashes.
func automaticBCSLogger(original []byte) ([]byte, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(original))
	if bcsLoggerInstalled(original) {
		return original, nil
	}
	if hash != bcsOriginalLoggerSHA {
		return nil, errBCSObserverUnavailable
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

func automaticBCSLoggerForJar(original []byte, jarPath string) ([]byte, error) {
	logger, err := automaticBCSLogger(original)
	if err != nil || bytes.Equal(logger, original) {
		return logger, err
	}
	owner, err := bcsTripStateOwner(jarPath)
	if err != nil {
		return nil, err
	}
	// Only the class reference added by our observer is rebound. Other JAR
	// entries, timestamps and resource changes do not affect compatibility.
	old := []byte("\x01\x00\x08system/I")
	if bytes.Count(logger, old) != 1 {
		return nil, fmt.Errorf("a referência da comunicação do BCS não confere")
	}
	reference := append([]byte{1, byte(len(owner) >> 8), byte(len(owner))}, []byte(owner)...)
	logger = bytes.Replace(logger, old, reference, 1)
	return logger, nil
}

// Read JVM method descriptors to locate the state API used by the observer.
// Obfuscated class names can change in automatic BCS updates; a whole-JAR hash
// or a version number cannot tell us which class actually owns that API.
func bcsClassMethods(data []byte) (map[string]bool, bool) {
	if len(data) < 10 || binary.BigEndian.Uint32(data) != 0xcafebabe {
		return nil, false
	}
	position := 8
	u2 := func() (uint16, bool) {
		if position+2 > len(data) {
			return 0, false
		}
		n := binary.BigEndian.Uint16(data[position:])
		position += 2
		return n, true
	}
	count, _ := u2()
	pool := make([]string, int(count))
	for i := 1; i < int(count); i++ {
		if position >= len(data) {
			return nil, false
		}
		tag := data[position]
		position++
		size := 0
		switch tag {
		case 1:
			n, ok := u2()
			if !ok || position+int(n) > len(data) {
				return nil, false
			}
			pool[i] = string(data[position : position+int(n)])
			size = int(n)
		case 3, 4, 9, 10, 11, 12, 17, 18:
			size = 4
		case 5, 6:
			size = 8
			i++
		case 7, 8, 16, 19, 20:
			size = 2
		case 15:
			size = 3
		default:
			return nil, false
		}
		position += size
		if position > len(data) {
			return nil, false
		}
	}
	position += 6 // access, this_class, super_class
	interfaces, ok := u2()
	if !ok {
		return nil, false
	}
	position += int(interfaces) * 2
	methods := map[string]bool{}
	for section := 0; section < 2; section++ {
		members, ok := u2()
		if !ok {
			return nil, false
		}
		for i := 0; i < int(members); i++ {
			access, a := u2()
			name, b := u2()
			descriptor, c := u2()
			attributes, d := u2()
			if !a || !b || !c || !d || int(name) >= len(pool) || int(descriptor) >= len(pool) {
				return nil, false
			}
			if section == 1 && access&0x0008 != 0 {
				methods[pool[name]+pool[descriptor]] = true
			}
			for j := 0; j < int(attributes); j++ {
				if position+6 > len(data) {
					return nil, false
				}
				size := uint64(binary.BigEndian.Uint32(data[position+2:]))
				position += 6
				if size > uint64(len(data)-position) {
					return nil, false
				}
				position += int(size)
			}
		}
	}
	return methods, true
}

func bcsTripStateOwner(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	owner := ""
	for _, entry := range r.File {
		if !strings.HasPrefix(entry.Name, "system/") || !strings.HasSuffix(entry.Name, ".class") || entry.UncompressedSize64 > 256<<10 {
			continue
		}
		input, err := entry.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(io.LimitReader(input, (256<<10)+1))
		input.Close()
		if err != nil {
			return "", err
		}
		methods, ok := bcsClassMethods(data)
		if !ok || !methods["h()B"] || !methods["I()J"] || !methods["i()Lr/n;"] || !methods["j()Lr/E;"] {
			continue
		}
		if owner != "" {
			return "", errBCSObserverUnavailable
		}
		owner = strings.TrimSuffix(entry.Name, ".class")
	}
	if owner == "" {
		return "", errBCSObserverUnavailable
	}
	return owner, nil
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
		if errors.Is(err, errBCSObserverUnavailable) {
			state.Note = bcsObserverNativeNote(c.Language)
			return state, nil
		}
		return state, err
	}
	jarSHA, err := bcsJarSHA(path)
	if err != nil {
		return state, err
	}
	logger, err := automaticBCSLoggerForJar(original, path)
	if err != nil {
		if errors.Is(err, errBCSObserverUnavailable) {
			state.Note = bcsObserverNativeNote(c.Language)
			return state, nil
		}
		return state, err
	}
	// If our logger is already present, do not reject the JAR merely because
	// replacing one entry necessarily changed the whole-archive SHA.
	if bytes.Equal(original, logger) {
		return state, nil
	}
	// This snapshot detects concurrent writes only; every installed BCS release
	// is accepted. Never compare this digest to a release/build allowlist.
	state.Changed, err = patchBCSJar(path, jarSHA, logger)
	return state, err
}

func bcsObserverNativeNote(lang string) string {
	return localText(lang,
		"BCS aceito. A integração nativa permanece ativa; a extensão de log da próxima viagem não foi aplicada à interface instalada.",
		"BCS accepted. Native integration remains active; the next-trip logging extension was not applied to the installed interface.",
		"BBS akzeptiert. Die native Integration bleibt aktiv; die Protokollerweiterung für die nächste Fahrt wurde nicht auf die installierte Schnittstelle angewendet.")
}
