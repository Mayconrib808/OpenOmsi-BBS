package main

// A connected player can request only its own duty. The private native admin
// endpoint remains on loopback and its password is never sent to a player.
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var livePluginName = regexp.MustCompile(`^bbs_next_trip_[a-f0-9]{16}$`)
var liveRequestID = regexp.MustCompile(`^[0-9]+_[a-f0-9]{8}$`)

func liveDutyArgument(line, tour string, trip, stop int) (string, error) {
	if !companyText(line, 64) || !companyText(tour, 64) || strings.ContainsAny(line+tour, "|\r\n\t") || trip < 0 || trip > 10000 || stop < 0 || stop > 10000 {
		return "", fmt.Errorf("invalid duty")
	}
	esc := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "%", "%25"), " ", "%20") }
	return fmt.Sprintf("%s %s %d %d", esc(line), esc(tour), trip, stop), nil
}

// Returns true only for this companion's messages addressed to the host.
func (p *companyGatewayPeer) liveTripRequest(packet []byte) (bool, error) {
	parts := strings.SplitN(string(packet), "|", 4)
	if len(parts) != 4 || parts[0] != "CMD" || parts[2] != "1" {
		return false, nil
	}
	// Relay the native client's actual dispatch acknowledgment to the same
	// companion, while keeping the original acknowledgment for the host.
	if strings.HasPrefix(parts[3], "duty-ok ") || strings.HasPrefix(parts[3], "duty-no ") {
		p.g.mu.Lock()
		id, request, plugin := p.id, p.liveRequest, p.livePlugin
		owned := id > 1 && p.g.buses[id] != nil && p.g.buses[id].owner == p
		p.g.mu.Unlock()
		v := strings.SplitN(request, " ", 2)
		if owned && companyGatewayID(parts[1]) == id && len(v) == 2 && livePluginName.MatchString(plugin) {
			message := ""
			if parts[3] == "duty-ok "+v[1] {
				message = "done " + v[0]
			}
			if strings.HasPrefix(parts[3], "duty-no ") {
				message = "no " + v[0] + " Native client rejected the duty"
			}
			if message != "" {
				_ = p.sendClient([]byte(fmt.Sprintf("CMD|1|%d|plugin %s %s", id, plugin, message)))
			}
		}
		return false, nil
	}
	words := strings.Fields(parts[3])
	if len(words) < 3 || words[0] != "plugin" || !livePluginName.MatchString(words[1]) {
		return false, nil
	}
	p.g.mu.Lock()
	id, password := p.id, p.g.adminPassword
	owned := id > 1 && p.g.buses[id] != nil && p.g.buses[id].owner == p
	p.g.mu.Unlock()
	// Never trust the ID supplied in a datagram to select another player.
	if !owned || companyGatewayID(parts[1]) != id {
		return true, nil
	}
	reply := func(text string) error {
		return p.sendClient([]byte(fmt.Sprintf("CMD|1|%d|plugin %s %s", id, words[1], text)))
	}
	if len(words) == 3 && words[2] == "hello" {
		if password == "" {
			return true, nil
		}
		return true, reply("ready")
	}
	if len(words) != 8 || words[2] != "take" || !liveRequestID.MatchString(words[3]) || len(strings.TrimPrefix(parts[3], "plugin "+words[1]+" ")) > 120 {
		return true, nil
	}
	if password == "" {
		return true, reply("no " + words[3] + " Host update required")
	}
	unesc := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "%20", " "), "%25", "%") }
	trip, e1 := strconv.Atoi(words[6])
	stop, e2 := strconv.Atoi(words[7])
	arg, err := liveDutyArgument(unesc(words[4]), unesc(words[5]), trip, stop)
	if e1 != nil || e2 != nil || err != nil {
		return true, reply("no " + words[3] + " Invalid duty")
	}
	request := words[3] + " " + arg
	p.g.mu.Lock()
	if p.liveRequest != "" && strings.HasPrefix(p.liveRequest, words[3]+" ") && p.liveRequest != request {
		p.g.mu.Unlock()
		return true, reply("no " + words[3] + " Request identity conflict")
	}
	if time.Since(p.liveRequestAt) < 2*time.Second {
		p.g.mu.Unlock()
		return true, nil
	}
	p.liveRequest, p.liveRequestAt = request, time.Now()
	p.livePlugin = words[1]
	p.g.mu.Unlock()
	ctx, cancel := context.WithTimeout(p.g.ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.g.backendWeb+"/admin", strings.NewReader(fmt.Sprintf("duty %d %s\n", id, arg)))
	if err != nil {
		return true, err
	}
	req.Header.Set("X-Admin-Password", password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := p.g.client.Do(req)
	if err != nil {
		return true, reply("no " + words[3] + " Native dispatch unavailable")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusAccepted {
		return true, reply("no " + words[3] + " Native dispatch rejected")
	}
	// HTTP acceptance is not a timetable confirmation. The companion reads
	// duty.get() and the actual destination before declaring the transition done.
	return true, nil
}
