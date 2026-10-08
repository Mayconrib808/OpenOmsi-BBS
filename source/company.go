package main

// A company profile contains metadata, hashes and administrator-provided links.
// It never contains a command to run or an archive to install automatically.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const companyProfileLimit = 8 << 20
const companyFileLimit = 30000
const multiplayerProtocol = 6
const multiplayerGameVersion = "0.2.11"

type CompanyProfile struct {
	SchemaVersion   int              `json:"schema_version"`
	CompanyID       string           `json:"company_id"`
	CompanyName     string           `json:"company_name"`
	OpenOMSIVersion string           `json:"openomsi_version"`
	Protocol        int              `json:"protocol"`
	Clock           *CompanyClock    `json:"clock,omitempty"`
	DirectoryURL    string           `json:"directory_url,omitempty"`
	Packages        []CompanyPackage `json:"packages"`
	Sessions        []CompanySession `json:"sessions"`
}

type CompanyPackage struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	DownloadURL string        `json:"download_url,omitempty"`
	Folders     []string      `json:"folders"`
	Files       []CompanyFile `json:"files"`
}

type CompanyFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type CompanySession struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	MapName           string   `json:"map_name"`
	MapFile           string   `json:"map_file"`
	Date              string   `json:"date"`
	ServerURL         string   `json:"server_url"`
	RequiredPackages  []string `json:"required_packages"`
	ClockToleranceSec int      `json:"clock_tolerance_seconds"`
	Fleet             []string `json:"fleet,omitempty"`
}

type CompanyProblem struct {
	Name, Detail, DownloadURL string
}

var companyIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var companyHashPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func companyText(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= max && !strings.ContainsFunc(s, unicode.IsControl)
}

func companyAssetKey(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), `\`, "/"))
}

func validCompanyAsset(s string) bool {
	s = strings.ReplaceAll(s, `\`, "/")
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, ":\x00\r\n") || strings.HasPrefix(s, "/") {
		return false
	}
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") {
			return false
		}
	}
	switch strings.ToLower(parts[0]) {
	case "maps", "vehicles", "sceneryobjects", "splines", "humans", "weather", "texture", "textures", "sounds", "fonts", "ticketpacks":
	default:
		return false
	}
	switch strings.ToLower(filepath.Ext(s)) {
	case ".exe", ".dll", ".jar", ".bat", ".cmd", ".ps1", ".lnk", ".zip", ".rar", ".7z", ".log", ".odr", ".osn", ".tmp", ".bak", ".backup", ".hof":
		return false
	}
	return true
}

func companyWebURL(s string, profile bool) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.ContainsFunc(s, unicode.IsControl) {
		return nil, fmt.Errorf("invalid web address")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("use an HTTP or HTTPS address")
	}
	if profile && u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if !strings.EqualFold(u.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("a remote company profile requires HTTPS")
		}
	}
	return u, nil
}

func validateCompanyProfile(p CompanyProfile) error {
	if p.SchemaVersion != 1 || !companyIDPattern.MatchString(p.CompanyID) || !companyText(p.CompanyName, 120) {
		return fmt.Errorf("invalid company identity or unsupported profile schema")
	}
	if !validOpenOMSIVersion(p.OpenOMSIVersion) || p.Protocol != multiplayerProtocol {
		return fmt.Errorf("company profile requires a valid openOMSI release label and network protocol %d", multiplayerProtocol)
	}
	if p.Clock != nil {
		if err := validateCompanyClock(*p.Clock); err != nil {
			return err
		}
	}
	if p.DirectoryURL != "" {
		if err := validateCompanyDirectoryURL(p.DirectoryURL); err != nil {
			return err
		}
	}
	if len(p.Packages) == 0 || len(p.Packages) > 100 || len(p.Sessions) == 0 || len(p.Sessions) > 16 {
		return fmt.Errorf("a profile needs 1-100 packages and 1-16 sessions")
	}
	packages := map[string]map[string]bool{}
	hashes := map[string]string{}
	count := 0
	for _, pkg := range p.Packages {
		if !companyIDPattern.MatchString(pkg.ID) || packages[pkg.ID] != nil || !companyText(pkg.Name, 120) || !companyText(pkg.Version, 80) || len(pkg.Files) == 0 || len(pkg.Folders) == 0 || len(pkg.Folders) > 40 {
			return fmt.Errorf("invalid or duplicate package: %s", pkg.ID)
		}
		if pkg.DownloadURL != "" {
			if _, err := companyWebURL(pkg.DownloadURL, true); err != nil {
				return fmt.Errorf("package %s download: %w", pkg.ID, err)
			}
		}
		files := map[string]bool{}
		folders := map[string]bool{}
		for _, folder := range pkg.Folders {
			key := companyAssetKey(folder)
			if !validCompanyAsset(folder+"/asset.cfg") || folders[key] {
				return fmt.Errorf("invalid or duplicate folder in %s", pkg.ID)
			}
			folders[key] = true
		}
		for _, file := range pkg.Files {
			key := companyAssetKey(file.Path)
			inside := false
			for folder := range folders {
				inside = inside || strings.HasPrefix(key, folder+"/")
			}
			if !inside || !validCompanyAsset(file.Path) || !companyHashPattern.MatchString(file.SHA256) || files[key] {
				return fmt.Errorf("invalid or duplicate asset in %s: %s", pkg.ID, file.Path)
			}
			if old, ok := hashes[key]; ok && !strings.EqualFold(old, file.SHA256) {
				return fmt.Errorf("conflicting hashes for %s", file.Path)
			}
			hashes[key] = file.SHA256
			files[key] = true
			count++
			if count > companyFileLimit {
				return fmt.Errorf("company asset list exceeds %d files", companyFileLimit)
			}
		}
		packages[pkg.ID] = files
	}
	seen := map[string]bool{}
	for _, session := range p.Sessions {
		fleetSeen := map[string]bool{}
		for _, bus := range session.Fleet {
			key := companyAssetKey(bus)
			if !validCompanyAsset(bus) || !strings.HasPrefix(key, "vehicles/") || !strings.HasSuffix(key, ".bus") || strings.Contains(bus, ";") || fleetSeen[key] || len(session.Fleet) > 1024 {
				return fmt.Errorf("invalid fleet in %s", session.ID)
			}
			fleetSeen[key] = true
		}
		if !companyIDPattern.MatchString(session.ID) || seen[session.ID] || !companyText(session.Name, 120) || !companyText(session.MapName, 120) {
			return fmt.Errorf("invalid or duplicate session: %s", session.ID)
		}
		seen[session.ID] = true
		if !validCompanyAsset(session.MapFile) || !strings.HasPrefix(companyAssetKey(session.MapFile), "maps/") || !strings.HasSuffix(companyAssetKey(session.MapFile), "/global.cfg") {
			return fmt.Errorf("invalid map in %s", session.ID)
		}
		if session.Date == "company" {
			if p.Clock == nil {
				return fmt.Errorf("session %s needs an explicit company clock", session.ID)
			}
		} else if _, err := time.Parse("2006-01-02", session.Date); err != nil {
			return fmt.Errorf("session %s needs YYYY-MM-DD or company", session.ID)
		}
		u, err := companyWebURL(session.ServerURL, false)
		if err != nil || (u != nil && (u.RawQuery != "" || (u.Path != "" && u.Path != "/"))) {
			return fmt.Errorf("session %s needs the server's HTTP(S) base address", session.ID)
		}
		if session.ClockToleranceSec < 1 || session.ClockToleranceSec > 300 {
			return fmt.Errorf("session %s clock tolerance must be 1-300 seconds", session.ID)
		}
		if len(session.RequiredPackages) == 0 {
			return fmt.Errorf("session %s has no required packages", session.ID)
		}
		covered, used := false, map[string]bool{}
		for _, id := range session.RequiredPackages {
			if packages[id] == nil || used[id] {
				return fmt.Errorf("unknown or duplicate package %s in %s", id, session.ID)
			}
			used[id] = true
			covered = covered || packages[id][companyAssetKey(session.MapFile)]
		}
		if !covered {
			return fmt.Errorf("session %s map must be covered by its file hashes", session.ID)
		}
	}
	return nil
}

func companyActiveWorld(raw json.RawMessage) bool {
	var counts map[string]json.RawMessage
	return json.Unmarshal(raw, &counts) == nil && counts != nil
}

func companyHTTPClient() *http.Client {
	return &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 4 || (len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https") {
			return fmt.Errorf("unsafe or excessive redirect")
		}
		if _, err := companyWebURL(req.URL.String(), false); err != nil {
			return err
		}
		return nil
	}}
}

func companyReadHTTP(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OpenOmsi-BBS/"+bridgeVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("response exceeds size limit")
	}
	return b, nil
}

func loadCompanyProfile(ctx context.Context, source, baseDir string, client *http.Client) (CompanyProfile, error) {
	var p CompanyProfile
	var b []byte
	var err error
	source = strings.Trim(strings.TrimSpace(source), `"`)
	if strings.Contains(source, "://") {
		if _, err = companyWebURL(source, true); err == nil {
			b, err = companyReadHTTP(ctx, client, source, companyProfileLimit)
		}
	} else {
		if !filepath.IsAbs(source) {
			source = filepath.Join(baseDir, source)
		}
		var f *os.File
		f, err = os.Open(source)
		if err == nil {
			b, err = io.ReadAll(io.LimitReader(f, companyProfileLimit+1))
			f.Close()
			if len(b) > companyProfileLimit {
				err = fmt.Errorf("company profile exceeds size limit")
			}
		}
	}
	if err != nil {
		return p, err
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, fmt.Errorf("company JSON: %w", err)
	}
	var trailing any
	if err = d.Decode(&trailing); err != io.EOF {
		return p, fmt.Errorf("trailing data after company profile")
	}
	return p, validateCompanyProfile(p)
}

func companyAssetPath(root, relative string) (string, error) {
	if !validCompanyAsset(relative) {
		return "", fmt.Errorf("unsafe asset path")
	}
	path := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(relative, `\`, "/")))
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		path = root
		for _, part := range strings.Split(strings.ReplaceAll(relative, `\`, "/"), "/") {
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", err
			}
			name := ""
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), part) {
					if name != "" {
						return "", fmt.Errorf("ambiguous asset path")
					}
					name = entry.Name()
				}
			}
			if name == "" {
				return "", os.ErrNotExist
			}
			path = filepath.Join(path, name)
		}
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, actual)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("asset outside OMSI root")
	}
	st, err := os.Stat(actual)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("asset is not a regular file")
	}
	return path, nil
}

func companyFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CompanyHost sets this only in its own executable. The normal bridge bypasses
// this audit at its multiplayer call site. Setup/tests keep it enabled so hashes
// remain useful for explicit administrator inventory maintenance.
var companyRuntimePackageChecksDisabled bool

func checkCompanyPackages(root string, p CompanyProfile, session CompanySession) []CompanyProblem {
	if companyRuntimePackageChecksDisabled {
		return nil
	}
	return checkCompanyPackagesWithOriginals(root, p, session, companyBBSBackupInventory(root))
}

// Strict hash comparison is an administrator/audit primitive. It is deliberately
// not a multiplayer compatibility gate anymore.
func checkCompanyPackagesWithOriginals(root string, p CompanyProfile, session CompanySession, bbsBackups map[string]bool) []CompanyProblem {
	wanted := map[string]bool{}
	for _, id := range session.RequiredPackages {
		wanted[id] = true
	}
	var problems []CompanyProblem
	checked := map[string]string{}
	reported := map[string]bool{}
	for _, pkg := range p.Packages {
		if !wanted[pkg.ID] {
			continue
		}
		var missing, different []string
		for _, file := range pkg.Files {
			if companyRuntimeArtifact(file.Path) {
				continue
			}
			key := companyAssetKey(file.Path)
			digest, ok := checked[key]
			if !ok {
				path, err := companyAssetPath(root, file.Path)
				if err != nil {
					if alias := companyCompatibilityAlias(file.Path); alias != "" {
						path, err = companyAssetPath(root, alias)
					}
				}
				if err == nil {
					digest, err = companyFileHash(path)
				}
				if err != nil {
					digest = ""
				}
				checked[key] = digest
			}
			if digest == "" {
				missing = append(missing, file.Path)
			} else if !strings.EqualFold(digest, file.SHA256) && !companyBBSOriginalMatches(root, file, bbsBackups) {
				different = append(different, file.Path)
			}
		}
		if len(missing)+len(different) != 0 {
			signature, _ := json.Marshal([][]string{missing, different})
			reportKey := pkg.Name + "\n" + pkg.DownloadURL + "\n" + string(signature)
			if reported[reportKey] {
				continue
			}
			reported[reportKey] = true
			detail := fmt.Sprintf("%s: ausentes/ilegíveis %d; conteúdo alterado %d. / missing or unreadable: %d; changed content: %d.", pkg.Version, len(missing), len(different), len(missing), len(different))
			list := append(append([]string(nil), missing...), different...)
			if len(list) > 8 {
				list = list[:8]
			}
			if len(list) != 0 {
				detail += "\n" + strings.Join(list, "\n")
			}
			problems = append(problems, CompanyProblem{pkg.Name, detail, pkg.DownloadURL})
		}
	}
	return problems
}

func refreshCompanyHashes(root string, original CompanyProfile) (CompanyProfile, []string, error) {
	if err := validateCompanyProfile(original); err != nil {
		return original, nil, err
	}
	b, err := json.Marshal(original)
	if err != nil {
		return original, nil, err
	}
	var updated CompanyProfile
	if err := json.Unmarshal(b, &updated); err != nil {
		return original, nil, err
	}
	checked := map[string]string{}
	var changed []string
	for i := range updated.Packages {
		var retained []CompanyFile
		for _, file := range updated.Packages[i].Files {
			if companyRuntimeArtifact(file.Path) {
				key := companyAssetKey(file.Path)
				if _, seen := checked[key]; !seen {
					changed = append(changed, file.Path)
					checked[key] = ""
				}
				continue
			}
			retained = append(retained, file)
		}
		updated.Packages[i].Files = retained
		for j := range updated.Packages[i].Files {
			file := &updated.Packages[i].Files[j]
			key := companyAssetKey(file.Path)
			digest, exists := checked[key]
			if !exists {
				path, e := companyAssetPath(root, file.Path)
				if e != nil {
					if alias := companyCompatibilityAlias(file.Path); alias != "" {
						path, e = companyAssetPath(root, alias)
					}
				}
				if e != nil {
					return original, nil, fmt.Errorf("cannot update declared asset %s: %w", file.Path, e)
				}
				digest, e = companyFileHash(path)
				if e != nil {
					return original, nil, fmt.Errorf("cannot update declared asset %s: %w", file.Path, e)
				}
				checked[key] = digest
				if !strings.EqualFold(digest, file.SHA256) {
					changed = append(changed, file.Path)
				}
			}
			file.SHA256 = digest
		}
	}
	all := updated.Sessions[0]
	all.ID = ""
	all.RequiredPackages = nil
	for _, pkg := range updated.Packages {
		all.RequiredPackages = append(all.RequiredPackages, pkg.ID)
	}
	if problems := checkCompanyPackagesWithOriginals(root, updated, all, nil); len(problems) != 0 {
		return original, nil, fmt.Errorf("profile update needs the same declared file inventory: %s", problems[0].Detail)
	}
	if err := validateCompanyProfile(updated); err != nil {
		return original, nil, err
	}
	sort.Strings(changed)
	return updated, changed, nil
}

func snapshotCompanyFolder(root, folder string) ([]CompanyFile, error) {
	folder = strings.ReplaceAll(folder, `\`, "/")
	if !validCompanyAsset(folder + "/asset.cfg") {
		return nil, fmt.Errorf("choose a game asset folder")
	}
	base := filepath.Join(root, filepath.FromSlash(folder))
	var files []CompanyFile
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked asset: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !validCompanyAsset(rel) || companyRuntimeArtifact(rel) {
			return nil
		}
		actual, err := companyAssetPath(root, rel)
		if err != nil {
			return err
		}
		digest, err := companyFileHash(actual)
		if err != nil {
			return err
		}
		files = append(files, CompanyFile{rel, digest})
		if len(files) > companyFileLimit {
			return fmt.Errorf("too many assets")
		}
		return nil
	})
	if err == nil && len(files) == 0 {
		err = fmt.Errorf("folder has no game assets")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

func refreshCompanyInventory(root string, original CompanyProfile) (CompanyProfile, []string, error) {
	if err := validateCompanyProfile(original); err != nil {
		return original, nil, err
	}
	data, err := json.Marshal(original)
	if err != nil {
		return original, nil, err
	}
	var updated CompanyProfile
	if err := json.Unmarshal(data, &updated); err != nil {
		return original, nil, err
	}
	folders := map[string][]CompanyFile{}
	changes := map[string]bool{}
	for i, pkg := range updated.Packages {
		previous := map[string]CompanyFile{}
		for _, file := range pkg.Files {
			previous[companyAssetKey(file.Path)] = file
		}
		current := map[string]CompanyFile{}
		for _, folder := range pkg.Folders {
			key := companyAssetKey(folder)
			files, seen := folders[key]
			if !seen {
				files, err = snapshotCompanyFolder(root, folder)
				if err != nil {
					return original, nil, fmt.Errorf("cannot update folder %s: %w", folder, err)
				}
				folders[key] = files
			}
			for _, file := range files {
				current[companyAssetKey(file.Path)] = file
			}
		}
		updated.Packages[i].Files = nil
		for key, file := range current {
			updated.Packages[i].Files = append(updated.Packages[i].Files, file)
			old, existed := previous[key]
			if !existed {
				changes["+ "+file.Path] = true
			} else if !strings.EqualFold(old.SHA256, file.SHA256) {
				changes["~ "+file.Path] = true
			}
		}
		for key, old := range previous {
			if _, exists := current[key]; !exists {
				changes["- "+old.Path] = true
			}
		}
		sort.Slice(updated.Packages[i].Files, func(a, b int) bool { return updated.Packages[i].Files[a].Path < updated.Packages[i].Files[b].Path })
	}
	if err := validateCompanyProfile(updated); err != nil {
		return original, nil, err
	}
	var list []string
	for change := range changes {
		list = append(list, change)
	}
	sort.Strings(list)
	return updated, list, nil
}

var companyReportTemplate = template.Must(template.New("requirements").Parse(`<!doctype html>
<html lang="{{.Lang}}"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>{{.Title}}</title><style>body{font:17px system-ui,sans-serif;background:#101923;color:#eef4fa;max-width:900px;margin:40px auto;padding:0 20px}article{background:#1c2a39;border-radius:12px;padding:20px;margin:18px 0}h1{font-size:28px}h2{font-size:21px}p{line-height:1.5;white-space:pre-wrap}a{display:inline-block;background:#a8d6ff;color:#102133;padding:10px 16px;border-radius:8px;text-decoration:none}small{color:#bdccda}</style>
<h1>{{.Title}}</h1><p>{{.Company}}</p><p>{{.Intro}}</p>
{{range .Problems}}<article><h2>{{.Name}}</h2><p>{{.Detail}}</p>{{if .DownloadURL}}<a href="{{.DownloadURL}}" target="_blank" rel="noopener noreferrer">{{$.Link}}</a>{{else}}<small>{{$.NoLink}}</small>{{end}}</article>{{end}}
<p>{{.Retry}}</p></html>`))

func writeCompanyReport(runtimeDir, lang, company string, problems []CompanyProblem) (string, error) {
	data := struct {
		Lang, Title, Company, Intro, Link, NoLink, Retry string
		Problems                                         []CompanyProblem
	}{
		normalizeLanguage(lang), localText(lang, "Antes de entrar na sessão", "Before joining the session", "Vor dem Beitritt"), company,
		localText(lang, "A viagem não foi aberta. Confira os itens abaixo. Os links foram cadastrados pelo administrador da empresa.", "The trip was not opened. Check the items below. Links were provided by the company administrator.", "Die Fahrt wurde nicht gestartet. Prüfe die folgenden Punkte. Die Links wurden vom Firmenadministrator hinterlegt."),
		localText(lang, "Abrir página do download", "Open download page", "Downloadseite öffnen"),
		localText(lang, "Peça ao administrador o link ou a configuração correta.", "Ask the administrator for the link or correct configuration.", "Frage den Administrator nach dem Link oder der richtigen Konfiguration."),
		localText(lang, "Corrija os itens indicados; o administrador pode precisar ajustar a sessão. Depois, inicie a viagem novamente pelo BBS. Para jogar sozinho, desative o multiplayer na opção 8 do Setup.", "Resolve the listed items; the administrator may need to adjust the session. Then start the trip again through BBS. To play alone, disable multiplayer with Setup option 8.", "Behebe die aufgeführten Punkte; der Administrator muss gegebenenfalls die Sitzung anpassen. Starte die Fahrt anschließend erneut über BBS. Für Einzelspieler deaktiviere Multiplayer mit Setup-Option 8."), problems,
	}
	var b bytes.Buffer
	if err := companyReportTemplate.Execute(&b, data); err != nil {
		return "", err
	}
	path := filepath.Join(runtimeDir, "multiplayer-requirements.html")
	if err := os.WriteFile(path, b.Bytes(), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func compatibleCompanyServer(version string, protocol int) bool {
	return protocol == multiplayerProtocol && validOpenOMSIVersion(strings.TrimSpace(version))
}
