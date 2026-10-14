package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type pedepeFuelSnapshot struct {
	Bus    string  `json:"bus"`
	Field  string  `json:"field"`
	Litres float64 `json:"litres"`
}

// Fuel belongs to the selected player vehicle, never the first AI bus. Map
// names, bus names, capacities and fuel amounts are not hard-coded.
func parsePeDePeFuelSnapshot(data []byte, root string) (*pedepeFuelSnapshot, error) {
	lines := strings.Split(strings.ReplaceAll(decodeText(data), "\r\n", "\n"), "\n")
	type vehicle struct {
		bus   string
		fuel  map[string]string
		names map[string]string
	}
	var vehicles []vehicle
	current, marked, selected := -1, -1, -1
	for i := 0; i < len(lines); i++ {
		switch strings.ToLower(strings.TrimSpace(lines[i])) {
		case "[vehicle]":
			bus, j, ok := pedepeSituationNext(lines, i+1)
			if !ok {
				return nil, fmt.Errorf("saved vehicle has no bus path")
			}
			vehicles = append(vehicles, vehicle{bus: bus, fuel: map[string]string{}, names: map[string]string{}})
			current, i = len(vehicles)-1, j
		case "[ismyvehicle]":
			marked = current
		case "[myvehicle]":
			if value, j, ok := pedepeSituationNext(lines, i+1); ok {
				if index, e := strconv.Atoi(value); e == nil {
					selected = index
				}
				i = j
			}
		case "[vars]":
			if current < 0 {
				continue
			}
			count, j, ok := pedepeSituationNext(lines, i+1)
			n, err := strconv.Atoi(count)
			if !ok || err != nil || n < 0 || n > 100000 {
				return nil, fmt.Errorf("invalid saved fuel variable list")
			}
			pos := j + 1
			for k := 0; k < n; k++ {
				name, a, ok := pedepeSituationNext(lines, pos)
				if !ok {
					return nil, fmt.Errorf("incomplete saved fuel variable list")
				}
				value, z, ok := pedepeSituationNext(lines, a+1)
				if !ok {
					return nil, fmt.Errorf("incomplete saved fuel variable value")
				}
				field := strings.ToLower(strings.TrimSpace(name))
				if field == "engine_tank_content" || field == "fuel" {
					vehicles[current].fuel[field] = value
					vehicles[current].names[field] = strings.TrimSpace(name)
				}
				pos = z + 1
			}
			i = pos - 1
		}
	}
	if marked < 0 && selected >= 0 && selected < len(vehicles) {
		marked = selected
	}
	if marked < 0 && len(vehicles) == 1 {
		marked = 0
	}
	if marked < 0 || marked >= len(vehicles) {
		return nil, fmt.Errorf("saved situation does not identify the player bus")
	}
	v := vehicles[marked]
	for _, field := range []string{"engine_tank_content", "fuel"} {
		raw, found := v.fuel[field]
		if !found {
			continue
		}
		litres, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
		if err != nil || math.IsNaN(litres) || math.IsInf(litres, 0) || litres < 0 || litres > math.MaxFloat32 {
			return nil, fmt.Errorf("invalid saved player fuel: %s", raw)
		}
		return &pedepeFuelSnapshot{Bus: normalizePeDePeAsset(root, v.bus), Field: v.names[field], Litres: litres}, nil
	}
	return nil, nil // Missing fuel is not zero; retain the bus's own initialization.
}

func peDePeSavedFuel(in pedepeNativeInvocation, root string) (*pedepeFuelSnapshot, error) {
	if in.Situation == "" || !in.HasSchedule || in.Line == "" || in.Tour == "" || in.Trip == "" || in.Probe || in.Server {
		return nil, nil
	}
	if in.Root != "" {
		root = in.Root
	}
	path, err := peDePeSituationPath(root, in.Situation)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > 32<<20 {
		return nil, fmt.Errorf("invalid saved fuel situation")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePeDePeFuelSnapshot(data, root)
}

// Merge into the native startup assignment. It is applied by openOMSI after
// script initialization, before the player's normal simulation/telemetry frames.
func peDePeFuelArgs(args []string, snapshot *pedepeFuelSnapshot, root string) ([]string, error) {
	if snapshot == nil {
		return append([]string(nil), args...), nil
	}
	if nativeRoot, ok := pedepeArgValue(args, "--root"); ok && nativeRoot != "" {
		root = nativeRoot
	}
	if bus, ok := pedepeArgValue(args, "--bus"); ok && companyGatewayBusKey(normalizePeDePeAsset(root, bus)) != companyGatewayBusKey(snapshot.Bus) {
		return nil, fmt.Errorf("saved fuel belongs to a different BCS bus")
	}
	existing, _ := pedepeArgValue(args, "--setvar")
	var assignments []string
	for _, entry := range strings.Split(existing, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		field, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(strings.TrimSpace(field), snapshot.Field) {
			assignments = append(assignments, entry)
		}
	}
	assignments = append(assignments, snapshot.Field+"="+strconv.FormatFloat(snapshot.Litres, 'g', -1, 64))
	out := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		if args[i] == "--setvar" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--setvar=") {
			continue
		}
		out = append(out, args[i])
	}
	return append(out, "--setvar", strings.Join(assignments, ",")), nil
}
