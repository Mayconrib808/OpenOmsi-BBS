package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateRoundTripFunc func(*http.Request) (*http.Response, error)

func (f updateRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func anchoredFixture(t *testing.T, origin, commit string, payload []byte) ([]byte, *http.Client) {
	t.Helper()
	envelope, err := json.Marshal(bridgeReleaseAnchor{Payload: base64.StdEncoding.EncodeToString(payload), SourceCommit: commit})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != githubReleaseURL(commit) {
			t.Fatalf("unexpected publisher URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}
	return envelope, client
}

func TestAnchoredUpdateMatchesImmutablePublisherCommit(t *testing.T) {
	origin := "https://v3-updates.example"
	commit := strings.Repeat("a", 40)
	release := bridgeRelease{Product: "bbs-openomsi-multiplayer", Channel: "preview", Schema: 1, Sequence: 2026101209, Version: "3.0.2-dev.9", Installer: bridgeReleaseFile{URL: origin + "/packages/" + strings.Repeat("b", 64) + ".exe", SHA256: strings.Repeat("b", 64), Size: 1024}}
	payload, _ := json.Marshal(release)
	envelope, client := anchoredFixture(t, origin, commit, payload)
	got, err := decodeAnchoredBridgeRelease(context.Background(), client, envelope, origin)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != release.Version || got.Sequence != release.Sequence {
		t.Fatalf("unexpected release: %+v", got)
	}
}

func TestAnchoredUpdateRejectsCloudflareMetadataDifferentFromGitHub(t *testing.T) {
	origin := "https://v3-updates.example"
	commit := strings.Repeat("c", 40)
	release := bridgeRelease{Product: "bbs-openomsi-multiplayer", Channel: "preview", Schema: 1, Sequence: 2026101209, Version: "3.0.2-dev.9", Installer: bridgeReleaseFile{URL: origin + "/packages/" + strings.Repeat("d", 64) + ".exe", SHA256: strings.Repeat("d", 64), Size: 1024}}
	trusted, _ := json.Marshal(release)
	envelope, _ := anchoredFixture(t, origin, commit, trusted)
	var anchor bridgeReleaseAnchor
	if err := json.Unmarshal(envelope, &anchor); err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), trusted...)
	tampered[len(tampered)-2] ^= 1
	anchor.Payload = base64.StdEncoding.EncodeToString(tampered)
	envelope, _ = json.Marshal(anchor)
	client := &http.Client{Transport: updateRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(trusted)), Body: io.NopCloser(bytes.NewReader(trusted)), Header: make(http.Header)}, nil
	})}
	if _, err := decodeAnchoredBridgeRelease(context.Background(), client, envelope, origin); err == nil {
		t.Fatal("Cloudflare metadata differing from publisher commit was accepted")
	}
}

func TestAnchoredUpdateRejectsInvalidCommit(t *testing.T) {
	origin := "https://v3-updates.example"
	release := bridgeRelease{Product: "bbs-openomsi-multiplayer", Channel: "preview", Schema: 1, Sequence: 2026101209, Version: "3.0.2-dev.9", Installer: bridgeReleaseFile{URL: origin + "/packages/" + strings.Repeat("e", 64) + ".exe", SHA256: strings.Repeat("e", 64), Size: 1024}}
	payload, _ := json.Marshal(release)
	envelope, _ := json.Marshal(bridgeReleaseAnchor{Payload: base64.StdEncoding.EncodeToString(payload), SourceCommit: "main"})
	if _, err := decodeAnchoredBridgeRelease(context.Background(), &http.Client{}, envelope, origin); err == nil {
		t.Fatal("mutable/non-SHA publisher ref was accepted")
	}
}
