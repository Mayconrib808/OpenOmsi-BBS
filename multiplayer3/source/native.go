package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
)

func option(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}
func canonicalMap(raw string) (string, error) {
	p := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	lower := strings.ToLower(p)
	if i := strings.Index(lower, "/maps/"); i >= 0 {
		p = p[i+1:]
		lower = lower[i+1:]
	}
	if len(p) > 512 || !strings.HasPrefix(lower, "maps/") || !strings.HasSuffix(lower, "/global.cfg") {
		return "", fmt.Errorf("mapa inválido na viagem")
	}
	for _, c := range p {
		if c < 32 {
			return "", fmt.Errorf("mapa inválido")
		}
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("mapa inválido")
		}
	}
	return strings.ToLower(p), nil
}
func decodeSituation(b []byte) string {
	if len(b) >= 2 && b[0] == 255 && b[1] == 254 {
		var u []uint16
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[i:]))
		}
		return string(utf16.Decode(u))
	}
	return strings.TrimPrefix(string(b), "\ufeff")
}
func tripMap(args []string, root string) (string, error) {
	if raw := option(args, "--map"); raw != "" {
		return canonicalMap(raw)
	}
	p := option(args, "--situation")
	if p == "" {
		return "", fmt.Errorf("a viagem não informou mapa ou situação")
	}
	p = strings.ReplaceAll(p, "\\", string(filepath.Separator))
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > 16<<20 {
		return "", fmt.Errorf("situação inválida")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return "", e
	}
	lines := strings.Split(strings.ReplaceAll(decodeSituation(b), "\r", ""), "\n")
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "[map]") && i+1 < len(lines) {
			return canonicalMap(lines[i+1])
		}
	}
	return "", fmt.Errorf("a situação não contém o mapa")
}
func nativeArguments(args []string, address, name string) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--lan-join" || a == "--lan-host" || a == "--lan-name" {
			i++
			continue
		}
		if strings.HasPrefix(a, "--lan-join=") || strings.HasPrefix(a, "--lan-host=") || strings.HasPrefix(a, "--lan-name=") {
			continue
		}
		result = append(result, a)
	}
	return append(result, "--lan-join", address, "--lan-name", name)
}
func nativeEnvironment(base []string, content string, server bool) []string {
	result := []string{}
	for _, v := range base {
		k, _, _ := strings.Cut(v, "=")
		if strings.EqualFold(k, "OMSI_CONTENT") || strings.EqualFold(k, "OMSI_CONTENT_ZIP") || strings.EqualFold(k, "OMSI_PLUGIN_HOST32") || strings.EqualFold(k, "OMSI_NO_LAN_MODS") || strings.EqualFold(k, "OMSI_NO_PLUGINS") {
			continue
		}
		result = append(result, v)
	}
	result = append(result, "OMSI_CONTENT="+content, "OMSI_NO_LAN_MODS=1")
	if server {
		result = append(result, "OMSI_NO_PLUGINS=1")
	}
	return result
}
func resolveExistingParent(p string) string {
	current := p
	var missing []string
	for {
		if real, e := filepath.EvalSymlinks(current); e == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real
		}
		parent := filepath.Dir(current)
		if parent == current {
			return p
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
func insideDirectory(parent, child string) bool {
	p, e := filepath.Abs(parent)
	if e != nil {
		return true
	}
	c, e := filepath.Abs(child)
	if e != nil {
		return true
	}
	// Resolve existing junctions/symlinks as well as textual paths.
	p = resolveExistingParent(p)
	c = resolveExistingParent(c)
	rel, e := filepath.Rel(strings.ToLower(p), strings.ToLower(c))
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func contentFolder(cfg localConfig) string {
	dir := filepath.Dir(cfg.Game)
	if _, e := os.Stat(filepath.Join(dir, "Omsi.exe")); e == nil {
		dir = filepath.Join(dir, "openOMSI")
	}
	return dir
}

// Copy the current PeDePe-generated Lua byte for byte. Preserve real game mods.
func forwardPlugin(cfg localConfig, dir string) error {
	source := filepath.Join(adapterDirectory(dir), "plugins", "bbs.lua")
	if _, e := os.Stat(source); os.IsNotExist(e) {
		// Preserve the already configured 3.0.1 drop-in layout on upgrade.
		source = filepath.Join(filepath.Dir(dir), "plugins", "bbs.lua")
	}
	b, e := readSmallFile(source, 1<<20)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("plugin BCS excessivamente grande")
	}
	targetDir := filepath.Join(contentFolder(cfg), "Plugins")
	if cfg.Root != "" && samePath(resolveExistingParent(filepath.Join(cfg.Root, "Plugins")), resolveExistingParent(targetDir)) {
		return fmt.Errorf("use a pasta client dentro de OpenOMSI-BBS")
	}
	if e = os.MkdirAll(targetDir, 0755); e != nil {
		return e
	}
	target := filepath.Join(targetDir, "bbs.lua")
	old, _ := os.ReadFile(target)
	if bytes.Equal(old, b) {
		return nil
	}
	return os.WriteFile(target, b, 0600)
}
func mirrorPlugin(ctx context.Context, cfg localConfig, dir string) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	lastError := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if e := forwardPlugin(cfg, dir); e != nil {
				if e.Error() != lastError {
					log.Printf("BCS plugin: %v", e)
					lastError = e.Error()
				}
			} else {
				lastError = ""
			}
		}
	}
}

type nativeProcess struct {
	*exec.Cmd
	stopDriver    chan struct{}
	driverStopped chan struct{}
	gameLog       *os.File
}

func (p *nativeProcess) Wait() error {
	err := p.Cmd.Wait()
	if p.gameLog != nil {
		_ = p.gameLog.Close()
	}
	if p.stopDriver != nil {
		close(p.stopDriver)
		<-p.driverStopped
	}
	return err
}

func launchNative(cfg localConfig, args []string, dir string) (*nativeProcess, error) {
	env := nativeEnvironment(os.Environ(), contentFolder(cfg), false)
	in := parsePeDePeNativeInvocation(args)
	if in.HasSchedule && !in.Probe && !in.Server {
		helper, e := preparePluginHost(cfg, dir)
		if e != nil {
			return nil, e
		}
		env = pluginEnvironment(env, helper)
		if e := prepareBCSReadyPlugin(cfg); e != nil {
			return nil, fmt.Errorf("%s: %w", tr("errorreadyplugin"), e)
		}
		log.Printf("BCS readiness plugin prepared: %s", filepath.Join(contentFolder(cfg), "Plugins", bcsReadyPluginName))
	}
	if e := forwardPlugin(cfg, dir); e != nil {
		return nil, fmt.Errorf("não foi possível encaminhar o plugin nativo BCS: %w", e)
	}
	var driver *driverSync
	if in.HasSchedule && in.Line != "" && in.Tour != "" && in.Trip != "" && !in.Probe && !in.Server {
		if path := option(args, "--driver"); path != "" {
			var e error
			driver, e = prepareDriver(path, filepath.Join(dir, "logs", "trips", randomID()))
			if e != nil {
				return nil, e
			}
			args = replaceOption(args, "--driver", driver.OpenPath)
		}
	}
	cmd := exec.Command(cfg.Game, args...)
	cmd.Dir = filepath.Dir(cfg.Game)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	var bcsLog *os.File
	if bcsLaunchOutput != nil {
		path, e := bcsGameLogPath()
		if e != nil {
			return nil, e
		}
		bcsLog, e = openBCSGameLog(path)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", tr("errorbcsgamelog"), e)
		}
		log.Printf("BCS game log prepared: %s", path)
		// BCS GUI launches may have an invalid inherited stdout handle. The
		// actual game output must still reach both disk logs in that case.
		out := gameLogOutput(os.Stdout, bcsLog, bcsLaunchOutput)
		cmd.Stdout, cmd.Stderr = out, out
	}
	if e := cmd.Start(); e != nil {
		if bcsLog != nil {
			_ = bcsLog.Close()
		}
		return nil, e
	}
	process := &nativeProcess{Cmd: cmd, gameLog: bcsLog}
	if driver != nil {
		process.stopDriver = make(chan struct{})
		process.driverStopped = make(chan struct{})
		go func() {
			defer close(process.driverStopped)
			tick := time.NewTicker(200 * time.Millisecond)
			defer tick.Stop()
			lastError := ""
			poll := func() {
				changed, e := driver.poll()
				if e != nil {
					if e.Error() != lastError {
						log.Printf("BCS driver: %v", e)
						lastError = e.Error()
					}
				} else {
					lastError = ""
					if changed {
						log.Printf("BCS saved driver published: stops=%d", driver.Last.Stops[0])
					}
				}
			}
			for {
				select {
				case <-tick.C:
					poll()
				case <-process.stopDriver:
					poll()
					poll()
					return
				}
			}
		}()
	}
	return process, nil
}

func replaceOption(args []string, flag, value string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		if out[i] == flag && i+1 < len(out) {
			out[i+1] = value
			i++
		} else if strings.HasPrefix(out[i], flag+"=") {
			out[i] = flag + "=" + value
		}
	}
	return out
}

func withoutLANArguments(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--lan-join" || a == "--lan-host" || a == "--lan-name" {
			i++
			continue
		}
		if strings.HasPrefix(a, "--lan-join=") || strings.HasPrefix(a, "--lan-host=") || strings.HasPrefix(a, "--lan-name=") {
			continue
		}
		out = append(out, a)
	}
	return out
}
