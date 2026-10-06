package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockedLaunchDiagnosticsIncludeBCSAndSelectedTimetable(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(t.TempDir(), "OMSI")
	tt := filepath.Join(root, "maps", "Sample", "TTData")
	line := filepath.Join(tt, "10.ttl")
	profile := filepath.Join(tt, "OUT.ttp")
	writeTestFile(t, line, "selected line")
	writeTestFile(t, profile, "selected profile")
	writeTestFile(t, filepath.Join(tt, "UNRELATED.ttp"), "do not collect unrelated routes")
	writeTestFile(t, filepath.Join(root, "Busbetrieb-Simulator", "log.txt"), "BCS log before facade startup")
	writeTestFile(t, filepath.Join(appDir(dir), "bridge-v"+bridgeVersion+".log"), "launch blocked before facade")
	if err := writeTimetableSourceList(root, appDir(dir), []string{line, profile, profile}); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Root = root
	path, err := collectLogs(dir, c)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	found := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		found[f.Name] = string(b)
	}
	for name, want := range map[string]string{
		"bcs-log.txt":                          "BCS log before facade startup",
		"timetable/maps/Sample/TTData/10.ttl":  "selected line",
		"timetable/maps/Sample/TTData/OUT.ttp": "selected profile",
	} {
		if found[name] != want {
			t.Fatalf("blocked-launch ZIP is missing %s: %v", name, found)
		}
	}
	if _, ok := found["timetable/maps/Sample/TTData/UNRELATED.ttp"]; ok {
		t.Fatal("unrelated timetable was collected")
	}
}

func TestTimetableDiagnosticsRejectOutsideOrUnsupportedSources(t *testing.T) {
	root := t.TempDir()
	sourceList := filepath.Join(t.TempDir(), "sources.txt")
	rel := "maps/Sample/TTData/OUT.ttp"
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), "selected profile")
	outside := filepath.Join(t.TempDir(), "private.ttp")
	writeTestFile(t, outside, "private")
	lines := rel + "\n" + rel + "\n../private.ttp\n/maps/Other.ttp\nmaps/Sample/TTData/../../../../private.ttp\nmaps/Sample/secret.exe\nmaps/Sample/secret.txt\nC:/private.ttp\n"
	link := filepath.Join(root, "maps", "Sample", "TTData", "link.ttp")
	if err := os.Symlink(outside, link); err == nil {
		lines += "maps/Sample/TTData/link.ttp\n"
	}
	writeTestFile(t, sourceList, lines)
	files, notes := collectTimetableSources(root, sourceList)
	if len(files) != 1 || files["timetable/"+rel] != filepath.Join(root, filepath.FromSlash(rel)) {
		t.Fatalf("only the selected in-root profile should be collected: %v", files)
	}
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, "\n"), "skipped") {
		t.Fatal("skipped sources were not recorded", notes)
	}
}
