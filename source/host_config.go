package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type hostMapOptions struct {
	Enabled     bool `json:"enabled"`
	Port        int  `json:"port"`
	WebPort     int  `json:"web_port"`
	MaxPlayers  int  `json:"max_players"`
	Traffic     int  `json:"traffic"`
	Passengers  bool `json:"passengers"`
	Timetable   bool `json:"timetable"`
	IdleMinutes int  `json:"idle_minutes"`
}
type hostAgentConfig struct {
	Schema           int                       `json:"schema"`
	Root             string                    `json:"root"`
	Server           string                    `json:"server"`
	Language         string                    `json:"language"`
	HostKey          string                    `json:"host_key"`
	RoomID           string                    `json:"room_id"`
	ControlKey       string                    `json:"control_key"`
	ControlPort      int                       `json:"control_port"`
	StartWithWindows bool                      `json:"start_with_windows"`
	Company          CompanyProfile            `json:"company"`
	Maps             map[string]hostMapOptions `json:"maps"`
}
type hostInstalledMap struct{ Name, File string }

func hostRandomCode() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
func hostAgentSettingsPath(packageDir string) string {
	return filepath.Join(companyHostDataDir(packageDir), "HostAgent.local.json")
}
func defaultHostAgentConfig() hostAgentConfig {
	return hostAgentConfig{Schema: 1, Language: "pt", HostKey: hostRandomCode() + hostRandomCode(), RoomID: hostRandomCode(), ControlKey: hostRandomCode(), ControlPort: 27199, Maps: map[string]hostMapOptions{},
		Company: CompanyProfile{SchemaVersion: 1, CompanyID: "transfort-br", CompanyName: "Transfort - BR", OpenOMSIVersion: multiplayerGameVersion, Protocol: multiplayerProtocol, Clock: &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: -480}}}
}
func defaultHostMapOptions(c hostAgentConfig) hostMapOptions {
	port, web := 27015, 27025
	for {
		used := false
		for _, m := range c.Maps {
			if m.Port == port || m.WebPort == web {
				used = true
			}
		}
		if !used {
			break
		}
		port++
		web++
	}
	return hostMapOptions{Enabled: true, Port: port, WebPort: web, MaxPlayers: 16, Traffic: 30, Passengers: true, Timetable: true, IdleMinutes: 15}
}
func loadHostAgentConfig(path string) (hostAgentConfig, error) {
	var c hostAgentConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if len(b) > companyProfileLimit+65536 {
		return c, fmt.Errorf("host configuration is too large")
	}
	err = json.Unmarshal(b, &c)
	if c.Maps == nil {
		c.Maps = map[string]hostMapOptions{}
	}
	return c, err
}
func saveHostAgentConfig(path string, c hostAgentConfig) error {
	if err := validateHostAgentConfig(c, false); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeCompanyHostFileAtomic(path, append(data, '\n'))
}
func validateHostAgentConfig(c hostAgentConfig, running bool) error {
	if c.Schema != 1 || !filepath.IsAbs(c.Root) || !filepath.IsAbs(c.Server) {
		return fmt.Errorf("selecione as pastas do OMSI 2 e do servidor dedicado")
	}
	for _, p := range []string{filepath.Join(c.Root, "Omsi.exe"), c.Server} {
		if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
			return fmt.Errorf("executável não encontrado: %s", p)
		}
	}
	if len(c.HostKey) < 32 || len(c.HostKey) > 128 || strings.ContainsAny(c.HostKey, "\r\n\t ") || len(c.ControlKey) != 32 || len(c.RoomID) != 32 || c.ControlPort < 1024 || c.ControlPort > 65535 {
		return fmt.Errorf("invalid host credentials or control port")
	}
	if err := validateCompanyProfile(c.Company); err != nil {
		return err
	}
	if c.Company.Clock == nil {
		return fmt.Errorf("configure o relógio da empresa")
	}
	if running && c.Company.DirectoryURL == "" {
		return fmt.Errorf("configure a URL fixa do diretório online antes de iniciar o agente; veja docs/AUTO_HOST.md")
	}
	ports, webs := map[int]bool{}, map[int]bool{}
	enabled := 0
	for _, s := range c.Company.Sessions {
		m, ok := c.Maps[s.ID]
		if !ok || s.Date != "company" {
			return fmt.Errorf("mapa sem configuração de hospedagem: %s", s.Name)
		}
		if m.Port < 1024 || m.Port > 65535 || m.WebPort < 1024 || m.WebPort > 65535 || m.MaxPlayers < 1 || m.MaxPlayers > 128 || m.Traffic < 0 || m.Traffic > 1000 || m.IdleMinutes < 0 || m.IdleMinutes > 1440 || m.WebPort == c.ControlPort || ports[m.Port] || webs[m.WebPort] {
			return fmt.Errorf("portas repetidas ou opções inválidas em %s", s.Name)
		}
		ports[m.Port], webs[m.WebPort] = true, true
		if m.Enabled {
			enabled++
			if len(s.Fleet) == 0 {
				return fmt.Errorf("selecione a frota de %s", s.Name)
			}
		}
	}
	if running && enabled == 0 {
		return fmt.Errorf("ative pelo menos um mapa")
	}
	return nil
}
func hostDiscoverMaps(root string) ([]hostInstalledMap, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var base string
	for _, d := range dirs {
		if d.IsDir() && strings.EqualFold(d.Name(), "maps") {
			base = filepath.Join(root, d.Name())
			break
		}
	}
	if base == "" {
		return nil, fmt.Errorf("pasta maps não encontrada")
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var result []hostInstalledMap
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(filepath.Base(base), entry.Name(), "global.cfg"))
		path, e := companyAssetPath(root, rel)
		if e != nil {
			continue
		}
		data, e := os.ReadFile(path)
		if e != nil || len(data) > 4<<20 {
			continue
		}
		name := entry.Name()
		lines := strings.Split(strings.ReplaceAll(companyBBSBackupText(data), "\r\n", "\n"), "\n")
		for i, line := range lines {
			if strings.EqualFold(strings.TrimSpace(line), "[name]") && i+1 < len(lines) {
				n := strings.TrimSpace(lines[i+1])
				if companyText(n, 120) {
					name = n
				}
				break
			}
		}
		result = append(result, hostInstalledMap{name, rel})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}
func sortHostFleet(fleet []string) {
	sort.Slice(fleet, func(i, j int) bool { return companyAssetKey(fleet[i]) < companyAssetKey(fleet[j]) })
}
func hostDiscoverFleet(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var base string
	for _, d := range entries {
		if d.IsDir() && strings.EqualFold(d.Name(), "vehicles") {
			base = filepath.Join(root, d.Name())
			break
		}
	}
	if base == "" {
		return nil, fmt.Errorf("pasta Vehicles não encontrada")
	}
	var buses []string
	err = filepath.WalkDir(base, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".bus") {
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			if validCompanyAsset(rel) && !strings.Contains(rel, ";") {
				buses = append(buses, rel)
			}
		}
		if len(buses) > 10000 {
			return fmt.Errorf("mais de 10000 variantes de ônibus")
		}
		return nil
	})
	sort.Slice(buses, func(i, j int) bool { return companyAssetKey(buses[i]) < companyAssetKey(buses[j]) })
	return buses, err
}
func hostSuggestedBus(path string) bool {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	for _, word := range strings.FieldsFunc(name, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') }) {
		if word == "ai" {
			return false
		}
	}
	return !strings.HasPrefix(name, "ai_") && !strings.HasSuffix(name, "_ai")
}
func hostAddMap(c *hostAgentConfig, m hostInstalledMap) (int, error) {
	for i, s := range c.Company.Sessions {
		if companyAssetKey(s.MapFile) == companyAssetKey(m.File) {
			return i, nil
		}
	}
	if len(c.Company.Sessions) >= 16 {
		return 0, fmt.Errorf("limite de 16 mapas cadastrados")
	}
	path, err := companyAssetPath(c.Root, m.File)
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256(data)
	key := "map-" + hex.EncodeToString(digest[:4]) + "-" + fmt.Sprint(len(c.Company.Sessions)+1)
	opts := defaultHostMapOptions(*c)
	c.Company.Packages = append(c.Company.Packages, CompanyPackage{ID: key, Name: m.Name, Version: "1", Folders: []string{filepath.ToSlash(filepath.Dir(m.File))}, Files: []CompanyFile{{Path: m.File, SHA256: hex.EncodeToString(digest[:])}}})
	c.Company.Sessions = append(c.Company.Sessions, CompanySession{ID: key, Name: m.Name, MapName: m.Name, MapFile: m.File, Date: "company", ServerURL: fmt.Sprintf("http://127.0.0.1:%d", opts.WebPort), RequiredPackages: []string{key}, ClockToleranceSec: 180})
	c.Maps[key] = opts
	return len(c.Company.Sessions) - 1, nil
}
func hostImportProfile(c *hostAgentConfig, path string) error {
	p, err := loadCompanyProfile(context.Background(), path, ".", companyHTTPClient())
	if err != nil {
		return err
	}
	if p.Clock == nil {
		return fmt.Errorf("o perfil precisa de relógio da empresa")
	}
	old := c.Maps
	candidate := *c
	candidate.Maps = map[string]hostMapOptions{}
	for i, s := range p.Sessions {
		fleet, err := companyHostFleet(p, s)
		if err != nil {
			return err
		}
		s.Date = "company"
		s.Fleet = fleet
		p.Sessions[i] = s
		opts, ok := old[s.ID]
		if !ok {
			opts = defaultHostMapOptions(candidate)
		}
		candidate.Maps[s.ID] = opts
		p.Sessions[i].ServerURL = fmt.Sprintf("http://127.0.0.1:%d", opts.WebPort)
	}
	candidate.Company = p
	*c = candidate
	return nil
}
func hostSetDirectory(c *hostAgentConfig, address string) error {
	address = strings.TrimRight(strings.TrimSpace(address), "/")
	if address == "" {
		c.Company.DirectoryURL = ""
		return nil
	}
	if !strings.Contains(address, "/rooms/") {
		address += "/rooms/" + c.RoomID
	}
	if err := validateCompanyDirectoryURL(address); err != nil {
		return err
	}
	c.Company.DirectoryURL = address
	return nil
}
func hostMapConfigText(company string, m hostMapOptions) []byte {
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	return []byte(fmt.Sprintf("# OpenOmsi + BBS — managed map configuration\nname = %s\nport = %d\nweb_port = %d\nmax_players = %d\ntraffic = %d\npassengers = %d\ntimetable = %d\ntunnel = 1\nradius = 0\nmetar_sync = 0\nshare_positions = 0\n", company, m.Port, m.WebPort, m.MaxPlayers, m.Traffic, bit(m.Passengers), bit(m.Timetable)))
}
