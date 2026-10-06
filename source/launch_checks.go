package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read the same settings location and boolean aliases as openOMSI 0.2.0.
// A bad HOME is ignored by openOMSI on Windows; USERPROFILE is its fallback.
func openOMSIUserDir() string {
	if p := os.Getenv("HOME"); p != "" && filepath.IsAbs(p) {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return os.Getenv("USERPROFILE")
}

func readTimeSync(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	on := false
	for _, line := range strings.Split(strings.TrimPrefix(string(b), "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k = strings.ToLower(strings.TrimSpace(k))
		if ok && (k == "time_sync" || k == "real_time_sync") {
			v = strings.ToLower(strings.TrimSpace(v))
			on = v == "1" || v == "true" || v == "on" || v == "yes"
		}
	}
	return on, nil
}

func ensureBCSClock(c Config) error {
	homeDir := openOMSIUserDir()
	if homeDir == "" {
		return fmt.Errorf("%s", localText(c.Language, "Não foi possível localizar as configurações do openOMSI.", "Could not locate openOMSI settings.", "Die openOMSI-Einstellungen konnten nicht gefunden werden."))
	}
	on, err := readTimeSync(filepath.Join(homeDir, ".openomsi", "settings.cfg"))
	if err != nil {
		return err
	}
	if on {
		return fmt.Errorf("%s", localText(c.Language,
			"Desative a sincronização com o horário real nas opções do openOMSI e inicie a viagem novamente pelo BCS. Ela substitui o horário da viagem.",
			"Turn off real-time clock synchronization in openOMSI settings, then start the trip again through BCS. It overrides the trip time.", "Deaktiviere die Synchronisierung mit der Echtzeit in den openOMSI-Einstellungen und starte die Fahrt erneut über BBS. Diese Einstellung überschreibt die Abfahrtszeit der Fahrt."))
	}
	return nil
}

func timetableContentPath(rel, mapDir string) bool {
	rel = strings.ToLower(strings.ReplaceAll(rel, "\\", "/"))
	prefix := strings.ToLower(filepath.ToSlash(mapDir)) + "/"
	if !strings.HasPrefix(rel, prefix) {
		return false
	}
	within := strings.TrimPrefix(rel, prefix)
	return strings.HasPrefix(within, "ttdata/") || (strings.HasPrefix(within, "chrono/") && (strings.Contains(within, "/ttdata/") || strings.HasSuffix(within, "/chrono.cfg")))
}

// The loose content folder has priority over CLI ZIPs in 0.2.0. Environment
// archives and Archives/ also affect TTData/Chrono. Refuse an unexamined
// override instead of synchronizing files different from the game's sources.
// No user settings or map files are changed.
func ensureOriginalTimetableSources(c Config, mapRel string) error {
	mapDir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(mapRel)))
	var contentDirs []string
	if override := os.Getenv("OMSI_CONTENT"); override != "" {
		contentDirs = append(contentDirs, override)
	} else {
		openDir := filepath.Dir(c.OpenOMSI)
		if fileExists(filepath.Join(openDir, "Omsi.exe")) {
			if st, e := os.Stat(filepath.Join(openDir, "maps")); e == nil && st.IsDir() {
				openDir = filepath.Join(openDir, "openOMSI")
			}
		}
		contentDirs = append(contentDirs, openDir)
		if homeDir := openOMSIUserDir(); homeDir != "" {
			contentDirs = append(contentDirs, filepath.Join(homeDir, ".openomsi", "content"))
		}
	}
	var archives []string
	for _, dir := range contentDirs {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(c.Root, dir)
		}
		if !samePath(dir, c.Root) {
			mapPath := filepath.Join(dir, filepath.FromSlash(mapDir))
			var conflict string
			e := filepath.WalkDir(mapPath, func(p string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					if os.IsNotExist(walkErr) {
						return nil
					}
					return walkErr
				}
				if !d.IsDir() {
					rel, _ := filepath.Rel(dir, p)
					if timetableContentPath(filepath.ToSlash(rel), mapDir) {
						conflict = p
						return filepath.SkipAll
					}
				}
				return nil
			})
			if e != nil {
				return e
			}
			if conflict != "" {
				return timetableSourceConflict(c.Language, conflict)
			}
		}
		entries, e := os.ReadDir(filepath.Join(dir, "Archives"))
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
				archives = append(archives, filepath.Join(dir, "Archives", entry.Name()))
			}
		}
	}
	for _, path := range filepath.SplitList(os.Getenv("OMSI_CONTENT_ZIP")) {
		if path != "" {
			if !filepath.IsAbs(path) {
				path = filepath.Join(c.Root, path)
			}
			archives = append(archives, path)
		}
	}
	for _, path := range archives {
		z, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("read content archive %s: %w", path, err)
		}
		conflict := false
		for _, file := range z.File {
			if timetableContentPath(file.Name, mapDir) {
				conflict = true
				break
			}
		}
		z.Close()
		if conflict {
			return timetableSourceConflict(c.Language, path)
		}
	}
	return nil
}

func timetableSourceConflict(lang, path string) error {
	return fmt.Errorf("%s\n%s", localText(lang,
		"O openOMSI tem horários adicionais deste mapa fora da pasta original. Esta versão só alinha os horários da instalação original. Desative esse conteúdo adicional para esta viagem ou use o OMSI original pelo Setup, opção 3. O conteúdo foi preservado:",
		"openOMSI has additional timetables for this map outside the original installation. This version aligns original-installation timetables only. Disable that additional content for this trip or use original OMSI via Setup option 3. Content was preserved:", "openOMSI hat zusätzliche Fahrpläne für diese Karte außerhalb der Originalinstallation. Diese Version gleicht nur die Fahrpläne der Originalinstallation ab. Deaktiviere diese Zusatzinhalte für die Fahrt oder wechsle mit Setup-Option 3 zum originalen OMSI. Die Inhalte wurden unverändert beibehalten:"), path)
}
