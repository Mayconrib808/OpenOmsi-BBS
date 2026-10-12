package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type hostRequest struct {
	Key    string      `json:"key"`
	Config localConfig `json:"config"`
	Map    string      `json:"map"`
	Lease  lease       `json:"lease"`
}
type hostAnnouncement struct{ Key, Map, State, Address, Error string }

func hostPaths(dir, key string) (request, ready string) {
	folder := filepath.Join(dir, "logs", "host-sessions")
	return filepath.Join(folder, "host-"+key+".json"), filepath.Join(folder, "ready-"+key+".json")
}
func localHostAddress(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, port, e := net.SplitHostPort(u.Host)
	if e != nil || host != "127.0.0.1" {
		return false
	}
	n, e := strconv.Atoi(port)
	return e == nil && n > 0 && n <= 65535
}

func startHostAgent(ctx context.Context, cfg localConfig, dir, m string, l lease) (string, error) {
	key := randomID()
	requestPath, readyPath := hostPaths(dir, key)
	if e := os.MkdirAll(filepath.Dir(requestPath), 0700); e != nil {
		return "", e
	}
	b, _ := json.Marshal(hostRequest{Key: key, Config: cfg, Map: m, Lease: l})
	if e := writeAtomic(requestPath, b, 0600); e != nil {
		return "", e
	}
	exe, e := os.Executable()
	if e != nil {
		return "", e
	}
	cmd := exec.Command(exe, "--host-session", requestPath)
	detachHostAgent(cmd)
	if e = cmd.Start(); e != nil {
		_ = os.Remove(requestPath)
		release(webClient(), cfg.Invitation, m, l.Token)
		return "", e
	}
	if e = cmd.Process.Release(); e != nil {
		return "", e
	}
	deadline := time.NewTimer(15 * time.Minute)
	defer deadline.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("o servidor não ficou pronto em 15 minutos")
		case <-tick.C:
			b, e := readSmallFile(readyPath, 16<<10)
			if e != nil {
				continue
			}
			var a hostAnnouncement
			if json.Unmarshal(b, &a) != nil || a.Key != key || a.Map != m {
				continue
			}
			if a.State == "failed" {
				return "", fmt.Errorf("servidor do mapa: %s", a.Error)
			}
			if a.State == "online" && localHostAddress(a.Address) {
				return a.Address, nil
			}
		}
	}
}

// The independent process owns the server and tunnel. The BCS-launched adapter
// waits only for its own player's game and returns its real saved data on exit.
func runHostAgent(ctx context.Context, dir, path string) (result error) {
	if !insideDirectory(filepath.Join(dir, "logs", "host-sessions"), path) {
		return fmt.Errorf("arquivo de sessão inválido")
	}
	b, e := readSmallFile(path, 128<<10)
	if e != nil {
		return e
	}
	var request hostRequest
	if json.Unmarshal(b, &request) != nil || len(request.Key) != 32 || strings.ContainsAny(request.Key, "/\\.\r\n\x00") {
		return fmt.Errorf("sessão de host inválida")
	}
	expected, ready := hostPaths(dir, request.Key)
	if !samePath(path, expected) || request.Lease.Role != "host" || request.Lease.Token == "" {
		return fmt.Errorf("sessão de host inválida")
	}
	defer os.Remove(path)
	defer func() {
		if result != nil {
			b, _ := json.Marshal(hostAnnouncement{Key: request.Key, Map: request.Map, State: "failed", Error: result.Error()})
			_ = writeAtomic(ready, b, 0600)
		}
	}()
	m, e := canonicalMap(request.Map)
	if e != nil || m != request.Map {
		return fmt.Errorf("mapa de host inválido")
	}
	if e = validateInstall(request.Config, dir); e != nil {
		return e
	}
	if _, e = invitation(request.Config.Invitation); e != nil {
		return e
	}
	f, e := os.OpenFile(filepath.Join(dir, "logs", "host-"+mapID(m)+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		defer f.Close()
		log.SetOutput(f)
	}
	announce := func(state, address, message string) error {
		b, _ := json.Marshal(hostAnnouncement{Key: request.Key, Map: m, State: state, Address: address, Error: message})
		return writeAtomic(ready, b, 0600)
	}
	client := webClient()
	defer client.CloseIdleConnections()
	e = superviseHost(ctx, request.Config, dir, m, request.Lease, client, func(address string) error { return announce("online", address, "") })
	if e != nil {
		log.Print(e)
		_ = announce("failed", "", e.Error())
	} else {
		_ = announce("closed", "", "")
	}
	return e
}
