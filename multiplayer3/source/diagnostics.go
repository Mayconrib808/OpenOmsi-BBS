package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var diagnosticAddress = regexp.MustCompile(`https?://[^\s"<>]+`)
var diagnosticSecret = regexp.MustCompile(`(?i)\b[a-f0-9]{32,}\b|Bearer\s+\S+`)

// Export only bounded logs owned by the integration. Never include company
// admin files, invitations, driver records, the proprietary JAR or saved trips.
func diagnosticZIP(dir string, cfg localConfig) ([]byte, error) {
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	add := func(name string, b []byte) error {
		w, e := z.Create(name)
		if e == nil {
			_, e = w.Write(b)
		}
		return e
	}
	clean := func(text string) string {
		for _, value := range []string{cfg.Invitation, cfg.Root, cfg.Game, cfg.Player, dir} {
			if value != "" {
				text = strings.ReplaceAll(text, value, "[redacted]")
			}
		}
		return diagnosticSecret.ReplaceAllString(diagnosticAddress.ReplaceAllString(text, "[address]"), "[secret]")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "logs"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		if name != "setup.log" && name != "multiplayer.log" && name != "update-error.txt" && !(strings.HasPrefix(name, "host-") && strings.HasSuffix(name, ".log")) {
			continue
		}
		b, e := readDiagnosticTail(filepath.Join(dir, "logs", name), 2<<20)
		if e != nil {
			continue
		}
		if e = add("logs/"+name, []byte(clean(string(b)))); e != nil {
			return nil, e
		}
	}
	metadata, _ := json.MarshalIndent(map[string]any{"product": productName, "version": version, "language": language(), "server": cfg.Server, "client_mode": cfg.ClientMode, "multiplayer": readPreferences(dir).MultiplayerEnabled, "private_settings_included": false}, "", "  ")
	if err = add("diagnostics.json", metadata); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Native server output can exceed the export cap. Preserve its newest evidence
// instead of silently dropping the entire log once it becomes large.
func readDiagnosticTail(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	st, err := f.Stat()
	if err != nil { return nil, err }
	if st.Size() > limit {
		if _, err = f.Seek(st.Size()-limit, io.SeekStart); err != nil { return nil, err }
	}
	return io.ReadAll(io.LimitReader(f, limit))
}
