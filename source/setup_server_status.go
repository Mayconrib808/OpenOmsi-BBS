package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const setupServerPollInterval = 5 * time.Second
const setupServerCheckTimeout = 4 * time.Second

type setupServerStatus struct {
	Ready bool
	Key string
}

// Only the readiness fields are read. New P15 preferences remain opaque and
// untouched; checking never writes settings, starts a map or sends /wake.
type setupHostSettings struct {
	ControlPort int `json:"control_port"`
	ControlKey string `json:"control_key"`
	Company CompanyProfile `json:"company"`
}
type setupAgentStatus struct {
	Running bool `json:"running"`
	DirectoryError string `json:"directory_error"`
	Sessions []companyDirectoryState `json:"sessions"`
}

func setupHostSettingsPath(dir string) string {
	if root := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(root) {
		return filepath.Join(root, "OpenOmsi-BBS", "HostAgent.local.json")
	}
	if root, err := os.UserConfigDir(); err == nil && filepath.IsAbs(root) {
		return filepath.Join(root, "OpenOmsi-BBS", "HostAgent.local.json")
	}
	return filepath.Join(dir, "user-settings", "HostAgent.local.json")
}

func readSetupStatusJSON(path string, target any) error {
	f, err := os.Open(path)
	if err != nil { return err }
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, companyProfileLimit+65537))
	if err != nil { return err }
	if len(b) > companyProfileLimit+65536 { return fmt.Errorf("status configuration too large") }
	return json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef,0xbb,0xbf}), target)
}

func checkSetupServerStatus(ctx context.Context, client *http.Client, dir string, c Config) setupServerStatus {
	ctx, cancel := context.WithTimeout(ctx, setupServerCheckTimeout)
	defer cancel()
	var host setupHostSettings
	err := readSetupStatusJSON(setupHostSettingsPath(dir), &host)
	if err == nil {
		if host.ControlPort < 1024 || host.ControlPort > 65535 || len(host.ControlKey) != 32 {
			return setupServerStatus{Key:"server_unavailable"}
		}
		var local setupAgentStatus
		address := fmt.Sprintf("http://127.0.0.1:%d/status",host.ControlPort)
		if err := companyDirectoryHTTP(ctx, client, http.MethodGet, address, host.ControlKey, nil, &local); err != nil || !local.Running {
			return setupServerStatus{Key:"server_agent_offline"}
		}
		if local.DirectoryError != "" { return setupServerStatus{Key:"server_unavailable"} }
		var ids []string
		for _, session := range local.Sessions {
			if session.State != "disabled" && session.State != "error" && companyIDPattern.MatchString(session.SessionID) {
				ids = append(ids, session.SessionID)
			}
		}
		return checkSetupDirectoryStatus(ctx, client, host.Company.DirectoryURL, ids)
	}
	if !os.IsNotExist(err) { return setupServerStatus{Key:"server_unavailable"} }
	// Player PCs may monitor the configured company's agent remotely. Prefer
	// the enrolled local copy, avoiding a multi-megabyte profile fetch every 5s.
	var profile CompanyProfile
	source := c.CompanyProfile
	if source == "" { source = c.CompanyProfileSource }
	if source == "" { return setupServerStatus{Key:"server_unconfigured"} }
	if strings.Contains(source, "://") {
		if c.CompanyID == "" { return setupServerStatus{Key:"server_unconfigured"} }
		source = filepath.Join(managedCompanyCacheDir(dir), c.CompanyID+".json")
	} else if !filepath.IsAbs(source) { source = filepath.Join(dir, source) }
	if err := readSetupStatusJSON(source, &profile); err != nil || (c.CompanyID != "" && profile.CompanyID != c.CompanyID) {
		return setupServerStatus{Key:"server_unconfigured"}
	}
	ids := make([]string,0,len(profile.Sessions))
	for _, session := range profile.Sessions { ids = append(ids,session.ID) }
	return checkSetupDirectoryStatus(ctx,client,profile.DirectoryURL,ids)
}

func checkSetupDirectoryStatus(ctx context.Context, client *http.Client, directory string, ids []string) setupServerStatus {
	if directory == "" { return setupServerStatus{Key:"server_unconfigured"} }
	if validateCompanyDirectoryURL(directory) != nil { return setupServerStatus{Key:"server_unavailable"} }
	for _, id := range ids {
		if !companyIDPattern.MatchString(id) { return setupServerStatus{Key:"server_unavailable"} }
		var reply companyDirectoryReply
		err := companyDirectoryHTTP(ctx,client,http.MethodGet,directory+"/sessions/"+id,"",nil,&reply)
		if err != nil || reply.Session.SessionID != id { return setupServerStatus{Key:"server_unavailable"} }
		if !reply.HostOnline { return setupServerStatus{Key:"server_agent_offline"} }
		switch reply.Session.State {
		case "online": return setupServerStatus{Ready:true,Key:"server_online"}
		case "offline": return setupServerStatus{Ready:true,Key:"server_standby"}
		case "starting": return setupServerStatus{Ready:true,Key:"server_starting"}
		case "disabled", "error": continue
		default: return setupServerStatus{Key:"server_unavailable"}
		}
	}
	return setupServerStatus{Key:"server_no_maps"}
}
