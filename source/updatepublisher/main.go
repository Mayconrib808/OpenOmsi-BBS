// Offline release preparation. No Cloudflare credential is needed or embedded.
package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type settings struct {
	Seed     string `json:"private_seed"`
	Public   string `json:"public_key"`
	Sequence uint64 `json:"last_sequence"`
	Origin   string `json:"base_url"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	ask := func(label string) string {
		fmt.Print(label + ": ")
		s, _ := reader.ReadString('\n')
		return strings.TrimSpace(s)
	}
	run := func() error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		dir := filepath.Dir(exe)
		raw, err := os.ReadFile(filepath.Join(dir, "publisher-key.json"))
		if err != nil {
			return err
		}
		var c settings
		if err = json.Unmarshal(raw, &c); err != nil {
			return err
		}
		if !strings.HasPrefix(c.Origin, "https://") || strings.ContainsAny(c.Origin, "?\r\n#") || c.Origin == "" {
			return fmt.Errorf("configure a URL HTTPS da distribuição no arquivo privado do publicador")
		}
		seed, err := base64.StdEncoding.DecodeString(c.Seed)
		if err != nil || len(seed) != ed25519.SeedSize {
			return fmt.Errorf("chave privada inválida")
		}
		key := ed25519.NewKeyFromSeed(seed)
		pub := base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
		if pub != c.Public {
			return fmt.Errorf("chave pública não corresponde à privada")
		}
		path := strings.Trim(ask("Caminho do Atualizar-Bridge.exe já testado"), `"`)
		version := ask("Versão (ex.: 2.1.2)")
		if version == "" || len(version) > 64 {
			return fmt.Errorf("versão inválida")
		}
		notes := ask("Descrição da atualização")
		sequence := c.Sequence + 1
		if v := ask("Número da publicação [" + strconv.FormatUint(sequence, 10) + "]"); v != "" {
			sequence, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
		}
		if sequence <= c.Sequence {
			return fmt.Errorf("a publicação precisa ter um número maior que a anterior")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) < 2 || len(data) > 64<<20 || string(data[:2]) != "MZ" {
			return fmt.Errorf("atualizador inválido")
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		origin := strings.TrimRight(c.Origin, "/")
		manifest := map[string]any{"schema": 1, "sequence": sequence, "version": version, "notes": map[string]string{"pt": notes}, "installer": map[string]any{"url": origin + "/packages/" + hash + ".exe", "sha256": hash, "size": len(data)}}
		payload, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		envelope, err := json.Marshal(map[string]string{"payload": base64.StdEncoding.EncodeToString(payload), "signature": base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))})
		if err != nil {
			return err
		}
		output := filepath.Join(dir, "publicacao-"+strconv.FormatUint(sequence, 10))
		if err = os.Mkdir(output, 0700); err != nil {
			return err
		}
		if err = os.Mkdir(filepath.Join(output, "packages"), 0700); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(output, "packages", hash+".exe"), data, 0600); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(output, "stable.json"), envelope, 0600); err != nil {
			return err
		}
		c.Sequence = sequence
		updated, _ := json.MarshalIndent(c, "", "  ")
		if err = os.WriteFile(filepath.Join(dir, "publisher-key.json"), updated, 0600); err != nil {
			return err
		}
		fmt.Println("Pronto:", output, "\nEnvie primeiro packages/ para o bucket R2; por último substitua stable.json. Apenas versões testadas devem ser publicadas.")
		return nil
	}
	if err := run(); err != nil {
		fmt.Println("Erro:", err)
	}
	_ = ask("Enter para fechar")
}
