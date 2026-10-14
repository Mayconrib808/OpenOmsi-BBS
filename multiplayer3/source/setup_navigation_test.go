package main

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManualFinishCanReturnToSavedFormAndErrorsAreRecorded(t *testing.T) {
	root := t.TempDir()
	dir := appDirectory(t.TempDir())
	ensureLayout(dir)
	os.WriteFile(filepath.Join(root, "Omsi.exe"), []byte("original"), 0600)
	os.MkdirAll(filepath.Join(dir, "server"), 0755)
	os.WriteFile(filepath.Join(dir, "server", "openomsi.exe"), []byte("server"), 0600)
	game := filepath.Join(t.TempDir(), "openomsi.exe")
	os.WriteFile(game, []byte("client"), 0600)
	off := false
	writePreferences(dir, "en", &off)
	writeControls(dir, &off, nil)
	defer setLanguage("pt-BR")
	opened := make(chan string, 1)
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- setupWithOptions(dir, "", setupOptions{Open: func(s string) error { opened <- s; return nil }, CheckClosed: func() error { return nil }, Stop: stop})
	}()
	defer func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("setup did not stop")
		}
	}()
	var address string
	select {
	case address = <-opened:
	case <-time.After(2 * time.Second):
		t.Fatal("did not open")
	}
	u, _ := url.Parse(address)
	token := u.Query().Get("token")
	origin := u.Scheme + "://" + u.Host
	client := &http.Client{Timeout: 2 * time.Second}
	post := func(values url.Values) string {
		values.Set("token", token)
		req, _ := http.NewRequest("POST", address, strings.NewReader(values.Encode()))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatal(resp.StatusCode, string(body))
		}
		return string(body)
	}
	secret := "https://example.invalid/rooms/private-invitation"
	saved := post(url.Values{"action": {"save"}, "root": {root}, "game": {game}, "player": {"Saved Player"}, "room_mode": {"join"}, "invite": {secret}, "server_mode": {"recommended"}, "zone": {"Europe/Berlin"}, "offset": {"-8"}})
	if !strings.Contains(saved, "View or edit configuration") {
		t.Fatal("saved page has no return control")
	}
	before, _ := os.ReadFile(configPath(dir))
	finished := post(url.Values{"action": {"finish"}})
	if !strings.Contains(finished, "View or edit configuration") {
		t.Fatal("finish has no return control")
	}
	form := post(url.Values{"action": {"editconfiguration"}})
	for _, value := range []string{root, game, "Saved Player", secret} {
		if !strings.Contains(form, value) {
			t.Fatal("saved field missing", value)
		}
	}
	after, _ := os.ReadFile(configPath(dir))
	if !bytes.Equal(before, after) {
		t.Fatal("view/edit changed saved configuration")
	}
	errorPage := post(url.Values{"action": {"configurebcs"}})
	if !strings.Contains(errorPage, "bcs settings not found") {
		t.Fatal("underlying setup error not shown")
	}
	log, _ := os.ReadFile(filepath.Join(dir, "logs", "setup.log"))
	if !bytes.Contains(log, []byte("bcs settings not found")) || bytes.Contains(log, []byte(secret)) {
		t.Fatal("missing diagnostic or invitation leaked")
	}
}
