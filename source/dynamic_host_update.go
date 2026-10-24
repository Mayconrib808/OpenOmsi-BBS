package main

// The updater must activate a managed P14 sidecar as well as copy it. Keep the
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

// Only an automatic P14 choice, or this installation's managed path, is moved.
// Unknown/custom/P15 selections are preserved, including all map preferences.
func migrateDynamicHostSettings(data []byte, dir string, standalone bool) ([]byte, bool, error) {
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
	if json.Unmarshal(p, &protocol) != nil || protocol != 14 {
		return data, false, nil
	}
	selected := hostJSONString(doc, "server")
	target := filepath.Join(dir, filepath.FromSlash(managedDynamicServer))
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
	if strings.HasSuffix(strings.ToLower(normalized), "/"+managedDynamicServer) {
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
	server, ok := payload[managedDynamicServer]
	if !ok {
		return nil
	}
	marker := payload["app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json"]
	var metadata struct {
		NativeSHA256 string `json:"native_sha256"`
	}
	if json.Unmarshal(marker, &metadata) != nil || metadata.NativeSHA256 != fmt.Sprintf("%x", sha256.Sum256(server)) {
		return fmt.Errorf("dynamic server capability manifest does not match the installed executable")
	}
	type change struct {
		path          string
		before, after []byte
	}
	var plan []change
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
			after, changed, e := migrateDynamicHostSettings(before, dir, name == "CompanyHost.local.json")
			if e != nil {
				return e
			}
			if changed {
				plan = append(plan, change{path, before, after})
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
