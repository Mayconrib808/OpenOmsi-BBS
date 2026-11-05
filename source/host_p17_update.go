package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// P17 gets its own settings and company profile. Retained P15/P16 programs and
// preferences are never rewritten, including when an update is applied again.
func activateP17HostSidecar(dir string, payload map[string][]byte) (err error) {
	const server = "app/server-dinamico-p17/openomsi.exe"
	var marker struct {
		Protocol     int    `json:"protocol"`
		NativeSHA256 string `json:"native_sha256"`
	}
	if json.Unmarshal(payload["app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json"], &marker) != nil || marker.Protocol != 17 || marker.NativeSHA256 != fmt.Sprintf("%x", sha256.Sum256(payload[server])) || !bytes.HasPrefix(payload[server], []byte("MZ")) {
		return fmt.Errorf("P17 server metadata does not match authenticated runtime")
	}
	for _, name := range []string{"HostAgent.exe", "CompanyHost.exe"} {
		if !bytes.HasPrefix(payload["app/hosts-p17/"+name], []byte("MZ")) {
			return fmt.Errorf("P17 host companion missing: %s", name)
		}
	}
	type change struct {
		path string
		data []byte
	}
	var plan []change
	add := func(path string, doc map[string]json.RawMessage) error {
		if _, e := os.Lstat(path); !os.IsNotExist(e) {
			if e != nil {
				return e
			}
			return fmt.Errorf("P17 settings target already exists: %s", filepath.Base(path))
		}
		data, e := json.MarshalIndent(doc, "", "  ")
		if e != nil {
			return e
		}
		if len(data) >= hostPreferencesLimit {
			return fmt.Errorf("host preferences exceed size limit (32 MiB maximum)")
		}
		plan = append(plan, change{path, append(data, '\n')})
		return nil
	}
	for _, name := range []string{"HostAgent", "CompanyHost"} {
		target := filepath.Join(managedDataDir(dir), name+"-P17.local.json")
		if _, e := os.Lstat(target); e == nil {
			continue
		} else if !os.IsNotExist(e) {
			return e
		}
		var old []byte
		for _, suffix := range []string{"-P16.local.json", ".local.json"} {
			for _, base := range []string{managedDataDir(dir), dir} {
				path := filepath.Join(base, name+suffix)
				if st, e := os.Lstat(path); e == nil && !st.Mode().IsRegular() {
					return fmt.Errorf("host settings must be regular files")
				} else if e != nil && !os.IsNotExist(e) {
					return e
				}
				data, e := readHostPreferences(path)
				if e == nil {
					old = data
					break
				}
				if !os.IsNotExist(e) {
					return e
				}
			}
			if old != nil {
				break
			}
		}
		if old == nil {
			continue
		}
		var doc map[string]json.RawMessage
		if json.Unmarshal(bytes.TrimPrefix(old, []byte{0xef, 0xbb, 0xbf}), &doc) != nil || doc == nil {
			return fmt.Errorf("invalid legacy host preferences")
		}
		selected := hostJSONString(doc, "server")
		_, customJSON := hostJSONField(doc, "use_custom_server")
		var custom bool
		if customJSON != nil && json.Unmarshal(customJSON, &custom) != nil {
			return fmt.Errorf("invalid custom server preference")
		}
		managed := name == "HostAgent" && !custom
		for _, relative := range []string{"app/server/openomsi.exe", managedDynamicServer, managedDynamicP15Server, "app/server-dinamico-p16/openomsi.exe", server} {
			if samePath(selected, filepath.Join(dir, filepath.FromSlash(relative))) {
				managed = true
			}
		}
		if !managed {
			continue
		}
		upgradeCompany := func(company map[string]json.RawMessage) error {
			if company == nil {
				return fmt.Errorf("invalid host company preferences")
			}
			key, value := hostJSONField(company, "protocol")
			var protocol int
			if json.Unmarshal(value, &protocol) != nil || !supportedMultiplayerProtocol(protocol) {
				return fmt.Errorf("invalid previous host protocol")
			}
			company[key] = json.RawMessage("17")
			key, _ = hostJSONField(company, "openomsi_version")
			company[key], _ = json.Marshal("0.3.49")
			return nil
		}
		if name == "HostAgent" {
			key, raw := hostJSONField(doc, "company")
			var company map[string]json.RawMessage
			if json.Unmarshal(raw, &company) != nil {
				return fmt.Errorf("invalid host company preferences")
			}
			if e := upgradeCompany(company); e != nil {
				return e
			}
			doc[key], _ = json.Marshal(company)
			key, _ = hostJSONField(doc, "use_custom_server")
			doc[key] = json.RawMessage("true")
		} else {
			profilePath := hostJSONString(doc, "profile")
			if !filepath.IsAbs(profilePath) {
				return fmt.Errorf("CompanyHost profile must be an absolute local path")
			}
			st, e := os.Lstat(profilePath)
			if e != nil {
				return e
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("CompanyHost profile must be a regular file")
			}
			data, e := readHostPreferences(profilePath)
			if e != nil {
				return e
			}
			var profile map[string]json.RawMessage
			if json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &profile) != nil {
				return fmt.Errorf("invalid CompanyHost profile")
			}
			if e = upgradeCompany(profile); e != nil {
				return e
			}
			newProfile := filepath.Join(managedDataDir(dir), "CompanyHost-P17.profile.json")
			if e = add(newProfile, profile); e != nil {
				return e
			}
			key, _ := hostJSONField(doc, "profile")
			doc[key], _ = json.Marshal(newProfile)
		}
		key, _ := hostJSONField(doc, "server")
		doc[key], _ = json.Marshal(filepath.Join(dir, filepath.FromSlash(server)))
		if e := add(target, doc); e != nil {
			return e
		}
	}
	// Plan every clone before writing any file. Roll back only our own new files.
	var created []string
	defer func() {
		if err != nil {
			for _, path := range created {
				_ = os.Remove(path)
			}
		}
	}()
	for _, item := range plan {
		if e := os.MkdirAll(filepath.Dir(item.path), 0700); e != nil {
			return e
		}
		f, e := os.OpenFile(item.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		created = append(created, item.path)
		_, e = f.Write(item.data)
		if e == nil {
			e = f.Sync()
		}
		closeError := f.Close()
		if e != nil {
			return e
		}
		if closeError != nil {
			return closeError
		}
	}
	return nil
}
