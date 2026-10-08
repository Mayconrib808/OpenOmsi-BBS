//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var hostUser = syscall.NewLazyDLL("user32.dll")
var hostKernel = syscall.NewLazyDLL("kernel32.dll")
var hostShell = syscall.NewLazyDLL("shell32.dll")
var hostDialogs = syscall.NewLazyDLL("comdlg32.dll")
var hostGDI = syscall.NewLazyDLL("gdi32.dll")

func hostPtr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

type hostPoint struct{ X, Y int32 }
type hostMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          hostPoint
	Private        uint32
}
type hostWindowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type hostOpenFile struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefaultExtension             *uint16
	CustomData, Hook             uintptr
	Template                     *uint16
	Reserved                     uintptr
	ReservedCount, FlagsEx       uint32
}
type hostBrowseInfo struct {
	Owner, Root        uintptr
	DisplayName, Title *uint16
	Flags              uint32
	Callback, Param    uintptr
	Image              int32
}
type hostGUI struct {
	dir, path      string
	c              hostAgentConfig
	window, font   uintptr
	fields         map[int]uintptr
	maps           []hostInstalledMap
	buses, visible []string
	selected       map[string]bool
	current        int
	loading        bool
	statusBusy     bool
	wasRunning     bool
	status         chan string
	labels         []struct {
		h          uintptr
		pt, en, de string
	}
}

var activeHostGUI *hostGUI
var hostWindowProcedure = syscall.NewCallback(hostWindowProc)

const (
	hostRoot         = 301
	hostServer       = 302
	hostDirectory    = 303
	hostKey          = 304
	hostName         = 305
	hostID           = 306
	hostZone         = 307
	hostShift        = 308
	hostLanguage     = 309
	hostStartup      = 310
	hostInstalled    = 311
	hostMap          = 312
	hostFilter       = 313
	hostFleet        = 314
	hostPort         = 315
	hostWeb          = 316
	hostPlayers      = 317
	hostTraffic      = 318
	hostIdle         = 319
	hostEnabled      = 320
	hostPassengers   = 321
	hostTimetable    = 322
	hostStatus       = 323
	hostBrowseRoot   = 331
	hostBrowseServer = 332
	hostScan         = 333
	hostAdd          = 334
	hostImport       = 335
	hostSuggest      = 336
	hostClear        = 337
	hostSave         = 338
	hostStart        = 339
	hostStop         = 340
	hostExport       = 341
	hostLogs         = 342
	hostGuide        = 343
	hostCopyKey      = 344
)

func hostSend(window uintptr, msg uint32, w, l uintptr) uintptr {
	n, _, _ := hostUser.NewProc("SendMessageW").Call(window, uintptr(msg), w, l)
	return n
}
func hostText(window uintptr) string {
	n, _, _ := hostUser.NewProc("GetWindowTextLengthW").Call(window)
	if n > 32768 {
		n = 32768
	}
	b := make([]uint16, n+1)
	hostUser.NewProc("GetWindowTextW").Call(window, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func hostSetText(window uintptr, text string) {
	hostUser.NewProc("SetWindowTextW").Call(window, uintptr(unsafe.Pointer(hostPtr(text))))
}
func hostAgentShowError(err error) {
	hostUser.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(hostPtr(err.Error()))), uintptr(unsafe.Pointer(hostPtr("OpenOmsi + BBS — Servidor"))), 0x10)
}
func (g *hostGUI) t(pt, en, de string) string { return localText(g.c.Language, pt, en, de) }
func (g *hostGUI) control(class, text string, x, y, w, h int, style uintptr, id int) uintptr {
	instance, _, _ := hostKernel.NewProc("GetModuleHandleW").Call(0)
	hwnd, _, _ := hostUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(hostPtr(class))), uintptr(unsafe.Pointer(hostPtr(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), g.window, uintptr(id), instance, 0)
	hostSend(hwnd, 0x0030, g.font, 1)
	if id != 0 {
		g.fields[id] = hwnd
	}
	return hwnd
}
func (g *hostGUI) label(pt, en, de string, x, y, w int) {
	h := g.control("STATIC", g.t(pt, en, de), x, y, w, 23, 0, 0)
	g.labels = append(g.labels, struct {
		h          uintptr
		pt, en, de string
	}{h, pt, en, de})
}
func (g *hostGUI) button(pt, en, de string, x, y, w, id int) {
	h := g.control("BUTTON", g.t(pt, en, de), x, y, w, 30, 0x00010000, id)
	g.labels = append(g.labels, struct {
		h          uintptr
		pt, en, de string
	}{h, pt, en, de})
}
func (g *hostGUI) edit(id int, value string, x, y, w int) {
	g.control("EDIT", value, x, y, w, 28, 0x00800000|0x10000|0x80, id)
}
func (g *hostGUI) check(pt, en, de string, x, y, w, id int) {
	h := g.control("BUTTON", g.t(pt, en, de), x, y, w, 26, 0x10000|3, id)
	g.labels = append(g.labels, struct {
		h          uintptr
		pt, en, de string
	}{h, pt, en, de})
}
func (g *hostGUI) combo(id, x, y, w int) {
	g.control("COMBOBOX", "", x, y, w, 250, 0x10000|0x00200000|3, id)
}
func hostComboAdd(h uintptr, value string) {
	hostSend(h, 0x143, 0, uintptr(unsafe.Pointer(hostPtr(value))))
}
func (g *hostGUI) setCheck(id int, b bool) {
	var n uintptr
	if b {
		n = 1
	}
	hostSend(g.fields[id], 0xf1, n, 0)
}
func (g *hostGUI) checked(id int) bool { return hostSend(g.fields[id], 0xf0, 0, 0) == 1 }
func (g *hostGUI) applyLanguage() {
	for _, l := range g.labels {
		hostSetText(l.h, g.t(l.pt, l.en, l.de))
	}
}
func (g *hostGUI) createControls() {
	g.label("Servidor automático da empresa", "Automatic company server", "Automatischer Firmenserver", 20, 12, 810)
	g.combo(hostLanguage, 930, 9, 150)
	for _, s := range []string{"Português", "English", "Deutsch"} {
		hostComboAdd(g.fields[hostLanguage], s)
	}
	hostSend(g.fields[hostLanguage], 0x14e, uintptr(setupHostLanguageIndex(g.c.Language)), 0)
	g.label("Empresa", "Company", "Firma", 20, 49, 270)
	g.edit(hostName, g.c.Company.CompanyName, 20, 72, 300)
	g.label("Código da empresa", "Company ID", "Firmen-ID", 340, 49, 200)
	g.edit(hostID, g.c.Company.CompanyID, 340, 72, 220)
	g.label("Fuso horário", "Time zone", "Zeitzone", 580, 49, 200)
	g.edit(hostZone, g.c.Company.Clock.TimeZone, 580, 72, 230)
	g.label("Ajuste (minutos)", "Shift (minutes)", "Verschiebung (Minuten)", 830, 49, 250)
	g.edit(hostShift, strconv.Itoa(g.c.Company.Clock.ShiftMinutes), 830, 72, 250)
	g.label("Pasta original do OMSI 2", "Original OMSI 2 folder", "Originaler OMSI-2-Ordner", 20, 107, 400)
	g.edit(hostRoot, g.c.Root, 20, 130, 720)
	g.button("Procurar...", "Browse...", "Durchsuchen...", 750, 130, 155, hostBrowseRoot)
	g.button("Detectar mapas", "Scan maps", "Karten suchen", 915, 130, 165, hostScan)
	g.label("Servidor dedicado (openomsi.exe)", "Dedicated server (openomsi.exe)", "Dedizierter Server (openomsi.exe)", 20, 166, 700)
	g.edit(hostServer, g.c.Server, 20, 189, 885)
	g.button("Procurar...", "Browse...", "Durchsuchen...", 915, 189, 165, hostBrowseServer)
	g.label("URL fixa do diretório online (Worker)", "Stable directory URL (Worker)", "Feste Verzeichnis-URL (Worker)", 20, 224, 650)
	g.edit(hostDirectory, g.c.Company.DirectoryURL, 20, 247, 700)
	g.label("Chave privada do host", "Private host key", "Privater Host-Schlüssel", 740, 224, 340)
	g.edit(hostKey, g.c.HostKey, 740, 247, 175)
	hostSend(g.fields[hostKey], 0xcc, '*', 0)
	g.button("Copiar chave", "Copy key", "Schlüssel kopieren", 925, 247, 155, hostCopyKey)
	g.check("Iniciar agente ao entrar no Windows", "Start agent at Windows login", "Agent bei Windows-Anmeldung starten", 20, 281, 690, hostStartup)
	g.setCheck(hostStartup, g.c.StartWithWindows)
	g.label("Mapas instalados", "Installed maps", "Installierte Karten", 20, 318, 480)
	g.combo(hostInstalled, 20, 342, 395)
	g.button("Adicionar mapa", "Add map", "Karte hinzufügen", 425, 341, 150, hostAdd)
	g.button("Importar JSON atual", "Import existing JSON", "Vorhandenes JSON importieren", 595, 341, 240, hostImport)
	g.label("Mapa configurado", "Configured map", "Konfigurierte Karte", 20, 385, 320)
	g.combo(hostMap, 20, 409, 380)
	g.check("Hospedar este mapa", "Host this map", "Diese Karte hosten", 20, 444, 350, hostEnabled)
	for _, v := range []struct {
		id, y      int
		pt, en, de string
	}{{hostPort, 480, "Porta UDP", "UDP port", "UDP-Port"}, {hostWeb, 520, "Porta HTTP", "HTTP port", "HTTP-Port"}, {hostPlayers, 560, "Máximo de jogadores", "Max. players", "Max. Spieler"}, {hostTraffic, 600, "Tráfego", "Traffic", "Verkehr"}, {hostIdle, 640, "Fechar vazio após (min; 0 = nunca)", "Stop when empty (min; 0 = never)", "Leer stoppen (Min.; 0 = nie)"}} {
		g.label(v.pt, v.en, v.de, 20, v.y, 285)
		g.edit(v.id, "", 310, v.y-3, 90)
	}
	g.check("Passageiros", "Passengers", "Fahrgäste", 20, 680, 170, hostPassengers)
	g.check("Tabela / ônibus AI", "Timetable / AI buses", "Fahrplan / KI-Busse", 195, 680, 215, hostTimetable)
	g.label("Frota deste mapa — seleção em lote (Ctrl/Shift)", "Map fleet — bulk selection (Ctrl/Shift)", "Kartenflotte — Mehrfachauswahl (Strg/Umschalt)", 425, 385, 655)
	g.edit(hostFilter, "", 425, 409, 655)
	g.control("LISTBOX", "", 425, 444, 655, 238, 0x00800000|0x00200000|0x00100000|0x10000|0x800|0x1, hostFleet)
	hostSend(g.fields[hostFleet], 0x194, 2200, 0)
	g.button("Sugerir dirigíveis", "Suggest drivable buses", "Fahrbare Busse vorschlagen", 425, 692, 260, hostSuggest)
	g.button("Limpar seleção", "Clear selection", "Auswahl leeren", 695, 692, 220, hostClear)
	g.button("Salvar", "Save", "Speichern", 20, 735, 110, hostSave)
	g.button("Salvar e iniciar agente", "Save and start agent", "Speichern / Agent starten", 140, 735, 255, hostStart)
	g.button("Parar agente", "Stop agent", "Agent stoppen", 405, 735, 150, hostStop)
	g.button("Exportar perfil dos jogadores", "Export player profile", "Spielerprofil exportieren", 565, 735, 280, hostExport)
	g.button("Guia", "Guide", "Anleitung", 855, 735, 105, hostGuide)
	g.button("Logs", "Logs", "Protokoll", 970, 735, 110, hostLogs)
	g.control("STATIC", g.t("Configure uma vez. O agente fica aguardando os jogadores.", "Configure once. The agent waits for player requests.", "Einmal einrichten. Der Agent wartet auf Spieler."), 20, 779, 1060, 52, 0, hostStatus)
}
func setupHostLanguageIndex(lang string) int {
	switch normalizeLanguage(lang) {
	case "en":
		return 1
	case "de":
		return 2
	}
	return 0
}
func (g *hostGUI) readGeneral() error {
	g.c.Root = strings.Trim(hostText(g.fields[hostRoot]), `"`)
	g.c.Server = strings.Trim(hostText(g.fields[hostServer]), `"`)
	g.c.Company.CompanyName = strings.TrimSpace(hostText(g.fields[hostName]))
	g.c.Company.CompanyID = strings.TrimSpace(hostText(g.fields[hostID]))
	g.c.HostKey = strings.TrimSpace(hostText(g.fields[hostKey]))
	g.c.StartWithWindows = g.checked(hostStartup)
	shift, err := strconv.Atoi(hostText(g.fields[hostShift]))
	if err != nil {
		return fmt.Errorf("invalid clock shift")
	}
	g.c.Company.Clock = &CompanyClock{TimeZone: strings.TrimSpace(hostText(g.fields[hostZone])), ShiftMinutes: shift}
	return hostSetDirectory(&g.c, hostText(g.fields[hostDirectory]))
}
func (g *hostGUI) collectSelection() {
	if g.loading {
		return
	}
	for i, path := range g.visible {
		g.selected[path] = hostSend(g.fields[hostFleet], 0x187, uintptr(i), 0) == 1
	}
}
func (g *hostGUI) filterFleet() {
	g.collectSelection()
	filter := strings.ToLower(hostText(g.fields[hostFilter]))
	g.loading = true
	defer func() { g.loading = false }()
	hostSend(g.fields[hostFleet], 0x184, 0, 0)
	g.visible = nil
	for _, path := range g.buses {
		if !strings.Contains(strings.ToLower(path), filter) {
			continue
		}
		g.visible = append(g.visible, path)
		index := hostSend(g.fields[hostFleet], 0x180, 0, uintptr(unsafe.Pointer(hostPtr(path))))
		if g.selected[path] {
			hostSend(g.fields[hostFleet], 0x185, 1, index)
		}
	}
}
func (g *hostGUI) storeMap() error {
	if g.current < 0 || g.current >= len(g.c.Company.Sessions) {
		return nil
	}
	g.collectSelection()
	s := &g.c.Company.Sessions[g.current]
	m := g.c.Maps[s.ID]
	for _, field := range []struct {
		id    int
		value *int
	}{{hostPort, &m.Port}, {hostWeb, &m.WebPort}, {hostPlayers, &m.MaxPlayers}, {hostTraffic, &m.Traffic}, {hostIdle, &m.IdleMinutes}} {
		n, err := strconv.Atoi(strings.TrimSpace(hostText(g.fields[field.id])))
		if err != nil {
			return fmt.Errorf("preencha os números do mapa antes de trocar ou salvar")
		}
		*field.value = n
	}
	m.Enabled = g.checked(hostEnabled)
	m.Passengers = g.checked(hostPassengers)
	m.Timetable = g.checked(hostTimetable)
	g.c.Maps[s.ID] = m
	s.Fleet = nil
	for path, selected := range g.selected {
		if selected {
			s.Fleet = append(s.Fleet, path)
		}
	}
	sortHostFleet(s.Fleet)
	s.ServerURL = fmt.Sprintf("http://127.0.0.1:%d", m.WebPort)
	return nil
}
func (g *hostGUI) showMap(index int) {
	g.current = index
	g.selected = map[string]bool{}
	if index < 0 || index >= len(g.c.Company.Sessions) {
		return
	}
	s := g.c.Company.Sessions[index]
	m := g.c.Maps[s.ID]
	for _, path := range s.Fleet {
		g.selected[path] = true
	}
	for _, v := range []struct{ id, n int }{{hostPort, m.Port}, {hostWeb, m.WebPort}, {hostPlayers, m.MaxPlayers}, {hostTraffic, m.Traffic}, {hostIdle, m.IdleMinutes}} {
		hostSetText(g.fields[v.id], strconv.Itoa(v.n))
	}
	g.setCheck(hostEnabled, m.Enabled)
	g.setCheck(hostPassengers, m.Passengers)
	g.setCheck(hostTimetable, m.Timetable)
	g.visible = nil
	g.filterFleet()
	hostSend(g.fields[hostMap], 0x14e, uintptr(index), 0)
}
func (g *hostGUI) populateMaps(index int) {
	hostSend(g.fields[hostMap], 0x14b, 0, 0)
	for _, s := range g.c.Company.Sessions {
		hostComboAdd(g.fields[hostMap], s.Name)
	}
	g.showMap(index)
}
func (g *hostGUI) scan() error {
	root := strings.Trim(hostText(g.fields[hostRoot]), `"`)
	maps, err := hostDiscoverMaps(root)
	if err != nil {
		return err
	}
	buses, err := hostDiscoverFleet(root)
	if err != nil {
		return err
	}
	g.collectSelection()
	g.c.Root = root
	g.maps = maps
	g.buses = buses
	hostSend(g.fields[hostInstalled], 0x14b, 0, 0)
	for _, m := range maps {
		hostComboAdd(g.fields[hostInstalled], m.Name)
	}
	if len(maps) > 0 {
		hostSend(g.fields[hostInstalled], 0x14e, 0, 0)
	}
	g.filterFleet()
	return nil
}
func (g *hostGUI) pickFile(title, filter string) (string, error) {
	buffer := make([]uint16, 32768)
	filterBuffer := utf16.Encode([]rune(filter))
	f := hostOpenFile{Owner: g.window, Filter: &filterBuffer[0], File: &buffer[0], MaxFile: uint32(len(buffer)), Title: hostPtr(title), Flags: 0x1000 | 0x80000 | 0x8}
	f.Size = uint32(unsafe.Sizeof(f))
	ok, _, _ := hostDialogs.NewProc("GetOpenFileNameW").Call(uintptr(unsafe.Pointer(&f)))
	if ok == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(buffer), nil
}
func (g *hostGUI) pickFolder() string {
	buffer := make([]uint16, 32768)
	b := hostBrowseInfo{Owner: g.window, DisplayName: &buffer[0], Title: hostPtr(g.t("Selecione a pasta original do OMSI 2", "Select the original OMSI 2 folder", "Originalen OMSI-2-Ordner wählen")), Flags: 0x41}
	id, _, _ := hostShell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&b)))
	if id == 0 {
		return ""
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(id)
	hostShell.NewProc("SHGetPathFromIDListW").Call(id, uintptr(unsafe.Pointer(&buffer[0])))
	return syscall.UTF16ToString(buffer)
}
func (g *hostGUI) save() error {
	if err := g.readGeneral(); err != nil {
		return err
	}
	if err := g.storeMap(); err != nil {
		return err
	}
	var status hostAgentSnapshot
	if hostAgentLocalRequest(context.Background(), g.c, "GET", "/status", &status) == nil {
		return fmt.Errorf("pare o agente antes de alterar a configuração dos mapas")
	}
	if err := saveHostAgentConfig(g.path, g.c); err != nil {
		return err
	}
	if err := hostAgentSetStartup(g.c.StartWithWindows, g.path); err != nil {
		return err
	}
	hostSetText(g.fields[hostDirectory], g.c.Company.DirectoryURL)
	hostSetText(g.fields[hostStatus], g.t("Configuração salva.", "Configuration saved.", "Konfiguration gespeichert."))
	return nil
}
func (g *hostGUI) command(id, notification int) {
	if g.loading {
		return
	}
	var err error
	switch id {
	case hostLanguage:
		if notification != 1 {
			return
		}
		n := hostSend(g.fields[id], 0x147, 0, 0)
		if n < 3 {
			g.c.Language = []string{"pt", "en", "de"}[n]
			g.applyLanguage()
		}
	case hostFilter:
		if notification == 0x300 {
			g.filterFleet()
		}
	case hostMap:
		if notification != 1 {
			return
		}
		n := int(int32(hostSend(g.fields[id], 0x147, 0, 0)))
		err = g.storeMap()
		if err == nil {
			g.showMap(n)
		} else {
			hostSend(g.fields[id], 0x14e, uintptr(g.current), 0)
		}
	case hostBrowseRoot:
		if path := g.pickFolder(); path != "" {
			hostSetText(g.fields[hostRoot], path)
			err = g.scan()
		}
	case hostBrowseServer:
		var path string
		path, err = g.pickFile("openomsi.exe", "openOMSI\x00openomsi.exe\x00\x00")
		if path != "" {
			hostSetText(g.fields[hostServer], path)
		}
	case hostScan:
		err = g.scan()
	case hostAdd:
		err = g.storeMap()
		if err == nil {
			n := int(int32(hostSend(g.fields[hostInstalled], 0x147, 0, 0)))
			if n >= 0 && n < len(g.maps) {
				var index int
				index, err = hostAddMap(&g.c, g.maps[n])
				if err == nil {
					g.populateMaps(index)
				}
			}
		}
	case hostImport:
		var path string
		path, err = g.pickFile(g.t("Perfil atual da empresa", "Existing company profile", "Vorhandenes Firmenprofil"), "JSON\x00*.json\x00\x00")
		if path != "" {
			err = hostImportProfile(&g.c, path)
			if err == nil {
				hostSetText(g.fields[hostName], g.c.Company.CompanyName)
				hostSetText(g.fields[hostID], g.c.Company.CompanyID)
				hostSetText(g.fields[hostDirectory], g.c.Company.DirectoryURL)
				hostSetText(g.fields[hostZone], g.c.Company.Clock.TimeZone)
				hostSetText(g.fields[hostShift], strconv.Itoa(g.c.Company.Clock.ShiftMinutes))
				g.populateMaps(0)
			}
		}
	case hostSuggest:
		g.collectSelection()
		for _, path := range g.visible {
			g.selected[path] = hostSuggestedBus(path)
		}
		g.visible = nil
		g.filterFleet()
	case hostClear:
		g.selected = map[string]bool{}
		g.visible = nil
		g.filterFleet()
	case hostSave:
		err = g.save()
	case hostStart:
		err = g.save()
		if err == nil {
			err = validateHostAgentConfig(g.c, true)
		}
		if err == nil {
			_, err = checkOpenOMSICompatibility(g.c.Server, true, false)
		}
		if err == nil {
			executable, _ := os.Executable()
			cmd := exec.Command(executable, "--background", "--config", g.path)
			cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | 0x00000200, HideWindow: true}
			err = cmd.Start()
			if err == nil {
				go cmd.Wait()
				hostSetText(g.fields[hostStatus], g.t("Agente iniciando...", "Starting agent...", "Agent startet..."))
			}
		}
	case hostStop:
		err = hostAgentLocalRequest(context.Background(), g.c, "POST", "/stop", nil)
		if err == nil {
			hostSetText(g.fields[hostStatus], g.t("Parando os servidores e o agente...", "Stopping servers and agent...", "Server und Agent werden gestoppt..."))
		}
	case hostExport:
		err = g.readGeneral()
		if err == nil {
			err = g.storeMap()
		}
		if err == nil {
			err = validateCompanyProfile(g.c.Company)
		}
		if err == nil && g.c.Company.DirectoryURL == "" {
			err = fmt.Errorf("configure a URL fixa antes de exportar o perfil automático")
		}
		if err == nil {
			path := filepath.Join(companyHostDataDir(g.dir), "Companies", g.c.Company.CompanyID+".players.json")
			var data []byte
			data, err = json.MarshalIndent(g.c.Company, "", "  ")
			if err == nil {
				err = writeCompanyHostFileAtomic(path, append(data, '\n'))
			}
			if err == nil {
				hostSetText(g.fields[hostStatus], g.t("Perfil para enviar uma vez: ", "Send this profile once: ", "Dieses Profil einmal teilen: ")+path)
				hostOpenDocument(filepath.Dir(path))
			}
		}
	case hostGuide:
		hostOpenDocument(filepath.Join(g.dir, "docs", "AUTO_HOST.md"))
	case hostLogs:
		hostOpenDocument(hostAgentLogPath(g.dir))
	case hostCopyKey:
		err = hostClipboardText(g.window, hostText(g.fields[hostKey]))
	}
	if err != nil {
		hostSetText(g.fields[hostStatus], err.Error())
		hostAgentShowError(err)
	}
}
func hostOpenDocument(path string) {
	hostShell.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(hostPtr("open"))), uintptr(unsafe.Pointer(hostPtr(path))), 0, 0, 1)
}
func hostWindowProc(window uintptr, message uint32, w, l uintptr) uintptr {
	g := activeHostGUI
	if g != nil && window == g.window {
		switch message {
		case 0x111:
			g.command(int(w&0xffff), int((w>>16)&0xffff))
			return 0
		case 0x113:
			if !g.statusBusy {
				g.statusBusy = true
				config, language, window := g.c, g.c.Language, g.window
				go func() {
					var s hostAgentSnapshot
					err := hostAgentLocalRequest(context.Background(), config, "GET", "/status", &s)
					text := ""
					if err == nil {
						text = localText(language, "Agente ativo. ", "Agent running. ", "Agent aktiv. ")
						for _, state := range s.Sessions {
							text += state.SessionID + ": " + state.State + " (" + strconv.Itoa(state.Players) + ")  "
						}
						if s.DirectoryError != "" {
							text = s.DirectoryError
						}
					}
					g.status <- text
					hostUser.NewProc("PostMessageW").Call(window, 0x8001, 0, 0)
				}()
			}
			return 0
		case 0x8001:
			g.statusBusy = false
			select {
			case text := <-g.status:
				if text != "" {
					g.wasRunning = true
					hostSetText(g.fields[hostStatus], text)
				} else if g.wasRunning {
					g.wasRunning = false
					hostSetText(g.fields[hostStatus], g.t("Agente parado.", "Agent stopped.", "Agent gestoppt."))
				}
			default:
			}
			return 0
		case 0x10:
			hostUser.NewProc("DestroyWindow").Call(window)
			return 0
		case 0x2:
			hostUser.NewProc("PostQuitMessage").Call(0)
			return 0
		}
	}
	n, _, _ := hostUser.NewProc("DefWindowProcW").Call(window, uintptr(message), w, l)
	return n
}
func runHostAgentGUI(dir, path string, smoke bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	ole.NewProc("CoInitializeEx").Call(0, 2)
	defer ole.NewProc("CoUninitialize").Call()
	c := defaultHostAgentConfig()
	if !smoke {
		if saved, err := loadHostAgentConfig(path); err == nil {
			c = saved
		} else if !os.IsNotExist(err) {
			return err
		} else if previous, ok := loadPreviousCompanyHostOptions(dir); ok {
			c.Root, c.Server = previous.Root, previous.Server
			_ = hostImportProfile(&c, previous.Profile)
		}
	}
	if c.Company.Clock == nil {
		c.Company.Clock = &CompanyClock{TimeZone: "Europe/Berlin"}
	}
	g := &hostGUI{dir: dir, path: path, c: c, fields: map[int]uintptr{}, selected: map[string]bool{}, current: -1, status: make(chan string, 1)}
	activeHostGUI = g
	defer func() { activeHostGUI = nil }()
	font, _, _ := hostGDI.NewProc("CreateFontW").Call(uintptr(18), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(hostPtr("Segoe UI"))))
	g.font = font
	defer hostGDI.NewProc("DeleteObject").Call(font)
	instance, _, _ := hostKernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := hostUser.NewProc("LoadCursorW").Call(0, 32512)
	class := hostWindowClass{Procedure: hostWindowProcedure, Instance: instance, Cursor: cursor, Background: 16, Name: hostPtr("OpenOmsiBBSHostAgent")}
	class.Size = uint32(unsafe.Sizeof(class))
	ok, _, e := hostUser.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if ok == 0 {
		return fmt.Errorf("cannot register host window: %v", e)
	}
	defer hostUser.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class.Name)), instance)
	g.window, _, e = hostUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class.Name)), uintptr(unsafe.Pointer(hostPtr("OpenOmsi + BBS — Host Agent "+bridgeVersion))), 0x00CF0000, 20, 20, 1120, 885, 0, 0, instance, 0)
	if g.window == 0 {
		return fmt.Errorf("cannot create host window: %v", e)
	}
	g.createControls()
	if g.fields[hostRoot] == 0 || g.fields[hostFleet] == 0 || g.fields[hostMap] == 0 || g.fields[hostStart] == 0 {
		hostUser.NewProc("DestroyWindow").Call(g.window)
		return fmt.Errorf("host controls missing")
	}
	if smoke {
		g.buses = []string{"Vehicles/Apache Vip V/Vip 5 Auto.bus", "Vehicles/Apache Vip V/Vip 5 AI.bus", "Vehicles/Solaris/Urbino.bus"}
		g.c.Company.Sessions = []CompanySession{{ID: "sample-map", Name: "Fikcyjny Szczecin", Fleet: []string{g.buses[0]}}}
		g.c.Maps["sample-map"] = defaultHostMapOptions(g.c)
		g.populateMaps(0)
		g.command(hostSuggest, 0)
		if !g.selected[g.buses[0]] || g.selected[g.buses[1]] || !g.selected[g.buses[2]] {
			return fmt.Errorf("bulk fleet control suggested an AI variant")
		}
		if err := g.storeMap(); err != nil {
			return err
		}
		hostSetText(g.fields[hostRoot], `F:\SteamLibrary\steamapps\common\OMSI 2`)
		hostSetText(g.fields[hostServer], `D:\openOMSI-server\openomsi.exe`)
		hostSetText(g.fields[hostStatus], "CI preview — configure once; servers start on player demand.")
		hostUser.NewProc("ShowWindow").Call(g.window, 1)
		hostUser.NewProc("UpdateWindow").Call(g.window)
		for _, lang := range []string{"pt", "en", "de"} {
			g.c.Language = lang
			g.applyLanguage()
			if hostText(g.fields[hostName]) != c.Company.CompanyName {
				return fmt.Errorf("host language switch changed user input")
			}
			if output := os.Getenv("BRIDGE_SETUP_PREVIEWS"); output != "" {
				hostUser.NewProc("UpdateWindow").Call(g.window)
				if err := captureHostPreview(g.window, filepath.Join(output, "host-agent-"+lang+".png")); err != nil {
					return err
				}
			}
		}
		hostUser.NewProc("DestroyWindow").Call(g.window)
		return nil
	}
	if c.Root != "" {
		if err := g.scan(); err != nil {
			hostSetText(g.fields[hostStatus], err.Error())
		}
	}
	g.populateMaps(0)
	hostUser.NewProc("ShowWindow").Call(g.window, 1)
	hostUser.NewProc("UpdateWindow").Call(g.window)
	hostUser.NewProc("SetTimer").Call(g.window, 1, 2000, 0)
	var message hostMessage
	for {
		n, _, err := hostUser.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(n) == -1 {
			return err
		}
		if n == 0 {
			break
		}
		handled, _, _ := hostUser.NewProc("IsDialogMessageW").Call(g.window, uintptr(unsafe.Pointer(&message)))
		if handled == 0 {
			hostUser.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			hostUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}
	return nil
}
func hostClipboardText(window uintptr, text string) error {
	opened, _, _ := hostUser.NewProc("OpenClipboard").Call(window)
	if opened == 0 {
		return fmt.Errorf("clipboard is busy")
	}
	defer hostUser.NewProc("CloseClipboard").Call()
	buffer := utf16.Encode([]rune(text))
	buffer = append(buffer, 0)
	handle, _, _ := hostKernel.NewProc("GlobalAlloc").Call(2, uintptr(len(buffer)*2))
	if handle == 0 {
		return fmt.Errorf("cannot allocate clipboard")
	}
	ptr, _, _ := hostKernel.NewProc("GlobalLock").Call(handle)
	if ptr == 0 {
		hostKernel.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("cannot lock clipboard")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(buffer)), buffer)
	hostKernel.NewProc("GlobalUnlock").Call(handle)
	hostUser.NewProc("EmptyClipboard").Call()
	result, _, _ := hostUser.NewProc("SetClipboardData").Call(13, handle)
	if result == 0 {
		hostKernel.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("cannot set clipboard")
	}
	return nil
}
func hostAgentSetStartup(enabled bool, configPath string) error {
	root := os.Getenv("APPDATA")
	if !filepath.IsAbs(root) {
		return fmt.Errorf("APPDATA is unavailable")
	}
	path := filepath.Join(root, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "OpenOmsi-BBS-Host.cmd")
	if !enabled {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, "%", "%%") + `"` }
	return writeCompanyHostFileAtomic(path, []byte("@echo off\r\nstart \"\" /b "+quote(executable)+" --background --config "+quote(configPath)+"\r\n"))
}
