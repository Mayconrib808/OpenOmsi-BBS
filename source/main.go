package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type TripInfo struct {
	MapName     string
	Line        string
	Tour        string
	TripStart   string
	TripEnd     string
	RouteText   string
	StartPoint  string
	SelectedHof string
	TargetHof   string
	BlockTime   string
	ShiftID     string
}

func appendLog(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text)
}

func watchOpenOMSIReady(logPath string, startOffset int64, readyPath string, networkReady <-chan struct{}, done <-chan struct{}) {
	go func() {
		deadline := time.Now().Add(startupTimeout)
		var puttingSeen time.Time
		for time.Now().Before(deadline) {
			select {
			case <-done:
				return
			default:
			}
			if networkReady != nil {
				select {
				case <-networkReady:
					networkReady = nil
				default:
					select {
					case <-done:
						return
					case <-time.After(200 * time.Millisecond):
					}
					continue
				}
			}
			b, err := os.ReadFile(logPath)
			if err == nil && int64(len(b)) >= startOffset {
				chunk := strings.ToLower(string(b[startOffset:]))
				if strings.Contains(chunk, "auto-start finished:") || strings.Contains(chunk, "auto start finished:") {
					if err := os.WriteFile(readyPath, []byte("ready\r\n"), 0644); err != nil {
						appendLog(logPath, "WARN: could not publish openOMSI ready flag: "+err.Error()+"\r\n")
					} else {
						appendLog(logPath, "openOMSI readiness detected from auto-start completion; published compat ready flag.\r\n")
					}
					return
				}
				if strings.Contains(chunk, "putting the vehicle into service") && puttingSeen.IsZero() {
					puttingSeen = time.Now()
				}
				// Fallback for a future openOMSI build that changes/removes the exact
				// auto-start log line. Once the vehicle-service stage has been visible
				// for 15 seconds the map/player state is already far past raw map load.
				if !puttingSeen.IsZero() && time.Since(puttingSeen) >= 15*time.Second {
					if err := os.WriteFile(readyPath, []byte("ready-fallback\r\n"), 0644); err == nil {
						appendLog(logPath, "openOMSI readiness fallback after vehicle-service stage; published compat ready flag.\r\n")
					}
					return
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		appendLog(logPath, "WARN: openOMSI readiness detector timed out after 15 minutes.\r\n")
	}()
}

func qargs(a []string) string {
	out := make([]string, len(a))
	for i, x := range a {
		out[i] = fmt.Sprintf("%q", x)
	}
	return strings.Join(out, " ")
}

func decodeText(b []byte) string {
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
		u := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[i:i+2]))
		}
		return string(runesFromUTF16(u))
	}
	if len(b) >= 2 && b[0] == 0xfe && b[1] == 0xff {
		u := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, binary.BigEndian.Uint16(b[i:i+2]))
		}
		return string(runesFromUTF16(u))
	}
	return string(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}))
}

func runesFromUTF16(u []uint16) []rune {
	out := make([]rune, 0, len(u))
	for i := 0; i < len(u); i++ {
		r := rune(u[i])
		if r >= 0xD800 && r <= 0xDBFF && i+1 < len(u) {
			r2 := rune(u[i+1])
			if r2 >= 0xDC00 && r2 <= 0xDFFF {
				out = append(out, 0x10000+((r-0xD800)<<10)+(r2-0xDC00))
				i++
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

var tourRE = regexp.MustCompile(`Tour:\s*([0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)\s*-\s*([0-9]{1,2}:[0-9]{2}(?::[0-9]{2})?)\s*\(Linie:\s*(.*?)\s*\)`)
var umlaufRE = regexp.MustCompile(`\(Umlauf:\s*(.*?)\s*\)\s*$`)
var timePrefixRE = regexp.MustCompile(`(?:INFO|ERROR|WARN)\s+([0-9]{2}\.[0-9]{2}\.[0-9]{4}\s+[0-9]{2}:[0-9]{2}:[0-9]{2}:[0-9]+)`)

func afterMarker(line, marker string) (string, bool) {
	i := strings.Index(line, marker)
	if i < 0 {
		return "", false
	}
	return strings.TrimSpace(line[i+len(marker):]), true
}

func parseBCSLog(path string) (TripInfo, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return TripInfo{}, err
	}
	return parseBCSLogText(decodeText(b))
}

func parseBCSLogText(text string) (TripInfo, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var cur, last TripInfo
	var pendingSelectedHof, pendingTargetHof string
	pendingShiftID := ""
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "Spielmodus:") || line == "Start" {
			pendingShiftID = ""
		}
		if m := shiftIDLine.FindStringSubmatch(line); len(m) == 2 {
			pendingShiftID = m[1]
		}
		if v, ok := afterMarker(line, "Gewählte Hof-Datei:"); ok {
			pendingSelectedHof = v
		}
		if v, ok := afterMarker(line, "Zielpfad Hof-Datei:"); ok {
			pendingTargetHof = v
		}
		if v, ok := afterMarker(line, "Karte:"); ok {
			cur = TripInfo{MapName: v, SelectedHof: pendingSelectedHof, TargetHof: pendingTargetHof, ShiftID: pendingShiftID}
			if m := timePrefixRE.FindStringSubmatch(line); len(m) > 1 {
				cur.BlockTime = m[1]
			}
			continue
		}
		if cur.MapName != "" {
			if m := tourRE.FindStringSubmatch(line); len(m) == 4 {
				cur.TripStart = normalizeTime(m[1])
				cur.TripEnd = normalizeTime(m[2])
				cur.Line = strings.TrimSpace(m[3])
				if i+1 < len(lines) {
					next := strings.TrimSpace(lines[i+1])
					if um := umlaufRE.FindStringSubmatch(next); len(um) == 2 {
						cur.Tour = strings.TrimSpace(um[1])
						cur.RouteText = strings.TrimSpace(umlaufRE.ReplaceAllString(next, ""))
					}
				}
				if cur.Line != "" && cur.Tour != "" {
					last = cur
				}
			}
			if v, ok := afterMarker(line, "Startpunkt:"); ok {
				cur.StartPoint = v
				if cur.Line != "" && cur.Tour != "" {
					last = cur
				}
			}
		}
	}
	if last.MapName == "" || last.Line == "" || last.Tour == "" {
		return TripInfo{}, fmt.Errorf("nenhum bloco Karte/Tour/Umlauf valido encontrado")
	}
	return last, nil
}

func normalizeTime(t string) string {
	p := strings.Split(strings.TrimSpace(t), ":")
	if len(p) < 2 {
		return t
	}
	h, _ := strconv.Atoi(p[0])
	m, _ := strconv.Atoi(p[1])
	return fmt.Sprintf("%02d:%02d", h, m)
}

func candidateBCSLog(cfg Config) string {
	if cfg.BCSLog != "" && !strings.EqualFold(cfg.BCSLog, "auto") {
		if fileExists(cfg.BCSLog) {
			return cfg.BCSLog
		}
		return cfg.BCSLog
	}
	direct := []string{
		filepath.Join(cfg.Root, "Busbetrieb-Simulator", "log.txt"),
		filepath.Join(cfg.Root, "Bus Company Simulator", "log.txt"),
		filepath.Join(cfg.Root, "Busbetriebs-Simulator", "log.txt"),
		filepath.Join(cfg.Root, "BBS", "log.txt"),
		filepath.Join(cfg.Root, "log.txt"),
	}
	for _, p := range direct {
		if looksLikeBCSLog(p) {
			return p
		}
	}
	// Last-resort shallow scan of the OMSI root, deliberately skipping content-heavy folders.
	found := findNewestNamed(cfg.Root, "log.txt", 4, func(p string) bool { return looksLikeBCSLog(p) })
	return found
}

func candidateBackups(cfg Config, bcsLog string) string {
	if cfg.Backups != "" && !strings.EqualFold(cfg.Backups, "auto") {
		if fileExists(cfg.Backups) {
			return cfg.Backups
		}
		return cfg.Backups
	}
	direct := []string{
		filepath.Join(cfg.Root, "Busbetrieb-Simulator", "BBS_Backups.txt"),
		filepath.Join(cfg.Root, "BBS_Backups.txt"),
	}
	if bcsLog != "" {
		direct = append([]string{filepath.Join(filepath.Dir(bcsLog), "BBS_Backups.txt")}, direct...)
	}
	for _, p := range direct {
		if fileExists(p) {
			return p
		}
	}
	return findNewestNamed(cfg.Root, "BBS_Backups.txt", 4, func(p string) bool { return fileExists(p) })
}

// Read a bounded suffix while retaining UTF-16 encoding and code-unit alignment.
// BCS logs can be larger than 1 MiB; dropping the BOM made auto detection fail.
func readLogTail(p string, limit int64) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a readable log file")
	}
	head := make([]byte, 2)
	n, _ := f.ReadAt(head, 0)
	utf16Log := n == 2 && ((head[0] == 0xff && head[1] == 0xfe) || (head[0] == 0xfe && head[1] == 0xff))
	start := int64(0)
	if st.Size() > limit {
		start = st.Size() - limit
	}
	if utf16Log && start > 0 {
		start -= start % 2
	}
	if _, err = f.Seek(start, 0); err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+2))
	if err != nil {
		return "", err
	}
	if utf16Log && start > 0 {
		b = append(head, b...)
	}
	return decodeText(b), nil
}

func looksLikeBCSLog(p string) bool {
	s, err := readLogTail(p, 1024*1024)
	if err != nil {
		return false
	}
	return strings.Contains(s, "Karte:") && strings.Contains(s, "Tour:") && (strings.Contains(s, "Busbetrieb") || strings.Contains(s, "Spielmodus:") || strings.Contains(s, "OMSI Startvorbereitung"))
}

var heavyDirs = map[string]bool{
	"vehicles": true, "sceneryobjects": true, "splines": true, "maps": true, "texture": true, "textures": true, "sounds": true, "fonts": true, "ticketpacks": true, "humans": true, "money": true, "drivers": true, "openomsi": true,
}

func findNewestNamed(root, name string, maxDepth int, pred func(string) bool) string {
	type cand struct {
		p string
		t time.Time
	}
	var cs []cand
	root = filepath.Clean(root)
	rootDepth := strings.Count(root, string(os.PathSeparator))
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		depth := strings.Count(filepath.Clean(p), string(os.PathSeparator)) - rootDepth
		if d.IsDir() {
			if depth > maxDepth {
				return filepath.SkipDir
			}
			if depth == 1 && heavyDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		if depth > maxDepth || !strings.EqualFold(d.Name(), name) {
			return nil
		}
		if pred != nil && !pred(p) {
			return nil
		}
		if fi, e := d.Info(); e == nil {
			cs = append(cs, cand{p, fi.ModTime()})
		}
		return nil
	})
	if len(cs) == 0 {
		return ""
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].t.After(cs[j].t) })
	return cs[0].p
}

func parseBackupBus(path, targetDir string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(decodeText(b), "\r\n", "\n"), "\n")
	var buses []string
	for _, l := range lines {
		p := strings.TrimSpace(strings.Trim(l, `"`))
		if strings.EqualFold(filepath.Ext(p), ".bus") {
			buses = append(buses, p)
		}
	}
	if targetDir != "" {
		for i := len(buses) - 1; i >= 0; i-- {
			if samePath(filepath.Dir(buses[i]), targetDir) && fileExists(buses[i]) {
				return buses[i]
			}
		}
	}
	for i := len(buses) - 1; i >= 0; i-- {
		if fileExists(buses[i]) {
			return buses[i]
		}
	}
	return ""
}

func recentBusInDir(dir string) string {
	if dir == "" {
		return ""
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type cand struct {
		p string
		t time.Time
	}
	var cs []cand
	for _, e := range es {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bus") {
			continue
		}
		if fi, er := e.Info(); er == nil {
			cs = append(cs, cand{filepath.Join(dir, e.Name()), fi.ModTime()})
		}
	}
	if len(cs) == 0 {
		return ""
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].t.After(cs[j].t) })
	return cs[0].p
}

func resolveMap(root, wanted string) string {
	direct := filepath.Join(root, "maps", wanted, "global.cfg")
	if fileExists(direct) {
		return relOMSI(root, direct)
	}
	mapsDir := filepath.Join(root, "maps")
	es, err := os.ReadDir(mapsDir)
	if err != nil {
		return ""
	}
	nw := normName(wanted)
	type scored struct {
		p     string
		score int
	}
	var ss []scored
	for _, e := range es {
		if !e.IsDir() {
			continue
		}
		g := filepath.Join(mapsDir, e.Name(), "global.cfg")
		if !fileExists(g) {
			continue
		}
		score := 0
		nf := normName(e.Name())
		if nf == nw {
			score += 100
		}
		if strings.Contains(nf, nw) || strings.Contains(nw, nf) {
			score += 40
		}
		if b, er := os.ReadFile(g); er == nil {
			txt := decodeText(b)
			for _, marker := range []string{"[name]", "[friendlyname]"} {
				if v := cfgValueAfter(txt, marker); v != "" {
					nv := normName(v)
					if nv == nw {
						score += 90
					}
					if strings.Contains(nv, nw) || strings.Contains(nw, nv) {
						score += 30
					}
				}
			}
		}
		if score > 0 {
			ss = append(ss, scored{g, score})
		}
	}
	if len(ss) == 0 {
		return ""
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].score > ss[j].score })
	return relOMSI(root, ss[0].p)
}

func cfgValueAfter(text, marker string) string {
	ls := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, l := range ls {
		if strings.EqualFold(strings.TrimSpace(l), marker) && i+1 < len(ls) {
			return strings.TrimSpace(ls[i+1])
		}
	}
	return ""
}

func normName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func relOMSI(root, p string) string {
	r, e := filepath.Rel(root, p)
	if e != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}

type SituationPaint struct {
	Index      int
	Bus        string
	IsPlayer   bool
	ColorValue float64
}

func nonEmptyLine(lines []string, start int) (string, int) {
	for i := start; i < len(lines); i++ {
		v := strings.TrimSpace(lines[i])
		if v != "" {
			return v, i
		}
	}
	return "", len(lines)
}

// BCS prepares maps/<map>/laststn.osn before it starts Omsi.exe. OMSI stores the
// selected repaint as the player's numeric Colorscheme variable. openOMSI deliberately
// accepts the same numeric scheme with --paint, so this preserves the repaint without
// needing to know the CTI display name.
func paintFromLastSituation(root, mapRel, selectedBus string) (string, string) {
	if mapRel == "" {
		return "", "mapa desconhecido"
	}
	mapAbs := filepath.Join(root, filepath.FromSlash(mapRel))
	osn := filepath.Join(filepath.Dir(mapAbs), "laststn.osn")
	b, err := os.ReadFile(osn)
	if err != nil {
		return "", "laststn.osn nao encontrado: " + osn
	}
	text := decodeText(b)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	type veh struct {
		idx    int
		bus    string
		mine   bool
		scheme *float64
	}
	var vs []veh
	cur := -1
	myIndex := -1
	for i := 0; i < len(lines); i++ {
		k := strings.ToLower(strings.TrimSpace(lines[i]))
		switch k {
		case "[vehicle]":
			v, j := nonEmptyLine(lines, i+1)
			vs = append(vs, veh{idx: len(vs), bus: v})
			cur = len(vs) - 1
			if j > i {
				i = j
			}
		case "[ismyvehicle]":
			if cur >= 0 {
				vs[cur].mine = true
			}
		case "[myvehicle]":
			v, j := nonEmptyLine(lines, i+1)
			if n, e := strconv.Atoi(strings.TrimSpace(v)); e == nil {
				myIndex = n
			}
			if j > i {
				i = j
			}
		case "[vars]":
			if cur < 0 {
				continue
			}
			countLine, j := nonEmptyLine(lines, i+1)
			n, e := strconv.Atoi(strings.TrimSpace(countLine))
			if e != nil || n < 0 {
				continue
			}
			pos := j + 1
			for z := 0; z < n && pos < len(lines); z++ {
				name, nj := nonEmptyLine(lines, pos)
				if nj >= len(lines) {
					break
				}
				val, vj := nonEmptyLine(lines, nj+1)
				if vj >= len(lines) {
					break
				}
				if strings.EqualFold(strings.TrimSpace(name), "Colorscheme") {
					if f, e := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(val, ",", ".")), 64); e == nil {
						x := f
						vs[cur].scheme = &x
					}
				}
				pos = vj + 1
			}
			i = pos - 1
		}
	}
	pick := -1
	for i := range vs {
		if vs[i].mine {
			pick = i
			break
		}
	}
	if pick < 0 && myIndex >= 0 && myIndex < len(vs) {
		pick = myIndex
	}
	if pick < 0 && selectedBus != "" {
		rel := strings.ReplaceAll(filepath.ToSlash(selectedBus), "/", "\\")
		for i := range vs {
			if strings.EqualFold(filepath.Base(strings.ReplaceAll(vs[i].bus, "\\", string(os.PathSeparator))), filepath.Base(strings.ReplaceAll(rel, "\\", string(os.PathSeparator)))) {
				pick = i
				break
			}
		}
	}
	if pick < 0 && len(vs) == 1 {
		pick = 0
	}
	if pick < 0 {
		return "", "laststn.osn lido, mas veiculo do jogador nao identificado: " + osn
	}
	if vs[pick].scheme == nil {
		return "", fmt.Sprintf("laststn.osn: veiculo %d sem Colorscheme (%s)", pick, osn)
	}
	c := *vs[pick].scheme
	if math.IsNaN(c) || math.IsInf(c, 0) || c < 0 {
		return "", fmt.Sprintf("laststn.osn: Colorscheme=%.3f = pintura padrao", c)
	}
	idx := int64(c)
	return strconv.FormatInt(idx, 10), fmt.Sprintf("laststn.osn Colorscheme do veiculo %d (%s)", pick, osn)
}

func choosePaint(cfg Config, mapRel, busRel string) (string, string) {
	raw := strings.TrimSpace(cfg.Paint)
	if raw == "" || strings.EqualFold(raw, "auto") {
		return paintFromLastSituation(cfg.Root, mapRel, busRel)
	}
	switch strings.ToLower(raw) {
	case "off", "none", "default", "padrao", "padrão", "-1":
		return "", "bridge.ini: pintura padrao"
	default:
		return raw, "bridge.ini manual override"
	}
}

func chooseDate(cfg Config, trip TripInfo) (string, string) {
	raw := strings.TrimSpace(cfg.Date)
	if raw != "" && !strings.EqualFold(raw, "auto") && !strings.EqualFold(raw, "system") && !strings.EqualFold(raw, "today") {
		// Manual override. Useful when the date chosen in the BCS calendar is not today.
		return raw, "bridge.ini manual override"
	}

	// BCS shows a date picker before the trip. The date is not written explicitly to
	// the BCS log that the bridge can read. Its default selection is the current Windows
	// calendar date, so v0.4.2 mirrors that instead of inventing a historical date.
	// If the user chooses another day in BCS, set date=YYYY-MM-DD in bridge.ini.
	return time.Now().Format("2006-01-02"), "Windows local date (BCS default)"
}

func main() {
	packageDir := packageRoot()
	dir := appDir(packageDir)
	logPath := filepath.Join(dir, "bridge-v1.1.3.log")
	tripPath := filepath.Join(dir, "bridge-v1.1.3-trip.txt")
	cfg := readInstalledConfig(packageDir)

	appendLog(logPath, "\r\n============================================================\r\n")
	appendLog(logPath, "OpenOMSI BCS Bridge v"+bridgeVersion+" - by "+bridgeAuthor+"\r\n")
	appendLog(logPath, "Time: "+time.Now().Format(time.RFC3339)+"\r\n")
	appendLog(logPath, "Bridge argv: "+qargs(os.Args)+"\r\n")
	if cwd, err := os.Getwd(); err == nil {
		appendLog(logPath, "Working dir: "+cwd+"\r\n")
	}
	appendLog(logPath, "openOMSI: "+cfg.OpenOMSI+"\r\n")
	appendLog(logPath, "OMSI root: "+cfg.Root+"\r\n")

	if err := validateConfig(cfg, packageDir); err != nil {
		appendLog(logPath, "ERROR: invalid configuration: "+err.Error()+"\r\n")
		showLaunchError(cfg.Language, localText(cfg.Language, "Configure as pastas no Setup.exe antes de iniciar.\n", "Configure the folders in Setup.exe before launching.\n", "Stelle vor dem Start die Ordner in Setup.exe ein.\n")+err.Error())
		return
	}
	bcsLog := candidateBCSLog(cfg)
 initialTrip, initialTripErr := parseBCSLog(bcsLog)
 activeStatePath := launchSessionPath(cfg.Root,packageDir)
 release, lockErr := acquireBridgeLock(cfg.Root)
	if lockErr != nil {
  if lockErr == errBridgeBusy && initialTripErr == nil {
   deadline:=time.Now().Add(2*time.Second)
   for {
    state,e:=readLaunchSession(activeStatePath)
    if e==nil {
     if sameLaunchTrip(state.Trip,initialTrip) {
      appendLog(logPath,"Repeated call for the same BCS shift; the existing bridge owns this trip.\r\n")
      return
     }
     if validLaunchShiftID(initialTrip.ShiftID) && initialTrip.ShiftID!=state.Trip.ShiftID {
      if b,e:=os.ReadFile(bcsLog);e==nil {
       gate:=newCompletionGate(state.Trip.ShiftID,"")
       if gate.accepts(decodeText(b)) {
        appendLog(logPath,"Next BCS trip requested; the active bridge will handle the transition.\r\n")
        return
       }
      }
     }
    }
    if time.Now().After(deadline){break}
    time.Sleep(50*time.Millisecond)
   }
  }
		appendLog(logPath, "ERROR: "+lockErr.Error()+"\r\n")
		showLaunchError(cfg.Language, localText(cfg.Language, "Já existe uma viagem da bridge aberta para esta instalação.", "A bridge trip is already running for this installation.", "Für diese Installation läuft bereits eine Fahrt mit der Bridge."))
		return
	}
	released:=false
 releaseLock:=func(){if !released {release();released=true}}
 defer releaseLock()
 if initialTripErr==nil {if e:=writeLaunchSession(activeStatePath,initialTrip);e!=nil{appendLog(logPath,"WARN: launch metadata: "+e.Error()+"\r\n")}}
 defer clearLaunchSession(activeStatePath)
 compatible,compatErr:=checkOpenOMSICompatibility(cfg.OpenOMSI,false,cfg.Multiplayer)
 if compatErr!=nil {appendLog(logPath,"ERROR: OpenOMSI compatibility: "+compatErr.Error()+"\r\n");showLaunchError(cfg.Language,compatErr.Error());return}
 appendLog(logPath,"OpenOMSI capabilities accepted: "+compatible.Version+"\r\n")
	deployment, hostErr := preparePluginHost(cfg, packageDir)
	if hostErr != nil {
		appendLog(logPath, "ERROR: "+hostErr.Error()+"\r\n")
		showLaunchError(cfg.Language, hostErr.Error())
		return
	}
	host := deployment.Path
	appendLog(logPath, "BCS 32-bit plugin host: "+host+"\r\n")
	appendLog(logPath, fmt.Sprintf("BCS helper created by this launch: %t\r\n", deployment.Created))
	appendLog(logPath, pluginHostDiagnostics(cfg, packageDir))

	appendLog(logPath, "BCS log: "+bcsLog+"\r\n")
	// Keep diagnostics available even when route checks stop before the facade.
	_ = os.WriteFile(runtimeBCSLogPath(dir), []byte(bcsLog), 0644)
	_ = os.Remove(timetableSourceListPath(dir))
	trip, err := parseBCSLog(bcsLog)
	if err != nil {
		appendLog(logPath, "ERROR lendo a viagem do BCS: "+err.Error()+"\r\n")
		_ = os.WriteFile(tripPath, []byte("Could not detect the BCS trip.\r\nBCS log: "+bcsLog+"\r\nSet bcslog= in bridge.ini if necessary.\r\n"), 0644)
		showLaunchError(cfg.Language, localText(cfg.Language, "Não foi possível ler a viagem do BCS. Inicie uma viagem pelo BCS. Para jogar no OMSI original, desative a bridge pelo Setup.exe.", "Could not read the BCS trip. Start a trip through BCS. To play original OMSI, deactivate the bridge in Setup.exe.", "Die BBS-Fahrt konnte nicht gelesen werden. Starte eine Fahrt über BBS. Wenn du das originale OMSI nutzen möchtest, deaktiviere die Bridge in Setup.exe."))
		return
	}

	if e:=writeLaunchSession(activeStatePath,trip);e!=nil{appendLog(logPath,"WARN: launch metadata: "+e.Error()+"\r\n")}
 if len(os.Args)==3 && os.Args[1]=="--bcs-next-shift" && trip.ShiftID!=os.Args[2] {
  appendLog(logPath,"Next BCS selection changed before relaunch; refusing an unrelated shift.\r\n")
  return
 }
	mapRel := resolveMap(cfg.Root, trip.MapName)
	backups := candidateBackups(cfg, bcsLog)
	targetDir := ""
	if trip.TargetHof != "" {
		targetDir = filepath.Dir(trip.TargetHof)
	}
	busAbs := parseBackupBus(backups, targetDir)
	busSource := "BBS_Backups.txt"
	if busAbs == "" {
		busAbs = recentBusInDir(targetDir)
		busSource = "arquivo .bus mais recente da pasta alvo"
	}
	busRel := ""
	if busAbs != "" {
		busRel = relOMSI(cfg.Root, busAbs)
	}
	hof := ""
	if trip.TargetHof != "" {
		hof = strings.TrimSuffix(filepath.Base(trip.TargetHof), filepath.Ext(trip.TargetHof))
	}
	if hof == "" && trip.SelectedHof != "" {
		hof = strings.TrimSuffix(filepath.Base(trip.SelectedHof), filepath.Ext(trip.SelectedHof))
	}
	runDate, dateSource := chooseDate(cfg, trip)
	runPaint, paintSource := choosePaint(cfg, mapRel, busRel)

	diag := &strings.Builder{}
	fmt.Fprintf(diag, "OpenOMSI BCS Bridge v%s - by %s\r\n\r\n", bridgeVersion, bridgeAuthor)
	fmt.Fprintf(diag, "BCS log: %s\r\n", bcsLog)
	fmt.Fprintf(diag, "BCS Schicht ID: %s\r\n", trip.ShiftID)
	fmt.Fprintf(diag, "BBS backups: %s\r\n", backups)
	fmt.Fprintf(diag, "Mapa BCS: %s\r\nMapa openOMSI: %s\r\n", trip.MapName, mapRel)
	fmt.Fprintf(diag, "Linha: %s\r\nUmlauf/Tour: %s\r\nTrip/partida: %s\r\nFim: %s\r\nRota: %s\r\nStartpunkt: %s\r\n", trip.Line, trip.Tour, trip.TripStart, trip.TripEnd, trip.RouteText, trip.StartPoint)
	fmt.Fprintf(diag, "HOF alvo: %s\r\nHOF passado ao openOMSI: %s\r\n", trip.TargetHof, hof)
	fmt.Fprintf(diag, "Onibus: %s\r\nFonte do onibus: %s\r\n", busRel, busSource)
	fmt.Fprintf(diag, "Skin/repaint (--paint): %s\r\nFonte da skin: %s\r\n", func() string {
		if runPaint == "" {
			return "(pintura padrao)"
		}
		return runPaint
	}(), paintSource)
	fmt.Fprintf(diag, "Data: %s\r\nFonte da data: %s\r\n", func() string {
		if runDate == "" {
			return "(padrao do mapa/openOMSI)"
		}
		return runDate
	}(), dateSource)
	fmt.Fprintf(diag, "Carregar todos os tiles (--all): %v\r\nAutostart/IBIS (--autostart): %v\r\nCompatibilidade BCS (processo/janela Omsi.exe): %v\r\nSincronizacao temporaria de horario: %v\r\nEspera pelo marcador BCS: %d ms\r\n", cfg.AllTiles, cfg.AutoStart, cfg.BCSCompat, cfg.TimetableSync, cfg.BCSMarkerWaitMS)
	_ = os.WriteFile(tripPath, []byte(diag.String()), 0644)

	appendLog(logPath, "Detected map: "+trip.MapName+" -> "+mapRel+"\r\n")
	appendLog(logPath, "Detected line/tour/trip: "+trip.Line+" / "+trip.Tour+" / "+trip.TripStart+"\r\n")
	appendLog(logPath, "Detected route: "+trip.RouteText+"\r\n")
	appendLog(logPath, "Backups: "+backups+"\r\n")
	appendLog(logPath, "Detected bus: "+busRel+" ("+busSource+")\r\n")
	appendLog(logPath, "Detected HOF: "+hof+"\r\n")
	appendLog(logPath, "Detected paint: "+func() string {
		if runPaint == "" {
			return "(default)"
		}
		return runPaint
	}()+" ("+paintSource+")\r\n")
	appendLog(logPath, "Selected date: "+func() string {
		if runDate == "" {
			return "(default)"
		}
		return runDate
	}()+" ("+dateSource+")\r\n")
	appendLog(logPath, fmt.Sprintf("All tiles: %v | Autostart: %v | BCS compat facade: %v | Timetable sync: %v | BCS marker wait: %d ms\r\n", cfg.AllTiles, cfg.AutoStart, cfg.BCSCompat, cfg.TimetableSync, cfg.BCSMarkerWaitMS))

	var multiplayer *MultiplayerPlan
	if cfg.Multiplayer {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		mpTrip := MultiplayerTrip{trip.MapName, mapRel, busRel, runDate, trip.TripStart}
		var problems []CompanyProblem
		multiplayer, problems, err = prepareMultiplayer(ctx, cfg, dir, mpTrip, companyHTTPClient())
		cancel()
		if err != nil || len(problems) != 0 || multiplayer == nil {
			if err != nil {
				problems = append(problems, CompanyProblem{"Multiplayer", err.Error(), ""})
			}
			if len(problems) == 0 {
				problems = append(problems, CompanyProblem{"Multiplayer", "No compatible company session", ""})
			}
			appendLog(logPath, fmt.Sprintf("Multiplayer preflight refused the launch: %v\r\n", problems))
			if report, e := writeCompanyReport(dir, cfg.Language, cfg.CompanyID, problems); e == nil {
				_ = openMultiplayerDocument(report)
				showLaunchError(cfg.Language, localText(cfg.Language, "A sessão não está pronta para esta viagem. Confira a página de requisitos aberta.\n", "The session is not ready for this trip. Check the requirements page.\n", "Die Sitzung ist für diese Fahrt noch nicht bereit. Prüfe die geöffnete Anforderungsseite.\n")+report)
			} else {
				showLaunchError(cfg.Language, fmt.Sprint(problems))
			}
			return
		}
		// Preflight resolves the company calendar before timetable generation and --date.
		runDate = multiplayer.Trip.Date
		if multiplayer.Clock != nil {
			dateSource = "company clock: " + multiplayer.Clock.TimeZone + fmt.Sprintf(" %+d minutes", multiplayer.Clock.ShiftMinutes)
			appendLog(logPath, "Company date: "+runDate+" ("+dateSource+")\r\n")
			appendLog(tripPath, "\r\nCompany date: "+runDate+" ("+dateSource+")\r\n")
		}
		appendLog(logPath, fmt.Sprintf("Multiplayer company=%s session=%s map=%s date=%s player=%s\r\n", multiplayer.CompanyID, multiplayer.Session.ID, multiplayer.Session.MapFile, multiplayer.Session.Date, multiplayer.PlayerName))
		appendLog(tripPath, fmt.Sprintf("\r\nMultiplayer: %s / %s\r\n", multiplayer.CompanyName, multiplayer.Session.Name))
	}
	if mapRel == "" || busRel == "" || trip.Line == "" || trip.Tour == "" || trip.TripStart == "" {
		appendLog(logPath, "ERROR: dados insuficientes; nao vou abrir uma viagem errada. Veja bridge-v1.1.3-trip.txt\r\n")
		showLaunchError(cfg.Language, localText(cfg.Language, "Não consegui identificar o mapa, o ônibus e o horário desta viagem. Use o Setup, opção 5, para coletar o diagnóstico.", "Could not identify this trip's map, bus and time. Use Setup option 5 to collect diagnostics.", "Karte, Bus und Abfahrtszeit dieser Fahrt konnten nicht ermittelt werden. Sammle mit Setup-Option 5 die Diagnoseprotokolle."))
		return
	}
	for _, check := range []func() error{func() error { return ensureBCSClock(cfg) }, func() error { return ensureOriginalTimetableSources(cfg, mapRel) }} {
		if err := check(); err != nil {
			appendLog(logPath, "ERROR: launch check: "+err.Error()+"\r\n")
			appendLog(tripPath, "\r\n"+err.Error()+"\r\n")
			showLaunchError(cfg.Language, err.Error())
			return
		}
	}

	// BCS can schedule a map trip at a company time that differs from the map's
	// original .ttl departure. openOMSI intentionally moves the clock to the
	// original tour start in that case. Build a session-only content ZIP that
	// shifts the selected tour to the BCS departure, leaving the installed map
	// untouched. If matching or alignment fails, stop before either game process
	// is launched; falling through with original TTData still starts a wrong trip.
	timetable := TimetableSyncResult{Reason: "disabled in bridge.ini"}
	timetable = prepareTimetableSync(cfg.Root, dir, mapRel, runDate, trip)
	defer removeTimetableOverlay(timetable)
	if err := writeTimetableSourceList(cfg.Root, dir, timetable.DiagnosticSources); err != nil {
		appendLog(logPath, "Diagnostic timetable sources: "+err.Error()+"\r\n")
	}
	if !cfg.TimetableSync && timetable.Applied {
		timetable.Ready = false
		timetable.Reason = "timetable_sync=false prevents the required departure alignment"
	}
	timetableDiag := timetableSyncDiagnostic(timetable)
	appendLog(logPath, timetableDiag)
	appendLog(tripPath, "\r\n"+timetableDiag)
	if !timetable.Ready {
		showLaunchError(cfg.Language, localText(cfg.Language,
			"Não consegui alinhar com segurança o trajeto e o horário desta viagem. O jogo não foi iniciado. Use o Setup, opção 5, para coletar o diagnóstico.\nDetalhe: ",
			"Could not safely align this trip's route and departure time. The game was not started. Use Setup option 5 to collect diagnostics.\nDetail: ", "Strecke und Abfahrtszeit dieser Fahrt konnten nicht eindeutig abgeglichen werden. Das Spiel wurde nicht gestartet. Sammle mit Setup-Option 5 die Diagnoseprotokolle.\nDetails: ")+timetable.Reason)
		return
	}

	compatDir := filepath.Join(dir, "compat")
	driver, driverErr := prepareDriver(filepath.Join(cfg.Root, "Drivers", "bbs.odr"), compatDir)
	if driverErr != nil {
		appendLog(logPath, "ERROR preparando a ficha do motorista: "+driverErr.Error()+"\r\n")
		appendLog(tripPath, "\r\nERRO na ficha BCS: "+driverErr.Error()+"\r\n")
		return
	}
	appendLog(logPath, "BCS driver original: "+driver.NativePath+"\r\nopenOMSI converted driver: "+driver.OpenPath+"\r\n")
	appendLog(logPath, "Driver baseline Auswertungsdaten: "+driver.Last.evaluation()+"\r\n")
	appendLog(tripPath, "\r\nFicha BCS: "+driver.NativePath+"\r\nFicha convertida (--driver): "+driver.OpenPath+"\r\nNo terminal: F9, aguarde 2 segundos e finalize no BCS com o jogo aberto.\r\n")

	args := []string{"--root", cfg.Root, "--no-menu"}
	if timetable.Applied && timetable.OverrideZIP != "" {
		// Mount before --map so TTData is overridden while the map is loaded.
		args = append(args, "--content-zip", timetable.OverrideZIP)
	}
	args = append(args, "--map", mapRel)
	args = append(args, "--driver", driver.OpenPath)
	if cfg.AllTiles {
		args = append(args, "--all")
	}
	args = append(args, "--bus", busRel)
	if runPaint != "" {
		args = append(args, "--paint", runPaint)
	}
	args = append(args, "--time", trip.TripStart)
	if runDate != "" {
		args = append(args, "--date", runDate)
	}
	if hof != "" {
		args = append(args, "--hof", hof)
	}
	args = append(args, "--auto-entry")
	if cfg.AutoStart {
		args = append(args, "--autostart")
	}
	args = append(args, "--traffic", strconv.Itoa(cfg.Traffic))
	if cfg.Passengers {
		args = append(args, "--passengers")
	}
	args = append(args, "--schedule", "--line", trip.Line, "--tour", trip.Tour)
	if timetable.TripIndex > 0 {
		// openOMSI 0.2.0 accepts a clock or a 1-based ordinal. An identified ordinal avoids selecting the wrong direction.
		args = append(args, "--trip", strconv.Itoa(timetable.TripIndex))
	}
	args = append(args, multiplayerArguments(multiplayer)...)
	var multiplayerContent string
	if multiplayer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		err = recheckMultiplayer(ctx, multiplayer, companyHTTPClient())
		cancel()
		if err != nil {
			appendLog(logPath, "Multiplayer changed before launch: "+err.Error()+"\r\n")
			showLaunchError(cfg.Language, err.Error())
			return
		}
		multiplayerContent, err = os.MkdirTemp(compatDir, "multiplayer-content-")
		if err != nil {
			showLaunchError(cfg.Language, err.Error())
			return
		}
		defer os.RemoveAll(multiplayerContent)
	}

	appendLog(logPath, "Launching: "+fmt.Sprintf("%q ", cfg.OpenOMSI)+qargs(args)+"\r\n")
	f, _ := os.OpenFile(tripPath, os.O_WRONLY|os.O_APPEND, 0644)
	if f != nil {
		fmt.Fprintf(f, "\r\nCommand:\r\n%q %s\r\n", cfg.OpenOMSI, qargs(args))
		f.Close()
	}

	// Start the BCS facade BEFORE openOMSI. OmsiStartmenue.java can probe the
	// legacy windows very early; starting the facade after the simulator creates
	// a race where BCS gives up before Tform_start/TForm_main exist.
	var compatCmd *exec.Cmd
	var compatDone chan error
	bcsBeforeLaunch, baselineReadErr := os.ReadFile(bcsLog)
	closureGate := newCompletionGate(trip.ShiftID, decodeText(bcsBeforeLaunch))
	if baselineReadErr != nil {
		closureGate.AlreadyClosed = true
		appendLog(logPath, "WARN: nao foi possivel ler baseline do BCS para fechamento seguro: "+baselineReadErr.Error()+"\r\n")
	}
	if baselineReadErr == nil && closureGate.AlreadyClosed {
		appendLog(logPath, "ERROR: the detected BCS shift was already completed; refusing to replay it.\r\n")
		showLaunchError(cfg.Language, localText(cfg.Language, "A viagem encontrada no log já foi concluída. Selecione uma nova viagem no BCS. Para jogar no OMSI original, desative a bridge pelo Setup.exe.", "The trip found in the log is already completed. Select a new trip in BCS. To use original OMSI, deactivate the bridge in Setup.exe.", "Die im Protokoll gefundene Fahrt ist bereits abgeschlossen. Wähle eine neue Fahrt in BBS. Wenn du das originale OMSI nutzen möchtest, deaktiviere die Bridge in Setup.exe."))
		return
	}
	appendLog(logPath, fmt.Sprintf("BCS closure gate: shift ID=%q already-closed=%v; matching exact current shift, independent of log prefix.\r\n", trip.ShiftID, closureGate.AlreadyClosed))
	readyPath := filepath.Join(compatDir, "facade-ready-v1.1.3.flag")
	openOMSIReadyPath := filepath.Join(compatDir, "openomsi-ready-v1.1.3.flag")
	pidPath := filepath.Join(compatDir, "openomsi.pid")
	_ = os.Remove(readyPath)
	_ = os.Remove(openOMSIReadyPath)
	_ = os.Remove(pidPath)
	evaluationPath:=filepath.Join(compatDir,"facade-evaluation-complete.flag")
	_ = os.Remove(evaluationPath)
	if cfg.BCSCompat {
		compatPath := filepath.Join(compatDir, "Omsi.exe")
		if fileExists(compatPath) {
			compatCmd = exec.Command(compatPath, "0", cfg.Root, mapRel, trip.Line, trip.Tour, bcsLog, driver.OpenPath, driver.NativePath, trip.ShiftID)
			compatCmd.Dir = compatDir
			prepareCompanyHostProcess(compatCmd)
			if e := compatCmd.Start(); e != nil {
				appendLog(logPath, "ERROR: nao foi possivel iniciar facade BCS: "+e.Error()+"\r\n")
				return
			} else {
    closeFacadeJob,ownErr:=ownCompanyHostProcess(compatCmd)
    if ownErr!=nil { _ = compatCmd.Process.Kill();_ = compatCmd.Wait();appendLog(logPath,"ERROR owning BCS facade: "+ownErr.Error()+"\r\n");return }
    defer closeFacadeJob()
				compatDone = make(chan error, 1)
				go func() { compatDone <- compatCmd.Wait() }()
				appendLog(logPath, fmt.Sprintf("BCS compat facade v1.1.3 bootstrap started. PID=%d\r\n", compatCmd.Process.Pid))
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) && !fileExists(readyPath) {
					time.Sleep(25 * time.Millisecond)
				}
				if fileExists(readyPath) {
					appendLog(logPath, "BCS compat facade ready BEFORE openOMSI launch.\r\n")
				} else {
					appendLog(logPath, "ERROR: facade/memoria do motorista nao ficou pronta em 3 s. Veja compat\\compat-facade-v1.1.3.log.\r\n")
					_ = compatCmd.Process.Kill()
					<-compatDone
					return
				}
			}
		} else {
			appendLog(logPath, "ERROR: compat\\Omsi.exe nao encontrado. Extraia o pacote completo.\r\n")
			return
		}
	}

	if cfg.BCSMarkerWaitMS > 0 {
		present, waited := waitForBCSStartupMarker(cfg.Root, time.Duration(cfg.BCSMarkerWaitMS)*time.Millisecond)
		appendLog(logPath, fmt.Sprintf("BCS startup marker before openOMSI: present=%v waited=%s (bridge never creates/deletes it)\r\n", present, waited.Round(time.Millisecond)))
		if !present {
			if compatCmd != nil && compatCmd.Process != nil { _ = compatCmd.Process.Kill(); <-compatDone }
			showLaunchError(cfg.Language, localText(cfg.Language, "O BCS não preparou o início desta viagem. Volte ao BCS e inicie a viagem novamente. Se persistir, use o Setup, opção 5.", "BCS did not prepare this trip's startup. Return to BCS and start the trip again. If it persists, use Setup option 5.", "BBS hat den Start dieser Fahrt nicht vorbereitet. Kehre zu BBS zurück und starte die Fahrt erneut. Wenn das Problem weiter besteht, wähle Setup-Option 5."))
			return
		}
	}

	cmd := exec.Command(cfg.OpenOMSI, args...)
	cmd.Dir = cfg.Root
	cmd.Env = pluginEnvironment(os.Environ(), host)
	if multiplayer != nil {
		cmd.Env = multiplayerEnvironment(cmd.Env, multiplayerContent)
	}
	lf, er := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var openOMSIOutputStart int64
	if er == nil {
		defer lf.Close()
		if st, statErr := lf.Stat(); statErr == nil {
			openOMSIOutputStart = st.Size()
		}
		cmd.Stdout = lf
		cmd.Stderr = lf
	}
	prepareCompanyHostProcess(cmd)
	if er = cmd.Start(); er != nil {
		appendLog(logPath, "ERROR starting openOMSI: "+er.Error()+"\r\n")
		if compatCmd != nil && compatCmd.Process != nil {
			_ = compatCmd.Process.Kill()
			<-compatDone
		}
		return
	}
	closeGameJob,ownErr:=ownCompanyHostProcess(cmd)
 if ownErr!=nil {
  _ = cmd.Process.Kill();_ = cmd.Wait()
  if compatCmd!=nil && compatCmd.Process!=nil { _ = compatCmd.Process.Kill();<-compatDone }
  appendLog(logPath,"ERROR owning OpenOMSI child processes: "+ownErr.Error()+"\r\n")
  showLaunchError(cfg.Language,ownErr.Error())
  return
 }
 defer closeGameJob()
	appendLog(logPath, fmt.Sprintf("openOMSI started. PID=%d\r\n", cmd.Process.Pid))
	launchDone := make(chan struct{})
	watchersStopped:=false
	stopWatchers:=func(){if !watchersStopped {close(launchDone);watchersStopped=true}}
	defer stopWatchers()
	var networkReady <-chan struct{}
	var multiplayerErrors <-chan error
	if multiplayer != nil {
		watch := watchMultiplayerLaunch(logPath, openOMSIOutputStart, multiplayer, launchDone)
		networkReady, multiplayerErrors = watch.Ready, watch.Errors
	}
	if compatCmd != nil {
		if e := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\r\n"), 0644); e != nil {
			appendLog(logPath, "WARN: nao foi possivel publicar PID do openOMSI para facade: "+e.Error()+"\r\n")
		} else {
			appendLog(logPath, fmt.Sprintf("Published openOMSI PID %d to facade.\r\n", cmd.Process.Pid))
		}
		watchOpenOMSIReady(logPath, openOMSIOutputStart, openOMSIReadyPath, networkReady, launchDone)
	}

	childDone := make(chan error, 1)
	go func() { childDone <- cmd.Wait() }()
	compatExited := false
 nextGate:=newNextTripGate(trip.ShiftID,decodeText(bcsBeforeLaunch))
 nextTicker:=time.NewTicker(500*time.Millisecond)
 defer nextTicker.Stop()
 var nextTrip *TripInfo
 var transitionTimeout <-chan time.Time
 evaluationFrozen:=false
	waiting := true
	for waiting {
		select {
  case <-nextTicker.C:
   if current,e:=os.ReadFile(bcsLog);e==nil {
    text:=decodeText(current)
    if !evaluationFrozen && closureGate.accepts(text) {
     if e:=os.WriteFile(evaluationPath,[]byte(trip.ShiftID),0600);e!=nil{appendLog(logPath,"WARN: evaluation freeze marker: "+e.Error()+"\r\n")}else{evaluationFrozen=true}
    }
    if nextTrip==nil {
     if selected,ok:=nextGate.observe(text);ok {
      nextTrip=&selected
      stopWatchers()
      postOpenOMSIClose(uint32(cmd.Process.Pid))
      transitionTimeout=time.After(10*time.Second)
      appendLog(logPath,fmt.Sprintf("Next BCS shift %s selected after completion of %s; closing the owned game before relaunch.\r\n",selected.ShiftID,trip.ShiftID))
     }
    }
   }
  case <-transitionTimeout:
   transitionTimeout=nil
   appendLog(logPath,"The owned game did not close during the next-trip transition; ending this process only.\r\n")
   closeGameJob()
   _ = cmd.Process.Kill()
		case networkErr := <-multiplayerErrors:
			multiplayerErrors = nil
			appendLog(logPath, "Multiplayer startup guard: "+networkErr.Error()+"\r\n")
			// This child never confirmed the requested world. Stop this launch
			// before it can proceed as an unnoticed single-player BBS trip.
			_ = cmd.Process.Kill()
			if compatCmd != nil && !compatExited {
				_ = compatCmd.Process.Kill()
			}
			showLaunchError(cfg.Language, localText(cfg.Language, "A conexão multiplayer não confirmou a data e o horário desta viagem. O lançamento foi encerrado.\n", "Multiplayer did not confirm this trip's date and time. The launch was stopped.\n", "Multiplayer hat Datum und Uhrzeit dieser Fahrt nicht bestätigt. Der Start wurde beendet.\n")+networkErr.Error())
		case er = <-childDone:
			waiting = false
		case facadeErr := <-compatDone:
			compatExited = true
			compatDone = nil
			appendLog(logPath, fmt.Sprintf("BCS facade exited: %v\r\n", facadeErr))
			finished := false
			deadline := time.Now().Add(2 * time.Second)
			for {
				current, readErr := os.ReadFile(bcsLog)
				if readErr == nil && closureGate.accepts(decodeText(current)) {
					finished = true
					break
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if finished {
				select {
				case er = <-childDone:
					waiting = false
				default:
					n := postOpenOMSIClose(uint32(cmd.Process.Pid))
					appendLog(logPath, fmt.Sprintf("BCS confirmed shift %s and closed its facade; requested graceful openOMSI close on %d window(s).\r\n", trip.ShiftID, n))
				}
			} else {
				appendLog(logPath, "WARN: facade terminou sem confirmacao desta viagem; openOMSI continua aberto para preservar a sessao.\r\n")
			}
		}
	}
	if compatCmd != nil && !compatExited {
		_ = compatCmd.Process.Kill()
		<-compatDone
	}
	closeGameJob()
	stopWatchers()
	if nextTrip!=nil {
		clearLaunchSession(activeStatePath)
		releaseLock()
		self,e:=os.Executable()
		if e==nil { child:=exec.Command(self,"--bcs-next-shift",nextTrip.ShiftID);child.Dir=packageDir;e=child.Start();if e==nil{_ = child.Process.Release()} }
		if e!=nil {appendLog(logPath,"ERROR: next-trip relaunch: "+e.Error()+"\r\n");showLaunchError(cfg.Language,e.Error())}
	}
	if er != nil {
		appendLog(logPath, "openOMSI exited with error/status: "+er.Error()+"\r\n")
	} else {
		appendLog(logPath, "openOMSI exited normally.\r\n")
	}
}
