package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed setup.html
var setupPage string

type setupView struct {
	Version string
	ErrorDetail                                                                                                string
	BCSStatusKey, UpdateStatusKey                                                                              string
	Token, Lang, Stage, Root, Player, Game, Invite, Name, Zone, Offset, Mode, ClientMode, ServerMode, ServerVersion, Error string
	UpdatePage                                                                                                 string
	Adapter, Message, BCSStatus, UpdateStatus, UpdateVersion                                                   string
	MultiplayerEnabled, ProgramDisabled                                                                        bool
	Detected, AutomaticUpdates, UpdateAvailable, Launching, Preparing                                          bool
}

func (v *setupView) refreshStatuses() {
	if v.BCSStatusKey != "" {
		v.BCSStatus = tr(v.BCSStatusKey)
	}
	if v.UpdateStatusKey != "" {
		v.UpdateStatus = tr(v.UpdateStatusKey)
		if v.UpdateStatusKey == "updatefound" {
			v.UpdateStatus += " " + v.UpdateVersion
		}
	}
}

func newSetupView(c localConfig, dir, token string) setupView {
	c.Server, _ = normalizeServer(c.Server)
	root := c.Root
	if root == "" {
		root = detectRoot(dir)
	}
	player := c.Player
	if player == "" {
		player = os.Getenv("USERNAME")
	}
	game := c.Game
	if game == "" {
		game = detectClient(root, dir)
	}
	clientMode, _ := normalizeClientMode(c, dir)
	prefs := readPreferences(dir)
	return setupView{Version: version, MultiplayerEnabled: prefs.MultiplayerEnabled, ProgramDisabled: prefs.ProgramDisabled, Token: token, Lang: language(), Stage: "setup", Root: root, Player: player, Game: game, Invite: c.Invitation, Zone: "", Offset: "", Mode: "join", ClientMode: clientMode, ServerMode: c.Server.Mode, ServerVersion: c.Server.Version, Detected: root != "" && root == detectRoot(dir), Adapter: adapterDirectory(dir), AutomaticUpdates: readPreferences(dir).AutomaticUpdates, BCSStatus: tr("bcsnotchecked"), UpdateStatus: tr("updatenotchecked"), BCSStatusKey: "bcsnotchecked", UpdateStatusKey: "updatenotchecked"}
}
func setup(dir string) error { return setupWithReason(dir, "") }
func setupWithReason(dir, reason string) error {
	return setupWithOptions(dir, reason, setupOptions{Open: openSetup, CheckClosed: checkBCSClosed, Launching: len(os.Args) > 1 && os.Args[1] != "--setup"})
}

type setupOptions struct {
	Open          func(string) error
	CheckClosed   func() error
	Launching     bool
	Stop          <-chan struct{}
	CheckRelease  func(context.Context, *http.Client, string) (*bridgeRelease, error)
	ResolveClient func(context.Context, string) (string, error)
}

func setupWithOptions(dir, reason string, options setupOptions) error {
	setupCtx, stopSetup := context.WithCancel(context.Background())
	defer stopSetup()
	if e := ensureLayout(dir); e != nil {
		return e
	}
	c, _ := readConfig(dir)
	chosen := loadLanguage(dir)
	token := randomID()
	view := newSetupView(c, dir, token)
	view.Launching = options.Launching
	if reason == "version" {
		view.ServerMode = "recommended"
		view.Error = tr("errorversion")
	}
	if reason == "invite" {
		view.Error = tr("errorinvite")
	}
	if !chosen {
		view.Stage = "language"
	}
	if b, e := os.ReadFile(adminPath(dir)); e == nil {
		var a companySettings
		if json.Unmarshal(b, &a) == nil {
			view.Name = a.Name
			view.Zone = a.Clock.TimeZone
			view.Offset = strconv.FormatFloat(float64(a.Clock.ShiftMinutes)/60, 'f', -1, 64)
		}
	}
	page := template.Must(template.New("setup").Funcs(template.FuncMap{"t": tr}).Parse(setupPage))
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	address := "http://" + listener.Addr().String()
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 30 * time.Second}
	var mu sync.Mutex
	var update *bridgeRelease
	checking := false
	finished := false
	closeSoon := func() { finished = true; time.AfterFunc(300*time.Millisecond, func() { _ = srv.Close() }) }
	checkUpdate := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		client := bridgeUpdateClient()
		defer client.CloseIdleConnections()
		checker := options.CheckRelease
		if checker == nil {
			checker = checkBridgeRelease
		}
		release, err := checker(ctx, client, bridgeUpdateOrigin)
		mu.Lock()
		defer mu.Unlock()
		checking = false
		view.UpdateAvailable = false
		view.UpdatePage = ""
		update = nil
		if err != nil {
			view.ErrorDetail = setupErrorDetail(c, err)
			recordSetupError(dir, "check_update", c, err)
			view.UpdateStatusKey = "updateerror"
			view.refreshStatuses()
			if err == errUpdateNotConfigured {
				view.UpdateStatusKey = "updatepreview"
				view.refreshStatuses()
			}
			return
		}
		if release == nil {
			view.UpdateStatusKey = "updatecurrent"
			view.refreshStatuses()
			return
		}
		update = release
		view.UpdateAvailable = release.Installer.URL != ""
		view.UpdatePage = release.DetailsURL
		view.UpdateVersion = release.Version
		view.UpdateStatusKey = "updatefound"
		view.refreshStatuses()
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/" && r.URL.Path != "/updates" {
			http.NotFound(w, r)
			return
		}
		if r.Host != listener.Addr().String() {
			http.Error(w, tr("error"), 403)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.Method == "GET" {
			if r.URL.Query().Get("token") != token {
				http.Error(w, tr("error"), 403)
				return
			}
			if r.URL.Path == "/updates" {
				view.refreshStatuses()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"message": view.UpdateStatus, "available": view.UpdateAvailable, "page": view.UpdatePage, "checking": checking, "version": view.UpdateVersion, "preparing": view.Preparing, "notice": view.Message, "bcs": view.BCSStatus, "detail": view.ErrorDetail, "error": view.Error})
			} else {
				view.refreshStatuses()
				_ = page.Execute(w, view)
			}
			return
		}
		if r.Method != "POST" {
			http.Error(w, tr("error"), 405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if r.ParseForm() != nil || r.Form.Get("token") != token || r.Header.Get("Origin") != address {
			http.Error(w, tr("error"), 403)
			return
		}
		if r.Form.Get("action") == "language" {
			if e := saveLanguage(dir, r.Form.Get("language")); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
			} else {
				chosen = true
				view.Lang = language()
				view.Stage = "setup"
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		}
		if !chosen {
			http.Error(w, tr("error"), 403)
			return
		}
		view.Error, view.Message, view.ErrorDetail = "", "", ""
		if view.Preparing && (r.Form.Get("action") == "save" || r.Form.Get("action") == "configurebcs" || r.Form.Get("action") == "finish" || r.Form.Get("action") == "installupdate" || r.Form.Get("action") == "disableprogram" || r.Form.Get("action") == "togglemultiplayer" || r.Form.Get("action") == "editconfiguration") {
			view.Message = tr("clientdownload")
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		}
		switch r.Form.Get("action") {
		case "editconfiguration":
			view.Stage = "setup"
			view.Root, view.Game, view.Player, view.Invite = c.Root, c.Game, c.Player, c.Invitation
			view.ClientMode, _ = normalizeClientMode(c, dir)
			view.Mode = "join" // Editing an existing invitation must never create a duplicate room.
			view.ServerMode, view.ServerVersion = c.Server.Mode, c.Server.Version
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "togglemultiplayer":
			enabled := !readPreferences(dir).MultiplayerEnabled
			if e := writeControls(dir, &enabled, nil); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
			} else {
				view.MultiplayerEnabled = enabled
				view.Message = tr("controlsaved")
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "disableprogram":
			if c.Root == "" {
				view.Error = tr("bcsmustsave")
			} else if _, e := deactivateBCS(c.Root, dir, options.CheckClosed); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
			} else {
				disabled := true
				if e := writeControls(dir, nil, &disabled); e != nil {
					view.ErrorDetail = setupErrorDetail(c, e)
					recordSetupError(dir, "setup_action", c, e)
					view.Error = friendlyError(e)
				} else {
					view.ProgramDisabled = true
					view.Message = tr("programoff")
					view.BCSStatusKey = "programoff"
				}
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "preferences":
			automatic := r.Form.Get("automatic_updates") == "on"
			if e := writePreferences(dir, r.Form.Get("language"), &automatic); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
			} else {
				setLanguage(r.Form.Get("language"))
				view.Lang = language()
				view.AutomaticUpdates = automatic
				view.Message = tr("preferencessaved")
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "checkupdate":
			if !checking {
				checking = true
				view.UpdateStatusKey = "updatechecking"
				view.refreshStatuses()
				go checkUpdate()
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "installupdate":
			if update == nil || !view.UpdateAvailable {
				http.Error(w, tr("updateerror"), 400)
				return
			}
			if err := options.CheckClosed(); err != nil {
				view.ErrorDetail = setupErrorDetail(c, err)
				recordSetupError(dir, "setup_action", c, err)
				view.Error = friendlyError(err)
				view.refreshStatuses()
				_ = page.Execute(w, view)
				return
			}
			client := bridgeUpdateClient()
			defer client.CloseIdleConnections()
			path, err := downloadBridgeRelease(r.Context(), client, update.Installer, filepath.Join(dir, "updater"))
			if err == nil {
				err = verifyDownloadedBridgeRelease(path, update.Installer.SHA256)
			}
			if err == nil {
				err = launchUpdater(path, filepath.Dir(dir), os.Getpid())
			}
			if err != nil {
				view.Error = tr("updateerror")
				view.refreshStatuses()
				_ = page.Execute(w, view)
				return
			}
			view.Stage = "updating"
			view.refreshStatuses()
			_ = page.Execute(w, view)
			closeSoon()
			return
		case "configurebcs":
			if c.Root == "" {
				view.Error = tr("bcsmustsave")
			} else if c.Game == "" || c.ClientMode == "latest" {
				if e := options.CheckClosed(); e != nil {
					view.ErrorDetail = setupErrorDetail(c, e)
					recordSetupError(dir, "setup_action", c, e)
					view.Error = friendlyError(e)
					view.ErrorDetail = e.Error()
					view.refreshStatuses()
					_ = page.Execute(w, view)
					return
				}
				path, e := bcsSettingsPath(c.Root)
				if e == nil {
					var b []byte
					b, e = readSmallFile(path, 128<<10)
					if e == nil {
						_, e = updatedBCSSettings(b, c.Root, adapterDirectory(dir))
					}
				}
				if e != nil {
					view.ErrorDetail = setupErrorDetail(c, e)
					recordSetupError(dir, "setup_action", c, e)
					view.Error = friendlyError(e)
					view.ErrorDetail = e.Error()
					view.refreshStatuses()
					_ = page.Execute(w, view)
					return
				}
				view.Preparing = true
				view.Message = tr("clientdownload")
				snapshot := c
				go func() {
					resolver := options.ResolveClient
					if resolver == nil {
						resolver = resolveClient
					}
					resolved, err := prepareConfiguredClient(setupCtx, snapshot, dir, resolver)
					game := resolved.Game
					mu.Lock()
					defer mu.Unlock()
					view.Preparing = false
					if err == nil {
						snapshot.Game, snapshot.ClientMode = game, resolved.ClientMode
						err = saveConfig(dir, snapshot)
					}
					if err == nil {
						c = snapshot
						view.Game = game
						_, err = configureStandaloneBCS(c.Root, dir, options.CheckClosed)
					}
					if err != nil {
						view.ErrorDetail = setupErrorDetail(c, err)
						recordSetupError(dir, "setup_action", c, err)
						view.Error = friendlyError(err)
						view.ErrorDetail = err.Error()
						recordSetupError(dir, "configure_bcs", c, err)
						view.Message = ""
					} else {
						disabled := false
						_ = writeControls(dir, nil, &disabled)
						view.ProgramDisabled = false
						view.BCSStatusKey = "bcsconfigured"
						view.refreshStatuses()
						view.Message = tr("bcsconfigured")
					}
				}()
			} else if _, e := configureStandaloneBCS(c.Root, dir, options.CheckClosed); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
				view.ErrorDetail = e.Error()
			} else {
				disabled := false
				_ = writeControls(dir, nil, &disabled)
				view.ProgramDisabled = false
				view.BCSStatusKey = "bcsconfigured"
				view.refreshStatuses()
				view.Message = tr("bcsconfigured")
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		case "inspectbcs":
			if c.Root == "" {
				view.Error = tr("bcsmustsave")
			} else if ready, e := inspectBCS(c.Root, dir); e != nil {
				view.ErrorDetail = setupErrorDetail(c, e)
				recordSetupError(dir, "setup_action", c, e)
				view.Error = friendlyError(e)
			} else if ready {
				view.BCSStatusKey = "bcsconfigured"
				view.refreshStatuses()
			} else {
				view.BCSStatusKey = "bcsneedsconfiguration"
				view.refreshStatuses()
			}
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		}
		if r.Form.Get("action") == "finish" {
			if view.Stage != "created" && view.Stage != "ready" {
				http.Error(w, tr("error"), 403)
				return
			}
			view.Stage = "done"
			view.refreshStatuses()
			_ = page.Execute(w, view)
			if options.Launching {
				closeSoon()
			} else {
				finished = true // Keep navigation available without a later unsaved-config warning.
			}
			return
		}
		if r.Form.Get("action") != "save" {
			http.Error(w, tr("error"), 400)
			return
		}
		if validLanguage(r.Form.Get("language")) {
			_ = saveLanguage(dir, r.Form.Get("language"))
			view.Lang = language()
		}
		candidate := localConfig{RuntimePreferencesVersion: 1, ClientMode: r.Form.Get("client_mode"), Version: 3, Root: strings.Trim(strings.TrimSpace(r.Form.Get("root")), "\""), Game: strings.Trim(strings.TrimSpace(r.Form.Get("game")), "\""), Player: strings.TrimSpace(r.Form.Get("player")), Invitation: strings.TrimSpace(r.Form.Get("invite")), Server: serverPreference{Mode: r.Form.Get("server_mode"), Version: r.Form.Get("server_version")}}
		view.Root, view.Game, view.Player, view.Invite, view.Mode = candidate.Root, candidate.Game, candidate.Player, candidate.Invitation, r.Form.Get("room_mode")
		view.Name, view.Zone, view.Offset = r.Form.Get("name"), r.Form.Get("zone"), r.Form.Get("offset")
		view.ClientMode = candidate.ClientMode
		if candidate.ClientMode == "latest" {
			candidate.Game = ""
			if c.Game != "" && insideDirectory(filepath.Join(dir, "client"), c.Game) { candidate.Game = c.Game }
			view.Game = candidate.Game
		}
		view.ServerMode, view.ServerVersion = candidate.Server.Mode, candidate.Server.Version
		view.Error = ""
		var err error
		candidate.ClientMode, err = normalizeClientMode(candidate, dir)
		if err == nil && candidate.ClientMode == "manual" && candidate.Game == "" { err = fmt.Errorf("informe o caminho do openomsi.exe para o cliente manual") }
		if err == nil { candidate.Server, err = normalizeServer(candidate.Server) }
		if err == nil {
			err = validateConfiguredInstall(candidate, dir)
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		client := webClient()
		defer client.CloseIdleConnections()
		created := false
		if err == nil && !readPreferences(dir).MultiplayerEnabled {
			// Solo launch does not require the company directory or an invitation.
		} else if err == nil && view.Mode == "create" {
			clock, pe := parseCompanyClockInput(view.Zone, view.Offset)
			s := companySettings{Name: strings.TrimSpace(view.Name), Clock: clock}
			if pe != nil {
				err = fmt.Errorf("horário inválido")
			} else if strings.TrimSpace(s.Name) == "" {
				err = fmt.Errorf("informe um nome")
			} else if err = validateCompanyClock(s.Clock); err == nil {
				candidate.Invitation = defaultDirectory + "/rooms/" + randomID()
				var a companySettings
				err = requestJSON(ctx, client, "POST", candidate.Invitation+"/v3/settings", "", s, &a)
				if err == nil {
					b, _ := json.MarshalIndent(a, "", "  ")
					err = os.WriteFile(adminPath(dir), b, 0600)
					created = err == nil
					view.Invite = candidate.Invitation
				}
			}
		} else if err == nil {
			candidate.Invitation, err = invitation(candidate.Invitation)
			if err == nil {
				var s companySettings
				err = requestJSON(ctx, client, "GET", candidate.Invitation+"/v3/settings", "", nil, &s)
				if err == nil {
					err = validateCompanyClock(s.Clock)
				}
			}
		}
		if err == nil && r.Form.Get("edit") == "on" {
			var a companySettings
			b, re := os.ReadFile(adminPath(dir))
			if re != nil || json.Unmarshal(b, &a) != nil || a.AdminToken == "" || candidate.Invitation != c.Invitation {
				err = fmt.Errorf("chave do criador indisponível")
			} else {
				clock, pe := parseCompanyClockInput(view.Zone, view.Offset)
				s := companySettings{Name: strings.TrimSpace(view.Name), Clock: clock}
				if pe != nil {
					err = fmt.Errorf("horário inválido")
				} else if err = validateCompanyClock(s.Clock); err == nil {
					err = requestJSON(ctx, client, "POST", candidate.Invitation+"/v3/settings", a.AdminToken, s, nil)
					if err == nil {
						a.Name, a.Clock = s.Name, s.Clock
						b, _ := json.MarshalIndent(a, "", "  ")
						err = os.WriteFile(adminPath(dir), b, 0600)
					}
				}
			}
		}
		if err == nil {
			err = saveConfig(dir, candidate)
		}
		if err != nil {
			view.ErrorDetail = setupErrorDetail(c, err)
			recordSetupError(dir, "setup_action", c, err)
			view.Error = friendlyError(err)
			view.Stage = "setup"
			view.refreshStatuses()
			_ = page.Execute(w, view)
			return
		}
		c = candidate
		view.Invite = c.Invitation
		if created {
			view.Stage = "created"
			view.refreshStatuses()
			_ = page.Execute(w, view)
		} else {
			view.Stage = "ready"
			view.refreshStatuses()
			_ = page.Execute(w, view)
		}
	})
	if e = options.Open(address + "/?token=" + token); e != nil {
		return e
	}
	if options.Stop != nil {
		go func() { <-options.Stop; _ = srv.Close() }()
	}
	if view.AutomaticUpdates {
		checking = true
		view.UpdateStatusKey = "updatechecking"
		view.refreshStatuses()
		go checkUpdate()
	}
	timer := time.AfterFunc(30*time.Minute, func() { _ = srv.Close() })
	defer timer.Stop()
	e = srv.Serve(listener)
	mu.Lock()
	ok := finished
	mu.Unlock()
	if e == http.ErrServerClosed && ok {
		return nil
	}
	if e == http.ErrServerClosed {
		return fmt.Errorf("configuração fechada sem salvar")
	}
	return e
}
