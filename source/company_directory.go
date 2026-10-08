package main

// A stable directory carries discovery and wake requests only. The actual
// game remains the native openOMSI multiplayer connection to the verified host.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var companyRoomPath = regexp.MustCompile(`^/rooms/[a-f0-9]{32}$`)

func validateCompanyDirectoryURL(address string) error {
	u, err := companyWebURL(address, true)
	if err != nil || u.RawQuery != "" || u.RawPath != "" || !companyRoomPath.MatchString(u.Path) {
		return fmt.Errorf("o diretório precisa de uma URL HTTPS terminando em /rooms/ e o código de 32 caracteres")
	}
	return nil
}

type companyDirectoryState struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Address   string `json:"address,omitempty"`
	Players   int    `json:"players"`
	Error     string `json:"error,omitempty"`
}
type companyDirectoryReply struct {
	HostOnline bool                  `json:"host_online"`
	Session    companyDirectoryState `json:"session"`
}
type companyDirectoryDemand struct {
	SessionID string `json:"session_id"`
}

func companyDirectoryHTTP(ctx context.Context, client *http.Client, method, address, key string, body any, result any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return fmt.Errorf("directory redirects are not supported") }
	resp, err := copyClient.Do(req)
	if err != nil {
		return fmt.Errorf("não foi possível consultar o diretório da empresa")
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(io.LimitReader(resp.Body, companyProfileLimit+1))
	if err != nil || len(data) > companyProfileLimit {
		return fmt.Errorf("invalid directory response")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("diretório da empresa: HTTP %d", resp.StatusCode)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("invalid directory JSON")
	}
	return nil
}

func refreshCompanyDirectoryProfile(ctx context.Context, client *http.Client, pinned CompanyProfile) (CompanyProfile, error) {
	var p CompanyProfile
	if err := companyDirectoryHTTP(ctx, client, http.MethodGet, pinned.DirectoryURL+"/profile", "", nil, &p); err != nil {
		return p, err
	}
	if p.CompanyID != pinned.CompanyID || p.DirectoryURL != pinned.DirectoryURL {
		return p, fmt.Errorf("o diretório mudou de empresa ou endereço")
	}
	return p, validateCompanyProfile(p)
}

func waitCompanyDirectorySession(ctx context.Context, client *http.Client, profile CompanyProfile, id string) (CompanySession, error) {
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	var reply companyDirectoryReply
	demand := companyDirectoryDemand{id}
	wake := func() error {
		return companyDirectoryHTTP(ctx, client, http.MethodPost, profile.DirectoryURL+"/wake", "", demand, &reply)
	}
	if err := wake(); err != nil {
		return CompanySession{}, err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastWake := time.Now()
	for {
		if !reply.HostOnline {
			return CompanySession{}, fmt.Errorf("O agente do anfitrião está offline. Ligue o PC do servidor e inicie o agente no painel Empresa / servidor.")
		}
		if reply.Session.SessionID != id {
			return CompanySession{}, fmt.Errorf("directory returned a different session")
		}
		if reply.Session.State == "disabled" {
			return CompanySession{}, fmt.Errorf("este mapa está desativado no painel do anfitrião")
		}
		if reply.Session.State == "error" {
			return CompanySession{}, fmt.Errorf("Servidor da empresa: %s", reply.Session.Error)
		}
		if reply.Session.State == "online" {
			p, err := refreshCompanyDirectoryProfile(ctx, client, profile)
			if err != nil {
				return CompanySession{}, err
			}
			for _, s := range p.Sessions {
				if s.ID == id && s.ServerURL == reply.Session.Address {
					return s, nil
				}
			}
			// Publishing a new tunnel and a heartbeat can cross in flight. Never use
			// the previous address; wait for the next consistent snapshot.
		}
		select {
		case <-ctx.Done():
			return CompanySession{}, fmt.Errorf("o servidor não ficou disponível: %w", ctx.Err())
		case <-ticker.C:
			var err error
			if time.Since(lastWake) > 30*time.Second {
				err = wake()
				lastWake = time.Now()
			} else {
				err = companyDirectoryHTTP(ctx, client, http.MethodGet, profile.DirectoryURL+"/sessions/"+id, "", nil, &reply)
			}
			if err != nil {
				return CompanySession{}, err
			}
		}
	}
}

func companyDirectoryProfileLink(p CompanyProfile) string {
	return strings.TrimRight(p.DirectoryURL, "/") + "/profile"
}
