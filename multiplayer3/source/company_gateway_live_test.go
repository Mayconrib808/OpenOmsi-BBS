package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNativeDutyDispatchRequiresOwnedPlayerAndActualClientAck(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/admin" || r.Header.Get("X-Admin-Password") != "private-test-secret" {
			http.Error(w, "bad request", 403)
			return
		}
		requests <- string(b)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	g := &companyGateway{ctx: context.Background(), udp: udp, backendWeb: server.URL, adminPassword: "private-test-secret", client: server.Client(), buses: map[uint32]*companyGatewayBus{}}
	p := &companyGatewayPeer{g: g, id: 2, remote: client.LocalAddr().(*net.UDPAddr)}
	g.buses[2] = &companyGatewayBus{owner: p}
	const plugin = "bbs_next_trip_0123456789abcdef"
	const id = "4974938_1234abcd"
	request := func(from int, text string) {
		t.Helper()
		handled, e := p.liveTripRequest([]byte(fmt.Sprintf("CMD|%d|1|plugin %s %s", from, plugin, text)))
		if !handled || e != nil {
			t.Fatal(handled, e)
		}
	}
	request(3, "take "+id+" 130%20&%20N30 Mo-Fr%203 4 1")
	select {
	case body := <-requests:
		t.Fatal("another player was allowed", body)
	default:
	}
	request(2, "hello")
	if got := string(gatewayReadUDP(t, client)); got != "CMD|1|2|plugin "+plugin+" ready" {
		t.Fatal(got)
	}
	request(2, "take "+id+" 130%20&%20N30 Mo-Fr%203 4 1")
	select {
	case body := <-requests:
		if body != "duty 2 130%20&%20N30 Mo-Fr%203 4 1\n" {
			t.Fatal(body)
		}
	case <-time.After(time.Second):
		t.Fatal("dispatch was not sent")
	}
	// 202 only accepts the request. It must never produce a 'done' message.
	_ = client.SetReadDeadline(time.Now().Add(25 * time.Millisecond))
	var b [2048]byte
	if n, err := client.Read(b[:]); err == nil {
		t.Fatal("false acknowledgment", string(b[:n]))
	}
	_, err = p.liveTripRequest([]byte("CMD|3|1|duty-ok 130%20&%20N30 Mo-Fr%203 4 1"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.liveTripRequest([]byte("CMD|2|1|duty-ok 130%20&%20N30 Mo-Fr%203 4 1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(gatewayReadUDP(t, client)); got != "CMD|1|2|plugin "+plugin+" done "+id {
		t.Fatal(got)
	}
	request(2,"take "+id+" 130%20&%20N30 Mo-Fr%203 4 1")
	if got:=string(gatewayReadUDP(t,client));got!="CMD|1|2|plugin "+plugin+" done "+id {t.Fatal(got)}
	select {case body:=<-requests:t.Fatal("duplicate request reset an already confirmed trip",body);default:}
	request(2, "take "+id+" 130%20&%20N30 Mo-Fr%203 3 1")
	if got := string(gatewayReadUDP(t, client)); !strings.Contains(got, "Request identity conflict") {
		t.Fatal(got)
	}
}

func TestDutyDispatchDoesNotChangeClockOrOtherPlayers(t *testing.T) {
	if arg, err := liveDutyArgument("Line % 7", "Turno com espaços", 4, 1); err != nil || arg != "Line%20%25%207 Turno%20com%20espaços 4 1" {
		t.Fatal(arg, err)
	}
	for _, line := range []string{"", "7\nclock 0", "7|other", "7\tother"} {
		if _, err := liveDutyArgument(line, "duty", 0, 0); err == nil {
			t.Fatal("accepted injected command", line)
		}
	}
}
