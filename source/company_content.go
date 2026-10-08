package main

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
)

var companySituationTile = regexp.MustCompile(`(?i)^.+\.osn_-?[0-9]+_-?[0-9]+\.dds$`)

// Separate mutable/generated inventory policy from path validation so older
// profiles that recorded runtime companions or OS metadata can still be read
// and upgraded without making those machine-local files multiplayer identity.
func companyRuntimeArtifact(path string) bool {
	parts := strings.Split(companyAssetKey(path), "/")
	if len(parts) == 0 {
		return false
	}
	name := parts[len(parts)-1]
	switch name {
	case "thumbs.db", "ehthumbs.db", "desktop.ini", ".ds_store":
		return true
	}
	if len(parts) != 3 || parts[0] != "maps" {
		return false
	}
	return strings.HasSuffix(name, ".osn.owt") || companySituationTile.MatchString(name) ||
		name == "holidays.txt" || name == "timezone.txt.backup.txt" || name == "holidays.txt.backup.txt"
}

// Some OMSI map archives have circulated with Windows' hidden-extension mistake
// "timezone.txt.txt". The profile keeps the canonical OMSI name timezone.txt;
// if only the doubled name exists, verification may use it as a byte-for-byte
// compatibility alias. The hash still has to match the declared timezone.txt.
func companyCompatibilityAlias(path string) string {
	normalized := strings.ReplaceAll(path, `\`, "/")
	parts := strings.Split(companyAssetKey(normalized), "/")
	if len(parts) == 3 && parts[0] == "maps" && parts[2] == "timezone.txt" {
		return normalized + ".txt"
	}
	return ""
}

func companyCalendarAsset(path string) bool {
	parts := strings.Split(companyAssetKey(path), "/")
	return len(parts) == 3 && parts[0] == "maps" && filepath.Base(parts[2]) == "holidays.txt"
}

// BBS records originals it temporarily modifies in BBS_Backups.txt. Vehicle
// program inputs may use a verified original as their package identity. The
// map Holidays.txt is mutable runtime state and is excluded earlier instead of
// relying on backup discovery. Models, textures, timezone and timetable files
// stay exact.
func companyBBSManagedAsset(path string) bool {
	if companyCalendarAsset(path) {
		return true
	}
	key := companyAssetKey(path)
	parts := strings.Split(key, "/")
	if len(parts) < 3 || parts[0] != "vehicles" {
		return false
	}
	if len(parts) == 3 && strings.HasSuffix(key, ".bus") {
		return true
	}
	if !strings.Contains(key, "/script/") && !strings.Contains(key, "/scripts/") {
		return false
	}
	return strings.HasSuffix(key, ".osc") || strings.HasSuffix(key, "_varlist.txt") || strings.HasSuffix(key, "_stringvarlist.txt")
}

func companyBBSBackupInventory(root string, additionalLists ...string) map[string]bool {
	result := map[string]bool{}
	root, err := filepath.Abs(root)
	if err != nil {
		return result
	}
	prefix := strings.TrimRight(companyAssetKey(root), "/") + "/"
	paths := append([]string(nil), additionalLists...)
	for _, directory := range []string{"Busbetrieb-Simulator", "Busbetriebs-Simulator", "Bus Company Simulator", "BBS", ""} {
		paths = append(paths, filepath.Join(root, directory, "BBS_Backups.txt"))
	}
	for _, path := range paths {
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || st.Size() > 4<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(companyBBSBackupText(data), "\n") {
			key := companyAssetKey(strings.Trim(strings.TrimSpace(line), `"`))
			if strings.HasPrefix(key, prefix) {
				key = strings.TrimPrefix(key, prefix)
			}
			if validCompanyAsset(key) && companyBBSManagedAsset(key) {
				result[key] = true
			}
		}
	}
	return result
}

// CompanyHost does not link the launcher's text helpers. Accept both Unicode
// encodings used by Windows BBS logs without pulling game-launch code into it.
func companyBBSBackupText(data []byte) string {
	var order binary.ByteOrder
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		order = binary.LittleEndian
	} else if len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff {
		order = binary.BigEndian
	}
	if order == nil {
		return strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	}
	units := make([]uint16, 0, (len(data)-2)/2)
	for i := 2; i+1 < len(data); i += 2 {
		units = append(units, order.Uint16(data[i:i+2]))
	}
	return string(utf16.Decode(units))
}

func companyBBSOriginalMatches(root string, file CompanyFile, active map[string]bool) bool {
	if !active[companyAssetKey(file.Path)] || !companyBBSManagedAsset(file.Path) {
		return false
	}
	live, err := companyAssetPath(root, file.Path)
	if err != nil {
		return false
	}
	backup := live + ".backup"
	st, err := os.Lstat(backup)
	if err != nil || !st.Mode().IsRegular() || st.Size() > 32<<20 {
		return false
	}
	// The live file must remain readable: a matching backup cannot replace a
	// missing or inaccessible game input. Do not restore or change either file.
	f, err := os.Open(live)
	if err != nil {
		return false
	}
	_, err = io.Copy(io.Discard, f)
	_ = f.Close()
	if err != nil {
		return false
	}
	digest, err := companyFileHash(backup)
	return err == nil && strings.EqualFold(digest, file.SHA256)
}
