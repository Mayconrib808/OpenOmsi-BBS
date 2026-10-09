package main

// Each hosted map owns its tunnel. The official simulator's global pid-file
// cleanup is disabled (tunnel=0), so starting map B cannot kill map A's tunnel.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const companyTunnelRelease = "2026.9.3"

var companyTunnelDownloadLock = make(chan struct{}, 1)

func companyTunnelAsset() (string, string, error) {
	switch runtime.GOOS {
	case "windows":
		return "cloudflared-windows-amd64.exe", "f096265ec2fcbe9bb6e2d64268db167ced3fcbb83d894bdb9e2fcdb26f2ea7e2", nil
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "cloudflared-linux-arm64", "aaeb2d7d0da3614634c7e03ab13487a1522c2e79165ed2929cfe23d5e95b326d", nil
		}
		return "cloudflared-linux-amd64", "77e26d8d900e0b8469f416239d14b5f296525fdf79fee6f511ef55609e3fbac2", nil
	}
	return "", "", fmt.Errorf("sistema sem suporte ao túnel automático")
}

func companyTunnelVerified(path, expected string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1<<20 || st.Size() > 200<<20 {
		return false
	}
	h := sha256.New()
	_, err = io.Copy(h, io.LimitReader(f, (200<<20)+1))
	return err == nil && hex.EncodeToString(h.Sum(nil)) == expected
}

func ensureCompanyTunnel(ctx context.Context, serverDir string, output io.Writer) (string, error) {
	asset, digest, err := companyTunnelAsset()
	if err != nil {
		return "", err
	}
	select {
	case companyTunnelDownloadLock <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-companyTunnelDownloadLock }()
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	home, _ := os.UserHomeDir()
	cache := filepath.Join(companyHostDataDir(serverDir), "Tools", "cloudflared-"+companyTunnelRelease+filepath.Ext(name))
	for _, path := range []string{cache, os.Getenv("OMSI_CLOUDFLARED"), filepath.Join(home, ".openomsi", "bin", name), filepath.Join(serverDir, name)} {
		if path != "" && companyTunnelVerified(path, digest) {
			return path, nil
		}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		return "", err
	}
	url := "https://github.com/cloudflare/cloudflared/releases/download/" + companyTunnelRelease + "/" + asset
	fmt.Fprintln(output, "Preparando o túnel automático da empresa. O download só é necessário na primeira utilização.")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("não foi possível baixar o túnel oficial: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download do túnel oficial: HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp(filepath.Dir(cache), "cloudflared-*.download")
	if err != nil {
		return "", err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, (200<<20)+1))
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n < 1<<20 || n > 200<<20 || hex.EncodeToString(h.Sum(nil)) != digest {
		return "", fmt.Errorf("o download do túnel não corresponde ao SHA-256 publicado pela Cloudflare")
	}
	if err := os.Chmod(temporary, 0755); err != nil {
		return "", err
	}
	// Another process may have installed the identical verified tool meanwhile.
	if companyTunnelVerified(cache, digest) {
		return cache, nil
	}
	if err := os.Rename(temporary, cache); err != nil {
		return "", err
	}
	return cache, nil
}

type companyTunnelOutput struct {
	mu        sync.Mutex
	output    io.Writer
	line      []byte
	oversized bool
}

func (w *companyTunnelOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.output.Write(data)
	for _, c := range data[:n] {
		if c == '\n' {
			if !w.oversized {
				for _, field := range strings.Fields(string(w.line)) {
					candidate := strings.Trim(field, "|\"'(),")
					if address := companyHostTunnelURL("tunnel: the session is reachable at " + candidate); address != "" {
						fmt.Fprintf(w.output, "tunnel: the session is reachable at %s\n", address)
						break
					}
				}
			}
			w.line = nil
			w.oversized = false
		} else if !w.oversized {
			if len(w.line) >= 8192 {
				w.line = nil
				w.oversized = true
			} else {
				w.line = append(w.line, c)
			}
		}
	}
	return n, err
}

func startCompanyManagedTunnel(parent context.Context, serverDir string, port int, output io.Writer) (<-chan error, func()) {
	ctx, cancel := context.WithCancel(parent)
	failures := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fail := func(err error) {
			if ctx.Err() == nil {
				failures <- err
			}
		}
		bin, err := ensureCompanyTunnel(ctx, serverDir, output)
		if err != nil {
			fail(err)
			return
		}
		attempts := 0
		for ctx.Err() == nil {
			started := time.Now()
			child := exec.Command(bin, "tunnel", "--no-autoupdate", "--url", "http://127.0.0.1:"+strconv.Itoa(port))
			child.Dir = serverDir
			log := &companyTunnelOutput{output: output}
			child.Stdout = log
			child.Stderr = log
			prepareOpenOMSIProbe(child) // Keep this background helper's console hidden on Windows.
			prepareCompanyHostProcess(child)
			if err := child.Start(); err != nil {
				fail(fmt.Errorf("não foi possível iniciar o túnel: %w", err))
				return
			}
			stop, err := ownCompanyHostProcess(child)
			if err != nil {
				_ = child.Process.Kill()
				_ = child.Wait()
				fail(err)
				return
			}
			finished := make(chan error, 1)
			go func() { finished <- child.Wait() }()
			select {
			case <-ctx.Done():
				stop()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
				}
				return
			case <-finished:
				stop()
			}
			if time.Since(started) > time.Minute {
				attempts = 0
			}
			attempts++
			if attempts >= 3 {
				fail(fmt.Errorf("o túnel da empresa encerrou três vezes; verifique a conexão do host"))
				return
			}
			fmt.Fprintln(output, "O túnel da empresa encerrou. Reconectando este mapa...")
			timer := time.NewTimer(3 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	var once sync.Once
	return failures, func() { once.Do(func() { cancel(); <-done }) }
}
