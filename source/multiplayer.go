package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MultiplayerTrip struct{ MapName, MapFile, BusFile, Date, Start string }

type MultiplayerPlan struct {
	CompanyID, CompanyName, PlayerName string
	Session                            CompanySession
	Trip                               MultiplayerTrip
	Fleet                              map[string]bool
}

type companyServerStatus struct {
	Name       string          `json:"name"`
	Map        string          `json:"map"`
	Version    string          `json:"version"`
	Protocol   int             `json:"protocol"`
	Time       string          `json:"time"`
	Players    int             `json:"players"`
	MaxPlayers int             `json:"max_players"`
	Password   bool            `json:"password"`
	Vehicles   json.RawMessage `json:"vehicles"`
}

func multiplayerClock(s string) (float64, error) {
	p := strings.Split(s, ":")
	if len(p) < 2 || len(p) > 3 {
		return 0, fmt.Errorf("invalid clock: %s", s)
	}
	h, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	sec := 0.0
	var e3 error
	if len(p) == 3 {
		sec, e3 = strconv.ParseFloat(p[2], 64)
	}
	if e1 != nil || e2 != nil || e3 != nil || h < 0 || h > 23 || m < 0 || m > 59 || math.IsNaN(sec) || math.IsInf(sec, 0) || sec < 0 || sec >= 60 {
		return 0, fmt.Errorf("invalid clock: %s", s)
	}
	return float64(h*3600+m*60) + sec, nil
}

func companyVehicleList(raw json.RawMessage) ([]string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		var result []string
		for _, s := range strings.Split(text, ";") {
			if strings.TrimSpace(s) != "" {
				result = append(result, s)
			}
		}
		return result, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("server did not provide its vehicle list")
	}
	return list, nil
}

func readCompanyServer(ctx context.Context, client *http.Client, session CompanySession) (companyServerStatus, error) {
	var status companyServerStatus
	b, err := companyReadHTTP(ctx, client, strings.TrimRight(session.ServerURL, "/")+"/status", 128<<10)
	if err != nil {
		return status, err
	}
	if err = json.Unmarshal(b, &status); err != nil {
		return status, fmt.Errorf("server status is not valid JSON: %w", err)
	}
	return status, nil
}

func validateCompanyServer(status companyServerStatus, session CompanySession, trip MultiplayerTrip) error {
	version := strings.Fields(status.Version)
	if len(version) == 0 || version[0] != multiplayerGameVersion || status.Protocol != multiplayerProtocol {
		return fmt.Errorf("openOMSI/protocol incompatible: %s / %d; expected %s / %d", status.Version, status.Protocol, multiplayerGameVersion, multiplayerProtocol)
	}
	if !companyText(status.Name, 120) || companyAssetKey(status.Map) != companyAssetKey(session.MapFile) {
		return fmt.Errorf("server map differs from the company profile")
	}
	if status.Password {
		return fmt.Errorf("password-protected sessions are not supported by this development build")
	}
	if status.Players < 0 || status.MaxPlayers <= 0 || status.MaxPlayers > 128 || status.Players >= status.MaxPlayers {
		return fmt.Errorf("session is full or did not provide a valid player limit")
	}
	serverTime, err := multiplayerClock(status.Time)
	if err != nil {
		return err
	}
	startTime, err := multiplayerClock(trip.Start)
	if err != nil {
		return err
	}
	if math.Abs(serverTime-startTime) > float64(session.ClockToleranceSec) {
		return fmt.Errorf("horário incompatível / incompatible clock: sessão %s, BBS %s (limite %d s)", status.Time, trip.Start, session.ClockToleranceSec)
	}
	buses, err := companyVehicleList(status.Vehicles)
	if err != nil {
		return err
	}
	if trip.BusFile != "" {
		allowed := false
		for _, bus := range buses {
			allowed = allowed || companyAssetKey(bus) == companyAssetKey(trip.BusFile)
		}
		if !allowed {
			return fmt.Errorf("BBS bus is not offered by this server: %s", trip.BusFile)
		}
	}
	return nil
}

func companyRequiredFleet(p CompanyProfile, session CompanySession) map[string]bool {
	covered := map[string]bool{}
	for _, id := range session.RequiredPackages {
		for _, pkg := range p.Packages {
			if pkg.ID != id {
				continue
			}
			for _, file := range pkg.Files {
				key := companyAssetKey(file.Path)
				if strings.HasPrefix(key, "vehicles/") && strings.HasSuffix(key, ".bus") {
					covered[key] = true
				}
			}
		}
	}
	return covered
}

func validateCompanyFleet(status companyServerStatus, covered map[string]bool) error {
	buses, err := companyVehicleList(status.Vehicles)
	if err != nil {
		return err
	}
	if len(buses) == 0 {
		return fmt.Errorf("server has not published its fleet yet")
	}
	for _, bus := range buses {
		if !covered[companyAssetKey(bus)] {
			return fmt.Errorf("server offers an undeclared bus: %s; administrator must include its package or restrict server.cfg vehicles", bus)
		}
	}
	return nil
}

// Pick among explicitly registered company sessions, in profile order. No BBS
// credentials, company-membership API, world-clock mutation or server creation.
func prepareMultiplayer(ctx context.Context, c Config, runtimeDir string, trip MultiplayerTrip, client *http.Client) (*MultiplayerPlan, []CompanyProblem, error) {
	if !c.Multiplayer {
		return nil, nil, nil
	}
	if c.CompanyProfile == "" || c.CompanyID == "" || !companyText(c.PlayerName, 32) || strings.Contains(c.PlayerName, "|") {
		return nil, nil, fmt.Errorf("configure the company and player name in Setup option 8")
	}
	p, err := loadCompanyProfile(ctx, c.CompanyProfile, runtimeDir, client)
	if err != nil {
		return nil, nil, err
	}
	if p.CompanyID != c.CompanyID {
		return nil, nil, fmt.Errorf("the company profile identity changed; configure the company again in Setup option 8")
	}
	if _, err = time.Parse("2006-01-02", trip.Date); err != nil {
		return nil, nil, fmt.Errorf("BBS trip date is not known; configure date=YYYY-MM-DD")
	}
	if _, err = multiplayerClock(trip.Start); err != nil {
		return nil, nil, err
	}
	var candidates []CompanySession
	for _, session := range p.Sessions {
		matches := strings.EqualFold(strings.TrimSpace(session.MapName), strings.TrimSpace(trip.MapName))
		if trip.MapFile != "" {
			matches = companyAssetKey(session.MapFile) == companyAssetKey(trip.MapFile)
		}
		if matches && session.Date == trip.Date {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return nil, []CompanyProblem{{p.CompanyName, "Nenhuma sessão cadastrada para este mapa e esta data. / No registered session for this map and date: " + trip.MapName + " / " + trip.Date, ""}}, nil
	}
	// Bounded concurrent status reads keep an unavailable first room from
	// hiding a working one and avoid a long wait before BBS sees its facade.
	type answer struct {
		status companyServerStatus
		err    error
	}
	answers := make([]answer, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, session := range candidates {
		wg.Add(1)
		go func(i int, session CompanySession) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				answers[i].err = ctx.Err()
				return
			}
			defer func() { <-sem }()
			answers[i].status, answers[i].err = readCompanyServer(ctx, client, session)
			if answers[i].err == nil {
				answers[i].err = validateCompanyServer(answers[i].status, session, trip)
			}
		}(i, session)
	}
	wg.Wait()
	var unavailable, requirements []CompanyProblem
	for i, session := range candidates {
		problems := checkCompanyPackages(c.Root, p, session)
		if len(problems) != 0 && len(requirements) == 0 {
			requirements = problems
		}
		if answers[i].err != nil {
			unavailable = append(unavailable, CompanyProblem{session.Name, answers[i].err.Error(), ""})
			continue
		}
		if len(problems) != 0 {
			continue
		}
		fleet := companyRequiredFleet(p, session)
		if fleetErr := validateCompanyFleet(answers[i].status, fleet); fleetErr != nil {
			unavailable = append(unavailable, CompanyProblem{session.Name, fleetErr.Error(), ""})
			continue
		}
		covered := false
		for _, pkg := range p.Packages {
			for _, id := range session.RequiredPackages {
				if pkg.ID == id {
					for _, f := range pkg.Files {
						covered = covered || companyAssetKey(f.Path) == companyAssetKey(trip.BusFile)
					}
				}
			}
		}
		if !covered {
			unavailable = append(unavailable, CompanyProblem{session.Name, "O ônibus escolhido no BBS não está coberto pelos pacotes da sessão. / The BBS bus must be included in the session's hashed packages: " + trip.BusFile, ""})
			continue
		}
		return &MultiplayerPlan{CompanyID: p.CompanyID, CompanyName: p.CompanyName, PlayerName: c.PlayerName, Session: session, Trip: trip, Fleet: fleet}, nil, nil
	}
	if len(requirements) != 0 {
		return nil, append(requirements, unavailable...), nil
	}
	return nil, unavailable, nil
}

func recheckMultiplayer(ctx context.Context, plan *MultiplayerPlan, client *http.Client) error {
	if plan == nil {
		return nil
	}
	status, err := readCompanyServer(ctx, client, plan.Session)
	if err != nil {
		return err
	}
	if err = validateCompanyServer(status, plan.Session, plan.Trip); err != nil {
		return err
	}
	return validateCompanyFleet(status, plan.Fleet)
}

func multiplayerArguments(plan *MultiplayerPlan) []string {
	if plan == nil {
		return nil
	}
	return []string{"--lan-join", strings.TrimRight(plan.Session.ServerURL, "/"), "--lan-name", plan.PlayerName}
}

func multiplayerEnvironment(base []string, contentDir string) []string {
	var result []string
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "OMSI_CONTENT") && !strings.EqualFold(key, "OMSI_CONTENT_ZIP") && !strings.EqualFold(key, "OMSI_NO_LAN_MODS") {
			result = append(result, entry)
		}
	}
	// Exact checked original-installation assets + the bridge's timetable ZIP.
	// No automatic peer-to-peer mod transfer or unexamined content overlays.
	return append(result, "OMSI_CONTENT="+contentDir, "OMSI_NO_LAN_MODS=1")
}

var multiplayerWorldLine = regexp.MustCompile(`LAN: taking the host's world: (\d{4}-\d{2}-\d{2}) (\d{1,2}:\d{2}:\d{2}(?:\.\d+)?) weather `)

// The HTTP status omits date. Confirm the actual joined world's date and clock
// from this launch's log before publishing the BBS-ready flag. A successful
// /status response alone does not prove that openOMSI connected.
func checkMultiplayerLog(text string, plan *MultiplayerPlan) (bool, error) {
	for _, failure := range []string{"LAN: cannot join", "LAN: cannot reach", "LAN: turned away:", "LAN: disconnected from the server:", "LAN: the session is on the host's map"} {
		if at := strings.Index(text, failure); at >= 0 {
			line := strings.SplitN(text[at:], "\n", 2)[0]
			return false, fmt.Errorf("multiplayer connection failed: %s", strings.TrimSpace(line))
		}
	}
	m := multiplayerWorldLine.FindStringSubmatch(text)
	if len(m) == 0 {
		return false, nil
	}
	if m[1] != plan.Trip.Date {
		return false, fmt.Errorf("host date %s differs from BBS trip date %s", m[1], plan.Trip.Date)
	}
	joined, err := multiplayerClock(m[2])
	if err != nil {
		return false, err
	}
	start, err := multiplayerClock(plan.Trip.Start)
	if err != nil {
		return false, err
	}
	if math.Abs(joined-start) > float64(plan.Session.ClockToleranceSec) {
		return false, fmt.Errorf("joined host clock %s differs from BBS departure %s", m[2], plan.Trip.Start)
	}
	return true, nil
}

type multiplayerWatch struct {
	Ready  <-chan struct{}
	Errors <-chan error
}

func watchMultiplayerLaunch(logPath string, offset int64, plan *MultiplayerPlan, done <-chan struct{}) multiplayerWatch {
	ready, failures := make(chan struct{}), make(chan error, 1)
	go func() {
		deadline := time.NewTimer(2 * time.Minute)
		defer deadline.Stop()
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-deadline.C:
				failures <- fmt.Errorf("openOMSI did not confirm a compatible multiplayer world within two minutes")
				return
			case <-tick.C:
				b, err := os.ReadFile(logPath)
				if err != nil || offset < 0 || int64(len(b)) < offset {
					continue
				}
				// A logger may still be writing the last line; only complete lines
				// can confirm the world or report an authoritative failure.
				chunk := b[offset:]
				last := strings.LastIndexByte(string(chunk), '\n')
				if last < 0 {
					continue
				}
				ok, err := checkMultiplayerLog(string(chunk[:last+1]), plan)
				if err != nil {
					failures <- err
					return
				}
				if ok {
					close(ready)
					return
				}
			}
		}
	}()
	return multiplayerWatch{ready, failures}
}
