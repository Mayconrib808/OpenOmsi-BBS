package main

import (
	"bufio"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	OpenOMSI, Root, BCSLog, Backups, Date, Paint, Language    string
	Traffic, BCSMarkerWaitMS                                  int
	Passengers, AllTiles, AutoStart, BCSCompat, TimetableSync bool
	Multiplayer                                               bool
	CompanyProfile, CompanyID, PlayerName                     string
	CompanyProfileSource string
}

func defaultConfig() Config {
	return Config{
		OpenOMSI:        "",
		Root:            "",
		BCSLog:          `auto`,
		Backups:         `auto`,
		Traffic:         30,
		Passengers:      true,
		Date:            `auto`,
		AllTiles:        false,
		AutoStart:       true,
		Paint:           `auto`,
		BCSCompat:       true,
		TimetableSync:   true,
		BCSMarkerWaitMS: 2000,
	}
}

func readConfig(path string) Config {
	c := defaultConfig()
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(s.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		p := strings.SplitN(line, "=", 2)
		if len(p) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(p[0]))
		v := strings.TrimSpace(p[1])
		switch k {
		case "language":
			c.Language = normalizeLanguage(v)
		case "openomsi":
			if v != "" {
				c.OpenOMSI = v
			}
		case "root":
			if v != "" {
				c.Root = v
			}
		case "bcslog":
			if v != "" {
				c.BCSLog = v
			}
		case "backups":
			if v != "" {
				c.Backups = v
			}
		case "traffic":
			if n, e := strconv.Atoi(v); e == nil && n >= 0 {
				c.Traffic = n
			}
		case "passengers":
			c.Passengers = parseBool(v, true)
		case "date":
			c.Date = strings.TrimSpace(v)
		case "all_tiles", "all":
			c.AllTiles = parseBool(v, false)
		case "autostart":
			c.AutoStart = parseBool(v, true)
		case "paint", "repaint", "skin":
			c.Paint = strings.TrimSpace(v)
		case "bcs_compat", "bcs_compat_process", "compat":
			c.BCSCompat = parseBool(v, true)
		case "timetable_sync", "sync_timetable":
			c.TimetableSync = parseBool(v, true)
		case "bcs_marker_wait_ms", "marker_wait_ms":
			if n, e := strconv.Atoi(v); e == nil && n >= 0 && n <= 10000 {
				c.BCSMarkerWaitMS = n
			}
		case "multiplayer":
			c.Multiplayer = parseBool(v, false)
		case "company_profile":
			c.CompanyProfile = v
		case "company_profile_source":
			c.CompanyProfileSource = v
		case "company_id":
			c.CompanyID = v
		case "player_name":
			c.PlayerName = v
		}
	}
	return c
}

func parseBool(s string, d bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "sim", "ja", "on":
		return true
	case "0", "false", "no", "nao", "não", "nein", "off":
		return false
	default:
		return d
	}
}

func cleanInputPath(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || strings.ContainsAny(s, "\r\n\x00\"") {
		return "", fmt.Errorf("invalid or empty path")
	}
	p, err := filepath.Abs(s)
	if err != nil {
		return "", err
	}
	return filepath.Clean(p), nil
}

func peMachine(path string) (uint16, error) {
	f, err := pe.Open(path)
	if err != nil {
		return 0, fmt.Errorf("invalid Windows executable %s: %w", path, err)
	}
	defer f.Close()
	return f.Machine, nil
}

// The official 0.2.0 Windows archive does not ship the 32-bit plugin host.
// Locate the payload without writing anything to the game installation. The BCS
// plugin needs the launched helper's directory to be the original OMSI root.
func pluginHostSource(c Config, packageDir string) (string, error) {
	host := filepath.Join(appDir(packageDir), "compat", "omsi-plugin-host32.exe")
	if override := os.Getenv("OMSI_PLUGIN_HOST32"); override != "" {
		if !filepath.IsAbs(override) || strings.ContainsAny(override, "\r\n\x00\"") {
			return "", fmt.Errorf("%s", localText(c.Language, "OMSI_PLUGIN_HOST32 deve conter um caminho absoluto válido.", "OMSI_PLUGIN_HOST32 must contain a valid absolute path.", "OMSI_PLUGIN_HOST32 muss einen gültigen absoluten Pfad enthalten."))
		}
		host = filepath.Clean(override)
	}
	machine, err := peMachine(host)
	if err != nil || machine != pe.IMAGE_FILE_MACHINE_I386 {
		return "", fmt.Errorf("%s: %s", localText(c.Language,
			"Auxiliar BCS ausente ou inválido. Extraia o ZIP completo da bridge, incluindo app\\compat\\omsi-plugin-host32.exe. Confira também a variável OMSI_PLUGIN_HOST32, se definida",
			"Missing or invalid BCS helper. Extract the complete bridge ZIP, including app\\compat\\omsi-plugin-host32.exe. Also check OMSI_PLUGIN_HOST32 if set", "Das BBS-Hilfsprogramm fehlt oder ist ungültig. Entpacke die gesamte Bridge-ZIP-Datei einschließlich app\\compat\\omsi-plugin-host32.exe. Prüfe auch OMSI_PLUGIN_HOST32, falls diese Variable gesetzt ist"), host)
	}
	return host, nil
}

// Override only the simulator child's environment. Windows environment keys are
// case-insensitive; remove every spelling so an inherited value cannot win.
func pluginEnvironment(env []string, host string) []string {
	child := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "OMSI_PLUGIN_HOST32") {
			child = append(child, entry)
		}
	}
	return append(child, "OMSI_PLUGIN_HOST32="+host)
}

func validateConfig(c Config, packageDir string) error {
	if !filepath.IsAbs(c.Root) || !filepath.IsAbs(c.OpenOMSI) || strings.ContainsAny(c.Root+c.OpenOMSI, "\r\n\x00\"") {
		return fmt.Errorf("%s", localText(c.Language, "Configure primeiro os caminhos absolutos do OMSI e openOMSI.", "Configure absolute OMSI and openOMSI paths first.", "Stelle zuerst die vollständigen Pfade zu OMSI und openOMSI ein."))
	}
	for _, name := range []string{"maps", "vehicles", "Drivers"} {
		f, err := os.Stat(filepath.Join(c.Root, name))
		if err != nil || !f.IsDir() {
			return fmt.Errorf("%s: %s", localText(c.Language, "A pasta do OMSI não contém", "OMSI folder is missing", "Im OMSI-Ordner fehlt"), name)
		}
	}
	machine, err := peMachine(filepath.Join(c.Root, "Omsi.exe"))
	if err != nil {
		return err
	}
	if machine != pe.IMAGE_FILE_MACHINE_I386 {
		return fmt.Errorf("%s", localText(c.Language, "Escolha a pasta do OMSI 2 original de 32 bits.", "Choose the original 32-bit OMSI 2 folder.", "Wähle den Ordner des originalen OMSI 2 mit 32 Bit."))
	}
	if !strings.EqualFold(filepath.Base(c.OpenOMSI), "openomsi.exe") {
		return fmt.Errorf("%s", localText(c.Language, "Escolha openomsi.exe ou sua pasta.", "Choose openomsi.exe or its folder.", "Wähle openomsi.exe oder den zugehörigen Ordner."))
	}
	machine, err = peMachine(c.OpenOMSI)
	if err != nil {
		return err
	}
	if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("%s", localText(c.Language, "Esta bridge requer openOMSI para Windows x64.", "This bridge requires openOMSI for Windows x64.", "Diese Bridge benötigt openOMSI für Windows x64."))
	}
	if _, err := pluginHostSource(c, packageDir); err != nil {
		return err
	}
	if !c.BCSCompat {
		return fmt.Errorf("%s", localText(c.Language, "A compatibilidade com o BCS deve estar ativada.", "BCS compatibility must be enabled.", "Die BBS-Kompatibilität muss aktiviert sein."))
	}
	return nil
}

func encodeConfig(c Config) []byte {
	return []byte(fmt.Sprintf("# OpenOMSI BCS Bridge v%s - by %s\r\n# Generated by Setup.exe\r\nlanguage=%s\r\nroot=%s\r\nopenomsi=%s\r\nbcslog=%s\r\nbackups=%s\r\ntraffic=%d\r\npassengers=%t\r\ndate=%s\r\npaint=%s\r\nall_tiles=%t\r\nautostart=%t\r\nbcs_compat=%t\r\ntimetable_sync=%t\r\nbcs_marker_wait_ms=%d\r\nmultiplayer=%t\r\ncompany_profile=%s\r\ncompany_id=%s\r\nplayer_name=%s\r\ncompany_profile_source=%s\r\n", bridgeVersion, bridgeAuthor, normalizeLanguage(c.Language), c.Root, c.OpenOMSI, c.BCSLog, c.Backups, c.Traffic, c.Passengers, c.Date, c.Paint, c.AllTiles, c.AutoStart, c.BCSCompat, c.TimetableSync, c.BCSMarkerWaitMS, c.Multiplayer, c.CompanyProfile, c.CompanyID, c.PlayerName, c.CompanyProfileSource))
}
