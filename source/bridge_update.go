package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
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

// Record the pre-update version before replacing the installed Setup.exe.
// Older versions can be identified by their existing changelog marker or the
// package's local host inventory. No network, registry or user data is required.
func previousBridgeChangelogVersion(dir string) string {
	if marker, err := os.ReadFile(filepath.Join(appDir(dir), "changelog-seen.txt")); err == nil {
		seen := strings.TrimSpace(string(marker))
		switch seen {
		case "2.2.2", "2.2.2-hotfix.1", "2.2.2-hotfix.2", "2.2.2-hotfix.3", "2.2.2.1", "2.2.2.1-hotfix.1", "2.2.2.1-hotfix.2", "2.2.2.2", "2.2.2.2-hotfix.1", "2.2.2.2-test.1", "2.2.2.2-test.2", "2.2.2.3-test.1", "2.2.2.3", "2.2.2.4":
			return seen
		}
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "docs", "HOST-PESSOAL-2.2.0.3.json")); err == nil {
		var m struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(raw, &m) == nil && len(m.Version) > 0 && len(m.Version) < 70 {
			// 2.2.2.1 shipped with the previous launcher and never displayed
			// its changelog. If no acknowledged marker exists, treat that
			// release as unseen so users still receive BOTH its notes and
			// the new Hotfix 1 when they update.
			if strings.TrimSpace(m.Version) == "2.2.2.1" {
				return "2.2.2-hotfix.3"
			}
			return strings.TrimSpace(m.Version)
		}
	}
	return "" // Older installs without a version marker show all known changes.
}

var automaticUpdateTrace func(string, string, error)

func traceAutomaticUpdate(dir, stage string) {
	if automaticUpdateTrace != nil {
		automaticUpdateTrace(dir, stage, nil)
	}
}

func installAutomaticBridgeUpdate(dir string, c Config, payload map[string][]byte, checkClosed func(string) error) (state bcsNextTripPreparation, err error) {
	traceAutomaticUpdate(dir, "install-validate-payload")
	fromVersion := previousBridgeChangelogVersion(dir)
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
	preservedVoiceV2 := map[string]bool{}
	for name, data := range payload {
		// Respect the owner's approved Brazilian voice rebalance. The base
		// 2.2.2.4 signed package still ships its original WAV files; overwriting
		// a verified V2 user voice patch would regress audible speech again.
		// Recognize ONLY the eight exact known V2 SHA-256 digests: arbitrary
		// files never bypass the signed update integrity checks.
		if approvedPassengerVoiceV2File(dir, name) {
			preservedVoiceV2[name] = true
			continue
		}
		files[name] = data
	}
	// In P15, the standard app/server path must use the same authenticated
	// binary as the managed dynamic P15 server. Avoid a second 76 MB copy in
	// the downloaded ZIP: copy only AFTER the signed bundle is verified.
	if native, ok := payload[managedDynamicP15Server]; ok {
		files["app/server/openomsi.exe"] = native
		if steam, ok := payload["app/server-dinamico-p15/steam_api64.dll"]; ok {
			files["app/server/steam_api64.dll"] = steam
		}
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

	// Keep rollback on the target volume, never one complete old runtime in RAM.
	traceAutomaticUpdate(dir, "install-prepare-disk-rollback")
	backupRoot, e := os.MkdirTemp(dir, ".bridge-rollback-")
	if e != nil {
		return state, e
	}
	defer os.RemoveAll(backupRoot)
	previous := map[string]string{}
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
		equal, e := installedUpdateFileEquals(path, files[name])
		if e != nil && !os.IsNotExist(e) {
			return state, e
		}
		if equal {
			continue
		}
		if e == nil {
			backup := filepath.Join(backupRoot, filepath.FromSlash(name))
			if len(files[name]) > 8<<20 {
				traceAutomaticUpdate(dir, "install-backup "+name)
			}
			if e = copyUpdateRollback(path, backup); e != nil {
				return state, e
			}
			previous[name] = backup
		}
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
			if previous[name] == "" {
				restore = os.Remove(path)
			} else {
				restore = restoreUpdateRollback(previous[name], path)
			}
			if restore != nil {
				err = fmt.Errorf("%w; restauração de %s falhou: %v", err, name, restore)
			}
		}
	}()
	for _, name := range order {
		if len(files[name]) > 8<<20 {
			traceAutomaticUpdate(dir, "install-write "+name)
		}
		if err = writeSetupAtomic(filepath.Join(dir, filepath.FromSlash(name)), files[name]); err != nil {
			return state, err
		}
		written = append(written, name)
	}

	// Verify exactly what this signed payload wrote. Do not require the old base
	// manifest to be pristine: missing/stale old files are the reason recovery
	// exists in the first place.
	traceAutomaticUpdate(dir, "install-verify-written-files")
	for name, expected := range payload {
		if preservedVoiceV2[name] {
			if !approvedPassengerVoiceV2File(dir, name) {
				return state, fmt.Errorf("a voz brasileira personalizada foi alterada durante a instalação: %s", name)
			}
			continue
		}
		equal, e := installedUpdateFileEquals(filepath.Join(dir, filepath.FromSlash(name)), expected)
		if e != nil {
			return state, e
		}
		if !equal {
			return state, fmt.Errorf("falha ao confirmar componente atualizado: %s", name)
		}
	}

	if err = activateUpdatedDynamicHost(dir, payload); err != nil {
		return state, err
	}
	// Mark the source version only after all signed runtime files were written
	// and verified. This is an advisory changelog hint, never a trust check.
	if fromVersion != "" {
		if e := os.MkdirAll(appDir(dir), 0755); e == nil {
			_ = os.WriteFile(filepath.Join(appDir(dir), "changelog-from.txt"), []byte(fromVersion+"\n"), 0644)
		}
	}
	if fileExists(configPath(dir)) {
		c = readConfig(configPath(dir))
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

// Stream old and installed files through a fixed-size hash/copy buffer. The new
// signed payload is the only complete runtime retained in memory.
func installedUpdateFileEquals(path string, expected []byte) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() != int64(len(expected)) {
		return false, nil
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return false, err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)) == fmt.Sprintf("%x", sha256.Sum256(expected)), nil
}
func copyUpdateRollback(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func restoreUpdateRollback(source, destination string) error {
	// Reuse the atomic replacement/retry policy without reading a large backup.
	f, err := os.CreateTemp(filepath.Dir(destination), ".bridge-restore-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	in, err := os.Open(source)
	if err != nil {
		f.Close()
		return err
	}
	_, err = io.Copy(f, in)
	in.Close()
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceSetupAtomicFile(name, destination)
}


// The exact eight user-approved V2 WAV files, distributed as the optional
// Correcao_Vozes_PTBR_2.2.2.4_V2_mais_alta.zip. Keep them if present in an
// installed Bridge. This never trusts an arbitrary replacement or network
// download: a signed upgrade can still repair any other voice file.
var approvedPassengerVoiceV2Digests = map[string]string{
	"ExitThanks_1.wav": "55d24033269764c01f34f5e584b737182c041dd37f3eb1e0bfc3c008e7c184e8",
	"ExitThanks_2.wav": "2481870d31818cfc47ed64be63e40b7085199d7b296db73348d47acd683728fb",
	"ExitThanks_3.wav": "7f5c625b5c7ef05c484e382e072d536dde59846be6b8d5b5366bfbbef3d4199b",
	"MissedStop_1.wav": "814a84c321db0aea89ac85c63eda68c808464941c69e7df37d33e2a869066890",
	"MissedStop_2.wav": "69b509b818714030e34b7f3cb9083d1b459cd19ca37961c154302700f83206b4",
	"OpenDoor_1.wav": "287ed8845e1d159e8659954bfcb5ec7c9e5488f2dcf864319d8efe266892628c",
	"OpenDoor_2.wav": "18e1e2afe67216a7859f69350c33f2d30db29431f21dffa180a1bd5c55eb4372",
	"OpenDoor_3.wav": "48aaf1808e202482cb8372800be8829235da7423aab82f9dd5c085d3101fb236",
}

// Recognize only the exact eight independently hashed, user-approved WAVs.
func approvedPassengerVoiceV2Digest(name, actualSHA256 string) bool {
	const prefix = "OpenOmsi Coop 0.3.41/programa/bcs-assets/PassengerVoices/pt-BR/"
	if !strings.HasPrefix(name, prefix) { return false }
	part := strings.TrimPrefix(name, prefix)
	if strings.ContainsAny(part, "/\\") { return false }
	expected, ok := approvedPassengerVoiceV2Digests[part]
	return ok && expected == actualSHA256
}

func approvedPassengerVoiceV2File(dir, name string) bool {
	path := filepath.Join(dir, filepath.FromSlash(name))
	file, err := os.Open(path)
	if err != nil { return false }
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > (512<<10) { return false }
	fileHash := sha256.New()
	if _, err := io.Copy(fileHash, file); err != nil { return false }
	return approvedPassengerVoiceV2Digest(name, fmt.Sprintf("%x", fileHash.Sum(nil)))
}

