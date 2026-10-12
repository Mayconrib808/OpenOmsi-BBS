package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const installedPluginHostName = "OpenOMSI_BCS_PluginHost32_v3.exe"

var bundledPluginHostSHA256 string

var previousPluginHosts = map[string]bool{
	"faf9a79bd1430a1b0116cf33bd324114514a98c0ce46e02da6ffc4d80a14be67": true,
	// Known source-built 3.x previews; allows moving to a new package folder.
	"79f9f302d84e19c941886336d1b57894a81db73bd3f5a3433e99d846458b6877": true,
	"aae3a40b7a4a619daf829a7ce9692b93e49f0f0a32c11f0ab5a71932c7e1cd58": true,
	"b5863a0a9cc9f489045d42e9fdc96931b8848e7c7d9ad2deeb0cff49138cd07e": true,
	"beb6e7a61fced7ac91744031e7ebf8f3bc1f6b2985918a4dc46b27a33e7a3bd6": true,

	// Helper retained by the 2.1.x player-only updates.
	"676166e5efa539ed59ac605bf07d3defa05172d33f86dab633b7c4f7dd2453f4": true,
	"b26af8f2fc55f272ae89f0dcd02c423c26fb758daf24e860960bd2002c82b2d4": true,
	"26614aa1acd0a02c9ee272b8bf7fbea075daa3eba488ddfefb2e6c5f388e25f5": true,
	"1c7f1f25bd18edb2b010ec42de2e34db5f3fc8ed2b1d948003635528b8a11a1a": true,
	"2e6dca9ca9f2732d88e5ad3c3787479549d76de3d60c4b90bc874790ecfbf6fd": true,
}

type helperOwnership struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// bbs.dll looks for its own bbs.start beside the host executable, independent
// of the working directory. Only our source-backed helper is placed there.
// The BCS-owned marker, DLL, JAR and original Omsi.exe are never written.
func preparePluginHost(cfg localConfig, dir string) (string, error) {
	if !filepath.IsAbs(cfg.Root) {
		return "", fmt.Errorf("pasta do OMSI inválida")
	}
	b, err := readSmallFile(filepath.Join(dir, "compat", "omsi-plugin-host32.exe"), 8<<20)
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	if len(b) < 2 || string(b[:2]) != "MZ" || bundledPluginHostSHA256 == "" || hash != bundledPluginHostSHA256 {
		return "", fmt.Errorf("SHA-256 do auxiliar BCS inválido")
	}
	target := filepath.Join(cfg.Root, installedPluginHostName)
	statePath := filepath.Join(configDirectory(dir), "plugin-host.json")
	var ownership helperOwnership
	if state, e := os.ReadFile(statePath); e == nil {
		_ = json.Unmarshal(state, &ownership)
	}
	if s, err := os.Lstat(target); err == nil {
		if !s.Mode().IsRegular() {
			return "", fmt.Errorf("arquivo de auxiliar BCS diferente; preservado")
		}
		old, err := readSmallFile(target, 8<<20)
		if err != nil {
			return "", err
		}
		oldHash := fmt.Sprintf("%x", sha256.Sum256(old))
		if oldHash != hash && !previousPluginHosts[oldHash] && !(samePath(ownership.Path, target) && ownership.SHA256 == oldHash) {
			return "", fmt.Errorf("arquivo de auxiliar BCS diferente; preservado")
		}
		if oldHash != hash {
			if err = writeAtomic(target, b, s.Mode().Perm()); err != nil {
				return "", err
			}
		}
	} else if os.IsNotExist(err) {
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if err != nil {
			return "", err
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		ce := f.Close()
		if err == nil {
			err = ce
		}
		if err != nil {
			_ = os.Remove(target)
			return "", err
		}
	} else {
		return "", err
	}
	state, _ := json.Marshal(helperOwnership{Path: target, SHA256: hash})
	if err = writeAtomic(statePath, state, 0600); err != nil {
		return "", err
	}
	return target, nil
}

func pluginEnvironment(base []string, helper string) []string {
	var env []string
	for _, v := range base {
		k, _, _ := strings.Cut(v, "=")
		if !strings.EqualFold(k, "OMSI_PLUGIN_HOST32") {
			env = append(env, v)
		}
	}
	return append(env, "OMSI_PLUGIN_HOST32="+helper)
}

// A clean 3.x installation deploys its own helper before configuring BCS.
// No previous Bridge executable, registry activation or settings are needed.
func configureStandaloneBCS(root, dir string, closed func() error) (string, error) {
	if err := closed(); err != nil {
		return "", err
	}
	// Refuse an unknown BCS configuration before deploying anything.
	path, err := bcsSettingsPath(root)
	if err != nil {
		return "", err
	}
	before, err := readSmallFile(path, 128<<10)
	if err != nil {
		return "", err
	}
	if _, err = updatedBCSSettings(before, root, adapterDirectory(dir)); err != nil {
		return "", err
	}
	if _, err = preparePluginHost(localConfig{Root: root}, dir); err != nil {
		return "", err
	}
	return configureBCS(root, dir, closed)
}
