package main

import (
 "fmt"
 "os"
 "path/filepath"
 "strings"
)

// Convert the form without replacing settings that the graphical form does not
// expose. Validation of the actual simulator files still uses validateConfig.
func setupFormConfig(previous Config, root, executable, player string, multiplayer bool) (Config, error) {
 c := previous
 var err error
 if c.Root, err = cleanInputPath(root); err != nil { return previous, fmt.Errorf("Escolha a pasta do OMSI 2: %w", err) }
 if c.OpenOMSI, err = cleanInputPath(executable); err != nil { return previous, fmt.Errorf("Escolha o executável do OpenOMSI: %w", err) }
 if info, err := os.Stat(c.OpenOMSI); err == nil && info.IsDir() { c.OpenOMSI = filepath.Join(c.OpenOMSI, "openomsi.exe") }
 c.Language = normalizeLanguage(c.Language)
 c.Multiplayer = multiplayer
 c.PlayerName = strings.TrimSpace(player)
 if multiplayer && (!companyText(c.PlayerName, 32) || strings.Contains(c.PlayerName, "|")) { return previous, fmt.Errorf("Seu nome no multiplayer deve ter de 1 a 32 caracteres, sem | ou caracteres de controle.") }
 return c, nil
}
func setupFormProfileSource(c Config) string {
 if c.CompanyProfileSource != "" { return c.CompanyProfileSource }
 return c.CompanyProfile
}
func setupRegistryState(dir string, c Config, r registryAPI) (string, error) {
 active := 0
 other := false
 for _, view := range []int{64, 32} {
  debugger, err := r.Read(view, ifeoBridge, "Debugger"); if err != nil { return "", err }
  filter, err := r.Read(view, ifeoParent, "UseFilter"); if err != nil { return "", err }
  original, err := r.Read(view, ifeoBridge, "FilterFullPath"); if err != nil { return "", err }
  n, valid := filter.number()
  if debugger.Present && debugger.text() != "\"" + bridgePath(dir) + "\"" { other = true }
  if debugger.text() == "\"" + bridgePath(dir) + "\"" && samePath(original.text(), filepath.Join(c.Root, "Omsi.exe")) && valid && n == 1 { active++ }
 }
 state := "Ponte desativada. Salve e ative para iniciar suas viagens pelo BCS."
 if active == 2 { state = "Ponte ativada. Abra o BCS e inicie sua viagem normalmente." } else if active != 0 { state = "Ativação incompleta. Feche o jogo e use Salvar e ativar." } else if other { state = "Uma versão anterior da ponte está ativada. Use Salvar e ativar para atualizar." }
 if c.Multiplayer { state += "\r\nMultiplayer: " + c.CompanyID + " / " + c.PlayerName } else { state += "\r\nMultiplayer da empresa desativado." }
 return state, nil
}
