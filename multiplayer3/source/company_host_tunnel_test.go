package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTunnelAddressRequiresNonemptyExactURL(t *testing.T) {
	for _, address := range []string{"", " ", "|", "https://", "http://map-ready.trycloudflare.com", "https://map-ready.trycloudflare.com/", "https://map-ready.trycloudflare.com.evil.invalid", "prefix https://map-ready.trycloudflare.com"} {
		if validTunnel(address) {
			t.Errorf("accepted invalid tunnel address %q", address)
		}
	}
	if !validTunnel("https://map-ready.trycloudflare.com") {
		t.Fatal("rejected the public tunnel address")
	}
}

// cloudflared 2026.9.3 frames the URL with vertical bars. A bar trims to
// an empty field; accepting it used to stop the scan before the actual URL.
func TestCloudflareFramedOutputDiscoversTunnelAcrossWrites(t *testing.T) {
	const address = "https://map-ready.trycloudflare.com"
	const transcript = "2026-10-15T05:58:20Z INF +------------------------------------------+\n" +
		"2026-10-15T05:58:20Z INF | Your quick Tunnel has been created! |\n" +
		"2026-10-15T05:58:20Z INF | https://map-ready.trycloudflare.com |\r\n" +
		"2026-10-15T05:58:20Z INF | DNS Resolution PASS |\n"
	for _, chunk := range []int{1, 7, len(transcript)} {
		var output bytes.Buffer
		writer := &companyTunnelOutput{output: &output}
		for start := 0; start < len(transcript); start += chunk {
			end := start + chunk
			if end > len(transcript) {
				end = len(transcript)
			}
			if n, err := writer.Write([]byte(transcript[start:end])); err != nil || n != end-start {
				t.Fatalf("chunk %d: write failed: %d, %v", chunk, n, err)
			}
		}
		text := output.String()
		announcement := "tunnel: the session is reachable at " + address + "\n"
		if strings.Count(text, announcement) != 1 || strings.Contains(text, "tunnel: the session is reachable at \n") {
			t.Fatalf("chunk %d: tunnel URL lost or blank announcement emitted: %q", chunk, text)
		}
		reader := &tunnelLog{}
		_, _ = reader.Write(output.Bytes())
		if reader.get() != address {
			t.Fatalf("chunk %d: host never received the public tunnel URL", chunk)
		}
	}
}
