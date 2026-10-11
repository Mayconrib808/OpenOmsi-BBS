package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func collectLogs(dir string, c Config) (string, error) {
	runtimeDir := appDir(dir)
	outputDir := filepath.Join(dir, "Diagnosticos")
	if e := os.MkdirAll(outputDir, 0755); e != nil {
		return "", e
	}
	name := fmt.Sprintf("BCS_logs_v%s_%s.zip", bridgeVersion, time.Now().Format("20060102-150405.000"))
	tmp, e := os.CreateTemp(outputDir, ".logs-*.tmp")
	if e != nil {
		return "", e
	}
	temp := tmp.Name()
	defer os.Remove(temp)
	archive := zip.NewWriter(tmp)
	files := []string{"bridge.ini", "pedepe-native-v2.1.log", "bridge-v1.1.3.log", "bridge-v1.1.3-trip.txt", "setup-v1.1.3.log", "multiplayer-requirements.html", "compat/compat-facade-v1.1.3.log", "compat/driver-state-v1.1.3.txt", "compat/bcs-driver-before-v1.1.3.odr", "compat/bcs-driver-openomsi-v1.1.3.odr", "compat/bcs-driver-current-v1.1.3.odr", "compat/bcs-log-path-v1.1.3.txt"}
	sources := map[string]string{}
	// Only the agent log is collected. HostAgent.local.json has private keys.
	sources["HostAgent.log"] = filepath.Join(managedDataDir(dir), "HostAgent.log")
	for _, rel := range files {
		sources[filepath.Base(rel)] = filepath.Join(runtimeDir, filepath.FromSlash(rel))
	}
	if filepath.IsAbs(c.Root) {
		sources["bbs-native-current.odr"] = filepath.Join(c.Root, "Drivers", "bbs.odr")
	}
	if ref, e := os.ReadFile(runtimeBCSLogPath(runtimeDir)); e == nil {
		p := strings.TrimSpace(string(ref))
		if filepath.IsAbs(p) {
			sources["bcs-log.txt"] = p
		}
	}
	if sources["bcs-log.txt"] == "" {
		if filepath.IsAbs(c.BCSLog) {
			sources["bcs-log.txt"] = c.BCSLog
		} else if filepath.IsAbs(c.Root) {
			for _, folder := range []string{"Busbetrieb-Simulator", "Bus Company Simulator", "Busbetriebs-Simulator", "BBS"} {
				path := filepath.Join(c.Root, folder, "log.txt")
				if fileExists(path) {
					sources["bcs-log.txt"] = path
					break
				}
			}
		}
	}
	sourceList := timetableSourceListPath(runtimeDir)
	sources[filepath.Base(sourceList)] = sourceList
	timetableSources, notes := collectTimetableSources(c.Root, sourceList)
	for label, path := range timetableSources {
		sources[label] = path
	}
	var labels []string
	for label := range sources {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	total := int64(0)
	for _, label := range labels {
		p := sources[label]
		input, e := os.Open(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			notes = append(notes, label+": "+e.Error())
			continue
		}
		st, e := input.Stat()
		if e != nil || !st.Mode().IsRegular() {
			input.Close()
			notes = append(notes, label+": not a readable regular file")
			continue
		}
		if st.Size() > 64<<20 || total+st.Size() > 128<<20 {
			input.Close()
			notes = append(notes, label+": skipped (size limit) ")
			continue
		}
		entry, e := archive.Create(label)
		if e != nil {
			input.Close()
			archive.Close()
			tmp.Close()
			return "", e
		}
		n, e := io.Copy(entry, io.LimitReader(input, st.Size()))
		input.Close()
		total += n
		if e != nil {
			notes = append(notes, label+": "+e.Error())
		}
	}
	entry, e := archive.Create("PLUGIN_HOST.txt")
	if e == nil {
		_, e = io.WriteString(entry, pluginHostDiagnostics(c, dir))
	}
	if e != nil {
		archive.Close()
		tmp.Close()
		return "", e
	}
	entry, e = archive.Create("COLLECTION.txt")
	if e == nil {
		fmt.Fprintf(entry, "OpenOMSI BCS Bridge v%s - by %s\r\nCollected: %s\r\nReview this archive before sharing: game logs and profiles can contain local paths and account identifiers. No automatic upload.\r\n%s\r\n", bridgeVersion, bridgeAuthor, time.Now().Format(time.RFC3339), strings.Join(notes, "\r\n"))
	}
	if e != nil {
		archive.Close()
		tmp.Close()
		return "", e
	}
	if e = archive.Close(); e != nil {
		tmp.Close()
		return "", e
	}
	if e = tmp.Close(); e != nil {
		return "", e
	}
	dest := filepath.Join(outputDir, name)
	if _, e = os.Stat(dest); e == nil {
		return "", fmt.Errorf("log archive already exists")
	}
	if e = os.Rename(temp, dest); e != nil {
		return "", e
	}
	return dest, nil
}
