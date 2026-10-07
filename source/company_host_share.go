package main

// Host preferences and player exports contain paths and public profile
// metadata. The private server administration password stays in the temporary
// server.cfg and is never included here.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func companyHostDataDir(packageDir string) string {
	if root := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(root) {
		return filepath.Join(root, "OpenOmsi-BBS")
	}
	if root, err := os.UserConfigDir(); err == nil && filepath.IsAbs(root) {
		return filepath.Join(root, "OpenOmsi-BBS")
	}
	return filepath.Join(packageDir, "user-settings")
}

func companyHostLocalSettingsPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(companyHostDataDir(filepath.Dir(executable)), "CompanyHost.local.json")
}

func loadCompanyHostLocalOptions(path string) (companyHostOptions, bool) {
	var options companyHostOptions
	f, err := os.Open(path)
	if err != nil {
		return options, false
	}
	defer f.Close()
	text, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil || len(text) > 16<<10 {
		return options, false
	}
	if err := json.Unmarshal(text, &options); err != nil || !filepath.IsAbs(options.Server) || !filepath.IsAbs(options.Root) || !filepath.IsAbs(options.Profile) || options.Session == "" || strings.Contains(options.Profile, "://") {
		return companyHostOptions{}, false
	}
	if options.Config != "" && !filepath.IsAbs(options.Config) || options.Share != "" && !filepath.IsAbs(options.Share) {
		return companyHostOptions{}, false
	}
	return options, true
}

func loadPreviousCompanyHostOptions(packageDir string) (companyHostOptions, bool) {
	managed := filepath.Join(companyHostDataDir(packageDir), "CompanyHost.local.json")
	if options, found := loadCompanyHostLocalOptions(managed); found {
		return options, true
	}
	// Import the previous beside-executable format when this installation still
	// has it. Future upgrades use the managed path and need no new prompts.
	return loadCompanyHostLocalOptions(filepath.Join(packageDir, "CompanyHost.local.json"))
}

func writeCompanyHostFileAtomic(path string, data []byte) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("use an absolute local JSON destination")
	}
	if old, err := os.Lstat(path); err == nil && !old.Mode().IsRegular() {
		return fmt.Errorf("the JSON destination must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".company-host-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func saveCompanyHostLocalOptions(path string, options companyHostOptions) error {
	text, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		return err
	}
	return writeCompanyHostFileAtomic(path, append(text, '\n'))
}

// Only a quick-tunnel address printed by the owned dedicated server is
// automatically adopted. Arbitrary URLs appearing in object names or errors
// cannot replace the company's session address.
func companyHostTunnelURL(line string) string {
	var address string
	for _, marker := range []string{"Server address for the players: ", "tunnel: the session is reachable at "} {
		if i := strings.Index(line, marker); i >= 0 {
			address = strings.TrimSpace(line[i+len(marker):])
			break
		}
	}
	if address == "" {
		return ""
	}
	u, err := companyWebURL(address, false)
	if err != nil || u.Scheme != "https" || u.Port() != "" || u.RawQuery != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	label := strings.TrimSuffix(host, ".trycloudflare.com")
	if label == host || len(label) < 1 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return ""
	}
	for _, c := range label {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return ""
		}
	}
	return "https://" + host
}

// Stdout and stderr may write concurrently and split a single URL across
// several writes. Keep bounded partial lines and serialize the visible log.
type companyHostTunnelOutput struct {
	mu      sync.Mutex
	output  io.Writer
	urls    chan string
	line    []byte
	tooLong bool
}

func newCompanyHostTunnelOutput(output io.Writer) *companyHostTunnelOutput {
	return &companyHostTunnelOutput{output: output, urls: make(chan string, 1)}
}

func (w *companyHostTunnelOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.output.Write(b)
	for _, c := range b[:n] {
		if c == '\n' {
			if !w.tooLong {
				if address := companyHostTunnelURL(string(w.line)); address != "" {
					select {
					case <-w.urls:
					default:
					}
					w.urls <- address
				}
			}
			w.line, w.tooLong = w.line[:0], false
		} else if !w.tooLong {
			if len(w.line) >= 4096 {
				w.line, w.tooLong = w.line[:0], true
			} else {
				w.line = append(w.line, c)
			}
		}
	}
	return n, err
}

func companyHostPlayerProfile(profile CompanyProfile, sessionID, address string) (CompanyProfile, error) {
	if companyHostTunnelURL("Server address for the players: "+address) != address {
		return CompanyProfile{}, fmt.Errorf("the player profile needs the current HTTPS quick-tunnel base address")
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return CompanyProfile{}, err
	}
	var players CompanyProfile
	if err := json.Unmarshal(data, &players); err != nil {
		return players, err
	}
	found := false
	for i := range players.Sessions {
		if players.Sessions[i].ID == sessionID {
			players.Sessions[i].ServerURL = address
			found = true
		}
	}
	if !found {
		return players, fmt.Errorf("the hosted session is missing from the company profile")
	}
	return players, validateCompanyProfile(players)
}

func verifyCompanyHostTunnel(ctx context.Context, client *http.Client, address string, local companyHostStatus, session CompanySession, fleet []string) error {
	if companyHostTunnelURL("Server address for the players: "+address) != address {
		return fmt.Errorf("invalid quick-tunnel base address")
	}
	remote, err := readCompanyHostStatus(ctx, client, address)
	if err != nil {
		return fmt.Errorf("the player address is not ready: %w", err)
	}
	if err := validateCompanyHostStatus(remote, session, fleet); err != nil {
		return err
	}
	if remote.Version != local.Version || remote.Protocol != local.Protocol || remote.Name != local.Name || !companyActiveWorld(remote.World) {
		return fmt.Errorf("the player address does not report this active company server")
	}
	return nil
}

func exportCompanyHostPlayerProfile(path string, profile CompanyProfile, sessionID, address string) error {
	players, err := companyHostPlayerProfile(profile, sessionID, address)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(players, "", "  ")
	if err != nil {
		return err
	}
	return writeCompanyHostFileAtomic(path, append(data, '\n'))
}

func companyHostSharePath(profilePath, requested, companyID, packageDir string) (string, error) {
	if !companyIDPattern.MatchString(companyID) {
		return "", fmt.Errorf("invalid company identity for the player export")
	}
	if requested == "" {
		requested = filepath.Join(companyHostDataDir(packageDir), "Companies", companyID+".players.json")
	}
	if !filepath.IsAbs(requested) {
		return "", fmt.Errorf("--share requires an absolute local JSON path")
	}
	profilePath, err := filepath.Abs(profilePath)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Clean(profilePath), filepath.Clean(requested)) {
		return "", fmt.Errorf("the player export cannot overwrite the administrator profile")
	}
	if source, err := os.Stat(profilePath); err == nil {
		if destination, err := os.Stat(requested); err == nil && os.SameFile(source, destination) {
			return "", fmt.Errorf("the player export cannot overwrite the administrator profile")
		}
	}
	return filepath.Clean(requested), nil
}
