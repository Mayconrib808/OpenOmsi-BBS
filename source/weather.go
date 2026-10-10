package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Custom weather is part of upstream 0.2.0 and 0.2.11. Sending values instead
// of a local .owt path also lets the dedicated host distribute one shared sky
// to clients that have different installations.
var weatherRanges = map[string][2]float64{
	"vis": {50, 50000}, "br": {0, 1.5}, "wd": {0, 360}, "ws": {0, 50},
	"t": {-40, 50}, "rh": {0, 100}, "p": {900, 1100}, "c": {0, 4},
	"cb": {50, 5000}, "pt": {0, 2}, "pi": {0, 255}, "wet": {0, 1},
	"snow": {0, 1}, "snowroad": {0, 1},
}

func validBridgeWeather(text string) bool {
	if !strings.HasPrefix(text, "custom:") || len(text) > 512 {
		return false
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(strings.TrimPrefix(text, "custom:"), ";") {
		key, value, ok := strings.Cut(part, "=")
		r, known := weatherRanges[key]
		n, err := strconv.ParseFloat(value, 64)
		if !ok || !known || seen[key] || err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < r[0] || n > r[1] {
			return false
		}
		if (key == "c" || key == "pt" || key == "snow" || key == "snowroad") && n != math.Trunc(n) {
			return false
		}
		seen[key] = true
	}
	return len(seen) == len(weatherRanges)
}

func weatherSections(text string) map[string][]string {
	sections := map[string][]string{}
	key := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			key = strings.ToLower(strings.Trim(line, "[]"))
			sections[key] = nil
		} else if key != "" && line != "" {
			sections[key] = append(sections[key], line)
		}
	}
	return sections
}

func bridgeWeatherFromOWT(data []byte) (string, error) {
	if len(data) == 0 || len(data) > 65536 {
		return "", fmt.Errorf("invalid weather file size")
	}
	s := weatherSections(decodeText(data))
	read := func(key string, count int) ([]float64, error) {
		if len(s[key]) < count {
			return nil, fmt.Errorf("incomplete weather section [%s]", key)
		}
		a := make([]float64, count)
		for i := range a {
			n, e := strconv.ParseFloat(s[key][i], 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("invalid weather value [%s]", key)
			}
			a[i] = n
		}
		return a, nil
	}
	v := map[string][]float64{}
	for key, count := range map[string]int{"fog": 2, "wind": 2, "temp": 2, "press": 1, "precip": 5, "groundwet": 3} {
		a, e := read(key, count)
		if e != nil {
			return "", e
		}
		v[key] = a
	}
	if len(s["clouds"]) < 2 {
		return "", fmt.Errorf("incomplete weather section [clouds]")
	}
	clouds := map[string]int{"-1": 0, "cumulus 1": 1, "cumulus 2": 2, "cumulus 3": 3, "overcast 1": 4}
	cloud, ok := clouds[strings.ToLower(s["clouds"][0])]
	if !ok {
		return "", fmt.Errorf("unsupported weather cloud type")
	}
	base, e := strconv.ParseFloat(s["clouds"][1], 64)
	if e != nil || math.IsNaN(base) || math.IsInf(base, 0) {
		return "", fmt.Errorf("invalid cloud height")
	}
	clamp := func(n, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, n)) }
	// OMSI .owt stores a dew point, not a relative humidity, in [temp].
	t, dew := v["temp"][0], v["temp"][1]
	if t < -100 || t > 100 || dew < -100 || dew > 100 {
		return "", fmt.Errorf("invalid weather temperature")
	}
	a, b := 7.5, 237.3
	if t < 0 {
		a, b = 7.6, 240.7
	}
	abs := 6.1078 * 216.69 * math.Pow(10, a*dew/(b+dew)) / (273.15 + t)
	saturation := 6.112 * math.Exp(17.67*t/(t+243.5)) * 216.7 / (273.15 + t)
	snow, snowroad := 0, 0
	if _, ok := s["snow"]; ok {
		snow = 1
	}
	if _, ok := s["snowonroad"]; ok {
		snowroad = 1
	}
	pt := v["precip"][0]
	if pt < 0 || pt > 2 || pt != math.Trunc(pt) {
		return "", fmt.Errorf("invalid precipitation type")
	}
	wd := math.Mod(v["wind"][0], 360)
	if wd < 0 {
		wd += 360
	}
	text := fmt.Sprintf("custom:vis=%.0f;br=%.3f;wd=%.2f;ws=%.3f;t=%.3f;rh=%.3f;p=%.3f;c=%d;cb=%.0f;pt=%.0f;pi=%.3f;wet=%.4f;snow=%d;snowroad=%d",
		clamp(v["fog"][0], 50, 50000), clamp(v["fog"][1], 0, 1.5), wd, clamp(v["wind"][1], 0, 50), clamp(t, -40, 50), clamp(abs/saturation*100, 0, 100), clamp(v["press"][0], 900, 1100), cloud, clamp(base, 50, 5000), pt, clamp(v["precip"][1], 0, 255), clamp(v["groundwet"][0]/255, 0, 1), snow, snowroad)
	if !validBridgeWeather(text) {
		return "", fmt.Errorf("weather cannot be represented by openOMSI")
	}
	return text, nil
}

// Only the laststn sidecar belonging to this map and calendar date is eligible.
// Stale situations are not evidence of the weather selected for a new BCS trip.
func readBCSWeather(root, mapFile, date string, now time.Time) (string, string, error) {
	mapPath := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(mapFile, "\\", "/")))
	paths := []string{filepath.Join(filepath.Dir(mapPath), "laststn.osn"), filepath.Join(root, "laststn.osn")}
	var lastErr error
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		if len(b) > 1<<20 {
			lastErr = fmt.Errorf("situation exceeds 1 MiB")
			continue
		}
		s := weatherSections(decodeText(b))
		savedMap := ""
		if len(s["map"]) > 0 {
			savedMap = filepath.FromSlash(strings.ReplaceAll(s["map"][0], "\\", "/"))
			if filepath.IsAbs(savedMap) {
				if relative, err := filepath.Rel(root, savedMap); err == nil {
					savedMap = relative
				}
			}
		}
		if companyAssetKey(savedMap) != companyAssetKey(mapFile) || len(s["time"]) < 2 {
			lastErr = fmt.Errorf("situation belongs to a different map")
			continue
		}
		y, e1 := strconv.Atoi(s["time"][0])
		d, e2 := strconv.Atoi(s["time"][1])
		day := time.Date(y, 1, d, 0, 0, 0, 0, time.UTC)
		if e1 != nil || e2 != nil || y < 1900 || y > 2200 || d < 1 || d > 366 || day.Year() != y || day.Format("2006-01-02") != date {
			lastErr = fmt.Errorf("situation belongs to a different date")
			continue
		}
		weatherPath := path + ".owt"
		st, err := os.Stat(weatherPath)
		if err != nil {
			lastErr = err
			continue
		}
		if now.Sub(st.ModTime()) > startupTimeout || st.ModTime().Sub(now) > time.Minute {
			lastErr = fmt.Errorf("weather snapshot is older than this startup window")
			continue
		}
		data, err := os.ReadFile(weatherPath)
		if err != nil {
			lastErr = err
			continue
		}
		// A writer can temporarily expose a partial preset. Require a stable pair.
		time.Sleep(100 * time.Millisecond)
		again, err := os.ReadFile(weatherPath)
		situation, serr := os.ReadFile(path)
		if err != nil || serr != nil || !bytes.Equal(data, again) || !bytes.Equal(b, situation) {
			lastErr = fmt.Errorf("BCS is still writing the weather snapshot")
			continue
		}
		weather, err := bridgeWeatherFromOWT(data)
		if err != nil {
			lastErr = err
			continue
		}
		return weather, weatherPath, nil
	}
	return "", "", lastErr
}
