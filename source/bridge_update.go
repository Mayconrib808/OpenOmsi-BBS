package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Compatibility fallback for packages using the old Omsi.exe interception.
// Native packages are identified from the BCS selection before this is called.
func registeredBridgeUpdateTarget(r registryAPI) (string, Config, error) {
	var found string
	var config Config
	for _, view := range []int{64, 32} {
		value, err := r.Read(view, ifeoBridge, "Debugger")
		if err != nil {
			return "", config, err
		}
		if !value.Present {
			continue
		}
		command := value.text()
		if len(command) < 2 || command[0] != '"' || command[len(command)-1] != '"' {
			return "", config, fmt.Errorf("registro da Bridge inválido")
		}
		executable := command[1 : len(command)-1]
		if !filepath.IsAbs(executable) || !strings.EqualFold(filepath.Base(executable), "OpenOMSI_BCS_Bridge.exe") || !strings.EqualFold(filepath.Base(filepath.Dir(executable)), "app") {
			return "", config, fmt.Errorf("o registro pertence a outro programa; nada foi alterado")
		}
		dir := filepath.Dir(filepath.Dir(executable))
		if found != "" && !samePath(found, dir) {
			return "", config, fmt.Errorf("há registros de instalações diferentes da Bridge; nada foi alterado")
		}
		if !fileExists(configPath(dir)) {
			return "", config, fmt.Errorf("a instalação registrada da Bridge está incompleta")
		}
		c := readConfig(configPath(dir))
		original, err := r.Read(view, ifeoBridge, "FilterFullPath")
		if err != nil {
			return "", config, err
		}
		if !filepath.IsAbs(c.Root) || !samePath(original.text(), filepath.Join(c.Root, "Omsi.exe")) {
			return "", config, fmt.Errorf("a pasta salva não corresponde à instalação registrada; nada foi alterado")
		}
		found, config = dir, c
	}
	if found == "" {
		return "", config, fmt.Errorf("não encontrei a pasta da Bridge nas configurações do BCS. Envie os logs do BCS ao administrador da empresa")
	}
	return found, config, nil
}

func bridgeUpdateBinaryAllowed(name string) bool {
	switch name {
	case "Setup.exe", "HostAgent.exe", "CompanyHost.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe":
		return true
	}
	return false
}

func readAutomaticBridgePayload(data []byte) (map[string][]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		if !bridgeUpdateBinaryAllowed(f.Name) || out[f.Name] != nil || f.UncompressedSize64 > 16<<20 {
			return nil, fmt.Errorf("componente inesperado no atualizador")
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}
		if len(b) < 2 || string(b[:2]) != "MZ" {
			return nil, fmt.Errorf("executável de atualização inválido")
		}
		out[f.Name] = b
	}
	if len(out["Setup.exe"]) == 0 || len(out["PeDePeAdapter/openomsi.exe"]) == 0 {
		return nil, fmt.Errorf("atualizador incompleto")
	}
	return out, nil
}

func automaticPlayerManifest(payload map[string][]byte) []byte {
	names := make([]string, 0, len(payload))
	for name := range payload {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("# Automatic Bridge update\n")
	for _, name := range names {
		fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256(payload[name]), name)
	}
	return []byte(b.String())
}

func automaticBridgeBinaryManifest(payload map[string][]byte) []byte {
	binaries := map[string][]byte{}
	for name, data := range payload {
		if bridgeUpdateBinaryAllowed(name) {
			binaries[name] = data
		}
	}
	return automaticPlayerManifest(binaries)
}

// Preserve the host's attested binary while sharing the newly installed Setup
// checksum, so old host/player patch combinations remain valid in either order.
func updatedHostManifest(previous []byte, setup []byte) []byte {
	var lines []string
	for _, line := range strings.Split(string(previous), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if len(line) >= 67 && line[64:66] == "  " && line[66:] == "Setup.exe" {
			line = fmt.Sprintf("%x  Setup.exe", sha256.Sum256(setup))
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func installAutomaticBridgeUpdate(dir string, c Config, payload map[string][]byte, checkClosed func(string) error) (state bcsNextTripPreparation, err error) {
	if len(payload) < 2 || len(payload["Setup.exe"]) == 0 || len(payload["PeDePeAdapter/openomsi.exe"]) == 0 {
		return state, fmt.Errorf("atualizador incompleto")
	}
	for name := range payload {
		if !bridgeUpdateBinaryAllowed(name) && !verifyCoopPayloadFile(name, payload[name]) {
			return state, fmt.Errorf("componente inesperado no atualizador")
		}
	}

	// Recovery-first policy: the downloaded updater is already authenticated by
	// the signed release manifest and its SHA-256. The old local installation is
	// allowed to be stale, partially missing or have mismatched executables; the
	// exact components carried by this signed payload are the repair source.
	// A diagnostic integrity check may still be useful, but it must never deny
	// the repair it is meant to perform.
	_, _ = verifyPackageForUpdate(dir, payload)

	files := map[string][]byte{}
	for name, data := range payload {
		files[name] = data
	}
	files["docs/PATCH-PLAYER-SHA256.txt"] = automaticBridgeBinaryManifest(payload)
	// The cumulative installer carries the complete seven-file runtime. Its
	// own checksums are the new runtime inventory, even if an older package
	// lacked a component or its old manifest is missing/damaged. Source files,
	// documentation and user settings are not part of this binary update.
	completeRuntime := true
	for _, name := range []string{"Setup.exe", "HostAgent.exe", "CompanyHost.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"} {
		if len(payload[name]) == 0 {
			completeRuntime = false
		}
	}
	if completeRuntime {
		files["docs/SHA256.txt"] = automaticPlayerManifest(payload)
	}
	if host, e := os.ReadFile(filepath.Join(dir, "docs", "PATCH-HOST-SHA256.txt")); e == nil {
		if completeRuntime {
			files["docs/PATCH-HOST-SHA256.txt"] = automaticBridgeBinaryManifest(payload)
		} else {
			files["docs/PATCH-HOST-SHA256.txt"] = updatedHostManifestForPayload(host, payload)
		}
	} else if !os.IsNotExist(e) {
		return state, e
	}

	previous := map[string][]byte{}
	var order []string
	for name := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		// Recovery can recreate subdirectories but must not follow a linked
		// subdirectory out of the selected installation. The package root may
		// itself be an equivalent Steam/junction path.
		for parent := filepath.Dir(path); parent != filepath.Clean(dir); parent = filepath.Dir(parent) {
			st, e := os.Lstat(parent)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return state, e
			}
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return state, fmt.Errorf("a subpasta de atualização não é um diretório local: %s", parent)
			}
		}
		if st, e := os.Lstat(path); e == nil && !st.Mode().IsRegular() {
			return state, fmt.Errorf("componente não é arquivo regular: %s", name)
		}
		b, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			return state, e
		}
		previous[name] = b
		order = append(order, name)
	}
	sort.Strings(order)
	var written []string
	defer func() {
		if err == nil {
			return
		}
		for i := len(written) - 1; i >= 0; i-- {
			name := written[i]
			path := filepath.Join(dir, filepath.FromSlash(name))
			var restore error
			if previous[name] == nil {
				restore = os.Remove(path)
			} else {
				restore = writeSetupAtomic(path, previous[name])
			}
			if restore != nil {
				err = fmt.Errorf("%w; restauração de %s falhou: %v", err, name, restore)
			}
		}
	}()
	for _, name := range order {
		if err = writeSetupAtomic(filepath.Join(dir, filepath.FromSlash(name)), files[name]); err != nil {
			return state, err
		}
		written = append(written, name)
	}

	// Verify exactly what this signed payload wrote. Do not require the old base
	// manifest to be pristine: missing/stale old files are the reason recovery
	// exists in the first place.
	for name, expected := range payload {
		actual, e := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if e != nil {
			return state, e
		}
		if !bytes.Equal(actual, expected) {
			return state, fmt.Errorf("falha ao confirmar componente atualizado: %s", name)
		}
	}

	if err = activateUpdatedDynamicHost(dir, payload); err != nil {
		return state, err
	}

	// BCS next-trip preparation is best-effort. A missing/moved/new BCS must not
	// roll back a successfully repaired Bridge. Setup can repair/reconfigure the
	// BCS integration after reopening.
	if filepath.IsAbs(c.Root) {
		if prepared, prepErr := prepareBCSNextTrip(c, checkClosed); prepErr == nil {
			state = prepared
		} else {
			_ = os.MkdirAll(appDir(dir), 0755)
			if f, e := os.OpenFile(filepath.Join(appDir(dir), "setup-v1.1.3.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); e == nil {
				fmt.Fprintf(f, "automatic-next-trip-warning result=%v\r\n", prepErr)
				f.Close()
			}
		}
	}
	return state, nil
}

func updatedHostManifestForPayload(previous []byte, payload map[string][]byte) []byte {
	var lines []string
	for _, line := range strings.Split(string(previous), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if len(line) >= 67 && line[64:66] == "  " {
			if b, ok := payload[line[66:]]; ok {
				line = fmt.Sprintf("%x  %s", sha256.Sum256(b), line[66:])
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
