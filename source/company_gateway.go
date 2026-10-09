package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type companyGatewayBus struct {
	owner *companyGatewayPeer
	bus   string
}
type companyGatewayPeer struct {
	g        *companyGateway
	key      string
	upstream *net.UDPConn
	remote   *net.UDPAddr
	// id and bus belong to g.mu; connection lifetime belongs to mu/closed.
	id        uint32
	bus       string
	mu        sync.Mutex
	ws        *companyGatewayWS
	closed    atomic.Bool
	closedAt  atomic.Int64
	lastAt    atomic.Int64
	closeOnce sync.Once
}
type companyGateway struct {
	mu                          sync.Mutex
	closed                      bool
	ctx                         context.Context
	cancel                      context.CancelFunc
	udp                         *net.UDPConn
	web                         net.Listener
	http                        *http.Server
	reserveUDP                  *net.UDPConn
	reserveWeb                  net.Listener
	backendUDP                  *net.UDPAddr
	backendWeb                  string
	backendPort, backendWebPort int
	port, webPort               int
	releaseOnce                 sync.Once
	closeOnce                   sync.Once
	ready                       atomic.Bool
	nextPeer                    atomic.Uint64
	peers                       map[string]*companyGatewayPeer
	buses                       map[uint32]*companyGatewayBus
	catalog                     map[string]string
	mask                        string
	fleet                       []string
	session                     CompanySession
	client                      *http.Client
	wg                          sync.WaitGroup
	limit                       int
}

func newCompanyGateway(ctx context.Context, cfg companyHostConfig, session CompanySession, fleet []string, catalog map[string]string, mask string) (*companyGateway, error) {
	child, cancel := context.WithCancel(ctx)
	g := &companyGateway{ctx: child, cancel: cancel, peers: map[string]*companyGatewayPeer{}, buses: map[uint32]*companyGatewayBus{}, catalog: catalog, mask: mask, fleet: fleet, session: session, client: companyHostClient(), limit: 256}
	var err error
	g.udp, err = net.ListenUDP("udp4", &net.UDPAddr{Port: cfg.Port})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("porta UDP %d indisponível: %w", cfg.Port, err)
	}
	g.web, err = net.Listen("tcp4", ":"+strconv.Itoa(cfg.WebPort))
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("porta HTTP %d indisponível: %w", cfg.WebPort, err)
	}
	g.reserveUDP, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		g.Close()
		return nil, err
	}
	g.reserveWeb, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		g.Close()
		return nil, err
	}
	g.backendUDP = g.reserveUDP.LocalAddr().(*net.UDPAddr)
	g.backendPort = g.backendUDP.Port
	g.backendWebPort = g.reserveWeb.Addr().(*net.TCPAddr).Port
	g.backendWeb = "http://127.0.0.1:" + strconv.Itoa(g.backendWebPort)
	g.port, g.webPort = g.udp.LocalAddr().(*net.UDPAddr).Port, g.web.Addr().(*net.TCPAddr).Port
	g.http = &http.Server{Handler: http.HandlerFunc(g.serveHTTP), ReadHeaderTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	g.wg.Add(2)
	go func() { defer g.wg.Done(); _ = g.http.Serve(g.web) }()
	go func() { defer g.wg.Done(); g.readUDP() }()
	return g, nil
}

func (g *companyGateway) releaseBackend() {
	g.releaseOnce.Do(func() {
		if g.reserveUDP != nil {
			_ = g.reserveUDP.Close()
		}
		if g.reserveWeb != nil {
			_ = g.reserveWeb.Close()
		}
	})
}

func (g *companyGateway) Close() {
	g.closeOnce.Do(func() {
		g.mu.Lock()
		g.closed = true
		peers := make([]*companyGatewayPeer, 0, len(g.peers))
		for _, p := range g.peers {
			peers = append(peers, p)
		}
		g.mu.Unlock()
		g.cancel()
		if g.http != nil {
			_ = g.http.Close()
		} else if g.web != nil {
			_ = g.web.Close()
		}
		if g.udp != nil {
			_ = g.udp.Close()
		}
		g.releaseBackend()
		for _, p := range peers {
			p.close()
		}
		g.wg.Wait()
		g.client.CloseIdleConnections()
	})
}

func (g *companyGateway) newPeer(key string, remote *net.UDPAddr) *companyGatewayPeer {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || !g.ready.Load() {
		return nil
	}
	if p := g.peers[key]; p != nil {
		return p
	}
	if len(g.peers) >= g.limit {
		return nil
	}
	for id, record := range g.buses {
		if at := record.owner.closedAt.Load(); at != 0 && time.Since(time.Unix(0, at)) > 150*time.Second {
			delete(g.buses, id)
		}
	}
	if len(g.buses) >= 4096 {
		return nil
	}
	upstream, err := net.DialUDP("udp4", nil, g.backendUDP)
	if err != nil {
		return nil
	}
	p := &companyGatewayPeer{g: g, key: key, upstream: upstream, remote: remote}
	p.lastAt.Store(time.Now().UnixNano())
	g.peers[key] = p
	g.wg.Add(1)
	go func() { defer g.wg.Done(); defer p.close(); p.readNative() }()
	return p
}

func (p *companyGatewayPeer) close() {
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		p.closedAt.Store(time.Now().UnixNano())
		_ = p.upstream.Close()
		p.mu.Lock()
		if p.ws != nil {
			_ = p.ws.conn.Close()
		}
		p.mu.Unlock()
		p.g.mu.Lock()
		if p.g.peers[p.key] == p {
			delete(p.g.peers, p.key)
		}
		p.g.mu.Unlock()
	})
}

func (p *companyGatewayPeer) send(packet []byte) error {
	if p.closed.Load() {
		return net.ErrClosed
	}
	p.lastAt.Store(time.Now().UnixNano())
	p.g.mu.Lock()
	data, err := p.g.incoming(p, packet)
	p.g.mu.Unlock()
	if err != nil {
		return err
	}
	_ = p.upstream.SetWriteDeadline(time.Now().Add(4 * time.Second))
	_, err = p.upstream.Write(data)
	return err
}

func (p *companyGatewayPeer) sendClient(packet []byte) error {
	if len(packet) > companyGatewayPacketLimit {
		return fmt.Errorf("datagrama muito grande")
	}
	if p.remote != nil {
		_, err := p.g.udp.WriteToUDP(packet, p.remote)
		return err
	}
	p.mu.Lock()
	ws := p.ws
	p.mu.Unlock()
	if ws == nil {
		return net.ErrClosed
	}
	return ws.write(2, packet)
}

func (p *companyGatewayPeer) readNative() {
	buf := make([]byte, companyGatewayPacketLimit+1)
	for {
		_ = p.upstream.SetReadDeadline(time.Now().Add(time.Second))
		n, err := p.upstream.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && !p.closed.Load() && p.g.ctx.Err() == nil && time.Since(time.Unix(0, p.lastAt.Load())) < 120*time.Second {
				continue
			}
			return
		}
		if n > companyGatewayPacketLimit {
			continue
		}
		p.g.mu.Lock()
		packet := p.g.outgoing(p, buf[:n])
		p.g.mu.Unlock()
		if err := p.sendClient(packet); err != nil {
			return
		}
	}
}

func (g *companyGateway) readUDP() {
	buf := make([]byte, companyGatewayPacketLimit+1)
	for {
		n, remote, err := g.udp.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n > companyGatewayPacketLimit {
			continue
		}
		key := "udp:" + remote.String()
		g.mu.Lock()
		p := g.peers[key]
		g.mu.Unlock()
		if p == nil {
			text := string(buf[:n])
			if !strings.HasPrefix(text, "HELLO|") && !strings.HasPrefix(text, "DISCOVER|") {
				continue
			}
			p = g.newPeer(key, remote)
		}
		if p != nil {
			if err := p.send(buf[:n]); err != nil {
				_ = p.sendClient([]byte(fmt.Sprintf("REJECT|%d|%s", multiplayerProtocol, err)))
				p.close()
			}
		}
	}
}

func (g *companyGateway) adaptStatus(status companyHostStatus) (companyHostStatus, error) {
	var vehicles []string
	var joined string
	if json.Unmarshal(status.Vehicles, &joined) == nil {
		for _, s := range strings.Split(joined, ";") {
			if strings.TrimSpace(s) != "" {
				vehicles = append(vehicles, s)
			}
		}
	} else if err := json.Unmarshal(status.Vehicles, &vehicles); err != nil {
		return status, fmt.Errorf("o servidor oficial não informou seus modelos locais")
	}
	if !status.FreePlayerVehicles || len(vehicles) != 0 {
		want := map[string]bool{}
		for _, s := range g.fleet {
			want[companyGatewayBusKey(s)] = true
		}
		if len(vehicles) != len(want) {
			return status, fmt.Errorf("o servidor selecionado não aplicou a configuração de modelos locais")
		}
		for _, s := range vehicles {
			if !want[companyGatewayBusKey(s)] {
				return status, fmt.Errorf("o servidor selecionado alterou a configuração de modelos locais")
			}
			delete(want, companyGatewayBusKey(s))
		}
		if len(want) != 0 {
			return status, fmt.Errorf("o servidor selecionado repetiu um modelo local")
		}
	}
	status.FreePlayerVehicles = true
	status.Vehicles = json.RawMessage(`[]`)
	return status, validateCompanyHostStatus(status, g.session, nil)
}

func (g *companyGateway) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.RawQuery != "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.URL.Path == "/ws" {
		g.serveWS(w, r)
		return
	}
	path := r.URL.Path
	if path != "/status" && path != "/status.json" && path != "/players" && path != "/players.json" && path != "/icon.png" {
		http.NotFound(w, r)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, g.backendWeb+path, nil)
	if err != nil {
		http.Error(w, "server unavailable", http.StatusServiceUnavailable)
		return
	}
	resp, err := g.client.Do(req)
	if err != nil {
		http.Error(w, "server loading", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(data) > 256<<10 {
		http.Error(w, "invalid server response", http.StatusBadGateway)
		return
	}
	if strings.HasPrefix(path, "/status") {
		var status companyHostStatus
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &status) != nil || json.Unmarshal(data, &raw) != nil {
			http.Error(w, "invalid server status", http.StatusBadGateway)
			return
		}
		if _, err := g.adaptStatus(status); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		raw["vehicles"] = json.RawMessage(`[]`)
		raw["free_player_vehicles"] = json.RawMessage(`true`)
		raw["bbs_vehicle_adapter"] = json.RawMessage(`"protocol-6"`)
		data, err = json.Marshal(raw)
	} else if strings.HasPrefix(path, "/players") {
		var players []map[string]json.RawMessage
		if err = json.Unmarshal(data, &players); err == nil {
			g.mu.Lock()
			for _, player := range players {
				var id uint32
				var bus string
				if json.Unmarshal(player["id"], &id) == nil && json.Unmarshal(player["bus"], &bus) == nil && bus != "" {
					if record := g.buses[id]; record != nil {
						player["bus"], _ = json.Marshal(record.bus)
					}
				}
			}
			g.mu.Unlock()
			data, err = json.Marshal(players)
		}
	}
	if err != nil {
		http.Error(w, "invalid server response", http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if path == "/icon.png" {
		w.Header().Set("Content-Type", "image/png")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	_, _ = w.Write(data)
}
