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
	Clock                              *CompanyClock
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
	World      json.RawMessage `json:"world"`
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
	return validateCompanyServerAt(status, session, trip, nil, time.Now())
}

func validateCompanyServerAt(status companyServerStatus, session CompanySession, trip MultiplayerTrip, clock *CompanyClock, now time.Time) error {
	if !compatibleCompanyServer(status.Version, status.Protocol) {
		return fmt.Errorf("openOMSI server compatibility check failed: %s / protocol %d; this bridge supports network protocol %d", status.Version, status.Protocol, multiplayerProtocol)
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
	if clock != nil && !companyActiveWorld(status.World) {
		return fmt.Errorf("Servidor ainda sem mundo ativo. Para a sessão com relógio da empresa, aguarde SINCRONIZADO no CompanyHost antes de iniciar a viagem. / Company world is not active yet; wait for SINCRONIZADO in CompanyHost before starting the trip")
	}
	serverTime, err := multiplayerClock(status.Time)
	if err != nil {
		return err
	}
	expected := trip.Start
	if clock != nil {
		civil, e := companyNow(*clock, now)
		if e != nil {
			return e
		}
		expected = civil.Format("15:04:05")
	}
	startTime, err := multiplayerClock(expected)
	if err != nil {
		return err
	}
	gap := math.Abs(serverTime - startTime)
	if clock != nil && gap > 43200 {
		gap = 86400 - gap
	}
	if gap > float64(session.ClockToleranceSec) {
		return fmt.Errorf("horário incompatível / incompatible clock: sessão %s, referência %s (limite %d s)", status.Time, expected, session.ClockToleranceSec)
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

func validateCompanyFleet(status companyServerStatus, _ map[string]bool) error {
	buses, err := companyVehicleList(status.Vehicles)
	if err != nil {
		return err
	}
	if len(buses) == 0 {
		return fmt.Errorf("server has not published its fleet yet")
	}
	return nil
}

func prepareMultiplayer(ctx context.Context, c Config, runtimeDir string, trip MultiplayerTrip, client *http.Client) (*MultiplayerPlan, []CompanyProblem, error) {
	return prepareMultiplayerAt(ctx, c, runtimeDir, trip, client, time.Now())
}

func prepareMultiplayerAt(ctx context.Context, c Config, runtimeDir string, trip MultiplayerTrip, client *http.Client, now time.Time) (*MultiplayerPlan, []CompanyProblem, error) {
	if !c.Multiplayer {
		return nil, nil, nil
	}
	if c.CompanyProfile == "" || c.CompanyID == "" || !companyText(c.PlayerName, 32) || strings.Contains(c.PlayerName, "|") {
		return nil, nil, fmt.Errorf("configure the company and player name in Setup option 8")
	}
	p, err := loadInstalledCompanyProfile(ctx, c, runtimeDir, client)
	if err != nil {
		return nil, nil, err
	}
	if p.CompanyID != c.CompanyID {
		return nil, nil, fmt.Errorf("the company profile identity changed; configure the company again in Setup option 8")
	}
	if p.Clock != nil && automaticBridgeDate(c.Date) {
		civil, e := companyNow(*p.Clock, now)
		if e != nil {
			return nil, nil, e
		}
		trip.Date = civil.Format("2006-01-02")
	}
	if _, err = time.Parse("2006-01-02", trip.Date); err != nil {
		return nil, nil, fmt.Errorf("BBS trip date is not known; configure date=YYYY-MM-DD")
	}
	if _, err = multiplayerClock(trip.Start); err != nil {
		return nil, nil, err
	}
	var candidates []CompanySession
	for _, session := range p.Sessions {
		nameMatches := strings.EqualFold(strings.TrimSpace(session.MapName), strings.TrimSpace(trip.MapName))
		fileMatches := trip.MapFile != "" && companyAssetKey(session.MapFile) == companyAssetKey(trip.MapFile)
		matches := nameMatches || fileMatches
		date := session.Date
		if date == "company" {
			civil, e := companyNow(*p.Clock, now)
			if e != nil {
				return nil, nil, e
			}
			date = civil.Format("2006-01-02")
		}
		if matches && date == trip.Date {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return nil, []CompanyProblem{{p.CompanyName, "Nenhuma sessão cadastrada para este mapa e esta data. / No registered session for this map and date: " + trip.MapName + " / " + trip.Date, ""}}, nil
	}
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
				var clock *CompanyClock
				if session.Date == "company" {
					clock = p.Clock
				}
				answers[i].err = validateCompanyServerAt(answers[i].status, session, trip, clock, now)
			}
		}(i, session)
	}
	wg.Wait()
	var unavailable, requirements []CompanyProblem
	bbsBackups := companyBBSBackupInventory(c.Root, candidateBackups(c, candidateBCSLog(c)))
	for i, session := range candidates {
		problems := checkCompanyPackagesWithOriginals(c.Root, p, session, bbsBackups)
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
		var clock *CompanyClock
		if session.Date == "company" {
			clock = p.Clock
		}
		return &MultiplayerPlan{CompanyID: p.CompanyID, CompanyName: p.CompanyName, PlayerName: c.PlayerName, Session: session, Trip: trip, Fleet: fleet, Clock: clock}, nil, nil
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
	if plan.Clock != nil {
		civil, err := companyNow(*plan.Clock, time.Now())
		if err != nil {
			return err
		}
		if civil.Format("2006-01-02") != plan.Trip.Date {
			return fmt.Errorf("company day changed during launch; start the BBS trip again")
		}
	}
	status, err := readCompanyServer(ctx, client, plan.Session)
	if err != nil {
		return err
	}
	if err = validateCompanyServerAt(status, plan.Session, plan.Trip, plan.Clock, time.Now()); err != nil {
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
	return append(result, "OMSI_CONTENT="+contentDir, "OMSI_NO_LAN_MODS=1")
}

var multiplayerWorldLine = regexp.MustCompile(`LAN: taking the host's world: (\d{4}-\d{2}-\d{2}) (\d{1,2}:\d{2}:\d{2}(?:\.\d+)?) weather `)

func checkMultiplayerLog(text string, plan *MultiplayerPlan) (bool, error) {
	return checkMultiplayerLogAt(text, plan, time.Now())
}

func checkMultiplayerLogAt(text string, plan *MultiplayerPlan, now time.Time) (bool, error) {
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
	if plan.Clock != nil {
		if m[1] != plan.Trip.Date {
			return false, fmt.Errorf("host date %s differs from BBS trip date %s; start the BBS trip again if the company day changed", m[1], plan.Trip.Date)
		}
		joined, e := time.Parse("2006-01-02 15:04:05", m[1]+" "+m[2])
		if e != nil {
			return false, e
		}
		expected, e := companyNow(*plan.Clock, now)
		if e != nil {
			return false, e
		}
		if expected.Format("2006-01-02") != plan.Trip.Date {
			return false, fmt.Errorf("company day changed during launch; start the BBS trip again")
		}
		if math.Abs(joined.Sub(expected).Seconds()) > float64(plan.Session.ClockToleranceSec) {
			return false, fmt.Errorf("host world %s differs from company clock %s", joined.Format("2006-01-02 15:04:05"), expected.Format("2006-01-02 15:04:05"))
		}
		return true, nil
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
