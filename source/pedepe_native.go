package main

import (
	"context"
	"fmt"
	"path/filepath"
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

// Only treat a launch as a company multiplayer trip when PeDePe supplied enough
// direct map/time context. Situation-based launches are handled separately once
// their saved situation metadata has been resolved; never guess a map or clock.
func shouldInjectPeDePeMultiplayer(in pedepeNativeInvocation, c Config) bool {
	if !c.Multiplayer || in.Probe || in.Server || in.HasLANJoin {
		return false
	}
	return strings.TrimSpace(in.Map) != "" && strings.TrimSpace(in.Clock) != "" && (strings.TrimSpace(in.Bus) != "" || in.HasSchedule)
}

func peDePeMultiplayerTrip(c Config, in pedepeNativeInvocation, now time.Time) (MultiplayerTrip, error) {
	root := strings.TrimSpace(in.Root)
	if root == "" || !filepath.IsAbs(root) {
		root = c.Root
	}
	mapFile := normalizePeDePeAsset(root, in.Map)
	busFile := normalizePeDePeAsset(root, in.Bus)
	date := strings.TrimSpace(in.Date)
	if date == "" {
		date = now.Format("2006-01-02")
	}
	clock := strings.TrimSpace(in.Clock)
	if mapFile == "" || clock == "" {
		return MultiplayerTrip{}, fmt.Errorf("PeDePe native launch did not provide map/time")
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
