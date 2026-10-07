//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const companyHostProcessFixtureRole = "OPENOMSI_COMPANY_HOST_TEST_ROLE"
const companyHostProcessFixtureDir = "OPENOMSI_COMPANY_HOST_TEST_DIR"

func companyHostProcessFixtureEnvironment(role, dir string) []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, companyHostProcessFixtureRole) && !strings.EqualFold(key, companyHostProcessFixtureDir) {
			result = append(result, entry)
		}
	}
	return append(result, companyHostProcessFixtureRole+"="+role, companyHostProcessFixtureDir+"="+dir)
}

// The same test executable supplies a dedicated-server fixture and its tunnel
// child. The server immediately creates its child when execution starts, so
// prepare/own must establish ownership before allowing the server to run.
func TestCompanyHostOwnedProcessFixture(t *testing.T) {
	role := os.Getenv(companyHostProcessFixtureRole)
	if role == "" {
		t.Skip("subprocess fixture")
	}
	dir := os.Getenv(companyHostProcessFixtureDir)
	if dir == "" {
		t.Fatal("fixture directory is missing")
	}
	if role != "server" && role != "tunnel" {
		t.Fatalf("unexpected fixture role: %s", role)
	}
	pidFile := filepath.Join(dir, role+".pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	if role == "server" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		tunnel := exec.Command(executable, "-test.run=^TestCompanyHostOwnedProcessFixture$")
		tunnel.Env = companyHostProcessFixtureEnvironment("tunnel", dir)
		if err := tunnel.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = tunnel.Process.Kill()
			_ = tunnel.Wait()
		}()
	}
	// Bound the fixtures independently if an assertion fails before cleanup.
	time.Sleep(30 * time.Second)
}

func waitCompanyHostFixturePID(t *testing.T, path string) uint32 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if text, err := os.ReadFile(path); err == nil {
			pid, err := strconv.ParseUint(strings.TrimSpace(string(text)), 10, 32)
			if err == nil && pid != 0 {
				return uint32(pid)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture did not publish its PID: %s", filepath.Base(path))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCompanyHostOwnedProcessStopsServerAndTunnelOnWindows(t *testing.T) {
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	server := exec.Command(executable, "-test.run=^TestCompanyHostOwnedProcessFixture$")
	server.Env = companyHostProcessFixtureEnvironment("server", dir)
	prepareCompanyHostProcess(server)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Process.Kill()
		_ = server.Wait()
	}()
	// A server that starts a tunnel immediately must not execute at all until
	// the supervisor has assigned its process job.
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "server.pid")); !os.IsNotExist(err) {
		t.Fatalf("server ran before process ownership was assigned: %v", err)
	}
	stop, err := ownCompanyHostProcess(server)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if pid := waitCompanyHostFixturePID(t, filepath.Join(dir, "server.pid")); pid != uint32(server.Process.Pid) {
		t.Fatalf("server PID mismatch: got %d, want %d", pid, server.Process.Pid)
	}
	tunnelPID := waitCompanyHostFixturePID(t, filepath.Join(dir, "tunnel.pid"))
	const synchronize = 0x00100000
	serverHandle, err := syscall.OpenProcess(synchronize, false, uint32(server.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(serverHandle)
	tunnelHandle, err := syscall.OpenProcess(synchronize, false, tunnelPID)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(tunnelHandle)
	for _, process := range []struct {
		name   string
		handle syscall.Handle
	}{{"server", serverHandle}, {"tunnel", tunnelHandle}} {
		status, err := syscall.WaitForSingleObject(process.handle, 0)
		if err != nil || status != syscall.WAIT_TIMEOUT {
			t.Fatal(fmt.Sprintf("%s was not alive before ownership cleanup", process.name), status, err)
		}
	}
	stop()
	stop() // Closing the ownership handle twice must be harmless.
	for _, process := range []struct {
		name   string
		handle syscall.Handle
	}{{"server", serverHandle}, {"tunnel", tunnelHandle}} {
		status, err := syscall.WaitForSingleObject(process.handle, 5000)
		if err != nil || status != syscall.WAIT_OBJECT_0 {
			t.Fatalf("owned %s survived cleanup: wait=%d error=%v", process.name, status, err)
		}
	}
}
