package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetupOpeningDoesNotActivateOrInspectBCSAndManualCheckWorks(t *testing.T) {
	root := t.TempDir()
	dir := appDirectory(root)
	_ = ensureLayout(dir)
	_ = os.WriteFile(filepath.Join(root, "Omsi.exe"), []byte("original"), 0600)
	jarDir := filepath.Join(root, "Busbetrieb-Simulator")
	_ = os.MkdirAll(jarDir, 0755)
	_ = os.WriteFile(filepath.Join(jarDir, "busbetrieb-simulator.jar"), []byte("original jar"), 0600)
	settingsPath := filepath.Join(jarDir, "config.cfg")
	before := bcsConfigFixture(root)
	_ = os.WriteFile(settingsPath, before, 0600)
	cfg := localConfig{Version: 3, Root: root, Player: "Test", Invitation: "https://company.example/rooms/" + strings.Repeat("b", 32)}
	_ = saveConfig(dir, cfg)
	configBefore, _ := os.ReadFile(configPath(dir))
	off := false
	_ = writePreferences(dir, "en", &off)
	defer setLanguage("pt-BR")
	var closedCalls, updateCalls atomic.Int32
	opened := make(chan string, 1)
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- setupWithOptions(dir, "", setupOptions{Open: func(address string) error { opened <- address; return nil }, CheckClosed: func() error { closedCalls.Add(1); return nil }, Stop: stop, CheckRelease: func(context.Context, *http.Client, string) (*bridgeRelease, error) {
			updateCalls.Add(1)
			return nil, nil
		}})
	}()
	defer func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("setup did not close")
		}
	}()
	var address string
	select {
	case address = <-opened:
	case <-time.After(2 * time.Second):
		t.Fatal("setup did not open")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Contains(b, []byte("Status has not been checked yet.")) || !bytes.Contains(b, []byte("Check for updates")) {
		t.Fatal("opening changed status or omitted manual check")
	}
	if closedCalls.Load() != 0 || updateCalls.Load() != 0 {
		t.Fatal("startup performed actions")
	}
	if current, _ := os.ReadFile(settingsPath); !bytes.Equal(current, before) {
		t.Fatal("BCS settings changed on opening")
	}
	if current, _ := os.ReadFile(configPath(dir)); !bytes.Equal(current, configBefore) {
		t.Fatal("company settings changed on opening")
	}
	u, _ := url.Parse(address)
	token := u.Query().Get("token")
	request, _ := http.NewRequest("POST", address, strings.NewReader(url.Values{"action": {"checkupdate"}, "token": {token}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", u.Scheme+"://"+u.Host)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	deadline := time.Now().Add(time.Second)
	for updateCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if updateCalls.Load() != 1 || closedCalls.Load() != 0 {
		t.Fatal("manual update check failed or activated BCS")
	}
	if readPreferences(dir).AutomaticUpdates {
		t.Fatal("manual check enabled automatic checks")
	}
}
