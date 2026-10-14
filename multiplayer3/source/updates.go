package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Set only when packaging a release for the deployed distribution service.
// The player never receives a publication credential or a signing seed.
var bridgeUpdateOrigin = "https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev"
var bridgeUpdateSequence = "2026101206"

// /preview.json is kept for already-installed dev.8 clients. Starting with
// dev.9, metadata is anchored to an immutable GitHub commit while binaries stay
// on Cloudflare. This avoids depending on an offline signing key for every
// preview hotfix without trusting mutable Cloudflare metadata by itself.
const bridgeUpdateManifestPath = "/preview-v2.json"
const bridgeLegacyManifestPath = "/preview.json"
const bridgeUpdateRepository = "Mayconrib808/OpenOmsi-BBS"
const bridgeUpdateReleasePath = "cloudflare-updates-v3/release-v2.json"
const bridgeUpdatePublicKey = "GKwwvTrPr3W3ctc9dfPrGtsjztkET1XAVPnAIlbwKLU="
const updateDownloadLimit int64 = 64 << 20

var errUpdateNotConfigured = errors.New("update distribution not configured")
var updateCommit = regexp.MustCompile(`^[a-f0-9]{40}$`)

type bridgeRelease struct {
	DetailsURL string            `json:"-"`
	Product    string            `json:"product"`
	Channel    string            `json:"channel"`
	Schema     int               `json:"schema"`
	Sequence   uint64            `json:"sequence"`
	Version    string            `json:"version"`
	Notes      map[string]string `json:"notes"`
	Installer  bridgeReleaseFile `json:"installer"`
}
type bridgeReleaseFile struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type bridgeReleaseEnvelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}
type bridgeReleaseAnchor struct {
	Payload      string `json:"payload"`
	SourceCommit string `json:"source_commit"`
}

func bridgeReleaseSequence() uint64 {
	n, _ := strconv.ParseUint(bridgeUpdateSequence, 10, 64)
	return n
}

func updateOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid HTTPS distribution address")
	}
	u.Path = ""
	return u, nil
}

func parseBridgeReleasePayload(payload []byte, origin string) (bridgeRelease, error) {
	var release bridgeRelease
	if len(payload) == 0 || len(payload) > 64<<10 {
		return release, fmt.Errorf("invalid update manifest")
	}
	if err := json.Unmarshal(payload, &release); err != nil {
		return release, fmt.Errorf("invalid update manifest")
	}
	base, err := updateOrigin(origin)
	u, e := url.Parse(release.Installer.URL)
	h, he := hex.DecodeString(release.Installer.SHA256)
	if err != nil || e != nil || release.Schema != 1 || v3Tag.FindStringSubmatch(release.Version) == nil || release.Product != "bbs-openomsi-multiplayer" || release.Channel != "preview" || release.Sequence == 0 ||
		len(release.Version) == 0 || len(release.Version) > 64 ||
		strings.ContainsAny(release.Version, "\r\n\x00") || release.Installer.Size < 2 ||
		release.Installer.Size > updateDownloadLimit || he != nil || len(h) != sha256.Size ||
		u.Scheme != "https" || u.Host != base.Host || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		u.Path != "/packages/"+release.Installer.SHA256+".exe" {
		return bridgeRelease{}, fmt.Errorf("invalid update package or address")
	}
	for lang, notes := range release.Notes {
		if (lang != "pt" && lang != "en" && lang != "de") || len(notes) > 12000 || strings.ContainsRune(notes, 0) {
			return bridgeRelease{}, fmt.Errorf("invalid release notes")
		}
	}
	return release, nil
}

// Legacy decoder retained so dev.9 can still understand the existing signed
// dev.8 manifest if the v2 endpoint has not propagated yet.
func decodeBridgeRelease(raw []byte, key ed25519.PublicKey, origin string) (bridgeRelease, error) {
	var release bridgeRelease
	if len(raw) > 64<<10 || len(key) != ed25519.PublicKeySize {
		return release, fmt.Errorf("invalid update response")
	}
	var envelope bridgeReleaseEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return release, fmt.Errorf("invalid signed update response")
	}
	payload, e1 := base64.StdEncoding.DecodeString(envelope.Payload)
	signature, e2 := base64.StdEncoding.DecodeString(envelope.Signature)
	if e1 != nil || e2 != nil || !ed25519.Verify(key, payload, signature) {
		return release, fmt.Errorf("update signature does not match the publisher")
	}
	return parseBridgeReleasePayload(payload, origin)
}

func githubReleaseURL(commit string) string {
	return "https://raw.githubusercontent.com/" + bridgeUpdateRepository + "/" + commit + "/" + bridgeUpdateReleasePath
}

func decodeAnchoredBridgeRelease(ctx context.Context, client *http.Client, raw []byte, origin string) (bridgeRelease, error) {
	var envelope bridgeReleaseAnchor
	if len(raw) == 0 || len(raw) > 64<<10 || json.Unmarshal(raw, &envelope) != nil || !updateCommit.MatchString(envelope.SourceCommit) {
		return bridgeRelease{}, fmt.Errorf("invalid anchored update response")
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil || len(payload) == 0 || len(payload) > 64<<10 {
		return bridgeRelease{}, fmt.Errorf("invalid anchored update payload")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleaseURL(envelope.SourceCommit), nil)
	if err != nil {
		return bridgeRelease{}, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "OpenOmsi-BBS-Setup/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return bridgeRelease{}, fmt.Errorf("publisher metadata could not be verified")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || (resp.ContentLength > 64<<10) {
		return bridgeRelease{}, fmt.Errorf("publisher metadata returned HTTP %d", resp.StatusCode)
	}
	trusted, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(trusted) > 64<<10 || !bytes.Equal(trusted, payload) {
		return bridgeRelease{}, fmt.Errorf("Cloudflare update metadata does not match the publisher commit")
	}
	return parseBridgeReleasePayload(payload, origin)
}

func bridgeUpdateClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(r *http.Request, previous []*http.Request) error {
		if len(previous) > 3 || len(previous) == 0 || r.URL.Scheme != "https" {
			return fmt.Errorf("update redirect refused")
		}
		if approvedGitHubAsset(previous[0].URL.String()) {
			if r.URL.Host != "release-assets.githubusercontent.com" {
				return fmt.Errorf("GitHub asset redirect refused")
			}
			return nil
		}
		if r.URL.Host != previous[0].URL.Host {
			return fmt.Errorf("update redirect refused")
		}
		return nil
	}}
}

func readUpdateManifest(ctx context.Context, client *http.Client, base *url.URL, manifestPath string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String()+manifestPath, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "OpenOmsi-BBS-Setup/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("update service could not be reached")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return nil, resp.StatusCode, fmt.Errorf("update response could not be read")
	}
	return raw, resp.StatusCode, nil
}

func checkBridgeRelease(ctx context.Context, client *http.Client, origin string) (*bridgeRelease, error) {
	if origin == "" {
		return nil, errUpdateNotConfigured
	}
	base, err := updateOrigin(origin)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	raw, status, err := readUpdateManifest(ctx, client, base, bridgeUpdateManifestPath)
	if err != nil {
		return nil, err
	}
	var release bridgeRelease
	if status == http.StatusOK {
		release, err = decodeAnchoredBridgeRelease(ctx, client, raw, origin)
	} else if status == http.StatusNotFound {
		// During the one-time dev.8 -> dev.9 migration the old signed endpoint
		// remains untouched. dev.9 can safely treat it as a fallback/current feed.
		raw, status, err = readUpdateManifest(ctx, client, base, bridgeLegacyManifestPath)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("update service returned HTTP %d", status)
		}
		key, _ := base64.StdEncoding.DecodeString(bridgeUpdatePublicKey)
		release, err = decodeBridgeRelease(raw, key, origin)
	} else {
		return nil, fmt.Errorf("update service returned HTTP %d", status)
	}
	if err != nil {
		return nil, err
	}
	if release.Sequence <= bridgeReleaseSequence() {
		return nil, nil
	}
	return &release, nil
}

func downloadBridgeRelease(ctx context.Context, client *http.Client, asset bridgeReleaseFile, directory string) (path string, err error) {
	if asset.Size < 2 || asset.Size > updateDownloadLimit {
		return "", fmt.Errorf("invalid update size")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("update download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || (resp.ContentLength >= 0 && resp.ContentLength != asset.Size) {
		return "", fmt.Errorf("update download has an unexpected status or size")
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(directory, ".bridge-download-*.tmp")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer func() { f.Close(); os.Remove(tmp) }()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, asset.Size+1))
	if err != nil || n != asset.Size || fmt.Sprintf("%x", h.Sum(nil)) != asset.SHA256 {
		return "", fmt.Errorf("update package failed its size or SHA-256 check")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	path = filepath.Join(directory, "Atualizar-Bridge-"+asset.SHA256+".exe")
	// Never reuse a cached executable without rechecking its bytes.
	if old, e := os.ReadFile(path); e == nil {
		if int64(len(old)) != asset.Size || fmt.Sprintf("%x", sha256.Sum256(old)) != asset.SHA256 {
			if err = os.Remove(path); err != nil {
				return "", err
			}
		} else {
			return path, nil
		}
	}
	if err = os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

func verifyDownloadedBridgeRelease(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 2)
	if _, err = io.ReadFull(f, magic); err != nil || !bytes.Equal(magic, []byte("MZ")) {
		return fmt.Errorf("update executable is invalid")
	}
	h := sha256.New()
	h.Write(magic)
	if _, err = io.Copy(h, io.LimitReader(f, updateDownloadLimit)); err != nil || fmt.Sprintf("%x", h.Sum(nil)) != expected {
		return fmt.Errorf("downloaded update was modified before installation")
	}
	return nil
}

// This is a per-user Setup preference, independent of the game's configuration.
// Toggling it never requires activating the Bridge or saving an incomplete form.
