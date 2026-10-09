package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type serverStatus struct {
	Map      string          `json:"map"`
	Version  string          `json:"version"`
	Protocol int             `json:"protocol"`
	Time     string          `json:"time"`
	Players  int             `json:"players"`
	Free     bool            `json:"free_player_vehicles"`
	Vehicles json.RawMessage `json:"vehicles"`
	World    json.RawMessage `json:"world"`
}

func activeWorld(raw json.RawMessage) bool {
	var counts map[string]json.RawMessage
	return json.Unmarshal(raw, &counts) == nil && counts != nil
}
func status(ctx context.Context, c *http.Client, address string) (serverStatus, error) {
	var s serverStatus
	e := requestJSON(ctx, c, "GET", strings.TrimRight(address, "/")+"/status", "", nil, &s)
	return s, e
}
func validStatus(s serverStatus, m string) error {
	mapFile, e := canonicalMap(s.Map)
	if e != nil || mapFile != m {
		return fmt.Errorf("o servidor retornou outro mapa")
	}
	if s.Protocol != 6 || !strings.HasPrefix(s.Version, "0.2.") || !s.Free {
		return fmt.Errorf("servidor sem protocolo 6 e ônibus livres")
	}
	var fleet []string
	var text string
	if json.Unmarshal(s.Vehicles, &fleet) != nil {
		if json.Unmarshal(s.Vehicles, &text) != nil {
			return fmt.Errorf("frota inválida")
		}
		if strings.TrimSpace(text) != "" {
			return fmt.Errorf("servidor restringe ônibus")
		}
	}
	if len(fleet) > 0 {
		return fmt.Errorf("servidor restringe ônibus")
	}
	return nil
}
func checkSession(ctx context.Context, c *http.Client, address, m string) error {
	if !validTunnel(address) {
		return fmt.Errorf("endereço da sessão inválido")
	}
	s, e := status(ctx, c, address)
	if e != nil {
		return fmt.Errorf("o servidor anunciado não respondeu; tente iniciar a viagem novamente")
	}
	if e = validStatus(s, m); e != nil {
		return e
	}
	if !activeWorld(s.World) {
		return fmt.Errorf("o mapa ainda não está pronto")
	}
	return nil
}

var tunnelPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

func validTunnel(s string) bool { return tunnelPattern.FindString(s) == s }

type tunnelLog struct {
	mu      sync.Mutex
	partial string
	address string
}

func (w *tunnelLog) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = os.Stderr.Write(b)
	w.partial += string(b)
	for {
		i := strings.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}
		line := w.partial[:i]
		w.partial = w.partial[i+1:]
		if strings.Contains(line, "Server address for the players:") || strings.Contains(line, "tunnel: the session is reachable at") {
			if a := tunnelPattern.FindString(line); a != "" {
				w.address = a
			}
		}
	}
	if len(w.partial) > 8192 {
		w.partial = ""
	}
	return len(b), nil
}
func (w *tunnelLog) get() string { w.mu.Lock(); defer w.mu.Unlock(); return w.address }
func ports() (int, int, error) {
	t, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, 0, e
	}
	defer t.Close()
	u, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		return 0, 0, e
	}
	defer u.Close()
	return t.Addr().(*net.TCPAddr).Port, u.LocalAddr().(*net.UDPAddr).Port, nil
}
func syncClock(ctx context.Context, c *http.Client, base, secret string, t time.Time) error {
	seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
	req, e := http.NewRequestWithContext(ctx, "POST", base+"/admin", strings.NewReader(fmt.Sprintf("speed 1\nclock %d\n", seconds)))
	if e != nil {
		return e
	}
	req.Header.Set("X-Admin-Password", secret)
	resp, e := c.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 202 {
		return fmt.Errorf("o relógio não confirmou a sincronização")
	}
	return nil
}
func clockAligned(raw string, t time.Time) bool {
	p := strings.Split(raw, ":")
	if len(p) < 2 || len(p) > 3 {
		return false
	}
	h, e := strconv.Atoi(p[0])
	if e != nil || h < 0 || h > 23 {
		return false
	}
	m, e := strconv.Atoi(p[1])
	if e != nil || m < 0 || m > 59 {
		return false
	}
	s := 0.0
	if len(p) == 3 {
		s, e = strconv.ParseFloat(p[2], 64)
		if e != nil || s < 0 || s >= 60 || math.IsNaN(s) {
			return false
		}
	}
	d := math.Abs(float64(h*3600+m*60) + s - float64(t.Hour()*3600+t.Minute()*60+t.Second()))
	return math.Min(d, 86400-d) <= 90
}
func hostAndPlay(ctx context.Context, cfg localConfig, dir, m string, args []string, l lease, c *http.Client) error {
	// Release only after terminating the owned dedicated process and its tunnel.
	var stop func()
	var finished chan error
	serverExited := false
	defer func() {
		if stop != nil {
			stop()
			if finished != nil && !serverExited {
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
				}
			}
		}
		release(c, cfg.Invitation, m, l.Token)
	}()
	work, e := os.MkdirTemp("", "openomsi-mp3-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(work)
	tcp, udp, e := ports()
	if e != nil {
		return e
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", tcp)
	secret := randomID() + randomID()
	civil, e := companyNow(l.Clock, time.Now())
	if e != nil {
		return e
	}
	name := "Empresa multiplayer"
	var settings companySettings
	if requestJSON(ctx, c, "GET", cfg.Invitation+"/v3/settings", "", nil, &settings) == nil {
		name = settings.Name
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, name)
	config := fmt.Sprintf("name = %s\nmotd = OpenOmsi + BBS %s\nmap = %s\ndate = %s\ntime = %s\nreal_time = 0\ntime_speed = 1\nvehicles =\nfree_player_vehicles = 1\ntraffic = 30\ntimetable = 1\npassengers = 1\nport = %d\nweb_port = %d\nmax_players = 16\ntunnel = 1\nradius = 0\nmetar_sync = 0\nshare_positions = 0\nadmin_password = %s\n", name, version, m, civil.Format("2006-01-02"), civil.Format("15:04:05"), udp, tcp, secret)
	conf := filepath.Join(work, "server.cfg")
	if e = os.WriteFile(conf, []byte(config), 0600); e != nil {
		return e
	}
	server := exec.Command(filepath.Join(dir, "server", "openomsi.exe"), "--root", cfg.Root, "--server", conf)
	server.Dir = filepath.Join(dir, "server")
	server.Env = nativeEnvironment(os.Environ(), contentFolder(cfg), true)
	out := &tunnelLog{}
	server.Stdout, server.Stderr = out, out
	prepareCompanyHostProcess(server)
	if e = server.Start(); e != nil {
		return e
	}
	stop, e = ownCompanyHostProcess(server)
	if e != nil {
		_ = server.Process.Kill()
		_ = server.Wait()
		return e
	}
	finished = make(chan error, 1)
	go func() { finished <- server.Wait() }()
	localClient := webClient()
	defer localClient.CloseIdleConnections()
	lastAck := time.Now()
	nextRenew := lastAck
	start := lastAck
	onlineAddress := ""
	ready := false
	var game *exec.Cmd
	var gameDone chan error
	gameEnded := false
	var idleSince time.Time
	var gameErr error
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-finished:
			serverExited = true
			return fmt.Errorf("o servidor do mapa encerrou; a próxima entrada poderá criar uma nova sessão")
		case gameErr = <-gameDone:
			gameEnded = true
			gameDone = nil
		case <-tick.C:
			now := time.Now()
			if now.Sub(lastAck) >= 35*time.Second {
				return fmt.Errorf("a conexão com o diretório foi perdida; o servidor local foi encerrado para evitar dois hosts")
			}
			if !ready && now.Sub(start) > 15*time.Minute {
				return fmt.Errorf("o mapa ou o túnel não ficou pronto em 15 minutos")
			}
			if !now.Before(nextRenew) {
				state := "starting"
				if ready {
					state = "online"
				}
				_, e = renew(ctx, c, cfg.Invitation, m, l.Token, state, onlineAddress)
				nextRenew = time.Now().Add(8 * time.Second)
				if e == nil {
					lastAck = time.Now()
				} else {
					var he httpError
					if errors.As(e, &he) && he.Code == 409 {
						return fmt.Errorf("a sessão mudou de host")
					}
					continue
				}
			}
			s, e := status(ctx, localClient, base)
			if e != nil {
				if ready {
					return fmt.Errorf("o servidor local não respondeu")
				}
				if time.Since(start) > 15*time.Minute {
					return fmt.Errorf("o servidor não abriu em 15 minutos")
				}
				continue
			}
			if e = validStatus(s, m); e != nil {
				return e
			}
			if !activeWorld(s.World) {
				continue
			}
			target, e := companyNow(l.Clock, time.Now())
			if e != nil {
				return e
			}
			if e = syncClock(ctx, localClient, base, secret, target); e != nil {
				return e
			}
			if !ready && clockAligned(s.Time, target) {
				a := out.get()
				if a != "" && checkSession(ctx, c, a, m) == nil {
					_, e = renew(ctx, c, cfg.Invitation, m, l.Token, "online", a)
					if e != nil {
						continue
					}
					lastAck = time.Now()
					ready = true
					onlineAddress = a
					game, e = launchNative(cfg, nativeArguments(args, base, cfg.Player), dir)
					if e != nil {
						return e
					}
					gameDone = make(chan error, 1)
					mirrorCtx, mirrorCancel := context.WithCancel(ctx)
					defer mirrorCancel()
					go mirrorPlugin(mirrorCtx, cfg, dir)
					ch := gameDone
					go func() { ch <- game.Wait() }()
				}
			}
			if ready {
				if a := out.get(); a != "" && a != onlineAddress && checkSession(ctx, c, a, m) == nil {
					if _, e = renew(ctx, c, cfg.Invitation, m, l.Token, "online", a); e == nil {
						onlineAddress = a
						lastAck = time.Now()
					}
				}
			}

			if gameEnded {
				if s.Players == 0 {
					if idleSince.IsZero() {
						idleSince = time.Now()
					}
					if time.Since(idleSince) > 60*time.Second {
						return gameErr
					}
				} else {
					idleSince = time.Time{}
				}
			}
		}
	}
}
