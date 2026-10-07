package main

// CompanyHost supervises only the dedicated server it starts on this machine.
// The shared company profile never carries the local administration password.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const companyHostPollInterval = 5 * time.Second
const companyHostMaxGap = 12 * time.Hour
const companyHostClockTolerance = 90 * time.Second

type companyHostOptions struct {
	Profile, Session, Server, Root, Config string
	Share                                  string
	onReady                                func()
}

type companyHostStatus struct {
	Name       string          `json:"name"`
	Map        string          `json:"map"`
	Version    string          `json:"version"`
	Protocol   int             `json:"protocol"`
	Time       string          `json:"time"`
	Players    int             `json:"players"`
	MaxPlayers int             `json:"max_players"`
	Vehicles   json.RawMessage `json:"vehicles"`
	World      json.RawMessage `json:"world"`
}

type companyHostConfig struct {
	Text          []byte
	WebPort, Port int
}

func parseCompanyHostConfig(text []byte) (companyHostConfig, error) {
	var cfg companyHostConfig
	if len(text) > 1<<20 {
		return cfg, fmt.Errorf("server.cfg exceeds 1 MiB")
	}
	if bytes.Contains(text, []byte{0}) {
		return cfg, fmt.Errorf("server.cfg contains invalid binary data")
	}
	cfg.Text = append([]byte(nil), text...)
	values := map[string]string{}
	for _, line := range strings.Split(string(bytes.TrimPrefix(text, []byte{0xef, 0xbb, 0xbf})), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	port := func(key string, fallback int) (int, error) {
		value, found := values[key]
		if !found {
			return fallback, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 65535 {
			return 0, fmt.Errorf("server.cfg %s must be 1-65535", key)
		}
		return n, nil
	}
	var err error
	if cfg.WebPort, err = port("web_port", 27025); err != nil {
		return cfg, err
	}
	cfg.Port, err = port("port", 27015)
	return cfg, err
}

// Preserve the owner's comments and unrelated settings. Replace every occurrence
// of controlled keys, so a later duplicate cannot silently restore real_time.
func renderCompanyHostConfig(cfg companyHostConfig, session CompanySession, fleet []string, now time.Time, password string) []byte {
	values := map[string]string{
		"map": session.MapFile, "date": now.Format("2006-01-02"),
		"time": now.Format("15:04:05"), "real_time": "0", "time_speed": "1",
		"vehicles": strings.Join(fleet, ";"), "admin_password": password,
	}
	keys := []string{"map", "date", "time", "real_time", "time_speed", "vehicles", "admin_password"}
	text := cfg.Text
	if bytes.HasPrefix(text, []byte{0xef, 0xbb, 0xbf}) {
		text = text[3:]
	}
	newline := "\n"
	if bytes.Contains(text, []byte("\r\n")) {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, found := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(key)
		if value, controlled := values[key]; found && controlled {
			lines[i] = key + " = " + value
			seen[key] = true
		}
	}
	for _, key := range keys {
		if !seen[key] {
			lines = append(lines, key+" = "+values[key])
		}
	}
	return []byte(strings.TrimRight(strings.Join(lines, newline), "\r\n") + newline)
}

func companyHostFleet(profile CompanyProfile, session CompanySession) ([]string, error) {
	packages := map[string]bool{}
	for _, id := range session.RequiredPackages {
		packages[id] = true
	}
	paths := map[string]string{}
	for _, pkg := range profile.Packages {
		if !packages[pkg.ID] {
			continue
		}
		for _, file := range pkg.Files {
			key := companyAssetKey(file.Path)
			if strings.HasPrefix(key, "vehicles/") && strings.HasSuffix(key, ".bus") {
				if strings.ContainsAny(file.Path, ";\r\n") {
					return nil, fmt.Errorf("bus path cannot contain a server.cfg fleet separator")
				}
				paths[key] = strings.ReplaceAll(file.Path, `\`, "/")
			}
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("the session needs at least one hashed .bus file in its required packages")
	}
	fleet := make([]string, 0, len(paths))
	for _, path := range paths {
		fleet = append(fleet, path)
	}
	sort.Slice(fleet, func(i, j int) bool { return companyAssetKey(fleet[i]) < companyAssetKey(fleet[j]) })
	return fleet, nil
}

func companyHostClient() *http.Client {
	return &http.Client{
		Timeout:   4 * time.Second,
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("local host requests cannot follow redirects")
		},
	}
}

func readCompanyHostStatus(ctx context.Context, client *http.Client, base string) (companyHostStatus, error) {
	var status companyHostStatus
	b, err := companyReadHTTP(ctx, client, base+"/status", 128<<10)
	if err != nil {
		return status, err
	}
	if err := json.Unmarshal(b, &status); err != nil {
		return status, fmt.Errorf("local server returned invalid status JSON")
	}
	return status, nil
}

func companyHostClock(s string) (float64, error) {
	p := strings.Split(s, ":")
	if len(p) < 2 || len(p) > 3 {
		return 0, fmt.Errorf("local server returned an invalid clock")
	}
	h, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	sec := 0.0
	var e3 error
	if len(p) == 3 {
		sec, e3 = strconv.ParseFloat(p[2], 64)
	}
	if e1 != nil || e2 != nil || e3 != nil || h < 0 || h > 23 || m < 0 || m > 59 || math.IsNaN(sec) || math.IsInf(sec, 0) || sec < 0 || sec >= 60 {
		return 0, fmt.Errorf("local server returned an invalid clock")
	}
	return float64(h*3600+m*60) + sec, nil
}

func validateCompanyHostStatus(status companyHostStatus, session CompanySession, fleet []string) error {
	if !compatibleCompanyServer(status.Version, status.Protocol) {
		return fmt.Errorf("local server reported %q / protocol %d; its company multiplayer capabilities are not supported", status.Version, status.Protocol)
	}
	if !companyText(status.Name, 120) || companyAssetKey(status.Map) != companyAssetKey(session.MapFile) {
		return fmt.Errorf("local server map does not match the selected company session")
	}
	if status.Players < 0 || status.MaxPlayers < 1 || status.MaxPlayers > 128 || status.Players > status.MaxPlayers {
		return fmt.Errorf("local server returned invalid player limits")
	}
	if _, err := companyHostClock(status.Time); err != nil {
		return err
	}
	var vehicles []string
	var text string
	if err := json.Unmarshal(status.Vehicles, &text); err == nil {
		for _, path := range strings.Split(text, ";") {
			if strings.TrimSpace(path) != "" {
				vehicles = append(vehicles, path)
			}
		}
	} else if err := json.Unmarshal(status.Vehicles, &vehicles); err != nil {
		return fmt.Errorf("local server did not publish its fleet")
	}
	wanted := map[string]bool{}
	for _, path := range fleet {
		wanted[companyAssetKey(path)] = true
	}
	seen := map[string]bool{}
	for _, path := range vehicles {
		key := companyAssetKey(path)
		if !wanted[key] || seen[key] {
			return fmt.Errorf("local server fleet differs from the company profile")
		}
		seen[key] = true
	}
	if len(seen) != len(wanted) {
		return fmt.Errorf("local server fleet differs from the company profile")
	}
	return nil
}

func postCompanyHostClock(ctx context.Context, client *http.Client, base, password string, target time.Time) error {
	seconds := float64(target.Hour()*3600+target.Minute()*60+target.Second()) + float64(target.Nanosecond())/1e9
	body := fmt.Sprintf("speed 1\nclock %.3f\n", seconds)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/admin", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Admin-Password", password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("local clock administration could not be reached")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("local clock administration returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type companyHostMonitor struct {
	serverCivil, sampledAt time.Time
	pending                bool
	failures               int
	lastGood               time.Time
}

func companyHostEnvironment(parent []string, contentDir string) []string {
	var result []string
	for _, entry := range parent {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "OMSI_CONTENT") && !strings.EqualFold(key, "OMSI_CONTENT_ZIP") && !strings.EqualFold(key, "OMSI_NO_LAN_MODS") {
			result = append(result, entry)
		}
	}
	return append(result, "OMSI_CONTENT="+contentDir, "OMSI_NO_LAN_MODS=1")
}

// Status has only HH:MM. Unwrap it against the previously observed calendar,
// never against Windows' local date. More than twelve hours cannot be resolved
// safely from this protocol, so stop instead of guessing a day.
func (m *companyHostMonitor) observe(sampledAt, target time.Time, clock string) (bool, error) {
	elapsed := sampledAt.Sub(m.sampledAt)
	wallElapsed := sampledAt.Round(0).Sub(m.sampledAt.Round(0))
	if elapsed < 0 || elapsed >= companyHostMaxGap || wallElapsed <= -companyHostMaxGap || wallElapsed >= companyHostMaxGap {
		return false, fmt.Errorf("clock monitoring was interrupted for too long; the server calendar is uncertain")
	}
	seconds, err := companyHostClock(clock)
	if err != nil {
		return false, err
	}
	expected := m.serverCivil.Add(elapsed)
	observed := time.Date(expected.Year(), expected.Month(), expected.Day(), 0, 0, 0, 0, time.UTC).Add(time.Duration(seconds * float64(time.Second)))
	if observed.Sub(expected) >= companyHostMaxGap {
		observed = observed.Add(-24 * time.Hour)
	} else if expected.Sub(observed) >= companyHostMaxGap {
		observed = observed.Add(24 * time.Hour)
	}
	gap := target.Sub(observed)
	if gap <= -companyHostMaxGap || gap >= companyHostMaxGap {
		return false, fmt.Errorf("the server calendar cannot be corrected safely without restarting")
	}
	synced := gap > -companyHostClockTolerance && gap < companyHostClockTolerance
	if m.pending && !synced {
		m.failures++
		if m.failures >= 2 {
			return false, fmt.Errorf("the server accepted clock commands but did not synchronize; stopping the session")
		}
	} else if synced {
		m.failures = 0
		m.lastGood = sampledAt
	}
	m.serverCivil, m.sampledAt = observed, sampledAt
	return synced, nil
}

func ensureCompanyHostPortsFree(cfg companyHostConfig) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.WebPort)))
	if err != nil {
		return fmt.Errorf("web port %d is already in use; stop the previous server first", cfg.WebPort)
	}
	defer listener.Close()
	udp, err := net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)))
	if err != nil {
		return fmt.Errorf("UDP port %d is already in use; stop the previous server first", cfg.Port)
	}
	return udp.Close()
}

func runCompanyHost(ctx context.Context, options companyHostOptions, output io.Writer) error {
	if strings.Contains(options.Profile, "://") {
		return fmt.Errorf("host supervision requires a local company JSON file")
	}
	if !filepath.IsAbs(options.Server) || !filepath.IsAbs(options.Root) {
		return fmt.Errorf("--server and --root require absolute paths")
	}
	for _, path := range []string{options.Server, filepath.Join(options.Root, "Omsi.exe")} {
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			return fmt.Errorf("required executable was not found: %s", path)
		}
	}
	profile, err := loadCompanyProfile(ctx, options.Profile, ".", companyHostClient())
	if err != nil {
		return err
	}
	executable, _ := os.Executable()
	sharePath, err := companyHostSharePath(options.Profile, options.Share, profile.CompanyID, filepath.Dir(executable))
	if err != nil {
		return err
	}
	var session CompanySession
	found := false
	for _, candidate := range profile.Sessions {
		if candidate.ID == options.Session {
			session, found = candidate, true
			break
		}
	}
	if !found || profile.Clock == nil || session.Date != "company" {
		return fmt.Errorf("select a session with date = company and a configured company clock")
	}
	if err := validateCompanyClock(*profile.Clock); err != nil {
		return err
	}
	fleet, err := companyHostFleet(profile, session)
	if err != nil {
		return err
	}
	if problems := checkCompanyPackages(options.Root, profile, session); len(problems) != 0 {
		return fmt.Errorf("server content differs from the company profile: %s: %s", problems[0].Name, problems[0].Detail)
	}
	if options.Config == "" {
		options.Config = filepath.Join(filepath.Dir(options.Server), "server.cfg")
	}
	options.Config, err = filepath.Abs(options.Config)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(options.Config)
	if err != nil {
		return fmt.Errorf("open the existing server.cfg first: %w", err)
	}
	cfg, err := parseCompanyHostConfig(text)
	if err != nil {
		return err
	}
	if err := ensureCompanyHostPortsFree(cfg); err != nil {
		return err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	password := hex.EncodeToString(secret)
	started := time.Now()
	civil, err := companyNow(*profile.Clock, started)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(options.Config), "company-host-*.cfg")
	if err != nil {
		return fmt.Errorf("cannot create the private host configuration: %w", err)
	}
	privateConfig := f.Name()
	defer os.Remove(privateConfig)
	_, writeErr := f.Write(renderCompanyHostConfig(cfg, session, fleet, civil, password))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	contentDir, err := os.MkdirTemp("", "openomsi-company-host-content-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(contentDir)
	child := exec.Command(options.Server, "--root", options.Root, "--server", privateConfig)
	child.Dir = filepath.Dir(options.Server)
	child.Env = companyHostEnvironment(os.Environ(), contentDir)
	tunnelOutput := newCompanyHostTunnelOutput(output)
	output = tunnelOutput
	child.Stdout, child.Stderr = output, output
	prepareCompanyHostProcess(child)
	if err := child.Start(); err != nil {
		return fmt.Errorf("cannot start the dedicated server: %w", err)
	}
	stopChild, err := ownCompanyHostProcess(child)
	if err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		return fmt.Errorf("cannot supervise the dedicated server: %w", err)
	}
	finished := make(chan error, 1)
	go func() { finished <- child.Wait() }()
	childExited := false
	defer func() {
		stopChild()
		if childExited {
			return
		}
		select {
		case <-finished:
			return
		default:
		}
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
		}
	}()
	base := "http://127.0.0.1:" + strconv.Itoa(cfg.WebPort)
	client := companyHostClient()
	defer client.CloseIdleConnections()
	publicClient := companyHostClient()
	defer publicClient.CloseIdleConnections()
	var tunnelAddress, exportedAddress string
	var nextShareAttempt time.Time
	shareFailureReported := false
	monitor := companyHostMonitor{serverCivil: civil, sampledAt: started}
	deadline := started.Add(startupTimeout)
	ready := false
	badStatus := 0
	ticker := time.NewTicker(companyHostPollInterval)
	defer ticker.Stop()
	fmt.Fprintf(output, "Servidor da empresa %s / %s iniciando. Relógio %s %+d min.\n", profile.CompanyName, session.Name, profile.Clock.TimeZone, profile.Clock.ShiftMinutes)
	fmt.Fprintln(output, "Mantenha esta janela aberta. Aguarde a confirmação de sincronização antes de iniciar a viagem no BCS.")
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-finished:
			childExited = true
			if err == nil {
				return fmt.Errorf("the dedicated server stopped")
			}
			return fmt.Errorf("the dedicated server stopped unexpectedly")
		case sampled := <-ticker.C:
			select {
			case address := <-tunnelOutput.urls:
				if address != tunnelAddress {
					tunnelAddress = address
					nextShareAttempt = time.Time{}
					shareFailureReported = false
				}
			default:
			}
			if !ready && sampled.After(deadline) {
				return fmt.Errorf("the dedicated server did not synchronize within %s", startupTimeout)
			}
			status, err := readCompanyHostStatus(ctx, client, base)
			if err != nil {
				if ready {
					badStatus++
					if badStatus >= 3 {
						return fmt.Errorf("local server status became unavailable; stopping the session")
					}
				}
				continue
			}
			badStatus = 0
			if err := validateCompanyHostStatus(status, session, fleet); err != nil {
				return err
			}
			if !companyActiveWorld(status.World) {
				if ready {
					return fmt.Errorf("the dedicated server stopped publishing its active world")
				}
				continue // Gateway starts before the map and server loop are loaded.
			}
			sampled = time.Now()
			target, err := companyNow(*profile.Clock, sampled)
			if err != nil {
				return err
			}
			synced, err := monitor.observe(sampled, target, status.Time)
			if err != nil {
				return err
			}
			if synced && monitor.pending && !ready {
				ready = true
				if options.onReady != nil {
					options.onReady()
				}
				fmt.Fprintf(output, "SINCRONIZADO: %s. Servidor local: %s\n", target.Format("2006-01-02 15:04:05"), base)
				fmt.Fprintln(output, "O sincronizador continua ativo nesta janela. No mesmo PC, o perfil local continua usando o endereço local.")
				if tunnelAddress == "" {
					fmt.Fprintln(output, "Ainda aguardando o endereço HTTPS do túnel para gerar o perfil dos outros jogadores. Se o túnel estiver desativado, este servidor fica disponível pelo endereço configurado.")
				}
			}
			if err := postCompanyHostClock(ctx, client, base, password, target); err != nil {
				return err
			}
			monitor.pending = true
			// Sharing is independent of clock supervision: a slow tunnel or an
			// unwritable export cannot stop the already running local game.
			if ready && tunnelAddress != "" && tunnelAddress != exportedAddress && !sampled.Before(nextShareAttempt) {
				nextShareAttempt = sampled.Add(30 * time.Second)
				shareErr := verifyCompanyHostTunnel(ctx, publicClient, tunnelAddress, status, session, fleet)
				if shareErr == nil {
					shareErr = exportCompanyHostPlayerProfile(sharePath, profile, session.ID, tunnelAddress)
				}
				if shareErr != nil {
					if !shareFailureReported {
						fmt.Fprintf(output, "Ainda não foi possível preparar o perfil para os outros jogadores: %v. O servidor local continua ativo; vou tentar novamente.\n", shareErr)
						shareFailureReported = true
					}
				} else {
					exportedAddress = tunnelAddress
					fmt.Fprintf(output, "PERFIL PARA OS JOGADORES: %s\n", sharePath)
					fmt.Fprintln(output, "Envie este arquivo aos jogadores ou atualize o perfil no link HTTPS da empresa. O endereço do túnel muda quando o servidor reinicia; este arquivo é atualizado automaticamente, mas o envio ou a hospedagem precisam usar a cópia nova.")
				}
			}
		}
	}
}
