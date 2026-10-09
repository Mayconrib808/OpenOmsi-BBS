package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const version = "3.0.0-dev.1"

type localConfig struct {
	Version    int    `json:"version"`
	Game       string `json:"game"`
	Root       string `json:"root"`
	Invitation string `json:"invitation"`
	Player     string `json:"player"`
}

func configPath(dir string) string { return filepath.Join(dir, "multiplayer.json") }
func readConfig(dir string) (localConfig, error) {
	var c localConfig
	b, e := os.ReadFile(configPath(dir))
	if e != nil {
		return c, fmt.Errorf("execute Configurar.exe uma vez antes da primeira viagem")
	}
	e = json.Unmarshal(b, &c)
	if e != nil || c.Version != 3 {
		return c, fmt.Errorf("configuração 3.0 inválida")
	}
	c.Invitation, e = invitation(c.Invitation)
	return c, e
}
func validateInstall(c localConfig, dir string) error {
	for _, p := range []string{c.Game, filepath.Join(c.Root, "Omsi.exe"), filepath.Join(dir, "server", "openomsi.exe")} {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("use caminhos completos")
		}
		s, e := os.Stat(p)
		if e != nil || !s.Mode().IsRegular() {
			return fmt.Errorf("arquivo não encontrado: %s", p)
		}
	}
	own, _ := os.Executable()
	if strings.EqualFold(filepath.Clean(own), filepath.Clean(c.Game)) || strings.EqualFold(filepath.Dir(c.Game), dir) {
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
	dir := filepath.Dir(exe)
	if len(os.Args) == 1 || os.Args[1] == "--setup" {
		if e := setup(dir); e != nil {
			showError(e.Error())
		}
		return
	}
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "--help" {
			if e := run(context.Background(), dir, os.Args[1:]); e != nil {
				showError(e.Error())
			}
			return
		}
	}
	f, e := os.OpenFile(filepath.Join(dir, "multiplayer.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		defer f.Close()
		log.SetOutput(f)
		os.Stdout = f
		os.Stderr = f
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if e := run(ctx, dir, os.Args[1:]); e != nil {
		log.Print(e)
		showError(e.Error())
	}
}
func run(ctx context.Context, dir string, args []string) error {
	cfg, e := readConfig(dir)
	if e != nil {
		return e
	}
	if e = validateInstall(cfg, dir); e != nil {
		return e
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
		cfg.Root = r
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
			log.Printf("primeiro jogador: hospedando %s", m)
			return hostAndPlay(ctx, cfg, dir, m, args, l, client)
		}
		if l.State == "online" && l.Address != "" {
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
