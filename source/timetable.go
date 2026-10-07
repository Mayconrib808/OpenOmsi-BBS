package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// TimetableSyncResult describes the temporary, session-only timetable overlay.
// The original map is never written by this code.
type TimetableSyncResult struct {
	Applied           bool
	Ready             bool // Identified trip AND departure are safe for launch.
	Reason            string
	SourceTTL         string
	SourceRelative    string
	TripName          string
	TripIndex         int // 1-based index inside the selected tour, as openOMSI --trip expects.
	OriginalDeparture float64
	BCSCivilDeparture float64
	BCSDeparture      float64
	RuntimeDayAdjustmentMinutes float64
	OffsetMinutes     float64
	OverrideZIP       string
	TourTrips         int
	ChronoSource      string
	CandidateDetails  string
	DiagnosticSources []string
}

type ttlTrip struct {
	Name      string
	Profile   string
	Departure float64
	ValueLine int
	Index     int // 1-based inside tour
}

type ttlTour struct {
	Name      string
	StartLine int
	EndLine   int
	Trips     []ttlTrip
	Invalid   string
}

type chronoScenario struct {
	Dir         string
	Config      string
	Index       int
	Active      bool
	Deactivates []string
	Start, End  *time.Time
}

type textCodec int

const (
	codecRaw textCodec = iota
	codecUTF8BOM
	codecUTF16LE
	codecUTF16BE
)

func detectTextCodec(b []byte) textCodec {
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
		return codecUTF16LE
	}
	if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
		return codecUTF16BE
	}
	if len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		return codecUTF8BOM
	}
	return codecRaw
}

func encodeLikeOriginal(s string, codec textCodec) []byte {
	switch codec {
	case codecUTF16LE, codecUTF16BE:
		u := utf16.Encode([]rune(s))
		out := make([]byte, 2+len(u)*2)
		if codec == codecUTF16LE {
			out[0], out[1] = 0xff, 0xfe
			for i, v := range u {
				binary.LittleEndian.PutUint16(out[2+i*2:], v)
			}
		} else {
			out[0], out[1] = 0xfe, 0xff
			for i, v := range u {
				binary.BigEndian.PutUint16(out[2+i*2:], v)
			}
		}
		return out
	case codecUTF8BOM:
		return append([]byte{0xef, 0xbb, 0xbf}, []byte(s)...)
	default:
		// For ANSI/code-page OMSI files decodeText deliberately returns the raw
		// bytes as a Go string. Converting back therefore preserves every byte we
		// did not modify, including map-specific non-ASCII text.
		return []byte(s)
	}
}

func nextMeaningful(lines []string, start, end int) (string, int, bool) {
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		v := strings.TrimSpace(strings.TrimSuffix(lines[i], "\r"))
		if v == "" {
			continue
		}
		return v, i, true
	}
	return "", -1, false
}

func parseTTL(text string) []ttlTour {
	lines := strings.Split(text, "\n")
	var tours []ttlTour
	for i := 0; i < len(lines); i++ {
		if !strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")), "[newtour]") {
			continue
		}
		name, nameLine, ok := nextMeaningful(lines, i+1, len(lines))
		if !ok {
			continue
		}
		end := len(lines)
		for j := nameLine + 1; j < len(lines); j++ {
			if strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(lines[j], "\r")), "[newtour]") {
				end = j
				break
			}
		}
		t := ttlTour{Name: name, StartLine: i, EndLine: end}
		for j := nameLine + 1; j < end; j++ {
			if !strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(lines[j], "\r")), "[addtrip]") {
				continue
			}
			tripName, n1, ok1 := nextMeaningful(lines, j+1, end)
			profile, n2, ok2 := nextMeaningful(lines, n1+1, end)
			departure, n3, ok3 := nextMeaningful(lines, n2+1, end)
			if !ok1 || !ok2 || !ok3 {
				t.Invalid = "incomplete [addtrip] record"
				continue
			}
			minutes, err := strconv.ParseFloat(strings.ReplaceAll(departure, ",", "."), 64)
			profileIndex, profileErr := strconv.Atoi(profile)
			if err != nil || !validDeparture(minutes) || profileErr != nil || profileIndex < 0 || strings.HasPrefix(tripName, "[") {
				t.Invalid = "invalid trip profile or departure in " + tripName
				continue
			}
			t.Trips = append(t.Trips, ttlTrip{Name: tripName, Profile: profile, Departure: minutes, ValueLine: n3, Index: len(t.Trips) + 1})
			j = n3
		}
		tours = append(tours, t)
		i = end - 1
	}
	return tours
}

func parseClockMinutes(s string) (float64, error) {
	p := strings.Split(strings.TrimSpace(s), ":")
	if len(p) < 2 || len(p) > 3 {
		return 0, fmt.Errorf("invalid clock %q", s)
	}
	h, err1 := strconv.Atoi(p[0])
	m, err2 := strconv.Atoi(p[1])
	sec := 0
	var err3 error
	if len(p) > 2 {
		sec, err3 = strconv.Atoi(p[2])
	}
	if err1 != nil || err2 != nil || err3 != nil || h < 0 || h > 71 || m < 0 || m > 59 || sec < 0 || sec > 59 {
		return 0, fmt.Errorf("invalid clock %q", s)
	}
	return float64(h*60+m) + float64(sec)/60.0, nil
}

func validDeparture(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 72*60
}

func formatMinutesClock(v float64) string {
	// Display only; preserve extended-hour semantics in the numeric .ttl value.
	total := int(math.Round(v))
	h := total / 60
	m := total % 60
	if m < 0 {
		m += 60
		h--
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

func parseRunDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
}

func parseChronoDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return nil, fmt.Errorf("invalid chrono date %q", s)
	}
	t, err := time.ParseInLocation("20060102", s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseChronoCfg(path string, runDate time.Time, idx int) chronoScenario {
	c := chronoScenario{Dir: filepath.Dir(path), Config: path, Index: idx}
	b, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	lines := strings.Split(strings.ReplaceAll(decodeText(b), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		key := strings.ToLower(strings.TrimSpace(lines[i]))
		switch key {
		case "[startdate]":
			if v, _, ok := nextMeaningful(lines, i+1, len(lines)); ok {
				c.Start, _ = parseChronoDate(v)
			}
		case "[enddate]":
			if v, _, ok := nextMeaningful(lines, i+1, len(lines)); ok {
				c.End, _ = parseChronoDate(v)
			}
		case "[deactivate_lines]":
			v, n, ok := nextMeaningful(lines, i+1, len(lines))
			if !ok {
				continue
			}
			count, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || count < 0 || count > 10000 {
				continue
			}
			pos := n + 1
			for len(c.Deactivates) < count && pos < len(lines) {
				line := strings.TrimSpace(lines[pos])
				pos++
				if line == "" {
					continue
				}
				c.Deactivates = append(c.Deactivates, line)
			}
		}
	}
	// OMSI/openOMSI: a chrono without either date is never active; bounds inclusive.
	if c.Start == nil && c.End == nil {
		return c
	}
	c.Active = true
	day := time.Date(runDate.Year(), runDate.Month(), runDate.Day(), 0, 0, 0, 0, runDate.Location())
	if c.Start != nil && day.Before(*c.Start) {
		c.Active = false
	}
	if c.End != nil && day.After(*c.End) {
		c.Active = false
	}
	return c
}

// chronoConfigOrder follows the documented original/openOMSI traversal closely:
// depth first, child folders before a folder's own files, case-insensitive name order.
func chronoConfigOrder(root string) []string {
	var out []string
	var visit func(string)
	visit = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) })
		for _, e := range entries {
			if e.IsDir() {
				visit(filepath.Join(dir, e.Name()))
			}
		}
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(e.Name(), "Chrono.cfg") {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	visit(root)
	return out
}

func exactTTLIn(dir, line string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	want := line + ".ttl"
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), want) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func timetableSourceTTL(root, mapRel, line, runDate string) (string, string, []string, error) {
	mapAbs := filepath.Join(root, filepath.FromSlash(mapRel))
	mapDir := filepath.Dir(mapAbs)
	day, err := parseRunDate(runDate)
	if err != nil {
		return "", "", nil, err
	}
	var active []chronoScenario
	chronoRoot := filepath.Join(mapDir, "Chrono")
	for i, p := range chronoConfigOrder(chronoRoot) {
		c := parseChronoCfg(p, day, i)
		if c.Active {
			active = append(active, c)
		}
	}

	// Keep every loaded TTData folder available for resolving .ttp references,
	// even when the winning .ttl comes from a higher-priority Chrono scenario.
	var sourceDirs []string
	for i := len(active) - 1; i >= 0; i-- {
		sourceDirs = append(sourceDirs, filepath.Join(active[i].Dir, "TTData"))
	}
	base := filepath.Join(mapDir, "TTData")
	sourceDirs = append(sourceDirs, base)

	blocked := map[string]bool{}
	var chosen, sourceKind string
	key := strings.ToLower(strings.TrimSpace(line))
	for i := len(active) - 1; i >= 0; i-- {
		ttDir := filepath.Join(active[i].Dir, "TTData")
		if chosen == "" && !blocked[key] {
			if p := exactTTLIn(ttDir, line); p != "" {
				chosen = p
				sourceKind = "Chrono/" + filepath.Base(active[i].Dir)
			}
		}
		for _, l := range active[i].Deactivates {
			blocked[strings.ToLower(strings.TrimSpace(l))] = true
		}
	}
	if chosen != "" {
		return chosen, sourceKind, sourceDirs, nil
	}
	if blocked[key] {
		return "", "", sourceDirs, fmt.Errorf("line %s is deactivated by an active Chrono scenario", line)
	}
	if p := exactTTLIn(base, line); p != "" {
		return p, "base TTData", sourceDirs, nil
	}
	return "", "", sourceDirs, fmt.Errorf("could not find loaded timetable line %s.ttl", line)
}

func resolveTripProfile(tripName string, ttlPath string, sourceDirs []string) string {
	want := tripName + ".ttp"
	// openOMSI merges TTPs independently of TTLs: the last active Chrono
	// profile wins even when it did not replace the line's TTL.
	dirs := append(append([]string{}, sourceDirs...), filepath.Dir(ttlPath))
	seen := map[string]bool{}
	for _, dir := range dirs {
		key := strings.ToLower(filepath.Clean(dir))
		if seen[key] {
			continue
		}
		seen[key] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(e.Name(), want) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

type tripMeta struct {
	Valid       bool
	Destination string
	Line        string
	FirstStop   string
	LastStop    string
}

func readTripMeta(path string) tripMeta {
	var m tripMeta
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	lines := strings.Split(strings.ReplaceAll(decodeText(b), "\r\n", "\n"), "\n")
	var stations []string
	hasType2 := false
	for i := 0; i < len(lines); i++ {
		key := strings.ToLower(strings.TrimSpace(lines[i]))
		if key == "[trip]" {
			// These are positional strings, not non-empty tokens. In normal
			// bus TTPs the first value (track) is often empty. Skipping it
			// incorrectly reads the line as destination and loses the ordinal.
			if i+3 < len(lines) {
				track := strings.TrimSpace(lines[i+1])
				m.Destination = strings.TrimSpace(lines[i+2])
				m.Line = strings.TrimSpace(lines[i+3])
				m.Valid = !strings.HasPrefix(track, "[") && !strings.HasPrefix(m.Destination, "[") && !strings.HasPrefix(m.Line, "[")
			}
		}
		if key == "[station_typ2]" {
			hasType2 = true
		}
		if key == "[station]" {
			// Preserve the actual first/last records, including unnamed ones.
			// The terminus above is a display string, not the final stop name.
			name := ""
			if i+3 < len(lines) {
				name = strings.TrimSpace(lines[i+3])
				if strings.HasPrefix(name, "[") {
					name = ""
				}
			}
			stations = append(stations, name)
		}
	}
	// openOMSI uses type-2 object IDs instead of legacy records when present.
	// Those IDs need map objects to supply names; do not invent endpoint names.
	if !hasType2 && len(stations) > 0 {
		m.FirstStop = stations[0]
		if len(stations) > 1 {
			m.LastStop = stations[len(stations)-1]
		}
	}
	return m
}

func comparable(s string) string {
	// Remove BCS decorations commonly present in route labels but not in TTP.
	s = strings.ReplaceAll(s, ".bug", "")
	return normName(s)
}

func routeEnds(route string) (string, string) {
	parts := strings.SplitN(route, " - ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", ""
}

func fuzzyEq(a, b string) bool {
	a, b = comparable(a), comparable(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func chooseTTLTrip(t ttlTour, info TripInfo, ttlPath string, sourceDirs []string) (ttlTrip, string, error) {
	if t.Invalid != "" {
		return ttlTrip{}, "", fmt.Errorf("invalid timetable: %s", t.Invalid)
	}
	if len(t.Trips) == 0 {
		return ttlTrip{}, "", fmt.Errorf("tour has no trips")
	}
	routeStart, routeEnd := routeEnds(info.RouteText)
	type scored struct {
		trip   ttlTrip
		score  int
		reason []string
	}
	var all []scored
	for _, tr := range t.Trips {
		s := scored{trip: tr}
		metaPath := resolveTripProfile(tr.Name, ttlPath, sourceDirs)
		if metaPath == "" {
			continue
		}
		meta := readTripMeta(metaPath)
		if !meta.Valid {
			return ttlTrip{}, "", fmt.Errorf("could not safely read loaded trip profile %s", tr.Name)
		}
		// Missing TTPs are skipped by openOMSI before --trip indexes its duty.
		tr.Index = len(all) + 1
		s.trip = tr
		endName := meta.Destination
		endEvidence := "destination matches BCS route"
		if meta.LastStop != "" {
			endName = meta.LastStop
			endEvidence = "last stop matches BCS route"
		}
		destinationMatch := locationEq(endName, routeEnd, info.Line)
		startMatch := locationEq(meta.FirstStop, routeStart, info.Line) || locationEq(meta.FirstStop, info.StartPoint, info.Line)
		if destinationMatch {
			s.score += 8
			s.reason = append(s.reason, endEvidence)
		}
		if startMatch {
			s.score += 5
			s.reason = append(s.reason, "first stop matches BCS start")
		}
		if comparable(meta.Line) == comparable(info.Line) {
			s.score += 2
			s.reason = append(s.reason, "TTP line matches")
		}
		// A line number or a partial trip name does not establish the route.
		// A contradictory physical endpoint disqualifies an origin match.
		if routeEnd != "" && !destinationMatch {
			s.score = -1
		}
		if meta.FirstStop != "" && (routeStart != "" || info.StartPoint != "") && !startMatch {
			s.score = -1
		}
		if !destinationMatch && !startMatch {
			s.score = -1
		}
		if len(t.Trips) == 1 && routeEnd == "" && routeStart == "" && info.StartPoint == "" {
			s.score = 1
			s.reason = append(s.reason, "single loaded trip in tour")
		}
		all = append(all, s)
	}
	if len(all) == 0 {
		return ttlTrip{}, "", fmt.Errorf("tour has no valid loaded trip profiles")
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	if all[0].score <= 0 {
		return ttlTrip{}, "", fmt.Errorf("no trip matches the BCS route")
	}
	if len(all) > 1 && all[0].score == all[1].score {
		// BCS displays civil hours: 00:20 can be the map's 24:20 service.
		// Require one exact civil departure, including overnight entries;
		// never choose the nearest departure across an unknown offset.
		at, err := parseClockMinutes(info.TripStart)
		if err == nil {
			var exact []scored
			for _, s := range all {
				if s.score == all[0].score && sameBCSDeparture(s.trip.Departure, at) {
					exact = append(exact, s)
				}
			}
			if len(exact) == 1 {
				return exact[0].trip, strings.Join(exact[0].reason, "; ") + "; exact BCS departure", nil
			}
		}
		return ttlTrip{}, "", fmt.Errorf("trip match is ambiguous (%d trips, best score %d)", len(t.Trips), all[0].score)
	}
	return all[0].trip, strings.Join(all[0].reason, "; "), nil
}

// Interpret a civil clock in the service day nearest this candidate. Explicit
// extended BCS hours keep their stated day. Equal civil times in two separate
// records still remain ambiguous; this helper does not select a candidate.
func operationalBCSDeparture(bcs, departure float64) float64 {
 if bcs < 0 || bcs >= 1440 || !validDeparture(departure) { return bcs }
 day := math.Round((departure-bcs)/1440)
 aligned := bcs + day*1440
 if aligned < 0 || !validDeparture(aligned) { return bcs }
 return aligned
}

func sameBCSDeparture(departure, bcs float64) bool {
 return math.Abs(departure-operationalBCSDeparture(bcs, departure)) < 0.001
}

// Match full place names, ignoring only a separate line decoration used by BCS.
// Substring matching made "Depot" match "Other Depot", and line 10 match 210.
func locationEq(a, b, line string) bool {
	key := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(strings.ToLower(s), ".bug", ""))
		parts := strings.Fields(s)
		kept := parts[:0]
		for _, p := range parts {
			if comparable(p) != comparable(line) || line == "" {
				kept = append(kept, p)
			}
		}
		return comparable(strings.Join(kept, " "))
	}
	x, y := key(a), key(b)
	return x != "" && y != "" && x == y
}

func findTour(tours []ttlTour, wanted string) (ttlTour, error) {
	for _, t := range tours {
		if strings.EqualFold(strings.TrimSpace(t.Name), strings.TrimSpace(wanted)) {
			return t, nil
		}
	}
	return ttlTour{}, fmt.Errorf("tour %q not found", wanted)
}

func replaceNumericLine(original string, v float64) string {
	cr := ""
	base := original
	if strings.HasSuffix(base, "\r") {
		cr = "\r"
		base = strings.TrimSuffix(base, "\r")
	}
	lead := base[:len(base)-len(strings.TrimLeft(base, " \t"))]
	trail := base[len(strings.TrimRight(base, " \t")):]
	// OMSI editor normally emits three decimals. Keep that stable and parser-friendly.
	return lead + strconv.FormatFloat(v, 'f', 3, 64) + trail + cr
}

func shiftTourTTL(text string, tour ttlTour, offset float64) (string, error) {
	lines := strings.Split(text, "\n")
	if tour.Invalid != "" || math.IsNaN(offset) || math.IsInf(offset, 0) {
		return "", fmt.Errorf("invalid timetable or offset")
	}
	if len(tour.Trips) == 0 {
		return "", fmt.Errorf("tour has no trips")
	}
	for _, tr := range tour.Trips {
		v := tr.Departure + offset
		// openOMSI and OMSI accept extended hours beyond 24:00. Negative times are
		// not a normal timetable representation, so fail safely rather than write
		// a map-specific guess.
		if !validDeparture(tr.Departure) || !validDeparture(v) {
			return "", fmt.Errorf("shift would create unsupported departure %.3f minutes", v)
		}
		if tr.ValueLine < 0 || tr.ValueLine >= len(lines) {
			return "", fmt.Errorf("internal TTL line index out of range")
		}
		lines[tr.ValueLine] = replaceNumericLine(lines[tr.ValueLine], v)
	}
	return strings.Join(lines, "\n"), nil
}

func writeContentZIP(zipPath, entryName string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(zipPath), ".timetable-*.zip.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	zw := zip.NewWriter(f)
	h := &zip.FileHeader{Name: filepath.ToSlash(entryName), Method: zip.Deflate}
	h.SetModTime(time.Now())
	w, err := zw.CreateHeader(h)
	if err == nil {
		_, err = w.Write(data)
	}
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	_ = os.Remove(zipPath)
	return os.Rename(tmp, zipPath)
}

func prepareTimetableSync(root, packageDir, mapRel, runDate string, info TripInfo) TimetableSyncResult {
	res := TimetableSyncResult{Reason: "not attempted"}
	if info.Line == "" || info.Tour == "" || info.TripStart == "" || mapRel == "" || runDate == "" {
		res.Reason = "missing map/date/line/tour/time"
		return res
	}
	ttlPath, chronoSource, sourceDirs, err := timetableSourceTTL(root, mapRel, info.Line, runDate)
	if err != nil {
		res.Reason = err.Error()
		return res
	}
	res.SourceTTL = ttlPath
	res.SourceRelative = relOMSI(root, ttlPath)
	res.ChronoSource = chronoSource
	res.DiagnosticSources = append(res.DiagnosticSources, ttlPath)
	res.DiagnosticSources = append(res.DiagnosticSources, chronoConfigOrder(filepath.Join(root, filepath.Dir(filepath.FromSlash(mapRel)), "Chrono"))...)
	raw, err := os.ReadFile(ttlPath)
	if err != nil {
		res.Reason = "read TTL: " + err.Error()
		return res
	}
	codec := detectTextCodec(raw)
	text := decodeText(raw)
	tours := parseTTL(text)
	tour, err := findTour(tours, info.Tour)
	if err != nil {
		res.Reason = err.Error()
		return res
	}
	res.TourTrips = len(tour.Trips)
	var candidates strings.Builder
	loaded := 0
	for i, tr := range tour.Trips {
		path := resolveTripProfile(tr.Name, ttlPath, sourceDirs)
		if path == "" {
			fmt.Fprintf(&candidates, "Unloaded profile: %q at %s (missing TTP)\r\n", tr.Name, formatMinutesClock(tr.Departure))
			continue
		}
		res.DiagnosticSources = append(res.DiagnosticSources, path)
		loaded++
		if i < 128 {
			meta := readTripMeta(path)
			fmt.Fprintf(&candidates, "Candidate #%d: trip=%q profile=%s departure=%s valid=%t first=%q last=%q display=%q line=%q source=%q\r\n",
				loaded, tr.Name, tr.Profile, formatMinutesClock(tr.Departure), meta.Valid, meta.FirstStop, meta.LastStop, meta.Destination, meta.Line, path)
		}
	}
	if len(tour.Trips) > 128 {
		fmt.Fprint(&candidates, "Further candidate details omitted (128-record limit).\r\n")
	}
	res.CandidateDetails = candidates.String()
	matched, why, err := chooseTTLTrip(tour, info, ttlPath, sourceDirs)
	if err != nil {
		res.Reason = err.Error()
		return res
	}
	bcsMin, err := parseClockMinutes(info.TripStart)
	if err != nil {
		res.Reason = err.Error()
		return res
	}
	res.TripName = matched.Name
	res.OriginalDeparture = matched.Departure
	res.BCSCivilDeparture = bcsMin
 res.BCSDeparture = operationalBCSDeparture(bcsMin, matched.Departure)
 res.OffsetMinutes = res.BCSDeparture - matched.Departure
 // Upstream duty_time adjusts by one day only. Fold only a selected 49h+
 // runtime record into that supported window, while retaining logical offset
 // zero and every installed byte. This is separate from a company time shift.
 if res.BCSDeparture >= 2880 {
  res.RuntimeDayAdjustmentMinutes = 1440 + math.Mod(res.BCSDeparture,1440) - res.BCSDeparture
 }
 if math.Abs(res.OffsetMinutes) < 0.001 && res.RuntimeDayAdjustmentMinutes == 0 {
  res.Ready = true
  res.TripIndex = matched.Index
  res.Reason = "already synchronized; matched " + why
  return res
 }
 shifted := text
 if math.Abs(res.OffsetMinutes) >= 0.001 {
  shifted, err = shiftTourTTL(text, tour, res.OffsetMinutes)
  if err != nil { res.Reason = err.Error(); return res }
 }
 if res.RuntimeDayAdjustmentMinutes != 0 {
  runtimeTrip := matched
  runtimeTrip.Departure += res.OffsetMinutes
  shifted, err = shiftTourTTL(shifted, ttlTour{Trips: []ttlTrip{runtimeTrip}}, res.RuntimeDayAdjustmentMinutes)
  if err != nil { res.Reason = err.Error(); return res }
 }
	entryRel := relOMSI(root, ttlPath)
	if filepath.IsAbs(entryRel) || strings.HasPrefix(entryRel, "../") || strings.HasPrefix(entryRel, `..\`) {
		res.Reason = "TTL is outside OMSI root"
		return res
	}
	name := "bcs-timetable-v" + bridgeVersion
	if info.ShiftID != "" {
		name += "-" + info.ShiftID
	}
	zipPath := filepath.Join(packageDir, "compat", name+".zip")
	if err := writeContentZIP(zipPath, entryRel, encodeLikeOriginal(shifted, codec)); err != nil {
		res.Reason = "write content ZIP: " + err.Error()
		return res
	}
	res.OverrideZIP = zipPath
	res.Applied = true
	res.Ready = true
	res.TripIndex = matched.Index
	res.Reason = "matched " + why
	return res
}

func removeTimetableOverlay(res TimetableSyncResult) {
	if res.OverrideZIP != "" {
		_ = os.Remove(res.OverrideZIP)
	}
}

func timetableSyncDiagnostic(res TimetableSyncResult) string {
	var b bytes.Buffer
	status := "BLOCKED"
	if res.Ready {
		status = "ALREADY ALIGNED"
		if res.Applied {
			status = "APPLIED"
		}
	}
	fmt.Fprintf(&b, "Timetable sync: %s\r\n", status)
	fmt.Fprintf(&b, "Timetable reason: %s\r\n", res.Reason)
	if res.SourceTTL != "" {
		fmt.Fprintf(&b, "Timetable source: %s\r\n", res.SourceTTL)
		fmt.Fprintf(&b, "Timetable source kind: %s\r\n", res.ChronoSource)
	}
	if res.TripName != "" {
		fmt.Fprintf(&b, "Matched timetable trip: %s (#%d of %d)\r\n", res.TripName, res.TripIndex, res.TourTrips)
		fmt.Fprintf(&b, "Original departure: %s (%.3f min)\r\n", formatMinutesClock(res.OriginalDeparture), res.OriginalDeparture)
		fmt.Fprintf(&b, "BCS civil departure: %s (%.3f min)\r\n", formatMinutesClock(res.BCSCivilDeparture), res.BCSCivilDeparture)
		fmt.Fprintf(&b, "BCS operational departure: %s (%.3f min)\r\n", formatMinutesClock(res.BCSDeparture), res.BCSDeparture)
		if res.RuntimeDayAdjustmentMinutes != 0 { fmt.Fprintf(&b, "Runtime-only day adjustment: %+.3f min (installed TTL unchanged)\r\n",res.RuntimeDayAdjustmentMinutes) }
		fmt.Fprintf(&b, "Applied offset: %+.3f min\r\n", res.OffsetMinutes)
	}
	if res.OverrideZIP != "" {
		fmt.Fprintf(&b, "Temporary content ZIP: %s\r\n", res.OverrideZIP)
	}
	if res.CandidateDetails != "" {
		fmt.Fprintf(&b, "Trip candidates:\r\n%s", res.CandidateDetails)
	}
	return b.String()
}
