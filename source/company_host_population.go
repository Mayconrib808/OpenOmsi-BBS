package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const companyHostDefaultMaxPeople = 1000
const companyHostMaxPeople = 4096

// The native server reads its people budget from the user settings, not server.cfg.
// Give each map its own native profile so simultaneous servers can use different
// budgets without writing the player's settings or changing the parent's environment.
func prepareCompanyHostPopulation(parent []string, config string, maxPeople int) (string, error) {
	if maxPeople < 1 || maxPeople > companyHostMaxPeople {
		return "", fmt.Errorf("population limit must be 1-%d", companyHostMaxPeople)
	}
	get := func(name string) string {
		var value string
		for _, entry := range parent {
			key, v, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, name) {
				value = v
			}
		}
		return value
	}
	profile := get("HOME")
	if profile == "" {
		profile = get("USERPROFILE")
	}
	var settings []byte
	if profile != "" {
		var err error
		settings, err = os.ReadFile(filepath.Join(profile, ".openomsi", "settings.cfg"))
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot read native settings: %w", err)
		}
	}
	if len(settings) > 1<<20 || !utf8.Valid(settings) || strings.ContainsRune(string(settings), 0) {
		return "", fmt.Errorf("native settings must be a UTF-8 text file under 1 MiB")
	}
	settings = []byte(strings.TrimPrefix(string(settings), "\ufeff"))
	// Native settings take the last occurrence. Only the population key is added;
	// the inherited density and the rest of the host's preferences are retained.
	settings = append(settings, []byte("\nai_max_humans="+strconv.Itoa(maxPeople)+"\n")...)
	directory, err := filepath.Abs(filepath.Join(filepath.Dir(config), "native-profile"))
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory, ".openomsi", "settings.cfg")
	if err := writeCompanyHostFileAtomic(path, settings); err != nil {
		return "", fmt.Errorf("cannot prepare this map's population settings: %w", err)
	}
	return directory, nil
}

func companyHostPopulationEnvironment(parent []string, profile string) []string {
	var result []string
	for _, entry := range parent {
		key, _, _ := strings.Cut(entry, "=")
		// Native Settings::path prefers HOME to USERPROFILE. This redirection is
		// scoped to the dedicated child; the agent and the user's game keep theirs.
		if !strings.EqualFold(key, "HOME") && !strings.EqualFold(key, "USERPROFILE") {
			result = append(result, entry)
		}
	}
	return append(result, "USERPROFILE="+profile)
}
