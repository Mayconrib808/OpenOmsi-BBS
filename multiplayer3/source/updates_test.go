package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignedUpdatesRejectWrongProductChannelOriginAndTampering(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	origin := "https://v3-updates.example"
	release := bridgeRelease{Product: "bbs-openomsi-multiplayer", Channel: "preview", Schema: 1, Sequence: 2026101202, Version: "3.0.2-dev.2", Installer: bridgeReleaseFile{URL: origin + "/packages/" + strings.Repeat("a", 64) + ".exe", SHA256: strings.Repeat("a", 64), Size: 1024}}
	sign := func(r bridgeRelease) []byte {
		b, _ := json.Marshal(r)
		envelope := bridgeReleaseEnvelope{Payload: base64.StdEncoding.EncodeToString(b), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, b))}
		out, _ := json.Marshal(envelope)
		return out
	}
	valid := sign(release)
	if _, err := decodeBridgeRelease(valid, public, origin); err != nil {
		t.Fatal(err)
	}
	wrong := release
	wrong.Product = "old-bridge-2"
	if _, err := decodeBridgeRelease(sign(wrong), public, origin); err == nil {
		t.Fatal("2.x package accepted by 3.x")
	}
	wrong = release
	wrong.Channel = "stable"
	if _, err := decodeBridgeRelease(sign(wrong), public, origin); err == nil {
		t.Fatal("wrong channel accepted")
	}
	wrong = release
	wrong.Installer.URL = "https://other.example/packages/" + strings.Repeat("a", 64) + ".exe"
	if _, err := decodeBridgeRelease(sign(wrong), public, origin); err == nil {
		t.Fatal("foreign origin accepted")
	}
	valid[len(valid)/2] ^= 1
	if _, err := decodeBridgeRelease(valid, public, origin); err == nil {
		t.Fatal("tampered signature accepted")
	}
}
