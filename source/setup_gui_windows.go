//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var guiUser = syscall.NewLazyDLL("user32.dll")
var guiGDI = syscall.NewLazyDLL("gdi32.dll")
var guiDialogs = syscall.NewLazyDLL("comdlg32.dll")

const (
	guiWMCommand     = 0x0111
	guiWMComplete    = 0x8001
	guiWMClose       = 0x0010
	guiWMDestroy     = 0x0002
	guiWMDrawItem    = 0x002B
	guiWMColorEdit   = 0x0133
	guiWMColorStatic = 0x0138
	guiWMColorButton = 0x0135
	guiWMSetFont     = 0x0030
	guiWSChild       = 0x40000000
	guiWSVisible     = 0x10000000
	guiWSTabStop     = 0x00010000
	guiBMAutoCheck   = 0x0003
	guiBMSetCheck    = 0x00F1
	guiBMGetCheck    = 0x00F0
	guiIDRoot        = 101
	guiIDOpenOMSI    = 102
	guiIDProfile     = 103
	guiIDSave        = 104
	guiIDDeactivate  = 105
	guiIDStatus      = 106
	guiIDLogs        = 107
	guiIDTutorial    = 108
	guiIDHost        = 109
	guiIDCompany     = 110
	guiIDCancel      = 111
	guiIDMultiplayer = 112
	guiIDLanguage    = 113
)

type guiPoint struct{ X, Y int32 }
type guiRect struct{ Left, Top, Right, Bottom int32 }
type guiMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          guiPoint
	Private        uint32
}
type guiWindowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type guiDrawItem struct {
	Type, ID, ItemID, Action, State uint32
	Window, DC                      uintptr
	Rect                            guiRect
	Data                            uintptr
}
type guiOpenFile struct {
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
type guiBrowseInfo struct {
	Owner, Root        uintptr
	DisplayName, Title *uint16
	Flags              uint32
	Callback, Param    uintptr
	Image              int32
}
type guiResult struct {
	config           Config
	status, openPath string
	err              error
}
type guiTranslation struct {
	window uintptr
	key    string
}
type setupGUI struct {
	dir                                                                          string
	config                                                                       Config
	window, root, executable, profile, player, multiplayer, status, cancelButton uintptr
	language, profileButton, guide, banner, icon, smallIcon                      uintptr
	translations                                                                 []guiTranslation
	controls                                                                     []uintptr
	background, fieldBrush, buttonBrush, accentBrush                             uintptr
	font, smallFont, titleFont                                                   uintptr
	results                                                                      chan guiResult
	cancel                                                                       context.CancelFunc
	busy, smoke, smokeCompleted                                                  bool
}

var activeSetupGUI *setupGUI
var guiWindowProcedure = syscall.NewCallback(setupWindowProcedure)

func guiRGB(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }
func guiText(window uintptr) string {
	n, _, _ := guiUser.NewProc("GetWindowTextLengthW").Call(window)
	if n > 32768 {
		n = 32768
	}
	text := make([]uint16, n+1)
	guiUser.NewProc("GetWindowTextW").Call(window, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	return syscall.UTF16ToString(text)
}
func guiSetText(window uintptr, text string) {
	guiUser.NewProc("SetWindowTextW").Call(window, uintptr(unsafe.Pointer(winPtr(text))))
}
func guiMessageBox(owner uintptr, text string, flags uintptr) uintptr {
	result, _, _ := guiUser.NewProc("MessageBoxW").Call(owner, uintptr(unsafe.Pointer(winPtr(text))), uintptr(unsafe.Pointer(winPtr("OpenOmsi - BBS "+bridgeVersion))), flags)
	return result
}
func guiFont(height int32, weight uintptr) uintptr {
	font, _, _ := guiGDI.NewProc("CreateFontW").Call(uintptr(height), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(winPtr("Segoe UI"))))
	return font
}
func (g *setupGUI) control(class, text string, x, y, width, height int, style uintptr, id int) uintptr {
	instance, _, _ := setupKernel.NewProc("GetModuleHandleW").Call(0)
	handle, _, _ := guiUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(winPtr(class))), uintptr(unsafe.Pointer(winPtr(text))), guiWSChild|guiWSVisible|style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), g.window, uintptr(id), instance, 0)
	guiUser.NewProc("SendMessageW").Call(handle, guiWMSetFont, g.font, 1)
	return handle
}
func (g *setupGUI) label(text string, x, y, width, height int) uintptr {
	return g.control("STATIC", text, x, y, width, height, 0, 0)
}
func (g *setupGUI) button(text string, x, y, width, height, id int) uintptr {
	h := g.control("BUTTON", text, x, y, width, height, guiWSTabStop|0xB, id)
	g.controls = append(g.controls, h)
	return h
}
func (g *setupGUI) edit(text string, x, y, width, id int) uintptr {
	h := g.control("EDIT", text, x, y, width, 34, guiWSTabStop|0x80|0x00800000, id)
	guiUser.NewProc("SendMessageW").Call(h, 0x00C5, 32768, 0)
	g.controls = append(g.controls, h)
	return h
}
func (g *setupGUI) text(key string) string { return setupText(g.config.Language, key) }
func (g *setupGUI) translatedLabel(key string, x, y, width, height int) uintptr {
	h := g.label(g.text(key), x, y, width, height)
	g.translations = append(g.translations, guiTranslation{h, key})
	return h
}
func (g *setupGUI) translatedButton(key string, x, y, width, height, id int) uintptr {
	h := g.button(g.text(key), x, y, width, height, id)
	g.translations = append(g.translations, guiTranslation{h, key})
	return h
}
func (g *setupGUI) multiplayerChecked() bool {
	checked, _, _ := guiUser.NewProc("SendMessageW").Call(g.multiplayer, guiBMGetCheck, 0, 0)
	return checked == 1
}
func (g *setupGUI) updateMultiplayerControls() {
	checked := g.multiplayerChecked()
	enabled := uintptr(0)
	if checked && !g.busy {
		enabled = 1
	}
	for _, h := range []uintptr{g.profile, g.player, g.profileButton} {
		guiUser.NewProc("EnableWindow").Call(h, enabled)
	}
	key := "optional"
	if checked {
		key = "profile_hint"
	}
	guiSetText(g.guide, g.text(key))
}
func (g *setupGUI) applyLanguage() {
	for _, item := range g.translations {
		guiSetText(item.window, g.text(item.key))
	}
	g.updateMultiplayerControls()
	guiSetText(g.status, g.text("ready"))
	guiUser.NewProc("InvalidateRect").Call(g.window, 0, 1)
}
func (g *setupGUI) populate() {
	guiSetText(g.root, g.config.Root)
	guiSetText(g.executable, g.config.OpenOMSI)
	guiSetText(g.profile, setupFormProfileSource(g.config))
	guiSetText(g.player, g.config.PlayerName)
	checked := uintptr(0)
	if g.config.Multiplayer {
		checked = 1
	}
	guiUser.NewProc("SendMessageW").Call(g.multiplayer, guiBMSetCheck, checked, 0)
	guiUser.NewProc("SendMessageW").Call(g.language, 0x014E, uintptr(setupLanguageIndex(g.config.Language)), 0)
	g.applyLanguage()
}
func (g *setupGUI) createControls() {
	// Both the header and Explorer/window icon are embedded in Setup.exe, so
	// branding remains available before configuration or when the ZIP is moved.
	header := g.control("STATIC", "", 34, 16, 320, 90, 0x000E, 0)
	guiUser.NewProc("SendMessageW").Call(header, 0x0172, 0, g.banner)
	title := g.label("v"+bridgeVersion, 380, 22, 260, 30)
	guiUser.NewProc("SendMessageW").Call(title, guiWMSetFont, g.titleFont, 1)
	g.translatedLabel("intro", 382, 61, 260, 46)
	g.translatedLabel("language", 668, 22, 256, 25)
	g.language = g.control("COMBOBOX", "", 668, 51, 256, 160, guiWSTabStop|0x0003|0x00200000, guiIDLanguage)
	g.controls = append(g.controls, g.language)
	for _, name := range setupLanguageNames {
		guiUser.NewProc("SendMessageW").Call(g.language, 0x0143, 0, uintptr(unsafe.Pointer(winPtr(name))))
	}
	g.translatedLabel("games", 34, 118, 860, 26)
	g.translatedLabel("root", 34, 149, 740, 25)
	g.root = g.edit(g.config.Root, 34, 176, 705, 201)
	g.translatedButton("folder", 754, 176, 170, 34, guiIDRoot)
	g.translatedLabel("executable", 34, 220, 740, 25)
	g.executable = g.edit(g.config.OpenOMSI, 34, 247, 705, 202)
	g.translatedButton("file", 754, 247, 170, 34, guiIDOpenOMSI)
	g.translatedLabel("company", 34, 303, 445, 26)
	g.multiplayer = g.control("BUTTON", g.text("enable"), 510, 301, 414, 30, guiWSTabStop|guiBMAutoCheck, guiIDMultiplayer)
	g.translations = append(g.translations, guiTranslation{g.multiplayer, "enable"})
	g.controls = append(g.controls, g.multiplayer)
	syscall.NewLazyDLL("uxtheme.dll").NewProc("SetWindowTheme").Call(g.multiplayer, uintptr(unsafe.Pointer(winPtr(""))), uintptr(unsafe.Pointer(winPtr(""))))
	g.translatedLabel("profile", 34, 334, 740, 25)
	g.profile = g.edit(setupFormProfileSource(g.config), 34, 361, 705, 203)
	g.profileButton = g.translatedButton("file", 754, 361, 170, 34, guiIDProfile)
	g.translatedLabel("player", 34, 403, 880, 25)
	g.player = g.edit(g.config.PlayerName, 34, 430, 420, 204)
	guiUser.NewProc("SendMessageW").Call(g.player, 0x00C5, 32, 0)
	g.guide = g.label("", 34, 474, 890, 28)
	guiUser.NewProc("SendMessageW").Call(g.guide, guiWMSetFont, g.smallFont, 1)
	g.translatedButton("save", 34, 514, 260, 46, guiIDSave)
	g.translatedButton("deactivate", 310, 514, 194, 46, guiIDDeactivate)
	g.translatedButton("status", 520, 514, 194, 46, guiIDStatus)
	g.cancelButton = g.translatedButton("cancel", 730, 514, 194, 46, guiIDCancel)
	guiUser.NewProc("EnableWindow").Call(g.cancelButton, 0)
	g.status = g.label(g.text("ready"), 34, 578, 890, 60)
	guiUser.NewProc("SendMessageW").Call(g.status, guiWMSetFont, g.smallFont, 1)
	g.translatedButton("logs", 34, 650, 194, 36, guiIDLogs)
	g.translatedButton("tutorial", 244, 650, 194, 36, guiIDTutorial)
	g.translatedButton("host", 454, 650, 224, 36, guiIDHost)
	g.translatedButton("admin", 694, 650, 230, 36, guiIDCompany)
	g.populate()
}
func (g *setupGUI) busyState(busy bool) {
	g.busy = busy
	for _, h := range g.controls {
		enabled := uintptr(1)
		if busy {
			enabled = 0
		}
		guiUser.NewProc("EnableWindow").Call(h, enabled)
	}
	enabled := uintptr(0)
	if busy {
		enabled = 1
	}
	guiUser.NewProc("EnableWindow").Call(g.cancelButton, enabled)
	g.updateMultiplayerControls()
}
func (g *setupGUI) work(status string, task func(context.Context) guiResult) {
	if g.busy {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.busyState(true)
	guiSetText(g.status, status)
	go func() {
		result := task(ctx)
		g.results <- result
		guiUser.NewProc("PostMessageW").Call(g.window, guiWMComplete, 0, 0)
	}()
}
func (g *setupGUI) finish() {
	result := <-g.results
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	g.busyState(false)
	if result.err != nil {
		logSetup(g.dir, "graphical-setup", result.err)
		guiSetText(g.status, g.text("failed")+result.err.Error())
		guiMessageBox(g.window, result.err.Error(), 0x10)
		return
	}
	if result.config.Root != "" {
		g.config = result.config
		g.populate()
	}
	guiSetText(g.status, result.status)
	if result.openPath != "" {
		_ = openDocument(result.openPath)
	}
	if g.smoke {
		g.smokeCompleted = true
	}
}
func (g *setupGUI) saveAndActivate() {
	checked, _, _ := guiUser.NewProc("SendMessageW").Call(g.multiplayer, guiBMGetCheck, 0, 0)
	c, err := setupFormConfig(g.config, guiText(g.root), guiText(g.executable), guiText(g.player), checked == 1)
	if err != nil {
		guiMessageBox(g.window, err.Error(), 0x10)
		return
	}
	source := strings.TrimSpace(guiText(g.profile))
	if c.Multiplayer && source == "" {
		guiMessageBox(g.window, g.text("need_profile"), 0x10)
		return
	}
	g.work(g.text("checking_games"), func(ctx context.Context) guiResult {
		if err := setupChangesAllowed(g.config.Language); err != nil {
			return guiResult{err: err}
		}
		if err := validateConfig(c, g.dir); err != nil {
			return guiResult{err: err}
		}
		if _, err := checkOpenOMSICompatibility(c.OpenOMSI, false, c.Multiplayer); err != nil {
			return guiResult{err: fmt.Errorf(g.text("incompatible"), err)}
		}
		if _, err := verifyPackage(g.dir); err != nil {
			return guiResult{err: fmt.Errorf(g.text("package"), err)}
		}
		if c.Multiplayer {
			var err error
			c, _, err = enrollCompany(ctx, g.dir, source, c.PlayerName, c)
			if err != nil {
				return guiResult{err: err}
			}
		}
		if err := ctx.Err(); err != nil {
			return guiResult{err: fmt.Errorf("%s", g.text("cancelled"))}
		}
		if err := saveInstalledConfig(g.dir, c); err != nil {
			return guiResult{err: err}
		}
		var err error
		if isElevated() {
			err = adminAction(g.dir, "activate")
		} else {
			err = elevateSetup("activate", c.Language)
		}
		if err != nil {
			return guiResult{config: c, err: err}
		}
		status := g.text("activated")
		if c.Multiplayer {
			status += g.text("server_required")
		}
		return guiResult{config: c, status: status}
	})
}
func setupChangesAllowed(language ...string) error {
	lang := "pt"
	if len(language) > 0 {
		lang = language[0]
	}
	running, err := gamesRunning()
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("%s", setupText(lang, "close_games"))
	}
	return nil
}

func (g *setupGUI) command(id int) {
	if id == guiIDCancel && g.busy {
		g.cancel()
		guiSetText(g.status, g.text("cancelling"))
		guiUser.NewProc("EnableWindow").Call(g.cancelButton, 0)
		return
	}
	if g.busy {
		return
	}
	switch id {
	case guiIDLanguage:
		index, _, _ := guiUser.NewProc("SendMessageW").Call(g.language, 0x0147, 0, 0)
		if index < uintptr(len(setupLanguages)) {
			g.config.Language = setupLanguages[index]
			g.applyLanguage()
		}
	case guiIDMultiplayer:
		g.updateMultiplayerControls()
	case guiIDRoot:
		if path, err := g.pickFolder(); err != nil {
			guiMessageBox(g.window, err.Error(), 0x10)
		} else if path != "" {
			guiSetText(g.root, path)
		}
	case guiIDOpenOMSI:
		if path, err := g.pickFile(g.text("pick_executable"), g.text("executable_filter")); err != nil {
			guiMessageBox(g.window, err.Error(), 0x10)
		} else if path != "" {
			guiSetText(g.executable, path)
		}
	case guiIDProfile:
		if path, err := g.pickFile(g.text("pick_profile"), g.text("profile_filter")); err != nil {
			guiMessageBox(g.window, err.Error(), 0x10)
		} else if path != "" {
			guiSetText(g.profile, path)
			guiUser.NewProc("SendMessageW").Call(g.multiplayer, guiBMSetCheck, 1, 0)
			g.updateMultiplayerControls()
		}
	case guiIDSave:
		g.saveAndActivate()
	case guiIDDeactivate:
		if guiMessageBox(g.window, g.text("confirm_deactivate"), 0x24) != 6 {
			return
		}
		g.work(g.text("deactivating"), func(ctx context.Context) guiResult {
			if err := setupChangesAllowed(g.config.Language); err != nil {
				return guiResult{err: err}
			}
			var err error
			if isElevated() {
				err = adminAction(g.dir, "deactivate")
			} else {
				err = elevateSetup("deactivate", g.config.Language)
			}
			return guiResult{err: err, status: g.text("deactivated")}
		})
	case guiIDStatus:
		c := readInstalledConfig(g.dir)
		c.Language = g.config.Language
		g.work(g.text("checking_status"), func(ctx context.Context) guiResult {
			status, err := setupRegistryState(g.dir, c, newWindowsRegistry())
			return guiResult{config: c, status: status, err: err}
		})
	case guiIDLogs:
		if guiMessageBox(g.window, g.text("confirm_logs"), 0x24) != 6 {
			return
		}
		c := readInstalledConfig(g.dir)
		c.Language = g.config.Language
		g.work(g.text("collecting"), func(ctx context.Context) guiResult {
			path, err := collectLogs(g.dir, c)
			return guiResult{err: err, status: g.text("zip_created") + path, openPath: filepath.Dir(path)}
		})
	case guiIDTutorial:
		if err := openDocument(filepath.Join(g.dir, "TUTORIAL.html")); err != nil {
			guiMessageBox(g.window, err.Error(), 0x10)
		}
	case guiIDHost, guiIDCompany:
		if err := openDocument(filepath.Join(g.dir, "HostAgent.exe")); err != nil {
			guiMessageBox(g.window, err.Error(), 0x10)
		}
	}
}
func (g *setupGUI) pickFile(title, filter string) (string, error) {
	buffer := make([]uint16, 32768)
	filterText := append(utf16.Encode([]rune(filter)), 0)
	info := guiOpenFile{Owner: g.window, Filter: &filterText[0], FilterIndex: 1, File: &buffer[0], MaxFile: uint32(len(buffer)), Title: winPtr(title), Flags: 0x00001000 | 0x00000800 | 0x00000008 | 0x00080000}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, _ := guiDialogs.NewProc("GetOpenFileNameW").Call(uintptr(unsafe.Pointer(&info)))
	if ok != 0 {
		return syscall.UTF16ToString(buffer), nil
	}
	err, _, _ := guiDialogs.NewProc("CommDlgExtendedError").Call()
	if err != 0 {
		return "", fmt.Errorf(g.text("file_dialog_error"), err)
	}
	return "", nil
}
func (g *setupGUI) pickFolder() (string, error) {
	display := make([]uint16, 260)
	info := guiBrowseInfo{Owner: g.window, DisplayName: &display[0], Title: winPtr(g.text("pick_folder")), Flags: 0x0001 | 0x0040}
	pidl, _, _ := setupShell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", nil
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(pidl)
	buffer := make([]uint16, 32768)
	ok, _, _ := setupShell.NewProc("SHGetPathFromIDListEx").Call(pidl, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
	if ok == 0 {
		return "", fmt.Errorf("%s", g.text("local_folder"))
	}
	return syscall.UTF16ToString(buffer), nil
}
func setupWindowProcedure(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
	g := activeSetupGUI
	if g == nil {
		result, _, _ := guiUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
		return result
	}
	switch message {
	case guiWMCommand:
		if int(wparam&0xFFFF) == guiIDLanguage && (wparam>>16)&0xFFFF != 1 {
			return 0
		}
		g.command(int(wparam & 0xFFFF))
		return 0
	case guiWMComplete:
		g.finish()
		return 0
	case guiWMColorEdit, guiWMColorStatic, guiWMColorButton:
		guiGDI.NewProc("SetTextColor").Call(wparam, guiRGB(236, 243, 251))
		color, brush := guiRGB(14, 25, 41), g.background
		if message == guiWMColorEdit {
			color, brush = guiRGB(32, 48, 67), g.fieldBrush
		}
		guiGDI.NewProc("SetBkColor").Call(wparam, color)
		return brush
	case guiWMDrawItem:
		item := (*guiDrawItem)(unsafe.Pointer(lparam))
		brush, foreground := g.buttonBrush, guiRGB(236, 243, 251)
		if item.ID == guiIDSave {
			brush, foreground = g.accentBrush, guiRGB(16, 26, 41)
		}
		if item.State&0x0004 != 0 {
			foreground = guiRGB(128, 142, 157)
		}
		guiUser.NewProc("FillRect").Call(item.DC, uintptr(unsafe.Pointer(&item.Rect)), brush)
		guiGDI.NewProc("SetTextColor").Call(item.DC, foreground)
		guiGDI.NewProc("SetBkMode").Call(item.DC, 1)
		guiGDI.NewProc("SelectObject").Call(item.DC, g.font)
		rect := item.Rect
		if item.State&0x0001 != 0 {
			rect.Top++
			rect.Left++
		}
		guiUser.NewProc("DrawTextW").Call(item.DC, uintptr(unsafe.Pointer(winPtr(guiText(item.Window)))), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), 0x0001|0x0004|0x0020)
		if item.State&0x0010 != 0 {
			guiUser.NewProc("DrawFocusRect").Call(item.DC, uintptr(unsafe.Pointer(&item.Rect)))
		}
		return 1
	case guiWMClose:
		if g.busy {
			guiMessageBox(window, g.text("busy"), 0x40)
			return 0
		}
		guiUser.NewProc("DestroyWindow").Call(window)
		return 0
	case guiWMDestroy:
		guiUser.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	result, _, _ := guiUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
	return result
}
func runSetupGUI(dir string, smoke bool) error {
	ole := syscall.NewLazyDLL("ole32.dll")
	initialized, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(initialized) >= 0 {
		defer ole.NewProc("CoUninitialize").Call()
	}
	// System-DPI-aware sizing keeps native controls sharp and the full form fits
	// a 900-pixel-high desktop. Virtualized coordinates work on older Windows.
	guiUser.NewProc("SetProcessDPIAware").Call()
	g := &setupGUI{dir: dir, config: readInstalledConfig(dir), results: make(chan guiResult, 1), smoke: smoke}
	activeSetupGUI = g
	defer func() { activeSetupGUI = nil }()
	g.background, _, _ = guiGDI.NewProc("CreateSolidBrush").Call(guiRGB(14, 25, 41))
	g.fieldBrush, _, _ = guiGDI.NewProc("CreateSolidBrush").Call(guiRGB(32, 48, 67))
	g.buttonBrush, _, _ = guiGDI.NewProc("CreateSolidBrush").Call(guiRGB(39, 57, 77))
	g.accentBrush, _, _ = guiGDI.NewProc("CreateSolidBrush").Call(guiRGB(255, 210, 60))
	g.font, g.smallFont, g.titleFont = guiFont(18, 400), guiFont(16, 400), guiFont(24, 700)
	defer func() {
		for _, object := range []uintptr{g.background, g.fieldBrush, g.buttonBrush, g.accentBrush, g.font, g.smallFont, g.titleFont} {
			if object != 0 {
				guiGDI.NewProc("DeleteObject").Call(object)
			}
		}
	}()
	instance, _, _ := setupKernel.NewProc("GetModuleHandleW").Call(0)
	g.icon, _, _ = guiUser.NewProc("LoadImageW").Call(instance, 101, 1, 32, 32, 0)
	g.smallIcon, _, _ = guiUser.NewProc("LoadImageW").Call(instance, 101, 1, 16, 16, 0)
	g.banner, _, _ = guiUser.NewProc("LoadImageW").Call(instance, 201, 0, 0, 0, 0x2000)
	if g.icon == 0 || g.smallIcon == 0 || g.banner == 0 {
		return fmt.Errorf("Setup branding resources are missing; rebuild the complete package")
	}
	defer guiUser.NewProc("DestroyIcon").Call(g.icon)
	defer guiUser.NewProc("DestroyIcon").Call(g.smallIcon)
	defer guiGDI.NewProc("DeleteObject").Call(g.banner)
	cursor, _, _ := guiUser.NewProc("LoadCursorW").Call(0, 32512)
	class := guiWindowClass{Procedure: guiWindowProcedure, Instance: instance, Icon: g.icon, SmallIcon: g.smallIcon, Cursor: cursor, Background: g.background, Name: winPtr("OpenOmsiBBSSetup")}
	class.Size = uint32(unsafe.Sizeof(class))
	registered, _, err := guiUser.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 {
		return fmt.Errorf("Não foi possível criar a interface: %v", err)
	}
	defer guiUser.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class.Name)), instance)
	style := uintptr(0x00C00000 | 0x00080000 | 0x00020000 | 0x02000000)
	rect := guiRect{Right: 960, Bottom: 704}
	guiUser.NewProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&rect)), style, 0, 0)
	width, height := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	screenWidth, _, _ := guiUser.NewProc("GetSystemMetrics").Call(0)
	screenHeight, _, _ := guiUser.NewProc("GetSystemMetrics").Call(1)
	x, y := (int(screenWidth)-width)/2, (int(screenHeight)-height)/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	g.window, _, err = guiUser.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class.Name)), uintptr(unsafe.Pointer(winPtr("OpenOmsi - BBS "+bridgeVersion))), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if g.window == 0 {
		return fmt.Errorf("Não foi possível abrir a janela: %v", err)
	}
	g.createControls()
	if g.language == 0 || g.multiplayer == 0 || g.root == 0 || g.executable == 0 || g.profile == 0 || g.player == 0 || g.status == 0 {
		guiUser.NewProc("DestroyWindow").Call(g.window)
		return fmt.Errorf("Falha ao criar os campos da interface.")
	}
	if smoke {
		guiSetText(g.root, `C:\Smoke\OMSI 2`)
		guiSetText(g.player, "SmokeTest")
		for i, lang := range setupLanguages {
			guiUser.NewProc("SendMessageW").Call(g.language, 0x014E, uintptr(i), 0)
			g.command(guiIDLanguage)
			if g.config.Language != lang || guiText(g.root) != `C:\Smoke\OMSI 2` || guiText(g.player) != "SmokeTest" {
				guiUser.NewProc("DestroyWindow").Call(g.window)
				return fmt.Errorf("GUI language switch lost the form")
			}
			for _, item := range g.translations {
				if guiText(item.window) != setupText(lang, item.key) {
					guiUser.NewProc("DestroyWindow").Call(g.window)
					return fmt.Errorf("GUI translation missing: %s/%s", lang, item.key)
				}
			}
			if previews := os.Getenv("BRIDGE_SETUP_PREVIEWS"); previews != "" {
				guiUser.NewProc("ShowWindow").Call(g.window, 1)
				guiUser.NewProc("RedrawWindow").Call(g.window, 0, 0, 0x0581)
				if err := captureSetupPreview(g.window, filepath.Join(previews, "setup-"+lang+".png")); err != nil {
					guiUser.NewProc("DestroyWindow").Call(g.window)
					return err
				}
			}
		}
		for _, checked := range []uintptr{0, 1, 0} {
			guiUser.NewProc("SendMessageW").Call(g.multiplayer, guiBMSetCheck, checked, 0)
			g.command(guiIDMultiplayer)
			for _, h := range []uintptr{g.profile, g.player, g.profileButton} {
				enabled, _, _ := guiUser.NewProc("IsWindowEnabled").Call(h)
				if (enabled != 0) != (checked != 0) {
					guiUser.NewProc("DestroyWindow").Call(g.window)
					return fmt.Errorf("GUI multiplayer field state incorrect")
				}
			}
		}
		g.results <- guiResult{status: "GUI languages, optional multiplayer and branding smoke completed."}
		guiUser.NewProc("PostMessageW").Call(g.window, guiWMComplete, 0, 0)
		guiUser.NewProc("PostMessageW").Call(g.window, guiWMClose, 0, 0)
	} else {
		guiUser.NewProc("ShowWindow").Call(g.window, 1)
		guiUser.NewProc("UpdateWindow").Call(g.window)
		guiUser.NewProc("SetFocus").Call(g.root)
		status, statusErr := setupRegistryState(dir, g.config, newWindowsRegistry())
		if statusErr == nil {
			guiSetText(g.status, status)
		}
	}
	var message guiMessage
	for {
		result, _, err := guiUser.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return fmt.Errorf("Falha na interface do Windows: %v", err)
		}
		if result == 0 {
			break
		}
		dialog, _, _ := guiUser.NewProc("IsDialogMessageW").Call(g.window, uintptr(unsafe.Pointer(&message)))
		if dialog == 0 {
			guiUser.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			guiUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}
	if smoke && !g.smokeCompleted {
		return fmt.Errorf("A fila de mensagens da interface não foi processada.")
	}
	return nil
}
