package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const version = "3.0.2-dev.13"

// Inherited BCS stdout is optional and can be invalid for GUI launches.
// Scheduled game output is also written directly to the game.log that BCS reads.
var bcsLaunchOutput io.Writer

const productName = "BBS+OpenOmsi Multiplayer"

type localConfig struct {
	RuntimePreferencesVersion int              `json:"runtime_preferences_version,omitempty"`
	ClientMode                string           `json:"client_mode,omitempty"`
	Server                    serverPreference `json:"server"`
	Version                   int              `json:"version"`
	Game                      string           `json:"game"`
	Root                      string           `json:"root"`
	Invitation                string           `json:"invitation"`
	Player                    string           `json:"player"`
}

func configPath(dir string) string { return filepath.Join(configDirectory(dir), "multiplayer.json") }
func readConfig(dir string) (localConfig, error) {
	var c localConfig
	b, e := os.ReadFile(configPath(dir))
	if e != nil {
		return c, fmt.Errorf("execute Setup.exe uma vez antes da primeira viagem")
	}
	e = json.Unmarshal(b, &c)
	if e != nil || c.Version != 3 {
		return c, fmt.Errorf("configuração 3.0 inválida")
	}
	if c.RuntimePreferencesVersion == 0 {
		if c.Server.Mode == "" || c.Server.Mode == "recommended" {
			c.Server.Mode = "latest"
		}
		c.RuntimePreferencesVersion = 1
	}
	c.ClientMode, e = normalizeClientMode(c, dir)
	if e != nil {
		return c, e
	}
	c.Server, e = normalizeServer(c.Server)
	if e != nil {
		return c, e
	}
	if readPreferences(dir).MultiplayerEnabled {
		c.Invitation, e = invitation(c.Invitation)
	}
	return c, e
}
func validateInstall(c localConfig, dir string) error {
	if _, e := normalizeServer(c.Server); e != nil {
		return e
	}

	paths := []string{filepath.Join(c.Root, "Omsi.exe"), filepath.Join(dir, "server", "openomsi.exe")}
	if c.Game != "" {
		paths = append(paths, c.Game)
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("use caminhos completos")
		}
		s, e := os.Stat(p)
		if e != nil || !s.Mode().IsRegular() {
			return fmt.Errorf("arquivo não encontrado: %s", p)
		}
	}
	own, _ := os.Executable()
	if c.Game != "" && (strings.EqualFold(filepath.Clean(own), filepath.Clean(c.Game)) || strings.EqualFold(filepath.Dir(c.Game), filepath.Dir(dir))) {
		return fmt.Errorf("selecione o openOMSI real, em outra pasta")
	}

	if strings.ContainsAny(c.Player, "\r\n") || len(c.Player) > 80 || strings.TrimSpace(c.Player) == "" {
		return fmt.Errorf("informe seu nome")
	}
	return nil
}
func main() {
	exe, e := os.Executable()
	if e != nil {
		return
	}
	launcher := filepath.Dir(exe)
	dir := appDirectory(launcher)
	if e = ensureLayout(dir); e != nil {
		showError(tr("errorfiles"))
		return
	}
	migrateConfig(launcher, dir)
	loadLanguage(dir)
	if len(os.Args) == 3 && os.Args[1] == "--host-session" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		_ = runHostAgent(ctx, dir, os.Args[2])
		return
	}
	if len(os.Args) == 1 || os.Args[1] == "--setup" || strings.EqualFold(filepath.Base(exe), "Setup.exe") {
		if e = setup(dir); e != nil {
			showError(friendlyError(e))
		}
		return
	}
	f, e := os.OpenFile(filepath.Join(dir, "logs", "multiplayer.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		defer f.Close()
		log.SetOutput(f)
		probe := false
		for _, a := range os.Args[1:] {
			if a == "--version" || a == "--help" {
				probe = true
			}
		}
		if !probe {
			bcsLaunchOutput = os.Stdout
			os.Stdout, os.Stderr = f, f
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if _, e = readConfig(dir); e != nil || !loadLanguage(dir) {
		if e = setup(dir); e != nil {
			log.Print(e)
			showError(friendlyError(e))
			return
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		e = run(ctx, dir, os.Args[1:])
		if e == nil {
			return
		}
		log.Print(e)
		if attempt == 0 && needsSetup(e) {
			reason := "invite"
			if strings.Contains(e.Error(), "ônibus livres") {
				reason = "version"
			}
			if e = setupWithReason(dir, reason); e == nil {
				continue
			}
		}
		showError(friendlyError(e))
		return
	}
}
func needsSetup(e error) bool {
	s := strings.ToLower(e.Error())
	var he httpError
	return (errors.As(e, &he) && he.Code == 404) || strings.Contains(s, "ônibus livres") || strings.Contains(s, "convite")
}

func run(ctx context.Context, dir string, args []string) error {
	cfg, e := readConfig(dir)
	if e != nil {
		return e
	}
	if e = validateConfiguredInstall(cfg, dir); e != nil {
		return e
	}
	if readPreferences(dir).ProgramDisabled {
		return fmt.Errorf("program disabled; open Setup.exe to reactivate")
	}
	probe := false
	for _, a := range args {
		if a == "--help" || a == "--version" {
			probe = true
		}
	}
	if !probe || cfg.Game == "" {
		previous := cfg.Game
		cfg, e = prepareConfiguredClient(ctx, cfg, dir, resolveClient)
		if e != nil {
			return e
		}
		if previous != cfg.Game {
			if e = saveConfig(dir, cfg); e != nil {
				return e
			}
	}
	}
	for _, a := range args {
		if a == "--help" || a == "--version" {
			cmd, e := launchNative(cfg, args, dir)
			if e != nil {
				return e
			}
			return cmd.Wait()
		}
	}
	if r := option(args, "--root"); r != "" {
		if !samePath(r, cfg.Root) {
			return fmt.Errorf("a pasta enviada pelo BCS difere da pasta configurada")
		}
		cfg.Root = r
	}
	args, e = prepareBCSLaunch(args, cfg.Root, adapterDirectory(dir))
	if e != nil {
		return e
	}
	if !readPreferences(dir).MultiplayerEnabled {
		cmd, e := launchNative(cfg, withoutLANArguments(args), dir)
		if e != nil {
			return e
		}
		return cmd.Wait()
	}
	m, e := tripMap(args, cfg.Root)
	if e != nil {
		return e
	}
	client := webClient()
	defer client.CloseIdleConnections()
	owner := randomID()
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		l, e := claim(ctx, client, cfg.Invitation, m, owner)
		if e != nil {
			return fmt.Errorf("não foi possível entrar no multiplayer: %w. O serviço precisa estar atualizado para 3.0", e)
		}
		if e = validateCompanyClock(l.Clock); e != nil {
			return e
		}
		if l.Role == "host" {
			log.Printf("%s %s", tr("host"), m)
			address, e := startHostAgent(ctx, cfg, dir, m, l)
			if e != nil {
				return e
			}
			mirrorCtx, mirrorCancel := context.WithCancel(ctx)
			defer mirrorCancel()
			go mirrorPlugin(mirrorCtx, cfg, dir)
			game, e := launchNative(cfg, nativeArguments(args, address, cfg.Player), dir)
			if e != nil {
				return e
			}
			return game.Wait()
		}
		if l.State == "online" && l.Address != "" {
			log.Print(tr("guest"))
			if e = checkSession(ctx, client, l.Address, m); e != nil {
				return e
			}
			mirrorCtx, mirrorCancel := context.WithCancel(ctx)
			defer mirrorCancel()
			go mirrorPlugin(mirrorCtx, cfg, dir)
			cmd, e := launchNative(cfg, nativeArguments(args, l.Address, cfg.Player), dir)
			if e != nil {
				return e
			}
			return cmd.Wait()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("o mapa não abriu em 15 minutos; confira multiplayer.log")
}
