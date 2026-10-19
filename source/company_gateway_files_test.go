package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func gatewayFileEcho(t *testing.T, f *gatewayFixture) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", f.g.backendPort))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	f.g.filesEnabled.Store(true)
	return l
}

func TestCompanyGatewayFilesPreserveStreamAndDoNotUsePlayerSlots(t *testing.T) {
	f := newGatewayProtocolFixture(t, 14)
	gatewayFileEcho(t, f)
	f.g.mu.Lock()
	f.g.limit = 0
	f.g.mu.Unlock()
	c, r := gatewayWSPathClient(t, f.g, "/tcp")
	data := bytes.Repeat([]byte{0, 255, 3, 9, 13}, 23000)
	_, _ = c.Write(gatewayMaskedFrame(2, false, data[:40000]))
	_, _ = c.Write(gatewayMaskedFrame(9, true, []byte("ping")))
	_, _ = c.Write(gatewayMaskedFrame(0, true, data[40000:]))
	var got []byte
	pong := false
	for len(got) < len(data) || !pong {
		op, body := gatewayReadWS(t, r)
		if op == 10 {
			pong = string(body) == "ping"
		} else if op == 2 {
			got = append(got, body...)
		} else {
			t.Fatal(op)
		}
	}
	if !bytes.Equal(got, data) {
		t.Fatal("encrypted stream bytes or order changed")
	}
	f.g.mu.Lock()
	peers, buses, streams := len(f.g.peers), len(f.g.buses), len(f.g.fileStreams)
	f.g.mu.Unlock()
	if peers != 0 || buses != 0 || streams != 1 {
		t.Fatal(peers, buses, streams)
	}
	// Closing the supervisor must close hijacked file streams and its native socket.
	done := make(chan struct{})
	go func() { f.g.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("file stream prevented host shutdown")
	}
}

func TestCompanyGatewayFilesBusyDoesNotBlockGameplay(t *testing.T) {
	f := newGatewayProtocolFixture(t, 14)
	f.g.filesEnabled.Store(true)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	f.g.mu.Lock()
	for i := 0; i < companyGatewayStreamSlots; i++ {
		f.g.fileStreams[&companyGatewayWS{conn: a}] = nil
	}
	f.g.mu.Unlock()
	c, r := gatewayWSPathClient(t, f.g, "/tcp")
	defer c.Close()
	op, body := gatewayReadWS(t, r)
	if op != 2 || string(body) != "ERR busy\n" {
		t.Fatal(op, string(body))
	}
	player := gatewayUDPClient(t, f.g)
	hello := strings.Replace(string(gatewayHello(gatewayTestKnown)), "HELLO|6|", "HELLO|14|", 1)
	_, _ = player.Write([]byte(hello))
	if !bytes.HasPrefix(gatewayReadUDP(t, player), []byte("WELCOME|14|")) {
		t.Fatal("file streams took gameplay capacity")
	}
}

func TestCompanyGatewayNativeTransfersPreserveIdentityAndDisableMapSharing(t *testing.T) {
	f := newGatewayProtocolFixture(t, 14)
	f.g.filesEnabled.Store(true)
	p := &companyGatewayPeer{g: f.g, id: 1}
	info := []byte("INFO|1|Driver|" + gatewayTestPrivate + "|Paint|810|Centro|12|2.5|0|table|tour||v2-" + strings.Repeat("ab", 32))
	f.g.mu.Lock()
	got, err := f.g.incoming(p, info)
	f.g.mu.Unlock()
	if err != nil || !bytes.Equal(got, info) {
		t.Fatal("driver filename, livery or fingerprint changed", string(got), err)
	}
	input := []byte("vehicles = restricted\naccept_uploads = 0\nshare_maps = 1\nname = Preserved\n")
	before := append([]byte(nil), input...)
	config := string(companyGatewayConfig(input, 27015, 27025, f.g.fleet, true))
	for _, setting := range []string{"vehicles = \n", "accept_uploads = 1\n", "share_maps = 0\n", "name = Preserved\n"} {
		if !strings.Contains(config, setting) {
			t.Fatal(config)
		}
	}
	if !bytes.Equal(input, before) {
		t.Fatal("user config mutated")
	}
	parent := []string{"PATH=kept", "OMSI_NO_LAN_MODS=1", "OMSI_CONTENT_ZIP=foreign.zip"}
	env := strings.Join(companyHostEnvironment(parent, "isolated", true), "\n")
	if strings.Contains(env, "OMSI_NO_LAN_MODS") || strings.Contains(env, "foreign.zip") || !strings.Contains(env, "OMSI_CONTENT=isolated") {
		t.Fatal(env)
	}
	status := companyHostStatus{Name: "Transfort", Map: hostTestSession().MapFile, Version: "0.3.38", Protocol: 14, Time: "13:15", MaxPlayers: 16, Vehicles: mustGatewayJSON([]string{}), World: []byte(`{"waiting":5}`)}
	if _, err := f.g.adaptStatus(status); err != nil {
		t.Fatal(err)
	}
	status.Vehicles = mustGatewayJSON([]string{gatewayTestKnown})
	if _, err := f.g.adaptStatus(status); err == nil {
		t.Fatal("native upload whitelist would refuse private buses")
	}
}

func TestCompanyGatewayBusTransfersVersionBoundary(t *testing.T) {
	for version, want := range map[string]bool{"0.3.37": false, "0.3.38": true, "0.3.38-beta.1": true, "0.3.38 (866c60d)": true, "0.3.39": true, "0.4.0": true, "1.0.0": true, "0.2.99": false, "0.1.0 (unknown)": false, "broken": false} {
		if got := openOMSIBusTransfers(version); got != want {
			t.Fatal(version, got, want)
		}
	}
}

func TestCompanyGatewayFileFramesHaveIndependentBounds(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 65556)
	ws := companyGatewayWS{r: bufio.NewReader(bytes.NewReader(gatewayMaskedFrame(2, true, data))), frameLimit: companyGatewayStreamLimit}
	_, _, got, err := ws.readFrame()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("native encrypted record was limited as a gameplay datagram", err)
	}
	ws = companyGatewayWS{r: bufio.NewReader(bytes.NewReader(gatewayMaskedFrame(2, true, bytes.Repeat([]byte{1}, companyGatewayStreamLimit+1)))), frameLimit: companyGatewayStreamLimit}
	if _, _, _, err := ws.readFrame(); err == nil {
		t.Fatal("oversized file frame accepted")
	}
}
