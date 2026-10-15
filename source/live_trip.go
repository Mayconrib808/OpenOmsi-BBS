package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed live_trip_plugin.lua
var liveTripPlugin string

type liveBCSTrip struct {
	ID, Map, Line, Tour, Start, End, Route, Hof string
}

var liveShiftRE = regexp.MustCompile(`(?m)\bSchicht ID:[ \t]*([1-9][0-9]*)[ \t]*\r?$`)
var liveClosedRE = regexp.MustCompile(`(?m)\bSchicht abschlie(?:ß|ss)en:[ \t]*([1-9][0-9]*)[ \t]*\r?$`)
var liveTourRE = regexp.MustCompile(`Tour:\s*([0-9]{1,2}:[0-9]{2})(?::[0-9]{2})?\s*-\s*([0-9]{1,2}:[0-9]{2})(?::[0-9]{2})?\s*\(Linie:\s*(.*?)\s*\)`)
var liveUmlaufRE = regexp.MustCompile(`\(Umlauf:\s*(.*?)\s*\)\s*$`)

func liveShiftClosed(text, id string) bool {
	for _, m := range liveClosedRE.FindAllStringSubmatch(text, -1) {
		if m[1] == id {
			return true
		}
	}
	return false
}
func readLiveBCSTrip(text string) (liveBCSTrip, error) {
	var t liveBCSTrip
	ids := liveShiftRE.FindAllStringSubmatchIndex(text, -1)
	if len(ids) == 0 {
		return t, fmt.Errorf("no BCS shift identity")
	}
	m := ids[len(ids)-1]
	t.ID = text[m[2]:m[3]]
	// HOF selection precedes Schicht ID. Keep the most recent selection, but
	// never parse an older complete trip when the newest block is incomplete.
	for _, line := range strings.Split(text[:m[0]], "\n") {
		if i := strings.Index(line, "Zielpfad Hof-Datei:"); i >= 0 {
			t.Hof = strings.TrimSpace(line[i+len("Zielpfad Hof-Datei:"):])
		}
	}
	lines := strings.Split(text[m[0]:], "\n")
	for i, line := range lines {
		if j := strings.Index(line, "Karte:"); j >= 0 {
			t.Map = strings.TrimSpace(line[j+6:])
		}
		if v := liveTourRE.FindStringSubmatch(line); len(v) == 4 {
			t.Start, t.End, t.Line = v[1], v[2], strings.TrimSpace(v[3])
			if i+1 < len(lines) {
				if u := liveUmlaufRE.FindStringSubmatch(strings.TrimSpace(lines[i+1])); len(u) == 2 {
					t.Tour = strings.TrimSpace(u[1])
					t.Route = strings.TrimSpace(liveUmlaufRE.ReplaceAllString(strings.TrimSpace(lines[i+1]), ""))
				}
			}
		}
	}
	if t.Map == "" || t.Line == "" || t.Tour == "" || t.Start == "" || t.End == "" || t.Route == "" || liveShiftClosed(text, t.ID) {
		return t, fmt.Errorf("BCS selection is incomplete or closed")
	}
	return t, nil
}

type liveTripCommand struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Phase    string `json:"phase"`
	Map      string `json:"map"`
	Bus      string `json:"bus"`
	Line     string `json:"line"`
	Tour     string `json:"tour"`
	Minute   int    `json:"minute"`
	Origin   string `json:"origin,omitempty"`
	Terminus string `json:"terminus,omitempty"`
}
type liveTripStatus struct {
	Token        string  `json:"token"`
	ID           string  `json:"id"`
	Phase        string  `json:"phase"`
	Error        string  `json:"error"`
	Map          string  `json:"map"`
	Bus          string  `json:"bus"`
	Line         string  `json:"line"`
	Tour         string  `json:"tour"`
	Trip         int     `json:"trip"`
	SelectedTrip int     `json:"selected_trip"`
	TripName     string  `json:"trip_name"`
	Destination  string  `json:"destination"`
	SignOK       bool    `json:"sign_ok"`
	Departure    float64 `json:"departure"`
}
type liveTripMonitor struct {
	cfg                              Config
	in                               pedepeNativeInvocation
	logPath, bcsPath, dataDir, token string
	current                          liveBCSTrip
	historical                       map[string]bool
	closed                           bool
	candidate                        liveBCSTrip
	stableAt                         time.Time
	pending                          *liveBCSTrip
	command                          liveTripCommand
	deadline                         time.Time
	preparedBytes                    []byte
	openBase, nativeBase             driverRecord
	nativeHash                       [32]byte
	blocked                          string
}

func findLiveBCSLog(c Config) string {
	if c.BCSLog != "" && !strings.EqualFold(c.BCSLog, "auto") {
		return c.BCSLog
	}
	for _, dir := range []string{"Busbetrieb-Simulator", "Bus Company Simulator", "Busbetriebs-Simulator", "BBS", ""} {
		p := filepath.Join(c.Root, dir, "log.txt")
		text, e := readLiveTripLog(p)
		if e == nil && liveShiftRE.MatchString(text) {
			return p
		}
	}
	return ""
}

func readLiveTripLog(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return "", e
	}
	const limit int64 = 2 << 20
	head := make([]byte, 2)
	n, _ := f.ReadAt(head, 0)
	utf16 := n == 2 && ((head[0] == 0xff && head[1] == 0xfe) || (head[0] == 0xfe && head[1] == 0xff))
	start := st.Size() - limit
	if start < 0 {
		start = 0
	}
	if utf16 && start > 0 {
		start -= start % 2
	}
	if _, e = f.Seek(start, 0); e != nil {
		return "", e
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+2))
	if e != nil {
		return "", e
	}
	if utf16 && start > 0 {
		b = append(head, b...)
	}
	return decodeText(b), nil
}

func liveDepartureMinute(t float64) int {
	if math.IsNaN(t) || math.IsInf(t, 0) {
		return -1
	}
	return ((int(math.Floor(t/60)) % 1440) + 1440) % 1440
}
func newLiveTripMonitor(cfg Config, in pedepeNativeInvocation, contentDir, logPath string) (*liveTripMonitor, error) {
	p := findLiveBCSLog(cfg)
	text, e := readLiveTripLog(p)
	if e != nil {
		return nil, fmt.Errorf("BCS log unavailable: %w", e)
	}
	t, e := readLiveBCSTrip(text)
	if e != nil {
		return nil, e
	}
	if !strings.EqualFold(t.Line, in.Line) || !strings.EqualFold(t.Tour, in.Tour) {
		return nil, fmt.Errorf("BCS selection differs from the native launch")
	}
	if strings.Contains(in.Trip, ":") && liveClock(t.Start) != liveClock(in.Trip) {
		return nil, fmt.Errorf("BCS departure differs from the native launch")
	}
	secret := make([]byte, 8)
	if _, e = rand.Read(secret); e != nil {
		return nil, e
	}
	token := hex.EncodeToString(secret)
	name := "bbs_next_trip_" + token
	dir := filepath.Join(contentDir, "plugins", name+".data")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	plugin := strings.ReplaceAll(liveTripPlugin, "__BRIDGE_TOKEN__", token)
	language, _ := json.Marshal(cfg.Language)
	plugin = strings.ReplaceAll(plugin, `"__BRIDGE_LANGUAGE__"`, string(language))
	if e = os.WriteFile(filepath.Join(contentDir, "plugins", name+".lua"), []byte(plugin), 0600); e != nil {
		return nil, e
	}
	monitor := &liveTripMonitor{cfg: cfg, in: in, logPath: logPath, bcsPath: p, dataDir: dir, token: token, current: t, historical: map[string]bool{}}
	for _, m := range liveShiftRE.FindAllStringSubmatch(text, -1) {
		monitor.historical[m[1]] = true
	}
	monitor.log("monitor ready for BCS shift " + t.ID + "; private plugin " + name)
	return monitor, nil
}
func liveClock(s string) int {
	p := strings.Split(s, ":")
	if len(p) < 2 {
		return -1
	}
	h, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	if e1 != nil || e2 != nil || h < 0 || h > 47 || m < 0 || m > 59 {
		return -1
	}
	return (h*60 + m) % 1440
}
func (m *liveTripMonitor) log(s string) {
	f, e := os.OpenFile(m.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if e != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s [Next trip] %s\r\n", time.Now().Format(time.RFC3339), s)
}
func (m *liveTripMonitor) writeCommand(phase string) error {
	m.command.Phase = phase
	b, e := json.Marshal(m.command)
	if e != nil {
		return e
	}
	return writeDriverAtomic(filepath.Join(m.dataDir, "command.json"), b)
}
func (m *liveTripMonitor) status() (liveTripStatus, error) {
	var s liveTripStatus
	b, e := os.ReadFile(filepath.Join(m.dataDir, "status.json"))
	if e != nil {
		return s, e
	}
	if len(b) > 16384 {
		return s, fmt.Errorf("invalid companion response")
	}
	e = json.Unmarshal(b, &s)
	if e == nil && s.Token != m.token {
		e = fmt.Errorf("stale companion response")
	}
	return s, e
}
func (m *liveTripMonitor) fail(reason string) {
	if m.pending != nil {
		m.blocked = m.pending.ID
		_ = m.writeCommand("cancel")
		m.log("shift " + m.pending.ID + ": " + reason)
	}
	m.pending = nil
	m.preparedBytes = nil
}
func (m *liveTripMonitor) poll(now time.Time, driver *driverSync) {
	text, e := readLiveTripLog(m.bcsPath)
	if e != nil {
		return
	}
	if !m.closed {
		if !liveShiftClosed(text, m.current.ID) {
			for _, v := range liveShiftRE.FindAllStringSubmatch(text, -1) {
				m.historical[v[1]] = true
			}
			return
		}
		m.closed = true
		for _, v := range liveClosedRE.FindAllStringSubmatchIndex(text, -1) {
			if text[v[2]:v[3]] == m.current.ID {
				for _, old := range liveShiftRE.FindAllStringSubmatch(text[:v[0]], -1) {
					m.historical[old[1]] = true
				}
				break
			}
		}
		// Freeze only after BCS explicitly accepts this exact shift.
		if driver != nil {
			if e = driver.freezeEvaluation(); e != nil {
				m.log("evaluation freeze: " + e.Error())
			}
		}
		m.log("BCS completed shift " + m.current.ID + "; retaining its evaluation")
	}
	t, e := readLiveBCSTrip(text)
	if m.pending != nil {
		if e != nil || t != *m.pending {
			m.fail("selection changed before confirmation")
			return
		}
		if now.After(m.deadline) {
			m.fail("confirmation timed out; current game kept open")
			return
		}
		s, e := m.status()
		if e != nil || s.ID != m.command.ID {
			return
		}
		if s.Phase == "error" {
			m.fail(s.Error)
			return
		}
		if s.Phase == "prepared" && m.command.Phase == "prepare" {
			if driver == nil {
				m.fail("no BCS personnel translator is active")
				return
			}
			open, e := os.ReadFile(driver.OpenPath)
			if e != nil {
				return
			}
			if !bytes.Equal(open, m.preparedBytes) {
				m.preparedBytes = append([]byte(nil), open...)
				return
			}
			origin, e := parseDriver(open, true)
			if e != nil {
				m.fail(e.Error())
				return
			}
			native, e := os.ReadFile(driver.NativePath)
			if e != nil {
				return
			}
			baseline, e := parseDriver(native, false)
			if e != nil {
				m.fail(e.Error())
				return
			}
			if origin.Ident[0] != baseline.Ident[0] {
				m.fail("BCS driver identity changed")
				return
			}
			m.openBase, m.nativeBase, m.nativeHash = origin, baseline, sha256.Sum256(native)
			if e = m.writeCommand("apply"); e != nil {
				m.fail(e.Error())
			}
			return
		}
		if (s.Phase != "applied" && s.Phase != "applied_unverified") || m.command.Phase != "apply" {
			return
		}
		if s.Phase == "applied" && !s.SignOK {
			m.fail("the game reported a confirmed trip without confirming its destination")
			return
		}
		if s.Trip <= 0 || s.Trip != s.SelectedTrip || !strings.EqualFold(s.Line, t.Line) || !strings.EqualFold(s.Tour, t.Tour) || liveDepartureMinute(s.Departure) != m.command.Minute {
			m.fail("the game confirmed a different timetable")
			return
		}
		if !samePath(s.Map, m.command.Map) || companyAssetKey(s.Bus) != companyAssetKey(m.command.Bus) {
			m.fail("the game confirmed a different map/bus")
			return
		}
		if driver != nil {
			driver.beginLiveTrip(m.openBase, m.nativeBase, m.nativeHash)
		}
		m.log(fmt.Sprintf("BCS shift %s -> %s; timetable confirmed: %s / %s / trip %d (%s)", m.current.ID, t.ID, s.Line, s.Tour, s.Trip, s.TripName))
		if s.SignOK {
			m.log("destination confirmed: " + s.Destination)
		} else {
			m.log("destination warning: " + s.Error)
		}
		m.current = t
		m.historical[t.ID] = true
		m.closed = false
		m.pending = nil
		m.preparedBytes = nil
		m.candidate = liveBCSTrip{}
		return
	}
	if e != nil || t.ID == m.current.ID || m.historical[t.ID] || t.ID == m.blocked {
		m.candidate = liveBCSTrip{}
		return
	}
	if t != m.candidate {
		m.candidate = t
		m.stableAt = now
		return
	}
	if now.Sub(m.stableAt) < time.Second {
		return
	}
	if !strings.EqualFold(t.Map, m.current.Map) || (t.Hof != "" && m.current.Hof != "" && !samePath(t.Hof, m.current.Hof)) {
		m.blocked = t.ID
		m.log("map or HOF/vehicle selection changed; a fresh launch is required")
		return
	}
	if m.in.Situation != "" {
		if meta, err := readPeDePeSituation(m.cfg.Root, m.in.Situation); err == nil && meta.Bus != "" && companyAssetKey(normalizePeDePeAsset(m.cfg.Root, meta.Bus)) != companyAssetKey(m.in.Bus) {
			m.blocked = t.ID
			m.log("BCS selected another vehicle; a fresh launch is required")
			return
		}
	}
	mapFile := m.in.Map
	if !filepath.IsAbs(mapFile) {
		mapFile = filepath.Join(m.cfg.Root, filepath.FromSlash(mapFile))
	}
	route := strings.Split(t.Route, " - ")
	origin, terminus := "", ""
	if len(route) == 2 {
		origin, terminus = strings.TrimSpace(route[0]), strings.TrimSpace(route[1])
	}
	idBytes := make([]byte, 4)
	if _, err := rand.Read(idBytes); err != nil {
		m.log(err.Error())
		return
	}
	m.command = liveTripCommand{Token: m.token, ID: t.ID + "_" + hex.EncodeToString(idBytes), Phase: "prepare", Map: mapFile, Bus: m.in.Bus, Line: t.Line, Tour: t.Tour, Minute: liveClock(t.Start), Origin: origin, Terminus: terminus}
	if m.command.Minute < 0 {
		m.blocked = t.ID
		m.log("invalid BCS departure")
		return
	}
	m.pending = &t
	// Native dispatch can take 20 seconds, followed by up to 15 seconds of
	// destination verification. Include preparation and slow frames as well.
	m.deadline = now.Add(60 * time.Second)
	if e = m.writeCommand("prepare"); e != nil {
		m.fail(e.Error())
		return
	}
	m.log(fmt.Sprintf("next BCS shift %s selected: %s / %s / %s; waiting for native preparation", t.ID, t.Line, t.Tour, t.Start))
}
