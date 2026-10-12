package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const githubReleaseAPI = "https://api.github.com/repos/Mayconrib808/OpenOmsi-BBS/releases"
const githubReleaseRoot = "https://github.com/Mayconrib808/OpenOmsi-BBS/releases/"

var v3Tag = regexp.MustCompile(`^v?(3)\.(\d+)\.(\d+)(?:-dev\.(\d+))?$`)

type githubRelease struct {
	Tag    string `json:"tag_name"`
	Draft  bool   `json:"draft"`
	Page   string `json:"html_url"`
	Assets []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	} `json:"assets"`
}

func newerV3(candidate, current string) bool {
	a, b := v3Tag.FindStringSubmatch(candidate), v3Tag.FindStringSubmatch(current)
	if a == nil || b == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(a[i])
		y, _ := strconv.Atoi(b[i])
		if x != y {
			return x > y
		}
	}
	if a[4] == "" || b[4] == "" {
		return a[4] == "" && b[4] != ""
	}
	x, _ := strconv.Atoi(a[4])
	y, _ := strconv.Atoi(b[4])
	return x > y
}
func selectGitHubV3(releases []githubRelease, current string) *bridgeRelease {
	var selected *bridgeRelease
	for _, r := range releases {
		if r.Draft || !newerV3(r.Tag, current) || (selected != nil && !newerV3(r.Tag, selected.Version)) {
			continue
		}
		tag := strings.TrimPrefix(r.Tag, "v")
		// Bind release URLs and updater assets to this exact repository and version.
		expectedPage := githubReleaseRoot + "tag/" + r.Tag
		if r.Page != expectedPage {
			continue
		}
		found := &bridgeRelease{Product: "bbs-openomsi-multiplayer", Channel: "preview", Schema: 1, Version: tag, DetailsURL: r.Page}
		name := "Atualizar-Multiplayer-" + tag + ".exe"
		for _, asset := range r.Assets {
			hash := strings.TrimPrefix(asset.Digest, "sha256:")
			h, e := hex.DecodeString(hash)
			expectedURL := githubReleaseRoot + "download/" + r.Tag + "/" + name
			if asset.Name == name && asset.URL == expectedURL && strings.HasPrefix(asset.Digest, "sha256:") && e == nil && len(h) == 32 && asset.Size >= 2 && asset.Size <= updateDownloadLimit {
				found.Installer = bridgeReleaseFile{URL: asset.URL, SHA256: hash, Size: asset.Size}
				break
			}
		}
		selected = found
	}
	return selected
}
func checkGitHubV3(ctx context.Context, client *http.Client) (*bridgeRelease, error) {
	var all []githubRelease
	// Pagination avoids losing 3.x behind newer 2.x publication dates.
	for page := 1; page <= 10; page++ {
		req, e := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s?per_page=100&page=%d", githubReleaseAPI, page), nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "BBS-OpenOmsi-Multiplayer/"+version)
		resp, e := client.Do(req)
		if e != nil {
			return nil, fmt.Errorf("update service could not be reached")
		}
		data, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if e != nil || resp.StatusCode != 200 {
			return nil, fmt.Errorf("GitHub update service returned HTTP %d", resp.StatusCode)
		}
		var releases []githubRelease
		if e = json.Unmarshal(data, &releases); e != nil {
			return nil, fmt.Errorf("invalid GitHub update response")
		}
		all = append(all, releases...)
		if len(releases) < 100 {
			return selectGitHubV3(all, version), nil
		}
	}
	return nil, fmt.Errorf("release listing exceeded limit")
}
func approvedGitHubAsset(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	p := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(p) != 6 || strings.Join(p[:4], "/") != "Mayconrib808/OpenOmsi-BBS/releases/download" || v3Tag.FindStringSubmatch(p[4]) == nil {
		return false
	}
	return p[5] == "Atualizar-Multiplayer-"+strings.TrimPrefix(p[4], "v")+".exe"
}
