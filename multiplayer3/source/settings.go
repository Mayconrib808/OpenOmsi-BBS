package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var preferencesMu sync.Mutex

type preferences struct {
	Language           string
	AutomaticUpdates   bool
	MultiplayerEnabled bool
	ProgramDisabled    bool
}

func readPreferences(dir string) preferences {
	preferencesMu.Lock()
	defer preferencesMu.Unlock()
	p := preferences{AutomaticUpdates: true, MultiplayerEnabled: true}
	b, err := os.ReadFile(filepath.Join(configDirectory(dir), "settings.json"))
	if err != nil {
		return p
	}
	var raw struct {
		Language           string `json:"language"`
		AutomaticUpdates   *bool  `json:"automatic_updates"`
		MultiplayerEnabled *bool  `json:"multiplayer_enabled"`
		ProgramDisabled    bool   `json:"program_disabled"`
	}
	if json.Unmarshal(b, &raw) != nil {
		return p
	}
	p.Language = raw.Language
	if raw.AutomaticUpdates != nil {
		p.AutomaticUpdates = *raw.AutomaticUpdates
	}
	if raw.MultiplayerEnabled != nil {
		p.MultiplayerEnabled = *raw.MultiplayerEnabled
	}
	p.ProgramDisabled = raw.ProgramDisabled
	return p
}

// Merge preferences so changing the language never resets the update choice
// or an unknown preference from a newer release.
func writePreferences(dir, lang string, automatic *bool) error {
	preferencesMu.Lock()
	defer preferencesMu.Unlock()
	if lang != "" && !validLanguage(lang) {
		return fmt.Errorf("idioma inválido")
	}
	if err := ensureLayout(dir); err != nil {
		return err
	}
	path := filepath.Join(configDirectory(dir), "settings.json")
	raw := map[string]json.RawMessage{}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &raw) != nil {
			return fmt.Errorf("arquivo de preferências inválido; preservado")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if lang != "" {
		raw["language"], _ = json.Marshal(lang)
	}
	if automatic != nil {
		raw["automatic_updates"], _ = json.Marshal(*automatic)
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, b, 0600)
}

func writeAtomic(path string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".multiplayer-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func writeControls(dir string, multiplayer, disabled *bool) error {
	preferencesMu.Lock()
	defer preferencesMu.Unlock()
	if err := ensureLayout(dir); err != nil {
		return err
	}
	path := filepath.Join(configDirectory(dir), "settings.json")
	raw := map[string]json.RawMessage{}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &raw) != nil {
			return fmt.Errorf("arquivo de preferências inválido; preservado")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if multiplayer != nil {
		raw["multiplayer_enabled"], _ = json.Marshal(*multiplayer)
	}
	if disabled != nil {
		raw["program_disabled"], _ = json.Marshal(*disabled)
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, b, 0600)
}
