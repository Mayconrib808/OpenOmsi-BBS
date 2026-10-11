package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func liveTestBlock(id, line, tour string) string {
	return "Schicht ID: " + id + "\nKarte: Test\nTour: 04:05:00 - 04:30:00 (Linie: " + line + ")\nA - B (Umlauf: " + tour + ")\n"
}
func TestLiveTripAcceptsActualBCSContinuationObserverBlock(t *testing.T) {
	// This event path does not call the launch screen that normally logs the
	// new shift. The optional logger observer appends the accepted I state.
	text := "Schicht ID: 4951232\nKarte: SP_Carrao City\nTour: 20:00:00 - 21:00:00 (Linie: 3539)\nMetro Bresser 3539 - Cidade Tiradentes 3539 (Umlauf: 02)\n" +
		"Schicht abschließen: 4951232\n---\nWeitere Event Tour Multiplayer\n"
	if _, e := readLiveBCSTrip(text); e == nil {
		t.Fatal("inferred a selection that BCS did not expose")
	}
	text += "[OpenOmsi-BBS next-trip log v1]\nSchicht ID: 4951233\nKarte: SP_Carrao City\nTour: 21:10:00 - 22:10:00 (Linie: 3539)\nCidade Tiradentes 3539 - Metro Bresser 3539 (Umlauf: 02)\n"
	trip, e := readLiveBCSTrip(text)
	if e != nil || trip.ID != "4951233" || trip.Line != "3539" || trip.Tour != "02" || trip.Start != "21:10" || trip.Route != "Cidade Tiradentes 3539 - Metro Bresser 3539" {
		t.Fatal(trip, e)
	}
}

func TestLiveTripUsesNewestCompleteBCSSelection(t *testing.T) {
	if liveDepartureMinute(-300) != 1435 || liveDepartureMinute(86400+300) != 5 {
		t.Fatal("overnight departure normalization failed")
	}
	text := liveTestBlock("1001", "130 & N30", "Mo-Fr 9") + "Schicht abschließen: 1001\n" + liveTestBlock("1002", "454", "03")
	trip, e := readLiveBCSTrip(text)
	if e != nil || trip.ID != "1002" || trip.Line != "454" || trip.Start != "04:05" {
		t.Fatal(trip, e)
	}
	for _, suffix := range []string{"Schicht ID: 1003\nKarte: Partial\n", "Schicht abschließen: 1002\n"} {
		if _, e = readLiveBCSTrip(text + suffix); e == nil {
			t.Fatal("accepted an incomplete or closed newest shift")
		}
	}
	if liveShiftClosed(text, "100") || !liveShiftClosed(text, "1001") {
		t.Fatal("completion ID mismatch")
	}
}
func TestLiveDriverPreservesOldEvaluationAndRebasesTwoTrips(t *testing.T) {
	b, e := os.ReadFile("testdata/bbs-before.odr")
	if e != nil {
		t.Fatal(e)
	}
	base, e := parseDriver(b, false)
	if e != nil {
		t.Fatal(e)
	}
	origin := base
	origin.Stops[0] += 10
	origin.Tickets += 8
	origin.Hectom += 30
	origin.Rating[0] += 8
	origin.Rating[1] += 6
	actual := origin
	actual.Stops[0] += 3
	actual.Tickets += 2
	actual.Hectom += 7
	actual.Rating[0] += 4
	actual.Rating[1] += 3
	out, e := liveDriverRecord(actual, origin, base)
	if e != nil {
		t.Fatal(e)
	}
	if out.Stops[0] != base.Stops[0]+3 || out.Tickets != base.Tickets+2 || out.Hectom != base.Hectom+7 || out.Rating[1] != base.Rating[1]+3 {
		t.Fatal("prior trip counted twice", out)
	}
	second := actual
	second.Stops[0] += 2
	out, e = liveDriverRecord(second, actual, out)
	if e != nil || out.Stops[0] != base.Stops[0]+5 {
		t.Fatal(out, e)
	}
	dir := t.TempDir()
	native := filepath.Join(dir, "bbs.odr")
	if e = os.WriteFile(native, b, 0600); e != nil {
		t.Fatal(e)
	}
	s, e := prepareDriver(native, dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.freezeEvaluation(); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.OpenPath, actual.encode(true), 0600); e != nil {
		t.Fatal(e)
	}
	if changed, e := s.poll(); changed || e != nil {
		t.Fatal("completed result changed", changed, e)
	}
	got, _ := os.ReadFile(native)
	if !bytes.Equal(got, b) {
		t.Fatal("old evaluation overwritten")
	}
	s.beginLiveTrip(origin, base, sha256.Sum256(b))
	_, _ = s.poll()
	changed, e := s.poll()
	if !changed || e != nil {
		t.Fatal(changed, e)
	}
	got, _ = os.ReadFile(native)
	published, e := parseDriver(got, false)
	if e != nil || published.Stops[0] != base.Stops[0]+3 {
		t.Fatal(published, e)
	}
}
func TestLiveTripGatewayUsesPrivateAdminForOwnPlayerOnly(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		b, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/admin" || r.Header.Get("X-Admin-Password") != "private-secret" || string(b) != "duty 2 130%20&%20N30 Mo-Fr%209 1 0\n" {
			t.Errorf("incorrect native dispatch: %s %q", r.URL.Path, b)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	g := &companyGateway{ctx: context.Background(), backendWeb: server.URL, adminPassword: "private-secret", client: server.Client(), buses: map[uint32]*companyGatewayBus{}}
	p := &companyGatewayPeer{g: g, id: 2}
	g.buses[2] = &companyGatewayBus{owner: p, bus: gatewayTestKnown}
	packet := []byte("CMD|2|1|plugin bbs_next_trip_0123456789abcdef take 1002_1234abcd 130%20&%20N30 Mo-Fr%209 1 0")
	if handled, e := p.liveTripRequest(packet); !handled || e != nil {
		t.Fatal(handled, e)
	}
	if requests != 1 {
		t.Fatal(requests)
	}
	// Native duty-ok must be relayed to this player's companion, not inferred
	// from the successful HTTP submission.
	udp, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer udp.Close()
	client, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	g.udp = udp
	p.remote = client.LocalAddr().(*net.UDPAddr)
	handled, e := p.liveTripRequest([]byte("CMD|2|1|duty-ok 130%20&%20N30 Mo-Fr%209 1 0"))
	if handled || e != nil {
		t.Fatal("native acknowledgment was not retained", handled, e)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 512)
	n, _, e := client.ReadFromUDP(buf)
	if e != nil || string(buf[:n]) != "CMD|1|2|plugin bbs_next_trip_0123456789abcdef done 1002_1234abcd" {
		t.Fatal("native confirmation was not relayed", string(buf[:n]), e)
	}
	// A duplicate is suppressed even though UDP can retransmit.
	_, _ = p.liveTripRequest(packet)
	if requests != 1 {
		t.Fatal("duplicate dispatch")
	}
	p.liveRequestAt = time.Time{}
	for _, bad := range []string{
		strings.Replace(string(packet), "CMD|2|", "CMD|3|", 1),
		strings.Replace(string(packet), " 1 0", " 1 0\nclock 0", 1),
		strings.Replace(string(packet), " 1 0", " -1 0", 1),
	} {
		_, _ = p.liveTripRequest([]byte(bad))
	}
	if requests != 1 {
		t.Fatal("foreign player or injected admin command accepted", requests)
	}
}
func TestLiveTripGatewayAcceptsOnlyQueuedNativeAdmin202(t *testing.T) {
	for _, code := range []int{202, 200, 204, 400, 401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()
			udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer udp.Close()
			client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			g := &companyGateway{ctx: context.Background(), backendWeb: server.URL, adminPassword: "private-secret", client: server.Client(), udp: udp, buses: map[uint32]*companyGatewayBus{}}
			p := &companyGatewayPeer{g: g, id: 3, remote: client.LocalAddr().(*net.UDPAddr)}
			g.buses[3] = &companyGatewayBus{owner: p, bus: gatewayTestKnown}
			packet := []byte("CMD|3|1|plugin bbs_next_trip_0123456789abcdef take 1002_1234abcd 4315 SAB01 15 0")
			if handled, err := p.liveTripRequest(packet); !handled || err != nil {
				t.Fatal(handled, err)
			}
			_ = client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			buf := make([]byte, 512)
			n, _, err := client.ReadFromUDP(buf)
			if code == http.StatusAccepted {
				// Queued HTTP acceptance must send neither a rejection nor a done.
				if e, ok := err.(net.Error); !ok || !e.Timeout() {
					t.Fatalf("HTTP 202 was treated as a native result: %q %v", buf[:n], err)
				}
			} else if err != nil || string(buf[:n]) != "CMD|1|3|plugin bbs_next_trip_0123456789abcdef no 1002_1234abcd Native dispatch rejected" {
				t.Fatalf("HTTP %d was not rejected: %q %v", code, buf[:n], err)
			}
		})
	}
}

func TestLiveTripMonitorStableSelectionAndConfirmedHandoff(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "log.txt")
	base := liveTestBlock("1001", "130 & N30", "Mo-Fr 9")
	_ = os.WriteFile(logPath, []byte(base), 0600)
	cfg := Config{Root: root, BCSLog: logPath}
	in := pedepeNativeInvocation{Map: "maps/Test/global.cfg", Bus: gatewayTestKnown, Line: "130 & N30", Tour: "Mo-Fr 9", Trip: "04:05"}
	m, e := newLiveTripMonitor(cfg, in, root, filepath.Join(root, "adapter.log"))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile("testdata/bbs-before.odr")
	native := filepath.Join(root, "native.odr")
	_ = os.WriteFile(native, b, 0600)
	driver, e := prepareDriver(native, root)
	if e != nil {
		t.Fatal(e)
	}
	next := liveTestBlock("1002", "454", "03")
	text := base + "Schicht abschließen: 1001\n" + next
	_ = os.WriteFile(logPath, []byte(text), 0600)
	now := time.Now()
	m.poll(now, driver)
	if m.pending != nil || !driver.EvaluationFrozen {
		t.Fatal("unstable selection applied")
	}
	m.poll(now.Add(2*time.Second), driver)
	if m.pending == nil || m.command.Phase != "prepare" {
		t.Fatal("fresh selection not prepared")
	}
	writeStatus := func(phase string) {
		s := liveTripStatus{Token: m.token, ID: m.command.ID, Phase: phase, Map: m.command.Map, Bus: in.Bus, Line: "454", Tour: "03", Trip: 2, SelectedTrip: 2, Departure: 14700, SignOK: true, Destination: "B"}
		data, _ := json.Marshal(s)
		_ = os.WriteFile(filepath.Join(m.dataDir, "status.json"), data, 0600)
	}
	writeStatus("prepared")
	m.poll(now.Add(3*time.Second), driver)
	m.poll(now.Add(4*time.Second), driver)
	if m.command.Phase != "apply" || !driver.EvaluationFrozen {
		t.Fatal("applied before saved baseline was stable")
	}
	writeStatus("applied")
	m.poll(now.Add(5*time.Second), driver)
	if m.current.ID != "1002" || m.pending != nil || driver.EvaluationFrozen {
		t.Fatal("confirmed handoff failed")
	}
	// Regression: after the confirmed handoff the old completed evaluation
	// must stop being frozen. BCS's finish check requires real new counters.
	oldRecord, err := parseDriver(b, false)
	if err != nil {
		t.Fatal(err)
	}
	progress := oldRecord
	progress.Stops[0] += 18
	progress.Hectom += 47
	progress.Tickets += 12
	if err := os.WriteFile(driver.OpenPath, progress.encode(true), 0600); err != nil {
		t.Fatal(err)
	}
	_, _ = driver.poll()
	changed, err := driver.poll()
	if err != nil || !changed {
		t.Fatalf("next-trip evaluation was not published: %t %v", changed, err)
	}
	newBytes, err := os.ReadFile(native)
	if err != nil {
		t.Fatal(err)
	}
	newRecord, err := parseDriver(newBytes, false)
	if err != nil || newRecord.Stops[0] != oldRecord.Stops[0]+18 || newRecord.Hectom != oldRecord.Hectom+47 || newRecord.Tickets != oldRecord.Tickets+12 {
		t.Fatalf("BCS would still read the previous trip's counters: %+v %v", newRecord, err)
	}
	// Historical blocks cannot reopen the accepted transaction.
	m.poll(now.Add(6*time.Second), driver)
	if m.pending != nil {
		t.Fatal("transaction replay")
	}
}

func TestLiveTripPatchKeepsBaseIntegrityChecks(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"Setup.exe": []byte("old setup"), "HostAgent.exe": []byte("old host"), "PeDePeAdapter/openomsi.exe": []byte("old adapter"), "app/OpenOMSI_BCS_Bridge.exe": []byte("bridge"), "app/compat/Omsi.exe": []byte("facade"), "app/compat/omsi-plugin-host32.exe": []byte("helper")}
	manifest(t, dir, files)
	setup, adapter, host := []byte("new setup"), []byte("new adapter"), []byte("new host")
	_ = os.WriteFile(filepath.Join(dir, "Setup.exe"), setup, 0600)
	_ = os.WriteFile(filepath.Join(dir, "PeDePeAdapter/openomsi.exe"), adapter, 0600)
	player := fmt.Sprintf("%x  Setup.exe\n%x  PeDePeAdapter/openomsi.exe\n", sha256.Sum256(setup), sha256.Sum256(adapter))
	_ = os.WriteFile(filepath.Join(dir, "docs/PATCH-PLAYER-SHA256.txt"), []byte(player), 0600)
	if n, e := verifyPackage(dir); e != nil || n != 6 {
		t.Fatal(n, e)
	}
	_ = os.WriteFile(filepath.Join(dir, "HostAgent.exe"), host, 0600)
	hostPatch := fmt.Sprintf("%x  Setup.exe\n%x  HostAgent.exe\n", sha256.Sum256(setup), sha256.Sum256(host))
	_ = os.WriteFile(filepath.Join(dir, "docs/PATCH-HOST-SHA256.txt"), []byte(hostPatch), 0600)
	if _, e := verifyPackage(dir); e != nil {
		t.Fatal(e)
	}
	// An unchanged component is still verified against the original manifest.
	_ = os.WriteFile(filepath.Join(dir, "app/compat/Omsi.exe"), []byte("tampered"), 0600)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("base integrity checks were bypassed")
	}
	_ = os.WriteFile(filepath.Join(dir, "app/compat/Omsi.exe"), files["app/compat/Omsi.exe"], 0600)
	_ = os.WriteFile(filepath.Join(dir, "PeDePeAdapter/openomsi.exe"), []byte("tampered"), 0600)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("updated adapter was not verified")
	}
	_ = os.WriteFile(filepath.Join(dir, "docs/PATCH-HOST-SHA256.txt"), []byte(fmt.Sprintf("%x  Setup.exe\n", sha256.Sum256([]byte("different setup")))), 0600)
	if _, e := verifyPackage(dir); e == nil {
		t.Fatal("conflicting patch pair accepted")
	}
}
