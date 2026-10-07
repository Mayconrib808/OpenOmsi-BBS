package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Release numbers are labels, not wire versions. The HTTP status of official
// releases can contain the workspace version (0.1.0) and a build identity.
// Compatibility is checked through the features used by this bridge and the
// server's network protocol instead of an exact release-number allowlist.
var openOMSIVersionPattern = regexp.MustCompile(`^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}(?:[-+][0-9A-Za-z][0-9A-Za-z.+-]{0,63})?(?: \([^()\r\n]{1,120}\))?$`)

func validOpenOMSIVersion(version string) bool {
	return !strings.ContainsFunc(version, unicode.IsControl) && openOMSIVersionPattern.MatchString(version)
}

type openOMSICompatibility struct {
	Version string
}

var openOMSIHelpFlag = regexp.MustCompile(`(?m)^[ \t]+(?:-[A-Za-z], )?(--[a-z][a-z0-9-]*)(?:[ \t=]|$)`)

func requiredOpenOMSIFlags(dedicated, multiplayer bool) []string {
	if dedicated {
		return []string{"--root", "--server"}
	}
	flags := []string{"--root", "--map", "--bus", "--date", "--time", "--schedule", "--line", "--tour", "--trip", "--auto-entry", "--no-menu", "--content-zip", "--driver", "--hof", "--paint", "--traffic", "--passengers", "--all", "--autostart"}
	if multiplayer {
		flags = append(flags, "--lan-join", "--lan-name")
	}
	return flags
}

func validateOpenOMSIHelp(help string, dedicated, multiplayer bool) error {
	found := map[string]bool{}
	for _, match := range openOMSIHelpFlag.FindAllStringSubmatch(help, -1) {
		found[match[1]] = true
	}
	var missing []string
	for _, flag := range requiredOpenOMSIFlags(dedicated, multiplayer) {
		if !found[flag] {
			missing = append(missing, flag)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("openOMSI does not expose required options: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Keep stdout/stderr bounded even for a malformed executable. The CLI probe
// only asks for metadata: it never loads the user's map or starts a session.
type openOMSIProbeOutput struct {
	bytes.Buffer
	tooLarge bool
}

func (w *openOMSIProbeOutput) Write(p []byte) (int, error) {
	const limit = 128 << 10
	if w.Len()+len(p) > limit {
		w.tooLarge = true
		return 0, fmt.Errorf("openOMSI CLI output exceeds %d bytes", limit)
	}
	return w.Buffer.Write(p)
}

func runOpenOMSICLIProbe(executable, option string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, option)
	cmd.WaitDelay = time.Second
	cmd.Dir = filepath.Dir(executable)
	cmd.Env = os.Environ()
	// The upstream startup helper honors this flag and avoids allocating its own
	// console. It is scoped to this metadata command, never the user's settings.
	var env []string
	for _, entry := range cmd.Env {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "OMSI_BACKGROUND") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "OMSI_BACKGROUND=1")
	prepareOpenOMSIProbe(cmd)
	var output openOMSIProbeOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("openOMSI %s did not finish within 8 seconds", option)
	}
	if output.tooLarge {
		return "", fmt.Errorf("openOMSI CLI output exceeds its size limit")
	}
	if err != nil {
		return "", fmt.Errorf("openOMSI %s failed: %w", option, err)
	}
	return output.String(), nil
}

func openOMSIPluginHostOverride(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	marker := []byte("OMSI_PLUGIN_HOST32")
	buffer := make([]byte, 32<<10)
	var tail []byte
	for {
		n, readErr := f.Read(buffer)
		chunk := append(tail, buffer[:n]...)
		if bytes.Contains(chunk, marker) {
			return true, nil
		}
		if readErr == io.EOF {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
		keep := len(marker) - 1
		if len(chunk) < keep {
			keep = len(chunk)
		}
		tail = append(tail[:0], chunk[len(chunk)-keep:]...)
	}
}

func checkOpenOMSICompatibility(executable string, dedicated, multiplayer bool) (openOMSICompatibility, error) {
	var result openOMSICompatibility
	if !filepath.IsAbs(executable) {
		return result, fmt.Errorf("openOMSI compatibility check requires an absolute executable path")
	}
	st, err := os.Stat(executable)
	if err != nil || !st.Mode().IsRegular() || st.Size() > 512<<20 {
		return result, fmt.Errorf("openOMSI executable is missing, invalid or too large: %s", executable)
	}
	help, err := runOpenOMSICLIProbe(executable, "--help")
	if err != nil {
		return result, err
	}
	if err := validateOpenOMSIHelp(help, dedicated, multiplayer); err != nil {
		return result, err
	}
	version, err := runOpenOMSICLIProbe(executable, "--version")
	if err != nil {
		return result, err
	}
	name, label, ok := strings.Cut(strings.TrimSpace(version), " ")
	if !ok || !strings.EqualFold(name, "openomsi") || !validOpenOMSIVersion(strings.TrimSpace(label)) {
		return result, fmt.Errorf("openOMSI returned an invalid version description: %q", strings.TrimSpace(version))
	}
	result.Version = strings.TrimSpace(label)
	if !dedicated {
		available, err := openOMSIPluginHostOverride(executable)
		if err != nil {
			return result, err
		}
		if !available {
			return result, fmt.Errorf("openOMSI does not expose the OMSI_PLUGIN_HOST32 override required for the BBS plugin")
		}
	}
	return result, nil
}
