package main

// The simulator resolves map/Chrono/timetable formats. This code reads only the
// BCS selection and confirms the resulting duty through the public plugin API.
import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed bcs_trip_guard.lua
var bcsTripGuardSource string

type bcsTripSelection struct {
	Token, ID, Language, Map, Bus string
	Line, Tour, Origin, Terminus  string
	Route, StartPoint             string
	Start, End                    int
}

var bcsShiftPattern = regexp.MustCompile(`(?m)\bSchicht ID:[ \t]*([1-9][0-9]*)[ \t]*\r?$`)
var bcsTourPattern = regexp.MustCompile(`Tour:\s*([0-9]{1,2}:[0-9]{2})(?::[0-9]{2})?\s*-\s*([0-9]{1,2}:[0-9]{2})(?::[0-9]{2})?\s*\(Linie:\s*(.*?)\s*\)`)
var bcsRunPattern = regexp.MustCompile(`\(Umlauf:\s*(.*?)\s*\)\s*$`)

func bcsMinute(s string) (int, bool) {
	p := strings.Split(s, ":")
	if len(p) != 2 {
		return 0, false
	}
	h, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	return h*60 + m, e1 == nil && e2 == nil && h >= 0 && h < 72 && m >= 0 && m < 60
}

func parseBCSTripSelection(text string, in pedepeNativeInvocation) (bcsTripSelection, error) {
	var s bcsTripSelection
	ids := bcsShiftPattern.FindAllStringSubmatchIndex(text, -1)
	if len(ids) == 0 {
		return s, fmt.Errorf("missing BCS shift")
	}
	last := ids[len(ids)-1]
	s.ID = text[last[2]:last[3]]
	block := text[last[0]:]
	if strings.Contains(block, "Schicht abschließen: "+s.ID) || strings.Contains(block, "Schicht abschliessen: "+s.ID) {
		return s, fmt.Errorf("BCS shift is already closed")
	}
	tour := bcsTourPattern.FindStringSubmatch(block)
	if len(tour) == 0 {
		return s, fmt.Errorf("incomplete latest BCS shift")
	}
	var ok bool
	if s.Start, ok = bcsMinute(tour[1]); !ok {
		return s, fmt.Errorf("invalid BCS departure")
	}
	if s.End, ok = bcsMinute(tour[2]); !ok {
		return s, fmt.Errorf("invalid BCS arrival")
	}
	s.Line = strings.TrimSpace(tour[3])
	for _, line := range strings.Split(block, "\n") {
		if i := strings.Index(line, "Startpunkt:"); i >= 0 {
			s.StartPoint = strings.TrimSpace(line[i+len("Startpunkt:"):])
		}
		if m := bcsRunPattern.FindStringSubmatch(line); len(m) > 0 {
			s.Tour = strings.TrimSpace(m[1])
			route := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), m[0]))
			s.Route = route
			if i := strings.Index(route, " - "); i >= 0 {
				s.Origin = strings.TrimSpace(route[:i])
			}
			if i := strings.LastIndex(route, " - "); i >= 0 {
				s.Terminus = strings.TrimSpace(route[i+3:])
			}
		}
	}
	if s.Tour == "" || s.Route == "" || s.StartPoint == "" {
		return s, fmt.Errorf("incomplete BCS route")
	}
	if !strings.EqualFold(s.Line, in.Line) || !strings.EqualFold(s.Tour, in.Tour) {
		return s, fmt.Errorf("BCS selection changed")
	}
	s.Map, s.Bus = in.Map, in.Bus
	return s, nil
}

func readBCSSelectionLog(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("invalid BCS log")
	}
	var bom [2]byte
	_, _ = f.ReadAt(bom[:], 0)
	const limit = int64(4 << 20)
	start := st.Size() - limit
	if start < 0 {
		start = 0
	}
	if bom == [2]byte{0xff, 0xfe} || bom == [2]byte{0xfe, 0xff} {
		start -= start % 2
	}
	if _, err = f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+4))
	if err != nil {
		return "", err
	}
	if start > 0 && (bom == [2]byte{0xff, 0xfe} || bom == [2]byte{0xfe, 0xff}) {
		b = append(bom[:], b...)
	}
	return decodeText(b), nil
}

func findBCSTripSelection(root string, in pedepeNativeInvocation) (bcsTripSelection, error) {
	var newest string
	var newestAt time.Time
	for _, folder := range []string{"Busbetrieb-Simulator", "Bus Company Simulator", "Busbetriebs-Simulator", "BBS", ""} {
		path := filepath.Join(root, folder, "log.txt")
		st, err := os.Stat(path)
		if err == nil && st.Mode().IsRegular() && st.ModTime().After(newestAt) {
			text, e := readBCSSelectionLog(path)
			if e == nil && bcsShiftPattern.MatchString(text) {
				newest, newestAt = path, st.ModTime()
			}
		}
	}
	if newest == "" || time.Since(newestAt) > 20*time.Minute {
		return bcsTripSelection{}, fmt.Errorf("current BCS log unavailable")
	}
	text, err := readBCSSelectionLog(newest)
	if err != nil {
		return bcsTripSelection{}, err
	}
	return parseBCSTripSelection(text, in)
}

type bcsTripGuard struct {
	selection            bcsTripSelection
	plugin, data, status string
	confirmed            atomic.Bool
	stop, done           chan struct{}
	failure              error
}

type bcsTripState struct {
	Token, ID, State, Error, Line, Tour, TripName string
	Departure                                     float64
	Trip                                          int
}

type tripSelectionError struct{ reason string }

func (e tripSelectionError) Error() string { return e.reason }

func prepareBCSTripGuard(cfg localConfig, args []string, dir string) (*bcsTripGuard, []string, error) {
	in := parsePeDePeNativeInvocation(args)
	selection, err := findBCSTripSelection(cfg.Root, in)
	if err != nil {
		log.Printf("BCS trip selection: %v", err)
		return nil, nil, tripSelectionError{tr("bcstripselection")}
	}
	selection.Token, selection.Language = randomID()[:16], language()
	selection.ID += "_" + randomID()[:8]
	name := "bbs_next_trip_" + selection.Token
	folder := filepath.Join(contentFolder(cfg), "Plugins")
	if err = os.MkdirAll(folder, 0700); err != nil {
		return nil, nil, err
	}
	// A previous process may have been killed before cleanup. Remove only
	// companions carrying our exact source marker, never user/vendor plugins.
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		stem := strings.TrimSuffix(entry.Name(), ".lua")
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".lua") && livePluginName.MatchString(stem) {
			path := filepath.Join(folder, entry.Name())
			old, e := readSmallFile(path, 256<<10)
			if e == nil && strings.HasPrefix(string(old), "-- Per-launch Bridge companion. PeDePe's bbs.lua remains unchanged.\n") {
				_ = os.Remove(path)
				_ = os.RemoveAll(filepath.Join(folder, stem+".data"))
			}
		}
	}
	g := &bcsTripGuard{selection: selection, plugin: filepath.Join(folder, name+".lua"), data: filepath.Join(folder, name+".data"), stop: make(chan struct{}), done: make(chan struct{})}
	g.status = filepath.Join(g.data, "status.json")
	if err = os.Mkdir(g.data, 0700); err != nil {
		return nil, nil, err
	}
	b, _ := json.Marshal(selection)
	quoted, _ := json.Marshal(string(b))
	source := strings.Replace(bcsTripGuardSource, "__BCS_SELECTION_JSON__", string(quoted), 1)
	if err = writeAtomic(g.plugin, []byte(source), 0600); err != nil {
		g.cleanup()
		return nil, nil, err
	}
	// Resolve the complete tour through openOMSI, then pick the BCS trip.
	out := replaceOption(args, "--trip", "1")
	if !pedepeHasArg(out, "--whole-tour") {
		out = append(out, "--whole-tour")
	}
	log.Printf("BCS selected duty: line=%q tour=%q departure=%02d:%02d route=%q -> %q; original trip=%q", selection.Line, selection.Tour, selection.Start/60, selection.Start%60, selection.Origin, selection.Terminus, in.Trip)
	return g, out, nil
}

func (g *bcsTripGuard) cleanup() { _ = os.Remove(g.plugin); _ = os.RemoveAll(g.data) }

func launchBCSNative(ctx context.Context, cfg localConfig, args []string, dir string) (*nativeProcess, error) {
	in := parsePeDePeNativeInvocation(args)
	if !in.HasSchedule || in.Line == "" || in.Tour == "" || in.Trip == "" || in.Probe || in.Server {
		return launchNative(cfg, args, dir)
	}
	g, resolved, err := prepareBCSTripGuard(cfg, args, dir)
	if err != nil {
		return nil, err
	}
	p, err := launchNativeProtected(cfg, resolved, dir, g)
	if err != nil {
		g.cleanup()
		return nil, err
	}
	go g.watch(ctx, p)
	return p, nil
}

func (g *bcsTripGuard) watch(ctx context.Context, p *nativeProcess) {
	defer close(g.done)
	deadline := time.Now().Add(15 * time.Minute)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-ctx.Done():
			g.failure = ctx.Err()
			_ = p.Process.Kill()
			return
		case <-tick.C:
			b, err := readSmallFile(g.status, 16<<10)
			var s bcsTripState
			if err == nil && json.Unmarshal(b, &s) == nil && s.Token == g.selection.Token && s.ID == g.selection.ID {
				minute := math.Mod(math.Floor(s.Departure/60), 1440)
				if minute < 0 {
					minute += 1440
				}
				if s.State == "ready" && s.Trip > 0 && strings.EqualFold(s.Line, g.selection.Line) && strings.EqualFold(s.Tour, g.selection.Tour) && minute == float64(g.selection.Start%1440) {
					g.confirmed.Store(true)
					log.Printf("BCS trip confirmed: line=%q tour=%q trip=%d name=%q departure=%.1f", s.Line, s.Tour, s.Trip, s.TripName, s.Departure)
					return
				}
				if s.State == "error" {
					log.Printf("BCS trip verification failed: %s", s.Error)
					g.failure = tripSelectionError{tr("bcstrip" + s.Error)}
					_ = p.Process.Kill()
					return
				}
			}
			if time.Now().After(deadline) {
				g.failure = tripSelectionError{tr("bcstriptimeout")}
				_ = p.Process.Kill()
				return
			}
		}
	}
}

// BCS watches a literal spawn marker in game.log. Keep the native diagnostics,
// but release that marker to BCS only after the real duty has been confirmed.
type bcsTripLoadingLog struct {
	output  io.Writer
	partial string
}

func (w *bcsTripLoadingLog) Write(b []byte) (int, error) {
	w.partial += string(b)
	for {
		i := strings.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}
		line := w.partial[:i+1]
		w.partial = w.partial[i+1:]
		if !strings.Contains(line, "[BBS trip confirmed]") {
			line = strings.ReplaceAll(line, "] spawned at", "] [BCS checking duty] vehicle placed at")
		}
		if _, err := io.WriteString(w.output, line); err != nil {
			return 0, err
		}
	}
	if len(w.partial) > 64<<10 {
		if _, err := io.WriteString(w.output, strings.ReplaceAll(w.partial, "] spawned at", "] [BCS checking duty] vehicle placed at")); err != nil {
			return 0, err
		}
		w.partial = ""
	}
	return len(b), nil
}
