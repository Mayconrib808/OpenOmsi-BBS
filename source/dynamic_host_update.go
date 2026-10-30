package main

// The updater must activate the matching managed P14/P15 sidecar as well as copy it. Keep the
// full P15 HostAgent executable and opaque preferences: its source is not here.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const managedDynamicServer = "app/server-dinamico-p14/openomsi.exe"
const managedDynamicP15Server = "app/server-dinamico-p15/openomsi.exe"

// Preserve all opaque P15 preferences and the existing full host executable.
// Only an installation-managed server is switched to the new Coop protocol.
func migrateManagedCoop15Host(data []byte, dir string) ([]byte, string, bool, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &doc); err != nil || doc == nil {
		return nil, "", false, fmt.Errorf("invalid host settings")
	}
	selected := strings.ToLower(strings.ReplaceAll(hostJSONString(doc, "server"), "\\", "/"))
	managed := false
	_, preference := hostJSONField(doc, "use_custom_server")
	var custom bool
	if preference != nil && json.Unmarshal(preference, &custom) != nil {
		return nil, "", false, fmt.Errorf("invalid custom server preference")
	}
	if !custom {
		managed = true
	}
	for _, relative := range []string{managedDynamicServer, managedDynamicP15Server, "app/server/openomsi.exe"} {
		if strings.HasSuffix(selected, "/"+strings.ToLower(relative)) {
			managed = true
		}
	}
	if !managed {
		return data, "", false, nil
	}
	key, raw := hostJSONField(doc, "company")
	var company map[string]json.RawMessage
	if json.Unmarshal(raw, &company) != nil || company == nil {
		return data, "", false, nil
	}
	_, p := hostJSONField(company, "protocol")
	var protocol int
	if json.Unmarshal(p, &protocol) != nil || (protocol != 14 && protocol != 15) {
		return data, "", false, nil
	}
	id := hostJSONString(company, "company_id")
	if !companyIDPattern.MatchString(id) {
		return data, "", false, nil
	}
	pk, _ := hostJSONField(company, "protocol")
	company[pk] = json.RawMessage("15")
	vk, _ := hostJSONField(company, "openomsi_version")
	company[vk], _ = json.Marshal("0.3.46")
	doc[key], _ = json.Marshal(company)
	sk, _ := hostJSONField(doc, "server")
	doc[sk], _ = json.Marshal(filepath.Join(dir, filepath.FromSlash(managedDynamicP15Server)))
	ck, _ := hostJSONField(doc, "use_custom_server")
	doc[ck] = json.RawMessage("true")
	after, err := json.MarshalIndent(doc, "", "  ")
	after = append(after, '\n')
	return after, id, !bytes.Equal(data, after), err
}

func hostJSONField(m map[string]json.RawMessage, name string) (string, json.RawMessage) {
	for key, value := range m {
		if strings.EqualFold(key, name) {
			return key, value
		}
	}
	return name, nil
}
func hostJSONString(m map[string]json.RawMessage, name string) string {
	_, b := hostJSONField(m, name)
	var s string
	_ = json.Unmarshal(b, &s)
	return s
}

// Only an automatic choice with an available matching protocol is moved.
// Unknown/custom selections and all opaque map preferences are preserved.
func migrateDynamicHostSettings(data []byte, dir string, standalone bool, available ...int) ([]byte, bool, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &doc); err != nil || doc == nil {
		return nil, false, fmt.Errorf("invalid host settings; no changes made")
	}
	_, company := hostJSONField(doc, "company")
	if standalone { // CompanyHost stores its profile by path, not inline.
		profile := hostJSONString(doc, "profile")
		if !filepath.IsAbs(profile) {
			return data, false, nil
		}
		var err error
		company, err = os.ReadFile(profile)
		if err != nil {
			return data, false, nil
		}
	}
	var profile map[string]json.RawMessage
	if json.Unmarshal(company, &profile) != nil {
		return data, false, nil
	}
	_, p := hostJSONField(profile, "protocol")
	var protocol int
	if json.Unmarshal(p, &protocol) != nil {
		return data, false, nil
	}
	if len(available) == 0 {
		available = []int{14}
	}
	supported := false
	for _, v := range available {
		if v == protocol {
			supported = true
		}
	}
	if !supported || (protocol != 14 && protocol != 15) {
		return data, false, nil
	}
	relative := managedDynamicServer
	if protocol == 15 {
		relative = managedDynamicP15Server
	}
	selected := hostJSONString(doc, "server")
	target := filepath.Join(dir, filepath.FromSlash(relative))
	if samePath(selected, target) {
		return data, false, nil
	}
	legacy := filepath.Join(dir, "app", "server", "openomsi.exe")
	_, customJSON := hostJSONField(doc, "use_custom_server")
	var custom bool
	if customJSON != nil && json.Unmarshal(customJSON, &custom) != nil {
		return nil, false, fmt.Errorf("invalid custom server preference")
	}
	managed := samePath(selected, legacy) || (!standalone && !custom)
	// Previous installers marked managed sidecars custom so the old agent would
	// not replace them. Recognize those exact package paths even after a move.
	normalized := strings.ReplaceAll(selected, "\\", "/")
	if strings.HasSuffix(strings.ToLower(normalized), "/"+relative) {
		old, e := os.ReadFile(selected)
		if os.IsNotExist(e) {
			managed = true
		} else if e == nil {
			digest := fmt.Sprintf("%x", sha256.Sum256(old))
			managed = digest == "8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240" ||
				digest == "0625f439103fbce94bbf30785cd7147d7d656cd97a1a5e61c67a722f7a95f841" ||
				digest == "7496bba46c7d7b844ec3988843a44eae11ebea8be5846ab3eace64fc51f95f57"
		}
	}
	if !managed {
		return data, false, nil
	}
	key, _ := hostJSONField(doc, "server")
	doc[key], _ = json.Marshal(target)
	if !standalone {
		key, _ = hostJSONField(doc, "use_custom_server")
		doc[key] = json.RawMessage("true")
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	return append(out, '\n'), true, err
}

// Invoked only after authenticated payload files were written and verified.
// Both settings files are planned first and rolled back together on any error.
func activateUpdatedDynamicHost(dir string, payload map[string][]byte) (err error) {
	var available []int
	for protocol, relative := range map[int]string{14: managedDynamicServer, 15: managedDynamicP15Server} {
		server, ok := payload[relative]
		if !ok {
			continue
		}
		marker := payload[filepath.ToSlash(filepath.Join(filepath.Dir(relative), "BBS-DYNAMIC-SERVER.json"))]
		var metadata struct {
			NativeSHA256 string `json:"native_sha256"`
			Protocol     int    `json:"protocol"`
		}
		if json.Unmarshal(marker, &metadata) != nil || metadata.NativeSHA256 != fmt.Sprintf("%x", sha256.Sum256(server)) ||
			(metadata.Protocol != 0 && metadata.Protocol != protocol) || (protocol == 15 && metadata.Protocol != 15) {
			return fmt.Errorf("dynamic server P%d capability manifest does not match the installed executable", protocol)
		}
		available = append(available, protocol)
	}
	if len(available) == 0 {
		return nil
	}
	type change struct {
		path          string
		before, after []byte
	}
	var plan []change
	coop15 := len(payload[bundledCoopFolder+"/programa/openomsi.exe"]) > 0 && len(payload[managedDynamicP15Server]) > 0
	upgradedCompanies := map[string]bool{}
	for _, base := range []string{managedDataDir(dir), dir} {
		for _, name := range []string{"HostAgent.local.json", "CompanyHost.local.json"} {
			path := filepath.Join(base, name)
			duplicate := false
			for _, c := range plan {
				if samePath(path, c.path) {
					duplicate = true
				}
			}
			if duplicate {
				continue
			}
			st, e := os.Lstat(path)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("host settings are not a regular file")
			}
			before, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if len(before) > 4<<20 {
				return fmt.Errorf("host settings exceed size limit")
			}
			after, changed, e := migrateDynamicHostSettings(before, dir, name == "CompanyHost.local.json", available...)
			if e != nil {
				return e
			}
			if coop15 && name == "HostAgent.local.json" {
				var id string
				after, id, changed, e = migrateManagedCoop15Host(before, dir)
				if e != nil {
					return e
				}
				if id != "" {
					upgradedCompanies[id] = true
				}
			}
			if changed {
				plan = append(plan, change{path, before, after})
			}
		}
	}
	if coop15 {
		// Upgrade only the known bundled client path. Keep every other INI
		// setting, comment and independently selected simulator byte for byte.
		path := configPath(dir)
		before, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil {
			lines := strings.Split(string(before), "\n")
			changed := false
			for i, line := range lines {
				key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), "=")
				if ok && strings.EqualFold(strings.TrimSpace(key), "openomsi") && samePath(strings.TrimSpace(value), filepath.Join(dir, "OpenOmsi Coop 0.3.38", "programa", "openomsi.exe")) {
					end := ""
					if strings.HasSuffix(line, "\r") {
						end = "\r"
					}
					lines[i] = key + "=" + filepath.Join(dir, bundledCoopFolder, "programa", "openomsi.exe") + end
					changed = true
				}
			}
			if changed {
				plan = append(plan, change{path, before, []byte(strings.Join(lines, "\n"))})
			}
			c := readConfig(path)
			if upgradedCompanies[c.CompanyID] {
				profiles := []string{filepath.Join(managedCompanyCacheDir(dir), c.CompanyID+".json")}
				if filepath.IsAbs(c.CompanyProfile) {
					profiles = append(profiles, c.CompanyProfile)
				}
				for _, path := range profiles {
					duplicate := false
					for _, prior := range plan {
						if samePath(path, prior.path) {
							duplicate = true
						}
					}
					if duplicate {
						continue
					}
					raw, e := os.ReadFile(path)
					if os.IsNotExist(e) {
						continue
					}
					if e != nil {
						return e
					}
					var profile map[string]json.RawMessage
					if json.Unmarshal(raw, &profile) != nil || hostJSONString(profile, "company_id") != c.CompanyID {
						continue
					}
					pk, _ := hostJSONField(profile, "protocol")
					profile[pk] = json.RawMessage("15")
					vk, _ := hostJSONField(profile, "openomsi_version")
					profile[vk], _ = json.Marshal("0.3.46")
					after, e := json.MarshalIndent(profile, "", "  ")
					if e != nil {
						return e
					}
					after = append(after, '\n')
					if !bytes.Equal(raw, after) {
						plan = append(plan, change{path, raw, after})
					}
				}
			}
		}
	}
	written := 0
	defer func() {
		if err != nil {
			for i := written - 1; i >= 0; i-- {
				if e := writeSetupAtomic(plan[i].path, plan[i].before); e != nil {
					err = fmt.Errorf("%w; host settings rollback failed: %v", err, e)
				}
			}
		}
	}()
	for _, c := range plan {
		current, e := os.ReadFile(c.path)
		if e != nil {
			return e
		}
		if !bytes.Equal(current, c.before) {
			return fmt.Errorf("host settings changed during update")
		}
		if err = writeSetupAtomic(c.path+".before-dynamic-update", c.before); err != nil {
			return err
		}
		if err = writeSetupAtomic(c.path, c.after); err != nil {
			return err
		}
		written++
	}
	return nil
}
