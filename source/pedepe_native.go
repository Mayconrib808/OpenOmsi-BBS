package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PeDePe's native BBS integration already chooses the map, bus, timetable,
// driver and trip-evaluation path. The 2.1 adapter treats that command line as
// authoritative, restoring only bridge behaviour PeDePe does not request
// (notably openOMSI --autostart for a real scheduled BBS trip) and adding the
// company multiplayer join when required.
type pedepeNativeInvocation struct {
	Root, Map, Bus, Date, Clock, Weather string
	Situation, Line, Tour, Trip          string
	HasSchedule, HasAutoStart            bool
	HasLANJoin, Server, Probe            bool
}

type pedepeSituationMetadata struct {
	Map, Bus, Date, Clock string
}

func pedepeArgValue(args []string, name string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"="), true
		}
	}
	return "", false
}

func pedepeHasArg(args []string, name string) bool {
	_, ok := pedepeArgValue(args, name)
	if ok {
		return true
	}
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}

func parsePeDePeNativeInvocation(args []string) pedepeNativeInvocation {
	var in pedepeNativeInvocation
	in.Root, _ = pedepeArgValue(args, "--root")
	in.Map, _ = pedepeArgValue(args, "--map")
	in.Bus, _ = pedepeArgValue(args, "--bus")
	in.Date, _ = pedepeArgValue(args, "--date")
	in.Clock, _ = pedepeArgValue(args, "--time")
	in.Weather, _ = pedepeArgValue(args, "--weather")
	in.Situation, _ = pedepeArgValue(args, "--situation")
	in.Line, _ = pedepeArgValue(args, "--line")
	in.Tour, _ = pedepeArgValue(args, "--tour")
	in.Trip, _ = pedepeArgValue(args, "--trip")
	in.HasSchedule = pedepeHasArg(args, "--schedule")
	in.HasAutoStart = pedepeHasArg(args, "--autostart")
	in.HasLANJoin = pedepeHasArg(args, "--lan-join")
	in.Server = pedepeHasArg(args, "--server")
	in.Probe = pedepeHasArg(args, "--help") || pedepeHasArg(args, "-h") || pedepeHasArg(args, "--version") || pedepeHasArg(args, "-V")
	return in
}

func normalizePeDePeAsset(root, asset string) string {
	asset = strings.TrimSpace(asset)
	if asset == "" {
		return ""
	}
	if filepath.IsAbs(asset) && filepath.IsAbs(root) {
		if rel, err := filepath.Rel(root, asset); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			asset = rel
		}
	}
	return filepath.Clean(asset)
}

func pedepeMapName(mapFile string) string {
	mapFile = filepath.Clean(strings.TrimSpace(mapFile))
	if mapFile == "" || mapFile == "." {
		return ""
	}
	dir := filepath.Dir(mapFile)
	if strings.EqualFold(filepath.Base(mapFile), "global.cfg") || strings.EqualFold(filepath.Base(mapFile), "global.cfgx") {
		return filepath.Base(dir)
	}
	if base := filepath.Base(dir); base != "." && base != string(filepath.Separator) {
		return base
	}
	return strings.TrimSuffix(filepath.Base(mapFile), filepath.Ext(mapFile))
}

func pedepeSituationNext(lines []string, start int) (string, int, bool) {
	for i := start; i < len(lines); i++ {
		v := strings.TrimSpace(lines[i])
		if v != "" {
			return v, i, true
		}
	}
	return "", len(lines), false
}

func peDePeSituationPath(root, raw string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	raw = strings.TrimSpace(raw)
	if root == "" || !filepath.IsAbs(root) {
		return "", fmt.Errorf("PeDePe situation launch did not provide an absolute OMSI root")
	}
	if raw == "" {
		return "", fmt.Errorf("PeDePe situation path is empty")
	}
	path := filepath.Clean(raw)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("PeDePe situation is outside the configured OMSI root: %s", path)
	}
	return path, nil
}

func peDePeSituationTime(lines []string, start int) (string, string, bool) {
	values := make([]string, 0, 5)
	pos := start
	for len(values) < 5 {
		v, idx, ok := pedepeSituationNext(lines, pos)
		if !ok {
			return "", "", false
		}
		values = append(values, v)
		pos = idx + 1
	}
	year, e1 := strconv.Atoi(values[0])
	dayOfYear, e2 := strconv.Atoi(values[1])
	hour, e3 := strconv.Atoi(values[2])
	minute, e4 := strconv.Atoi(values[3])
	second, e5 := strconv.ParseFloat(strings.ReplaceAll(values[4], ",", "."), 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || year < 1970 || year > 9999 || dayOfYear < 1 || dayOfYear > 366 || hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second >= 60 {
		return "", "", false
	}
	day := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, dayOfYear-1)
	if day.Year() != year {
		return "", "", false
	}
	return day.Format("2006-01-02"), fmt.Sprintf("%02d:%02d:%02d", hour, minute, int(second)), true
}

// PeDePe's OpenOMSI beta currently launches a saved BBS situation instead of
// passing --map/--bus/--date/--time directly. Resolve only the stable OMSI
// metadata needed by the company multiplayer layer: map, player bus and date/time.
func readPeDePeSituation(root, raw string) (pedepeSituationMetadata, error) {
	var meta pedepeSituationMetadata
	path, err := peDePeSituationPath(root, raw)
	if err != nil {
		return meta, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return meta, fmt.Errorf("could not read PeDePe situation: %s", path)
	}
	if info.Size() > 32<<20 {
		return meta, fmt.Errorf("PeDePe situation is unexpectedly large: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	lines := strings.Split(strings.ReplaceAll(decodeText(b), "\r\n", "\n"), "\n")
	var vehicles []string
	currentVehicle := -1
	playerVehicle := -1
	myVehicle := -1
	for i := 0; i < len(lines); i++ {
		switch strings.ToLower(strings.TrimSpace(lines[i])) {
		case "[map]":
			if v, j, ok := pedepeSituationNext(lines, i+1); ok {
				meta.Map = v
				i = j
			}
		case "[time]":
			if date, clock, ok := peDePeSituationTime(lines, i+1); ok {
				meta.Date, meta.Clock = date, clock
			}
		case "[vehicle]":
			if v, j, ok := pedepeSituationNext(lines, i+1); ok {
				vehicles = append(vehicles, v)
				currentVehicle = len(vehicles) - 1
				i = j
			}
		case "[ismyvehicle]":
			if currentVehicle >= 0 {
				playerVehicle = currentVehicle
			}
		case "[myvehicle]":
			if v, j, ok := pedepeSituationNext(lines, i+1); ok {
				if n, parseErr := strconv.Atoi(strings.TrimSpace(v)); parseErr == nil {
					myVehicle = n
				}
				i = j
			}
		}
	}
	if meta.Map == "" {
		// laststn.osn lives inside maps/<map>/; use that location when older
		// situations omit the explicit [map] entry.
		meta.Map = filepath.Join(filepath.Dir(path), "global.cfg")
	}
	if playerVehicle < 0 && myVehicle >= 0 && myVehicle < len(vehicles) {
		playerVehicle = myVehicle
	}
	if playerVehicle < 0 && len(vehicles) == 1 {
		playerVehicle = 0
	}
	if playerVehicle >= 0 && playerVehicle < len(vehicles) {
		meta.Bus = vehicles[playerVehicle]
	}
	return meta, nil
}

// PeDePe's current native beta launches BBS trips from laststn.osn and passes
// --schedule/--line/--tour/--trip, but it does not pass --autostart. The legacy
// bridge did, and openOMSI uses it to put the player bus into service before the
// run (the path that also performs the automatic IBIS/destination setup).
func shouldAddPeDePeAutoStart(in pedepeNativeInvocation) bool {
	if in.Probe || in.Server || in.HasAutoStart || !in.HasSchedule {
		return false
	}
	return strings.TrimSpace(in.Line) != "" && strings.TrimSpace(in.Tour) != ""
}

// A direct native launch has --map/--time. PeDePe's current beta instead sends
// --situation plus the complete schedule identity; both are valid company-trip
// forms. Probes, server mode and a future native --lan-join always stay untouched.
func shouldInjectPeDePeMultiplayer(in pedepeNativeInvocation, c Config) bool {
	if !c.Multiplayer || in.Probe || in.Server || in.HasLANJoin {
		return false
	}
	if strings.TrimSpace(in.Map) != "" && strings.TrimSpace(in.Clock) != "" && (strings.TrimSpace(in.Bus) != "" || in.HasSchedule) {
		return true
	}
	return in.HasSchedule && strings.TrimSpace(in.Situation) != "" && strings.TrimSpace(in.Line) != "" && strings.TrimSpace(in.Tour) != "" && strings.TrimSpace(in.Trip) != ""
}

func peDePeMultiplayerTrip(c Config, in pedepeNativeInvocation, now time.Time) (MultiplayerTrip, error) {
	root := strings.TrimSpace(in.Root)
	if root == "" || !filepath.IsAbs(root) {
		root = c.Root
	}
	mapValue := strings.TrimSpace(in.Map)
	busValue := strings.TrimSpace(in.Bus)
	date := strings.TrimSpace(in.Date)
	clock := strings.TrimSpace(in.Clock)

	if strings.TrimSpace(in.Situation) != "" && (mapValue == "" || busValue == "" || date == "" || clock == "") {
		meta, err := readPeDePeSituation(root, in.Situation)
		if err != nil {
			return MultiplayerTrip{}, err
		}
		if mapValue == "" {
			mapValue = meta.Map
		}
		if busValue == "" {
			busValue = meta.Bus
		}
		if date == "" {
			date = meta.Date
		}
		if clock == "" {
			// PeDePe currently uses --trip as the BBS departure clock (for
			// example 04:05). Prefer that over the saved-situation clock because
			// it identifies the selected trip; fall back to [time] for future
			// builds where --trip is a name or ordinal instead.
			if candidate := strings.TrimSpace(in.Trip); candidate != "" {
				if _, err := multiplayerClock(candidate); err == nil {
					clock = candidate
				}
			}
			if clock == "" {
				clock = meta.Clock
			}
		}
	}

	mapFile := normalizePeDePeAsset(root, mapValue)
	busFile := normalizePeDePeAsset(root, busValue)
	if date == "" {
		date = now.Format("2006-01-02")
	}
	if mapFile == "" || clock == "" {
		return MultiplayerTrip{}, fmt.Errorf("PeDePe native launch did not provide or resolve map/time")
	}
	if _, err := multiplayerClock(clock); err != nil {
		return MultiplayerTrip{}, err
	}
	return MultiplayerTrip{
		MapName: pedepeMapName(mapFile),
		MapFile: mapFile,
		BusFile: busFile,
		Date:    date,
		Start:   clock,
		Weather: strings.TrimSpace(in.Weather),
	}, nil
}

func peDePeCompanyPlan(ctx context.Context, c Config, packageDir string, args []string) (*MultiplayerPlan, error) {
	in := parsePeDePeNativeInvocation(args)
	if !shouldInjectPeDePeMultiplayer(in, c) {
		return nil, nil
	}
	trip, err := peDePeMultiplayerTrip(c, in, time.Now())
	if err != nil {
		return nil, err
	}
	plan, problems, err := prepareMultiplayerLaunch(ctx, c, appDir(packageDir), trip, companyHTTPClient())
	if err != nil {
		return nil, err
	}
	if plan != nil {
		return plan, nil
	}
	if len(problems) == 0 {
		return nil, fmt.Errorf("no compatible company multiplayer session is available")
	}
	parts := make([]string, 0, len(problems))
	for _, problem := range problems {
		label := strings.TrimSpace(problem.Name)
		if label == "" {
			label = "Multiplayer"
		}
		parts = append(parts, label+": "+strings.TrimSpace(problem.Detail))
	}
	return nil, fmt.Errorf("%s", strings.Join(parts, "; "))
}

func peDePeForwardArgs(original []string, plan *MultiplayerPlan) []string {
	out := append([]string(nil), original...)
	if shouldAddPeDePeAutoStart(parsePeDePeNativeInvocation(original)) {
		out = append(out, "--autostart")
	}
	if plan != nil {
		out = append(out, multiplayerArguments(plan)...)
	}
	return out
}
