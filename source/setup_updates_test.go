package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateSignedManifestRejectsTamperingAndUnsafeAssets(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("MZtest")))
	valid := bridgeRelease{Schema: 1, Sequence: 2, Version: "2.1.2", Notes: map[string]string{"pt": "Correções"}, Installer: bridgeReleaseFile{URL: "https://updates.example/packages/" + hash + ".exe", SHA256: hash, Size: 6}}
	sign := func(r bridgeRelease) []byte {
		p, _ := json.Marshal(r)
		raw, _ := json.Marshal(bridgeReleaseEnvelope{Payload: base64.StdEncoding.EncodeToString(p), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, p))})
		return raw
	}
	raw := sign(valid)
	if _, err := decodeBridgeRelease(raw, pub, "https://updates.example"); err != nil {
		t.Fatal(err)
	}
	var envelope bridgeReleaseEnvelope
	_ = json.Unmarshal(raw, &envelope)
	envelope.Payload = base64.StdEncoding.EncodeToString([]byte(`{"version":"bad"}`))
	bad, _ := json.Marshal(envelope)
	if _, err := decodeBridgeRelease(bad, pub, "https://updates.example"); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	for _, url := range []string{"http://updates.example/packages/" + hash + ".exe", "https://attacker.example/packages/" + hash + ".exe", "https://updates.example/other.exe", "https://updates.example/packages/" + hash + ".exe?token=x"} {
		r := valid
		r.Installer.URL = url
		if _, err := decodeBridgeRelease(sign(r), pub, "https://updates.example"); err == nil {
			t.Fatal("unsafe URL accepted", url)
		}
	}
	for _, size := range []int64{0, updateDownloadLimit + 1} {
		r := valid
		r.Installer.Size = size
		if _, err := decodeBridgeRelease(sign(r), pub, "https://updates.example"); err == nil {
			t.Fatal("unsafe size accepted")
		}
	}
}
func TestUpdateDownloadChecksBytesAndCleansPartialFiles(t *testing.T) {
	data := []byte("MZreal-installer")
	body := data
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	asset := bridgeReleaseFile{URL: server.URL, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Size: int64(len(data))}
	dir := t.TempDir()
	path, err := downloadBridgeRelease(context.Background(), server.Client(), asset, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyDownloadedBridgeRelease(path, asset.SHA256); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("MZtampered-cache"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = downloadBridgeRelease(context.Background(), server.Client(), asset, dir); err != nil {
		t.Fatal(err)
	}
	body = []byte("MZcorrupted")
	if _, err = downloadBridgeRelease(context.Background(), server.Client(), asset, dir); err == nil {
		t.Fatal("corrupted download accepted")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) != 0 {
		t.Fatal("partial download remains", matches)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = downloadBridgeRelease(ctx, server.Client(), asset, dir); err == nil {
		t.Fatal("cancelled request accepted")
	}
}
func TestUpdateUnconfiguredDoesNotInventEndpoint(t *testing.T) {
	if _, err := checkBridgeRelease(context.Background(), bridgeUpdateClient(), ""); err != errUpdateNotConfigured {
		t.Fatal(err)
	}
}
