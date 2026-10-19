package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const gatewayTestKnown = "Vehicles/MAN_SD202/MAN_D92.bus"
const gatewayTestPrivate = `C:\OMSI 2\Vehicles\Private Alice\Private.bus`

type gatewayFixture struct {
	g        *companyGateway
	native   *net.UDPConn
	web      *httptest.Server
	packets  chan []byte
	version  string
	protocol int
	mu       sync.Mutex
}

func newGatewayFixture(t *testing.T) *gatewayFixture {
	return newGatewayProtocolFixture(t, 6)
}

func newGatewayProtocolFixture(t *testing.T, protocol int) *gatewayFixture {
	t.Helper()
	root := t.TempDir()
	bus := filepath.Join(root, filepath.FromSlash(gatewayTestKnown))
	if err := os.MkdirAll(filepath.Dir(bus), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bus, []byte("[friendlyname]\nMAN\nD92\nTest\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fleet, catalog, mask, err := companyGatewayFleet(root, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	g, err := newCompanyGateway(context.Background(), companyHostConfig{}, hostTestSession(), fleet, catalog, mask)
	if err != nil {
		t.Fatal(err)
	}
	g.releaseBackend()
	udp, err := net.ListenUDP("udp4", g.backendUDP)
	if err != nil {
		g.Close()
		t.Fatal(err)
	}
	f := &gatewayFixture{g: g, native: udp, packets: make(chan []byte, 32), version: "0.2.23", protocol: protocol}
	if protocol == 7 {
		f.version = "0.2.48 (42ebc79 2026-10-13 18:11)"
	}
	if protocol == 14 {
		f.version = "0.3.34 (4deb94e 2026-10-16 20:40)"
	}
	web := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" || r.URL.Path == "/status.json" {
			f.mu.Lock()
			version, protocol := f.version, f.protocol
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(companyHostStatus{Name: "Transfort", Map: hostTestSession().MapFile, Version: version, Protocol: protocol, Time: "13:15", MaxPlayers: 16, Vehicles: mustGatewayJSON(fleet), World: json.RawMessage(`{"waiting":5}`)})
		} else {
			http.NotFound(w, r)
		}
	}))
	listener, err := net.Listen("tcp4", strings.TrimPrefix(g.backendWeb, "http://"))
	if err != nil {
		udp.Close()
		g.Close()
		t.Fatal(err)
	}
	web.Listener = listener
	web.Start()
	f.web = web
	status, err := readCompanyHostStatus(context.Background(), g.client, g.backendWeb)
	if err == nil {
		_, err = g.adaptStatus(status)
	}
	if err != nil {
		t.Fatal(err)
	}
	g.ready.Store(true)
	go f.simulateOfficial()
	t.Cleanup(func() { g.Close(); udp.Close(); web.Close() })
	return f
}
func mustGatewayJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func (f *gatewayFixture) simulateOfficial() {
	buf := make([]byte, companyGatewayPacketLimit+1)
	ids := map[string]uint32{}
	var next uint32
	for {
		n, from, err := f.native.ReadFromUDP(buf)
		if err != nil {
			return
		}
		packet := append([]byte(nil), buf[:n]...)
		select {
		case f.packets <- packet:
		default:
		}
		text := string(packet)
		f.mu.Lock()
		protocol := f.protocol
		f.mu.Unlock()
		if strings.HasPrefix(text, "HELLO|") {
			fields := strings.Split(text, "|")
			if len(fields) < 5 {
				continue
			}
			if fields[1] != fmt.Sprint(protocol) {
				_, _ = f.native.WriteToUDP([]byte(fmt.Sprintf("REJECT|%d|protocol mismatch", protocol)), from)
				continue
			}
			allowed := false
			for _, bus := range f.g.fleet {
				if fields[4] == bus || fields[4] == "" {
					allowed = true
				}
			}
			if !allowed {
				_, _ = f.native.WriteToUDP([]byte(fmt.Sprintf("REJECT|%d|unlisted bus", protocol)), from)
				continue
			}
			id := ids[from.String()]
			if id == 0 {
				next++
				id = next
				ids[from.String()] = id
			}
			_, _ = f.native.WriteToUDP([]byte(fmt.Sprintf("WELCOME|%d|%d|session|Host|map|date|time|weather|season|2", protocol, id)), from)
		} else {
			_, _ = f.native.WriteToUDP(packet, from)
		}
	}
}

func gatewayUDPClient(t *testing.T, g *companyGateway) *net.UDPConn {
	t.Helper()
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: g.port})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func gatewayReadUDP(t *testing.T, c *net.UDPConn) []byte {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	b := make([]byte, companyGatewayPacketLimit+1)
	n, err := c.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	return b[:n]
}
func gatewayHello(bus string) []byte {
	return []byte("HELLO|6|-|Alice|" + bus + "|maps/Test/global.cfg|2026-10-09|13:15|rain|autumn|nonce")
}

func TestCompanyGatewayOfficialPrivateBusAndOpaqueBinary(t *testing.T) {
	f := newGatewayFixture(t)
	c := gatewayUDPClient(t, f.g)
	_, _ = c.Write(gatewayHello(gatewayTestPrivate))
	welcome := gatewayReadUDP(t, c)
	if !bytes.HasPrefix(welcome, []byte("WELCOME|6|1|")) {
		t.Fatal(string(welcome))
	}
	if packet := <-f.packets; !strings.Contains(string(packet), "|"+f.g.mask+"|") || bytes.Contains(packet, []byte("Private Alice")) {
		t.Fatal("private path sent to simulator", string(packet))
	}
	info := []byte("INFO|1|Alice|" + gatewayTestPrivate + "|Paint|810|Centro|13.4|2.5|0.8|table-hash|810/Tour|display|figure.hum")
	_, _ = c.Write(info)
	if got := gatewayReadUDP(t, c); !bytes.Equal(got, info) {
		t.Fatal("real bus or geometry changed", string(got))
	}
	packet := <-f.packets
	if !strings.Contains(string(packet), "|"+f.g.mask+"|Paint|810|") {
		t.Fatal(string(packet))
	}
	binaryState := []byte{0, 255, 128, 17, 6, 1, 0, 0, 45, 93}
	_, _ = c.Write(binaryState)
	if got := gatewayReadUDP(t, c); !bytes.Equal(got, binaryState) {
		t.Fatal("binary simulation datagram modified", got)
	}
	if got := <-f.packets; !bytes.Equal(got, binaryState) {
		t.Fatal(got)
	}
	_, _ = c.Write([]byte("INFO|2|Alice|Vehicles/Other/bus.bus|"))
	if got := gatewayReadUDP(t, c); !bytes.HasPrefix(got, []byte("REJECT|6|")) {
		t.Fatal("another ID allowed", string(got))
	}
	select {
	case got := <-f.packets:
		t.Fatal("forged INFO reached simulator", string(got))
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCompanyGatewayStatusAndVersionUpdates(t *testing.T) {
	f := newGatewayFixture(t)
	for _, version := range []string{"0.2.11", "0.2.20-bbs-free2", "0.2.23", "0.2.99"} {
		f.mu.Lock()
		f.version = version
		f.mu.Unlock()
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", f.g.webPort))
		if err != nil {
			t.Fatal(err)
		}
		var status companyHostStatus
		err = json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || status.Version != version || !status.FreePlayerVehicles || string(status.Vehicles) != "[]" || string(status.World) != `{"waiting":5}` {
			t.Fatal(version, status, err, resp.StatusCode)
		}
	}
	f.mu.Lock()
	f.protocol = 8
	f.mu.Unlock()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", f.g.webPort))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatal("unsupported protocol advertised as compatible", resp.StatusCode)
	}
	for _, path := range []string{"/admin", "/players", "/players.json"} {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", f.g.webPort, path))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Fatal("private administration or positions exposed", path, resp.StatusCode)
		}
	}
}

func gatewayWSClient(t *testing.T, g *companyGateway) (net.Conn, *bufio.Reader) {
	return gatewayWSPathClient(t, g, "/ws")
}

func gatewayWSPathClient(t *testing.T, g *companyGateway, path string) (net.Conn, *bufio.Reader) {
	t.Helper()
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", g.webPort), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive, Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", path)
	r := bufio.NewReader(c)
	response, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 101 || response.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatal(response)
	}
	return c, r
}
func gatewayMaskedFrame(op byte, final bool, data []byte) []byte {
	first := op
	if final {
		first |= 128
	}
	frame := []byte{first}
	if len(data) < 126 {
		frame = append(frame, 128|byte(len(data)))
	} else if len(data) <= 65535 {
		frame = append(frame, 128|126, byte(len(data)>>8), byte(len(data)))
	} else {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(data)))
		frame = append(append(frame, 128|127), size[:]...)
	}
	mask := []byte{21, 35, 7, 99}
	frame = append(frame, mask...)
	for i, v := range data {
		frame = append(frame, v^mask[i%4])
	}
	return frame
}
func gatewayReadWS(t *testing.T, r *bufio.Reader) (byte, []byte) {
	t.Helper()
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		t.Fatal(err)
	}
	if h[1]&128 != 0 {
		t.Fatal("server frame is masked")
	}
	n := int(h[1] & 127)
	if n == 126 {
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			t.Fatal(err)
		}
		n = int(binary.BigEndian.Uint16(ext[:]))
	}
	if n > companyGatewayPacketLimit {
		t.Fatal("oversized frame")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		t.Fatal(err)
	}
	return h[0] & 15, data
}

func TestCompanyGatewayWebSocketFragmentsPingAndPrivateBus(t *testing.T) {
	f := newGatewayFixture(t)
	c, r := gatewayWSClient(t, f.g)
	hello := gatewayHello(gatewayTestPrivate)
	_, _ = c.Write(gatewayMaskedFrame(2, false, hello[:25]))
	_, _ = c.Write(gatewayMaskedFrame(9, true, []byte("ping")))
	_, _ = c.Write(gatewayMaskedFrame(0, true, hello[25:]))
	op, pong := gatewayReadWS(t, r)
	if op != 10 || string(pong) != "ping" {
		t.Fatal(op, string(pong))
	}
	op, welcome := gatewayReadWS(t, r)
	if op != 2 || !bytes.HasPrefix(welcome, []byte("WELCOME|6|1|")) {
		t.Fatal(op, string(welcome))
	}
	<-f.packets
	info := []byte("INFO|1|Alice|" + gatewayTestPrivate + "|Paint|810|Centro|12|2.5|0|table|tour||")
	_, _ = c.Write(gatewayMaskedFrame(2, true, info))
	op, got := gatewayReadWS(t, r)
	if op != 2 || !bytes.Equal(got, info) {
		t.Fatal("WebSocket filename round trip changed", op, string(got))
	}
	<-f.packets
	binaryState := bytes.Repeat([]byte{0, 201, 36, 128, 5}, 100)
	_, _ = c.Write(gatewayMaskedFrame(2, true, binaryState))
	op, got = gatewayReadWS(t, r)
	if op != 2 || !bytes.Equal(got, binaryState) {
		t.Fatal("WebSocket binary packet changed", op, len(got))
	}
}

func TestCompanyGatewayUploadRefusalUsesNativeStreamWithoutPlayerSlot(t *testing.T) {
	for _, protocol := range []int{6, 7, 14} {
		t.Run(fmt.Sprintf("protocol-%d", protocol), func(t *testing.T) {
			f := newGatewayProtocolFixture(t, protocol)
			// A mod stream must not reserve a gameplay place, even while all
			// places are occupied or the game's world is still loading.
			f.g.ready.Store(false)
			f.g.mu.Lock()
			f.g.limit = 0
			f.g.mu.Unlock()
			c, r := gatewayWSPathClient(t, f.g, "/tcp")
			greeting := []byte("OMSIMODS/3 123456789abc " + strings.Repeat("ab", 32) + "\n")
			// TCP chunks and WebSocket fragments are different boundaries.
			_, _ = c.Write(gatewayMaskedFrame(2, true, greeting[:8]))
			_, _ = c.Write(gatewayMaskedFrame(2, false, greeting[8:40]))
			_, _ = c.Write(gatewayMaskedFrame(9, true, []byte("ping")))
			op, data := gatewayReadWS(t, r)
			if op != 10 || string(data) != "ping" {
				t.Fatal("mod stream ping was lost", op, string(data))
			}
			_, _ = c.Write(gatewayMaskedFrame(0, true, greeting[40:]))
			op, data = gatewayReadWS(t, r)
			if op != 2 || string(data) != "ERR this server takes no uploads\n" {
				t.Fatal("native uploader would see a connection failure instead of a refusal", op, string(data))
			}
			op, data = gatewayReadWS(t, r)
			if op != 8 || !bytes.Equal(data, []byte{3, 232}) {
				t.Fatal("mod stream did not close normally", op, data)
			}
			f.g.mu.Lock()
			peers, buses := len(f.g.peers), len(f.g.buses)
			f.g.limit = 256
			f.g.mu.Unlock()
			if peers != 0 || buses != 0 || f.g.nextPeer.Load() != 0 {
				t.Fatal("mod stream used a gameplay identity", peers, buses)
			}
			select {
			case packet := <-f.packets:
				t.Fatal("file stream reached native game", packet)
			default:
			}
			f.g.ready.Store(true)
			player := gatewayUDPClient(t, f.g)
			hello := []byte(strings.Replace(string(gatewayHello(gatewayTestPrivate)), "HELLO|6|", fmt.Sprintf("HELLO|%d|", protocol), 1))
			_, _ = player.Write(hello)
			if got := gatewayReadUDP(t, player); !bytes.HasPrefix(got, []byte(fmt.Sprintf("WELCOME|%d|", protocol))) {
				t.Fatal("gameplay failed after upload refusal", string(got))
			}
		})
	}
}

func TestCompanyGatewayUploadEndpointRejectsOtherRequests(t *testing.T) {
	f := newGatewayProtocolFixture(t, 14)
	for _, path := range []string{"/tcp", "/tcp?target=127.0.0.1", "/tcp/admin", "/admin/"} {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", f.g.webPort, path))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := http.StatusNotFound
		if path == "/tcp" {
			want = http.StatusBadRequest
		}
		if resp.StatusCode != want {
			t.Fatal("request unexpectedly served", path, resp.StatusCode)
		}
	}
	for _, data := range [][]byte{bytes.Repeat([]byte("x"), 257), []byte("GET /admin HTTP/1.1\n")} {
		c, r := gatewayWSPathClient(t, f.g, "/tcp")
		_, _ = c.Write(gatewayMaskedFrame(2, true, data))
		if len(data) > 256 {
			op, answer := gatewayReadWS(t, r)
			if op != 8 || !bytes.Equal(answer, []byte{3, 241}) {
				t.Fatal("oversized greeting was accepted", op, answer)
			}
		} else if _, err := r.ReadByte(); err == nil {
			t.Fatal("non-mod request received a response")
		}
	}
}

func TestCompanyGatewayPrivateConfigurationDisablesFileTransfers(t *testing.T) {
	input := []byte("name = Preserved\naccept_uploads = 1\nshare_maps = 1\n# share_maps = 1\n")
	before := append([]byte(nil), input...)
	for _, base := range [][]byte{input, []byte("name = Preserved\n")} {
		cfg, err := parseCompanyHostConfig(companyGatewayConfig(base, 27015, 27025, []string{gatewayTestKnown}, false))
		if err != nil {
			t.Fatal(err)
		}
		for _, setting := range []string{"accept_uploads = 0\n", "share_maps = 0\n", "name = Preserved\n"} {
			if !bytes.Contains(cfg.Text, []byte(setting)) {
				t.Fatal("private runtime configuration is missing", setting, string(cfg.Text))
			}
		}
	}
	if !bytes.Equal(input, before) {
		t.Fatal("user configuration changed")
	}
}

func TestCompanyGatewayTransportBoundsAndMapIsolation(t *testing.T) {
	for _, frame := range [][]byte{{130, 0}, {130, 255, 0, 0, 0, 0, 0, 1, 0, 0}, {137, 254, 0, 126}, {9, 128}} {
		ws := companyGatewayWS{r: bufio.NewReader(bytes.NewReader(frame))}
		if _, _, _, err := ws.readFrame(); err == nil {
			t.Fatal("invalid/oversized frame accepted", frame)
		}
	}
	a, b := newGatewayFixture(t), newGatewayFixture(t)
	ca, cb := gatewayUDPClient(t, a.g), gatewayUDPClient(t, b.g)
	_, _ = ca.Write(gatewayHello(gatewayTestPrivate))
	gatewayReadUDP(t, ca)
	other := "Vehicles/Bob Private/Another.bus"
	_, _ = cb.Write(gatewayHello(other))
	gatewayReadUDP(t, cb)
	for _, x := range []struct {
		f   *gatewayFixture
		c   *net.UDPConn
		bus string
	}{{a, ca, gatewayTestPrivate}, {b, cb, other}} {
		info := []byte("INFO|1|Driver|" + x.bus + "||||12|2.5|0|table|||")
		_, _ = x.c.Write(info)
		if got := gatewayReadUDP(t, x.c); !bytes.Equal(got, info) {
			t.Fatal("one map used another map's bus", string(got))
		}
	}
	port, webPort := a.g.port, a.g.webPort
	a.g.Close()
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{Port: port})
	if err != nil {
		t.Fatal("old UDP listener survived stop", err)
	}
	udp.Close()
	web, err := net.Listen("tcp4", fmt.Sprintf(":%d", webPort))
	if err != nil {
		t.Fatal("old HTTP listener survived stop", err)
	}
	web.Close()
	_, _ = cb.Write([]byte{0, 5, 10})
	if got := gatewayReadUDP(t, cb); !bytes.Equal(got, []byte{0, 5, 10}) {
		t.Fatal("stopping a different map interrupted this map")
	}
}

func TestCompanyGatewayFleetIsModelsNotPlayerPermissions(t *testing.T) {
	f := newGatewayFixture(t)
	if f.g.fleet[0] != gatewayTestKnown || f.g.mask == gatewayTestPrivate {
		t.Fatal(f.g.fleet)
	}
	for _, path := range []string{"../Vehicles/x.bus", "Vehicles/../x.bus", "Vehicles/x;other.bus", "Vehicles/x|other.bus", "Vehicles//x.bus"} {
		if companyGatewayBusKey(path) != "" {
			t.Fatal("unsafe path accepted", path)
		}
	}
	if key := companyGatewayBusKey(`D:\OMSI 2\Vehicles\Private Alice\Private.bus`); key != "vehicles/private alice/private.bus" {
		t.Fatal(key)
	}
	if _, _, _, err := companyGatewayFleet(t.TempDir(), nil, "test"); err == nil {
		t.Fatal("missing local substitute silently ignored")
	}
	text := companyGatewayConfig([]byte("TUNNEL = 1\nport = 7\nport = 8\n# owner\nweather = rain\n"), 27050, 27051, f.g.fleet, false)
	if strings.Contains(string(text), "TUNNEL = 1") || strings.Contains(string(text), "port = 7") || !strings.Contains(string(text), "weather = rain") || strings.Count(string(text), "port = 27050") != 2 {
		t.Fatal(string(text))
	}
}

func TestCompanyGatewayManagedTunnelPublicationAndMapIsolation(t *testing.T) {
	var logs [2]bytes.Buffer
	owners := []*companyHostTunnelOutput{newCompanyHostTunnelOutput(&logs[0]), newCompanyHostTunnelOutput(&logs[1])}
	writers := []*companyTunnelOutput{{output: owners[0]}, {output: owners[1]}}
	addresses := []string{"https://map-a-session.trycloudflare.com", "https://map-b-session.trycloudflare.com"}
	for i, writer := range writers {
		for _, chunk := range []string{"2026-10-09 INF | https://map-", string('a'+rune(i)) + "-session.try", "cloudflare.com |\r\n"} {
			if _, err := writer.Write([]byte(chunk)); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case got := <-owners[i].urls:
			if got != addresses[i] {
				t.Fatal("wrong map tunnel", got)
			}
		default:
			t.Fatal("cloudflared URL did not reach publication")
		}
	}
	for _, invalid := range []string{"https://map-a-session.trycloudflare.com.evil.invalid", "https://map-a-session.trycloudflare.com/private", "http://map-a-session.trycloudflare.com", strings.Repeat("x", 8200) + " https://map-a-session.trycloudflare.com"} {
		_, _ = writers[0].Write([]byte("INF | " + invalid + " |\n"))
		select {
		case got := <-owners[0].urls:
			t.Fatal("invalid tunnel accepted", got)
		default:
		}
	}
	var concurrent sync.WaitGroup
	for i := 0; i < 4; i++ {
		concurrent.Add(1)
		go func() { defer concurrent.Done(); _, _ = writers[0].Write([]byte("ordinary tunnel log\n")) }()
	}
	concurrent.Wait()
	if strings.Count(logs[0].String(), "ordinary tunnel log") != 4 {
		t.Fatal("lost concurrent tunnel logs")
	}
	if strings.Contains(logs[1].String(), addresses[0]) {
		t.Fatal("mixed two maps' tunnel output")
	}
}

// Protocol 7 adds a rebind secret and challenge/proof fields. The bridge owns
// transport routing and bus substitution only; these native fields stay exact.
func TestCompanyGatewayProtocol7TransportsAndReconnect(t *testing.T) {
	for _, protocol := range []int{7, 14} {
		for _, transport := range []string{"udp", "websocket"} {
			t.Run(fmt.Sprintf("protocol-%d/%s", protocol, transport), func(t *testing.T) {
				f := newGatewayProtocolFixture(t, protocol)
				var send func([]byte)
				var read func() []byte
				if transport == "udp" {
					c := gatewayUDPClient(t, f.g)
					send = func(b []byte) {
						if _, e := c.Write(b); e != nil {
							t.Fatal(e)
						}
					}
					read = func() []byte { return gatewayReadUDP(t, c) }
				} else {
					c, r := gatewayWSClient(t, f.g)
					send = func(b []byte) {
						if _, e := c.Write(gatewayMaskedFrame(2, true, b)); e != nil {
							t.Fatal(e)
						}
					}
					read = func() []byte {
						op, b := gatewayReadWS(t, r)
						if op != 2 {
							t.Fatal(op)
						}
						return b
					}
				}
				tail := "|maps/Test/global.cfg|2026-10-13|61200.5|Weather/#CAVOK.owt|0|1234567890abcdef|fedcba9876543210|1122334455667788"
				hello := []byte(fmt.Sprintf("HELLO|%d|-|Alice|", protocol) + gatewayTestPrivate + tail)
				send(hello)
				if welcome := read(); !bytes.HasPrefix(welcome, []byte(fmt.Sprintf("WELCOME|%d|1|", protocol))) {
					t.Fatal(string(welcome))
				}
				if got := <-f.packets; string(got) != fmt.Sprintf("HELLO|%d|-|Alice|", protocol)+f.g.mask+tail {
					t.Fatal("reconnect proof changed", string(got))
				}
				for _, packet := range [][]byte{
					[]byte("CHALLENGE|fedcba9876543210"),
					[]byte(fmt.Sprintf("WELCOME|%d|1|abcdef123456|Host|map|date|time|weather|season|2|00112233445566778899aabbccddeeff", protocol)),
					[]byte{0, 255, 7, 128, 25, 0, 46, 255},
					append([]byte{0xea, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, []byte{0xeb, 14, 255, 0, 128}...),
					[]byte("CMD|1|1|admin-challenge abc 00112233445566778899aabbccddeeff 600000"),
				} {
					send(packet)
					if got := read(); !bytes.Equal(got, packet) {
						t.Fatal("native authentication or binary packet changed", got)
					}
					if got := <-f.packets; !bytes.Equal(got, packet) {
						t.Fatal(got)
					}
				}
				info := []byte("INFO|1|Alice|" + gatewayTestPrivate + "|Paint|810|Centro|12|2.5|0|table|tour|abcd|figure.hum")
				send(info)
				if got := read(); !bytes.Equal(got, info) {
					t.Fatal("private bus lost", string(got))
				}
				if got := <-f.packets; !bytes.Contains(got, []byte("|"+f.g.mask+"|")) {
					t.Fatal(string(got))
				}
				send([]byte("INFO|2|Other|Vehicles/Other/bus.bus|"))
				if got := read(); !bytes.HasPrefix(got, []byte(fmt.Sprintf("REJECT|%d|", protocol))) {
					t.Fatal("wrong rejection protocol", string(got))
				}
			})
		}
	}
}

func TestCompanyGatewayProtocol7StatusAndLegacyIsolation(t *testing.T) {
	legacy, current := newGatewayFixture(t), newGatewayProtocolFixture(t, 7)
	for _, f := range []*gatewayFixture{legacy, current} {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", f.g.webPort))
		if err != nil {
			t.Fatal(err)
		}
		var status map[string]json.RawMessage
		err = json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || string(status["protocol"]) != fmt.Sprint(f.protocol) || string(status["bbs_vehicle_adapter"]) != fmt.Sprintf(`"protocol-%d"`, f.protocol) {
			t.Fatal(status, resp.StatusCode, err)
		}
	}
	// A mismatched client is left to the native server to reject. We never
	// relabel its HELLO as 7, which could corrupt binary traffic later.
	c := gatewayUDPClient(t, current.g)
	hello := gatewayHello(gatewayTestKnown)
	_, _ = c.Write(hello)
	if got := gatewayReadUDP(t, c); string(got) != "REJECT|7|protocol mismatch" {
		t.Fatal(string(got))
	}
	if got := <-current.packets; !bytes.Equal(got, hello) {
		t.Fatal("client protocol translated", string(got))
	}
	if legacy.g.networkProtocol() != 6 || current.g.networkProtocol() != 7 {
		t.Fatal("maps shared protocol state")
	}
	current.mu.Lock()
	current.protocol = 6
	current.mu.Unlock()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", current.g.webPort))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 || current.g.networkProtocol() != 7 {
		t.Fatal("wire protocol changed in running map", resp.StatusCode)
	}
}
