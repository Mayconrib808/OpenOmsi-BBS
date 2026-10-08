package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type hostAgentMap struct {
	state      companyDirectoryState
	cancel     context.CancelFunc
	lastDemand int64
	idleSince  time.Time
	retryAfter time.Time
}
type hostAgent struct {
	mu             sync.Mutex
	config         hostAgentConfig
	maps           map[string]*hostAgentMap
	dirty          bool
	directoryError string
	output         io.Writer
	wg             sync.WaitGroup
	runServer      func(context.Context, companyHostOptions, io.Writer) error
}
type hostAgentSnapshot struct {
	Running        bool                    `json:"running"`
	DirectoryError string                  `json:"directory_error,omitempty"`
	Sessions       []companyDirectoryState `json:"sessions"`
}

func (a *hostAgent) snapshot() hostAgentSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := hostAgentSnapshot{Running: true, DirectoryError: a.directoryError}
	for _, session := range a.config.Company.Sessions {
		s.Sessions = append(s.Sessions, a.maps[session.ID].state)
	}
	return s
}
func (a *hostAgent) playerProfile() CompanyProfile {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.config.Company
	p.Sessions = append([]CompanySession(nil), p.Sessions...)
	for i := range p.Sessions {
		state := a.maps[p.Sessions[i].ID].state
		if state.Address != "" {
			p.Sessions[i].ServerURL = state.Address
		}
	}
	return p
}
func (a *hostAgent) demand(ctx context.Context, id string, requested int64, dir string, weather ...string) {
	a.mu.Lock()
	m := a.maps[id]
	if m == nil || !a.config.Maps[id].Enabled || requested <= m.lastDemand {
		a.mu.Unlock()
		return
	}
	m.lastDemand = requested
	if m.cancel != nil {
		m.idleSince = time.Now()
		a.mu.Unlock()
		return
	}
	if time.Now().Before(m.retryAfter) {
		a.mu.Unlock()
		return
	}
	childCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.state = companyDirectoryState{SessionID: id, State: "starting"}
	m.idleSince = time.Time{}
	a.dirty = true
	initialWeather := ""
	if len(weather) > 0 && validBridgeWeather(weather[0]) {
		initialWeather = weather[0]
	}
	a.mu.Unlock()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		mapDir := filepath.Join(companyHostDataDir(dir), "HostedMaps", a.config.Company.CompanyID, id)
		profilePath := filepath.Join(mapDir, "company.host.json")
		cfgPath := filepath.Join(mapDir, "server.cfg")
		data, err := json.Marshal(a.config.Company)
		if err == nil {
			err = writeCompanyHostFileAtomic(profilePath, data)
		}
		if err == nil {
			text := hostMapConfigText(a.config.Company.CompanyName, a.config.Maps[id])
			if initialWeather != "" {
				text = append(text, []byte("weather = "+initialWeather+"\n")...)
			}
			err = writeCompanyHostFileAtomic(cfgPath, text)
		}
		options := companyHostOptions{Root: a.config.Root, Server: a.config.Server, Profile: profilePath, Session: id, Config: cfgPath, Share: filepath.Join(mapDir, "players.json")}
		options.onStatus = func(status companyHostStatus, synced bool) {
			a.mu.Lock()
			defer a.mu.Unlock()
			m.state.Players = status.Players
			if m.state.Address != "" {
				if synced {
					m.state.State = "online"
				} else {
					m.state.State = "starting"
				}
			}
			if status.Players > 0 {
				m.idleSince = time.Time{}
			} else if m.state.State == "online" && m.idleSince.IsZero() {
				m.idleSince = time.Now()
			}
		}
		options.onPublicReady = func(address string) {
			a.mu.Lock()
			defer a.mu.Unlock()
			m.state.Address = address
			m.state.State = "online"
			m.idleSince = time.Now()
			a.dirty = true
		}
		if err == nil {
			runner := a.runServer
			if runner == nil {
				runner = runCompanyHost
			}
			err = runner(childCtx, options, a.output)
		}
		cancel()
		a.mu.Lock()
		defer a.mu.Unlock()
		m.cancel = nil
		m.state = companyDirectoryState{SessionID: id, State: "offline"}
		if err != nil {
			m.state.State = "error"
			m.state.Error = err.Error()
			m.retryAfter = time.Now().Add(30 * time.Second)
		}
		a.dirty = true
	}()
}
func (a *hostAgent) stopIdle(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, m := range a.maps {
		mins := a.config.Maps[id].IdleMinutes
		if mins > 0 && m.cancel != nil && m.state.State == "online" && m.state.Players == 0 && !m.idleSince.IsZero() && now.Sub(m.idleSince) >= time.Duration(mins)*time.Minute {
			m.state.State = "stopping"
			m.cancel()
		}
	}
}
func runHostAgent(ctx context.Context, config hostAgentConfig, dir string, output io.Writer) error {
	if err := validateHostAgentConfig(config, true); err != nil {
		return err
	}
	companyRuntimePackageChecksDisabled = true
	if _, err := checkOpenOMSICompatibility(config.Server, true, false); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.ControlPort))
	if err != nil {
		return fmt.Errorf("o agente já está aberto ou a porta de controle %d está ocupada", config.ControlPort)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a := &hostAgent{config: config, maps: map[string]*hostAgentMap{}, dirty: true, output: &hostAgentLog{output: output}}
	for _, s := range config.Company.Sessions {
		state := "offline"
		if !config.Maps[s.ID].Enabled {
			state = "disabled"
		}
		a.maps[s.ID] = &hostAgentMap{state: companyDirectoryState{SessionID: s.ID, State: state}}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer "+config.ControlKey {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a.snapshot())
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer "+config.ControlKey {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
		cancel()
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	client := companyHTTPClient()
	defer client.CloseIdleConnections()
	defer func() {
		cancel()
		a.wg.Wait()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = server.Shutdown(shutdown)
		snapshot := a.snapshot()
		for i := range snapshot.Sessions {
			snapshot.Sessions[i].State = "offline"
			snapshot.Sessions[i].Address = ""
		}
		_ = companyDirectoryHTTP(shutdown, client, "POST", config.Company.DirectoryURL+"/heartbeat", config.HostKey, map[string]any{"sessions": snapshot.Sessions, "offline": true}, nil)
	}()
	fmt.Fprintln(output, "Agente ativo. Os mapas serão abertos quando um jogador iniciar uma viagem.")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		a.mu.Lock()
		dirty := a.dirty
		a.mu.Unlock()
		var cycleErr error
		if dirty {
			p := a.playerProfile()
			cycleErr = companyDirectoryHTTP(ctx, client, "POST", p.DirectoryURL+"/publish", config.HostKey, p, nil)
			if cycleErr == nil {
				a.mu.Lock()
				if profilesEqualAddresses(p, a.config.Company, a.maps) {
					a.dirty = false
				}
				a.mu.Unlock()
			}
		}
		if cycleErr == nil {
			cycleErr = companyDirectoryHTTP(ctx, client, "POST", config.Company.DirectoryURL+"/heartbeat", config.HostKey, map[string]any{"sessions": a.snapshot().Sessions}, nil)
		}
		var requests struct {
			Requests []struct {
				SessionID   string `json:"session_id"`
				RequestedAt int64  `json:"requested_at"`
				Weather     string `json:"weather"`
			} `json:"requests"`
		}
		if cycleErr == nil {
			cycleErr = companyDirectoryHTTP(ctx, client, "GET", config.Company.DirectoryURL+"/requests", config.HostKey, nil, &requests)
		}
		a.mu.Lock()
		if cycleErr != nil {
			a.directoryError = cycleErr.Error()
		} else {
			a.directoryError = ""
		}
		a.mu.Unlock()
		if cycleErr == nil {
			for _, r := range requests.Requests {
				a.demand(ctx, r.SessionID, r.RequestedAt, dir, r.Weather)
			}
		}
		a.stopIdle(time.Now())
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func profilesEqualAddresses(p, base CompanyProfile, maps map[string]*hostAgentMap) bool {
	for _, s := range p.Sessions {
		address := maps[s.ID].state.Address
		if address == "" {
			for _, original := range base.Sessions {
				if original.ID == s.ID {
					address = original.ServerURL
					break
				}
			}
		}
		if address != s.ServerURL {
			return false
		}
	}
	return true
}

type hostAgentLog struct {
	mu     sync.Mutex
	output io.Writer
}

func (w *hostAgentLog) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.output.Write(b)
}
func hostAgentLocalRequest(ctx context.Context, c hostAgentConfig, method, path string, result any) error {
	return companyDirectoryHTTP(ctx, companyHostClient(), method, fmt.Sprintf("http://127.0.0.1:%d%s", c.ControlPort, path), c.ControlKey, nil, result)
}
func hostAgentLogPath(dir string) string {
	return filepath.Join(companyHostDataDir(dir), "HostAgent.log")
}
func openHostAgentLog(dir string) (*os.File, error) {
	path := hostAgentLogPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 8<<20 {
		_ = os.Rename(path, path+".previous")
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
}
