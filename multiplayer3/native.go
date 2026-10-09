package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
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
		if strings.EqualFold(k, "OMSI_CONTENT") || strings.EqualFold(k, "OMSI_NO_LAN_MODS") || strings.EqualFold(k, "OMSI_NO_PLUGINS") {
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
func contentFolder(cfg localConfig) string {
	if value := os.Getenv("OMSI_CONTENT"); value != "" {
		return value
	}
	dir := filepath.Dir(cfg.Game)
	if _, e := os.Stat(filepath.Join(dir, "Omsi.exe")); e == nil {
		dir = filepath.Join(dir, "openOMSI")
	}
	return dir
}

// Copy the current PeDePe-generated Lua byte for byte. Preserve real game mods.
func forwardPlugin(cfg localConfig, dir string) error {
	source := filepath.Join(dir, "plugins", "bbs.lua")
	b, e := os.ReadFile(source)
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = forwardPlugin(cfg, dir)
		}
	}
}
func launchNative(cfg localConfig, args []string, dir string) (*exec.Cmd, error) {
	if e := forwardPlugin(cfg, dir); e != nil {
		return nil, fmt.Errorf("não foi possível encaminhar o plugin nativo BCS: %w", e)
	}
	cmd := exec.Command(cfg.Game, args...)
	cmd.Dir = filepath.Dir(cfg.Game)
	cmd.Env = nativeEnvironment(os.Environ(), contentFolder(cfg), false)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd, cmd.Start()
}
