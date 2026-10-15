package main

// A stable directory carries discovery and wake requests only. The actual
// game remains the native openOMSI multiplayer connection to the verified host.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	Weather   string `json:"weather,omitempty"`
}

// Only discovery's idempotent reads/wake may retry. Authentication failures,
// redirects, invalid profiles and changed company identities remain failures.
type companyDirectoryRequestError struct {
	status int
	cause  error
}

func (e *companyDirectoryRequestError) Error() string { return companyDirectoryErrorText(e, "pt") }
func (e *companyDirectoryRequestError) Unwrap() error { return e.cause }

func companyDirectoryErrorText(err error, language string) string {
	var e *companyDirectoryRequestError
	if !errors.As(err, &e) {
		return err.Error()
	}
	if e.status != 0 {
		return fmt.Sprintf(localText(language, "Diretório da empresa: HTTP %d", "Company directory: HTTP %d", "Firmenverzeichnis: HTTP %d"), e.status)
	}
	detail := e.cause.Error()
	return localText(language, "Não foi possível consultar o diretório da empresa: ", "Could not contact the company directory: ", "Das Firmenverzeichnis konnte nicht erreicht werden: ") + detail
}

func companyDirectoryTransportError(err error) error {
	// url.Error includes the invitation URL. Keep its actual network reason,
	// while excluding the room code and any authorization from shared logs/UI.
	var e *url.Error
	if errors.As(err, &e) {
		err = e.Err
	}
	return &companyDirectoryRequestError{cause: err}
}

func retryableCompanyDirectoryError(err error) bool {
	var e *companyDirectoryRequestError
	if !errors.As(err, &e) {
		return false
	}
	switch e.status {
	case 408, 429, 500, 502, 503, 504:
		return true
	}
	var network net.Error
	return e.status == 0 && !errors.Is(e.cause, context.Canceled) &&
		(errors.As(e.cause, &network) || errors.Is(e.cause, io.EOF) || errors.Is(e.cause, io.ErrUnexpectedEOF))
}

func retryCompanyDirectoryRequest(ctx context.Context, request func(context.Context) error) error {
	return retryCompanyDirectoryRequestWithin(ctx, 30*time.Second, 2*time.Second, request)
}

func retryCompanyDirectoryRequestWithin(parent context.Context, budget, interval time.Duration, request func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	for {
		if err := parent.Err(); err != nil {
			return err
		}
		err := request(ctx)
		if err == nil || !retryableCompanyDirectoryError(err) {
			return err
		}
		// Keep the last real cause when the grace period expires. The caller's
		// cancellation takes priority; it must never wait through another retry.
		if parent.Err() != nil {
			return parent.Err()
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if parent.Err() != nil {
				return parent.Err()
			}
			return err
		case <-timer.C:
		}
	}
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OpenOmsi-BBS/"+bridgeVersion)
	req.Header.Set("Cache-Control", "no-cache")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return fmt.Errorf("directory redirects are not supported") }
	resp, err := copyClient.Do(req)
	if err != nil {
		return companyDirectoryTransportError(err)
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(io.LimitReader(resp.Body, companyProfileLimit+1))
	if err != nil {
		return companyDirectoryTransportError(err)
	}
	if len(data) > companyProfileLimit {
		return fmt.Errorf("invalid directory response")
	}
	if resp.StatusCode != http.StatusOK {
		return &companyDirectoryRequestError{status: resp.StatusCode}
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

func waitCompanyDirectorySession(ctx context.Context, client *http.Client, profile CompanyProfile, id string, weather ...string) (CompanySession, error) {
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	var reply companyDirectoryReply
	demand := companyDirectoryDemand{SessionID: id}
	if len(weather) > 0 && validBridgeWeather(weather[0]) {
		demand.Weather = weather[0]
	}
	wake := func() error {
		return retryCompanyDirectoryRequest(ctx, func(attempt context.Context) error {
			return companyDirectoryHTTP(attempt, client, http.MethodPost, profile.DirectoryURL+"/wake", "", demand, &reply)
		})
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
			var p CompanyProfile
			err := retryCompanyDirectoryRequest(ctx, func(attempt context.Context) error {
				var err error
				p, err = refreshCompanyDirectoryProfile(attempt, client, profile)
				return err
			})
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
				err = retryCompanyDirectoryRequest(ctx, func(attempt context.Context) error {
					return companyDirectoryHTTP(attempt, client, http.MethodGet, profile.DirectoryURL+"/sessions/"+id, "", nil, &reply)
				})
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
