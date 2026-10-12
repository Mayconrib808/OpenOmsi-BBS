package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Recommended is the bundled, patched build that preserves free player buses.
type serverPreference struct {
	Mode    string `json:"mode"`
	Version string `json:"version,omitempty"`
}

var releaseVersion = regexp.MustCompile(`^0\.[0-9]{1,4}\.[0-9]{1,6}$`)

func normalizeServer(p serverPreference) (serverPreference, error) {
	if p.Mode == "" {
		p.Mode = "recommended"
	}
	p.Version = strings.TrimPrefix(strings.TrimSpace(p.Version), "v")
	switch p.Mode {
	case "recommended", "latest":
		p.Version = ""
	case "manual":
		if !releaseVersion.MatchString(p.Version) {
			return p, fmt.Errorf("informe o número da versão, por exemplo 0.2.27")
		}
	default:
		return p, fmt.Errorf("modo de servidor inválido")
	}
	return p, nil
}

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type serverRelease struct {
	Tag        string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}
type serverReceipt struct {
	Version       string            `json:"version"`
	ArchiveSHA256 string            `json:"archive_sha256"`
	Files         map[string]string `json:"files"`
}

func trustedDownload(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	switch u.Hostname() {
	case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}
func downloadClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || !trustedDownload(r.URL.String()) {
			return fmt.Errorf("redirecionamento de download inválido")
		}
		return nil
	}}
}
func hashFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func verifiedRuntime(dir string, receipt serverReceipt) error {
	for _, name := range []string{"openomsi.exe", "steam_api64.dll", "vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll"} {
		if receipt.Files[name] == "" {
			return fmt.Errorf("runtime incompleto: %s", name)
		}
	}
	for name, want := range receipt.Files {
		if filepath.Base(name) != name || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(want) {
			return fmt.Errorf("manifesto inválido")
		}
		got, e := hashFile(filepath.Join(dir, name))
		if e != nil || got != want {
			return fmt.Errorf("arquivo do servidor ausente ou alterado: %s", name)
		}
	}
	return nil
}
func recommendedServer(dir string) (string, error) {
	folder := filepath.Join(dir, "server")
	b, e := os.ReadFile(filepath.Join(folder, "bbs-server.json"))
	if e != nil {
		return "", e
	}
	var receipt serverReceipt
	if e = json.Unmarshal(b, &receipt); e != nil {
		return "", e
	}
	if e = verifiedRuntime(folder, receipt); e != nil {
		return "", e
	}
	return filepath.Join(folder, "openomsi.exe"), nil
}
func cachedServer(dir, version string) (string, error) {
	folder := filepath.Join(dir, "server-cache", version)
	b, e := os.ReadFile(filepath.Join(folder, "verified.json"))
	if e != nil {
		return "", e
	}
	var receipt serverReceipt
	if e = json.Unmarshal(b, &receipt); e != nil || receipt.Version != version {
		return "", fmt.Errorf("cache inválido")
	}
	if e = verifiedRuntime(folder, receipt); e != nil {
		return "", e
	}
	return filepath.Join(folder, "openomsi.exe"), nil
}
func releaseSelection(r serverRelease, requested string) (string, releaseAsset, error) {
	return runtimeReleaseSelection(r, requested, false)
}
func runtimeReleaseSelection(r serverRelease, requested string, client bool) (string, releaseAsset, error) {
	v := strings.TrimPrefix(r.Tag, "v")
	if !releaseVersion.MatchString(v) || r.Draft || r.Prerelease || (requested != "" && v != requested) {
		return "", releaseAsset{}, fmt.Errorf("release de servidor inválida")
	}
	suffix := "-server-windows-x64.zip"
	if client {
		suffix = "-windows-x64.zip"
	}
	name := "openOMSI-" + v + suffix
	for _, a := range r.Assets {
		if a.Name == name {
			if !trustedDownload(a.URL) || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(a.Digest) || a.Size < 1 || a.Size > 256<<20 {
				return "", a, fmt.Errorf("release sem download e SHA-256 verificáveis")
			}
			return v, a, nil
		}
	}
	return "", releaseAsset{}, fmt.Errorf("a versão %s não oferece servidor Windows x64", v)
}
func resolveServer(ctx context.Context, dir string, p serverPreference) (string, error) {
	return resolveRuntime(ctx, dir, p, false)
}
func resolveClient(ctx context.Context, dir string) (string, error) {
	return resolveRuntime(ctx, dir, serverPreference{Mode: "latest"}, true)
}
func resolveRuntime(ctx context.Context, dir string, p serverPreference, client bool) (string, error) {
	cacheName := "server-cache"
	cached := cachedServer
	unpack := unpackServer
	if client {
		cacheName = "client"
		cached = cachedClient
		unpack = unpackClient
	}
	p, e := normalizeServer(p)
	if e != nil {
		return "", e
	}
	if p.Mode == "recommended" {
		return recommendedServer(dir)
	}
	if p.Mode == "manual" {
		if exe, e := cached(dir, p.Version); e == nil {
			return exe, nil
		}
	}
	endpoint := "https://api.github.com/repos/openOMSI-org/openOMSI/releases/latest"
	if p.Mode == "manual" {
		endpoint = "https://api.github.com/repos/openOMSI-org/openOMSI/releases/tags/v" + p.Version
	}
	c := webClient()
	defer c.CloseIdleConnections()
	var r serverRelease
	if e = requestJSON(ctx, c, "GET", endpoint, "", nil, &r); e != nil {
		if client {
			return "", fmt.Errorf("não foi possível consultar a versão mais recente do cliente openOMSI; verifique a conexão e tente novamente: %w", e)
		}
		return "", fmt.Errorf("não foi possível consultar a versão do servidor; escolha Recomendada para usar o pacote local: %w", e)
	}
	v, a, e := runtimeReleaseSelection(r, p.Version, client)
	if e != nil {
		return "", e
	}
	if exe, e := cached(dir, v); e == nil {
		return exe, nil
	}
	cache := filepath.Join(dir, cacheName)
	if e = os.MkdirAll(cache, 0755); e != nil {
		return "", e
	}
	stage, e := os.MkdirTemp(cache, "download-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, "server.zip")
	req, e := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if e != nil {
		return "", e
	}
	dl := downloadClient()
	defer dl.CloseIdleConnections()
	resp, e := dl.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download do servidor retornou HTTP %d", resp.StatusCode)
	}
	f, e := os.Create(archive)
	if e != nil {
		return "", e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, a.Size+1))
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if n != a.Size || "sha256:"+digest != a.Digest {
		return "", fmt.Errorf("download incompleto ou SHA-256 incorreto; nenhum servidor foi instalado")
	}
	runtime := filepath.Join(stage, "runtime")
	receipt, e := unpack(archive, runtime, v, digest)
	if e != nil {
		return "", e
	}
	b, _ := json.MarshalIndent(receipt, "", "  ")
	if e = os.WriteFile(filepath.Join(runtime, "verified.json"), b, 0600); e != nil {
		return "", e
	}
	target := filepath.Join(cache, v)
	if e = os.Rename(runtime, target); e != nil {
		if exe, existing := cached(dir, v); existing == nil {
			return exe, nil
		}
		return "", fmt.Errorf("cache %s inválido ou em uso; remova somente essa pasta e tente novamente", v)
	}
	return filepath.Join(target, "openomsi.exe"), nil
}
func unpackServer(archive, dest, version, digest string) (serverReceipt, error) {
	result := serverReceipt{Version: version, ArchiveSHA256: digest, Files: map[string]string{}}
	z, e := zip.OpenReader(archive)
	if e != nil {
		return result, e
	}
	defer z.Close()
	prefix := ""
	found := false
	for _, f := range z.File {
		if path.Base(f.Name) == "openomsi.exe" {
			if found {
				return result, fmt.Errorf("ZIP contém mais de um servidor")
			}
			prefix = strings.TrimSuffix(f.Name, "openomsi.exe")
			found = true
		}
	}
	if !found {
		return result, fmt.Errorf("ZIP sem openomsi.exe")
	}
	if e = os.MkdirAll(dest, 0755); e != nil {
		return result, e
	}
	var total uint64
	for _, f := range z.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.Contains(f.Name, "\\") || strings.HasPrefix(f.Name, "/") || path.Clean(f.Name) != f.Name || strings.Contains(f.Name, "../") || f.Mode()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("caminho inválido no ZIP")
		}
		if path.Base(name) != name {
			continue
		}
		lower := strings.ToLower(name)
		if lower != "openomsi.exe" && !strings.HasSuffix(lower, ".dll") && !strings.HasSuffix(lower, ".txt") {
			continue
		}
		total += f.UncompressedSize64
		if total > 512<<20 {
			return result, fmt.Errorf("servidor excede limite de extração")
		}
		if _, exists := result.Files[lower]; exists {
			return result, fmt.Errorf("arquivo duplicado no ZIP")
		}
		in, e := f.Open()
		if e != nil {
			return result, e
		}
		out, e := os.OpenFile(filepath.Join(dest, lower), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return result, e
		}
		h := sha256.New()
		_, e = io.Copy(io.MultiWriter(out, h), io.LimitReader(in, int64(f.UncompressedSize64)+1))
		ce := out.Close()
		in.Close()
		if e != nil {
			return result, e
		}
		if ce != nil {
			return result, ce
		}
		result.Files[lower] = hex.EncodeToString(h.Sum(nil))
	}
	return result, verifiedRuntime(dest, result)
}

// Keep ownership during a large first download; guests keep waiting for this host.
func prepareOwnedServer(ctx context.Context, c *http.Client, cfg localConfig, dir, m string, l lease) (string, error) {
	downloadCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	type answer struct {
		exe string
		err error
	}
	done := make(chan answer, 1)
	go func() { exe, e := resolveServer(downloadCtx, dir, cfg.Server); done <- answer{exe, e} }()
	tick := time.NewTicker(8 * time.Second)
	defer tick.Stop()
	lastAck := time.Now()
	for {
		select {
		case <-downloadCtx.Done():
			return "", downloadCtx.Err()
		case result := <-done:
			if result.err != nil {
				return "", result.err
			}
			if _, e := renew(ctx, c, cfg.Invitation, m, l.Token, "starting", ""); e != nil {
				return "", e
			}
			return result.exe, nil
		case <-tick.C:
			if _, e := renew(ctx, c, cfg.Invitation, m, l.Token, "starting", ""); e == nil {
				lastAck = time.Now()
			} else {
				var he httpError
				if errors.As(e, &he) && he.Code == 409 {
					return "", fmt.Errorf("a reserva do mapa expirou durante o download")
				}
				if time.Since(lastAck) > 25*time.Second {
					return "", fmt.Errorf("perdida a reserva do mapa durante o download: %w", e)
				}
			}
		}
	}
}
