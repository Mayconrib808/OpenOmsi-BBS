package main

import (
	"crypto/sha256"
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
	if c.Root, err = cleanInputPath(root); err != nil {
		return previous, fmt.Errorf(localText(c.Language, "Escolha a pasta do OMSI 2: %w", "Choose the OMSI 2 folder: %w", "Wähle den OMSI-2-Ordner: %w"), err)
	}
	if c.OpenOMSI, err = cleanInputPath(executable); err != nil {
		return previous, fmt.Errorf(localText(c.Language, "Escolha o executável do OpenOMSI: %w", "Choose the OpenOMSI executable: %w", "Wähle die OpenOMSI-Programmdatei: %w"), err)
	}
	if info, err := os.Stat(c.OpenOMSI); err == nil && info.IsDir() {
		c.OpenOMSI = filepath.Join(c.OpenOMSI, "openomsi.exe")
	}
	c.Language = normalizeLanguage(c.Language)
	c.Multiplayer = multiplayer
	c.PlayerName = strings.TrimSpace(player)
	if multiplayer && (!companyText(c.PlayerName, 32) || strings.Contains(c.PlayerName, "|")) {
		return previous, fmt.Errorf("%s", localText(c.Language, "Seu nome no multiplayer deve ter de 1 a 32 caracteres, sem | ou caracteres de controle.", "Your multiplayer name must have 1 to 32 characters, without | or control characters.", "Dein Multiplayer-Name muss 1 bis 32 Zeichen lang sein, ohne | oder Steuerzeichen."))
	}
	return c, nil
}
func setupFormProfileSource(c Config) string {
	if c.CompanyProfileSource != "" {
		return c.CompanyProfileSource
	}
	return c.CompanyProfile
}
func setupRegistryState(dir string, c Config, r registryAPI) (string, error) {
	active := 0
	other := false
	for _, view := range []int{64, 32} {
		debugger, err := r.Read(view, ifeoBridge, "Debugger")
		if err != nil {
			return "", err
		}
		filter, err := r.Read(view, ifeoParent, "UseFilter")
		if err != nil {
			return "", err
		}
		original, err := r.Read(view, ifeoBridge, "FilterFullPath")
		if err != nil {
			return "", err
		}
		n, valid := filter.number()
		if debugger.Present && debugger.text() != "\""+bridgePath(dir)+"\"" {
			other = true
		}
		if debugger.text() == "\""+bridgePath(dir)+"\"" && samePath(original.text(), filepath.Join(c.Root, "Omsi.exe")) && valid && n == 1 {
			active++
		}
	}
	state := localText(c.Language, "Ponte desativada. Salve e ative para iniciar suas viagens pelo BCS.", "Bridge inactive. Save and activate to start your trips from BCS/BBS.", "Bridge deaktiviert. Speichern und aktivieren, um Fahrten über BBS zu starten.")
	if active == 2 {
		state = localText(c.Language, "Ponte ativada. Abra o BCS e inicie sua viagem normalmente.", "Bridge active. Open BCS/BBS and start your trip normally.", "Bridge aktiviert. Öffne BBS und starte deine Fahrt wie gewohnt.")
	} else if active != 0 {
		state = localText(c.Language, "Ativação incompleta. Feche o jogo e use Salvar e ativar.", "Activation incomplete. Close the game and use Save and activate.", "Aktivierung unvollständig. Spiel schließen und Speichern / aktivieren wählen.")
	} else if other {
		state = localText(c.Language, "Uma versão anterior da ponte está ativada. Use Salvar e ativar para atualizar.", "An earlier bridge package is active. Use Save and activate to update.", "Ein früheres Bridge-Paket ist aktiviert. Zum Aktualisieren Speichern / aktivieren wählen.")
	}
	if c.Multiplayer {
		state += "\r\nMultiplayer: " + c.CompanyID + " / " + c.PlayerName
	} else {
		state += localText(c.Language, "\r\nMultiplayer da empresa desativado.", "\r\nCompany multiplayer disabled.", "\r\nFirmen-Multiplayer deaktiviert.")
	}
	return state, nil
}

func setupNativeSelected(dir string, c Config) bool {
	jar, err := installedBCSJar(c.Root)
	if err != nil || jar == "" {
		return false
	}
	settings, err := nativeBCSSettingsDir(jar)
	if err != nil {
		return false
	}
	data, err := readBCSUpdateSettings(filepath.Join(settings, "config.cfg"))
	if err != nil {
		return false
	}
	root, adapter, enabled := nativeBCSUpdateSelection(data)
	return enabled && samePath(root, c.Root) && samePath(adapter, filepath.Join(dir, "PeDePeAdapter"))
}

// Report the integration BCS actually uses, including next-trip preparation.
func setupCurrentState(dir string, c Config, r registryAPI) (string, error) {
	if !setupNativeSelected(dir, c) {
		return setupRegistryState(dir, c, r)
	}
	jar, err := installedBCSJar(c.Root)
	if err != nil {
		return "", err
	}
	logger, err := bcsLoggerBytes(jar)
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(logger))
	prepared := hash == bcsAutoLoggerSHA || hash == bcs503AutoLoggerSHA
	return setupNativeActivationState(c, prepared), nil
}
func setupNativeActivationState(c Config, prepared bool) string {
	state := localText(c.Language,
		"PeDePeAdapter selecionado no BCS. Use Salvar e ativar para preparar a próxima viagem.",
		"PeDePeAdapter selected in BCS. Save and activate to prepare next-trip integration.",
		"PeDePeAdapter im BBS ausgewählt. Speichern / aktivieren, um die nächste Fahrt vorzubereiten.")
	if prepared {
		state = localText(c.Language,
			"Ponte ativada via PeDePeAdapter. Próxima viagem preparada. Abra o BCS normalmente.",
			"Bridge active via PeDePeAdapter. Next-trip integration ready. Open BCS normally.",
			"Bridge über PeDePeAdapter aktiviert. Nächste Fahrt vorbereitet. BBS normal öffnen.")
	}
	if c.Multiplayer {
		state += "\r\nMultiplayer: " + c.CompanyID + " / " + c.PlayerName
	}
	return state
}
