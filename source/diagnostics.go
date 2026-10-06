package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runtimeBCSLogPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, "compat", "bcs-log-path-v"+bridgeVersion+".txt")
}

func timetableSourceListPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, "timetable-sources-v"+bridgeVersion+".txt")
}

func safeTimetableRelative(rel string) bool {
	if !strings.HasPrefix(strings.ToLower(rel), "maps/") || strings.ContainsAny(rel, "\\:\r\n") {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".ttl", ".ttp", ".cfg":
		return true
	}
	return false
}

// Record only the selected line, its loaded profiles, and Chrono configuration.
// No map assets or files outside the configured OMSI installation are selected.
func writeTimetableSourceList(root, runtimeDir string, paths []string) error {
	var b strings.Builder
	seen := map[string]bool{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if err != nil || !safeTimetableRelative(rel) || seen[strings.ToLower(rel)] {
			continue
		}
		seen[strings.ToLower(rel)] = true
		fmt.Fprintln(&b, rel)
		if len(seen) >= 512 {
			break
		}
	}
	return os.WriteFile(timetableSourceListPath(runtimeDir), []byte(b.String()), 0644)
}

func collectTimetableSources(root, sourceList string) (map[string]string, []string) {
	out := map[string]string{}
	var notes []string
	if !filepath.IsAbs(root) {
		return out, notes
	}
	f, err := os.Open(sourceList)
	if os.IsNotExist(err) {
		return out, notes
	}
	if err != nil {
		return out, []string{"timetable source list: " + err.Error()}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 1<<20 {
		return out, []string{"timetable source list: invalid file or size"}
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return out, []string{"timetable root: " + err.Error()}
	}
	b, err := os.ReadFile(sourceList)
	if err != nil {
		return out, []string{"timetable source list: " + err.Error()}
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		rel := strings.TrimSpace(line)
		if rel == "" {
			continue
		}
		if !safeTimetableRelative(rel) {
			notes = append(notes, "timetable: skipped unsafe or unsupported source")
			continue
		}
		key := strings.ToLower(rel)
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(seen) > 512 {
			notes = append(notes, "timetable: further sources skipped (512-file limit)")
			break
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		full, err := filepath.EvalSymlinks(path)
		if err != nil {
			notes = append(notes, "timetable/"+rel+": "+err.Error())
			continue
		}
		inside, err := filepath.Rel(base, full)
		if err != nil || filepath.IsAbs(inside) || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
			notes = append(notes, "timetable/"+rel+": skipped source outside OMSI root")
			continue
		}
		out["timetable/"+rel] = path
	}
	return out, notes
}
