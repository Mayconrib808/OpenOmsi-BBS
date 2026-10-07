package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (u *setupUI) chooseLanguage() error {
	fmt.Println("\n1 - Português (Brasil)\n2 - English\n3 - Deutsch")
	for {
		s, e := u.line("Idioma / Language / Sprache", "Idioma / Language / Sprache", "Idioma / Language / Sprache", "")
		if e != nil {
			return e
		}
		if s == "1" {
			u.lang = "pt"
			return nil
		}
		if s == "2" {
			u.lang = "en"
			return nil
		}
		if s == "3" {
			u.lang = "de"
			return nil
		}
		fmt.Println("Escolha 1, 2 ou 3 / Choose 1, 2 or 3 / Wähle 1, 2 oder 3.")
	}
}
func (u *setupUI) configure(c Config) (Config, error) {
	if e := u.chooseLanguage(); e != nil {
		return c, e
	}
	c.Language = u.lang
	fmt.Println(usageNotices(u.lang))
	u.say("\nCole o caminho da pasta; aspas de 'Copiar como caminho' são aceitas.", "\nPaste the folder path; quotes from 'Copy as path' are accepted.", "\nFüge den Ordnerpfad ein. Anführungszeichen aus „Als Pfad kopieren“ werden akzeptiert.")
	for {
		s, e := u.line("Pasta do OMSI 2 original (contém Omsi.exe)", "Original OMSI 2 folder (contains Omsi.exe)", "Ordner des originalen OMSI 2 (enthält Omsi.exe)", c.Root)
		if e != nil {
			return c, e
		}
		p, e := cleanInputPath(s)
		if e != nil {
			fmt.Println(e)
			continue
		}
		st, e := os.Stat(filepath.Join(p, "Omsi.exe"))
		if e != nil || !st.Mode().IsRegular() {
			u.say("Omsi.exe não encontrado nessa pasta.", "Omsi.exe was not found in that folder.", "Omsi.exe wurde in diesem Ordner nicht gefunden.")
			continue
		}
		c.Root = p
		break
	}
	suggestion := c.OpenOMSI
	if suggestion == "" && fileExists(filepath.Join(c.Root, "openomsi.exe")) {
		suggestion = filepath.Join(c.Root, "openomsi.exe")
	}
	for {
		s, e := u.line("Pasta do openOMSI ou caminho de openomsi.exe", "openOMSI folder or path to openomsi.exe", "openOMSI-Ordner oder vollständiger Pfad zu openomsi.exe", suggestion)
		if e != nil {
			return c, e
		}
		p, e := cleanInputPath(s)
		if e != nil {
			fmt.Println(e)
			continue
		}
		if st, e := os.Stat(p); e == nil && st.IsDir() {
			p = filepath.Join(p, "openomsi.exe")
		}
		c.OpenOMSI = p
		if e = validateConfig(c, u.dir); e != nil {
			u.say("As pastas não passaram na validação:", "Folder validation failed:", "Die Ordner konnten nicht bestätigt werden:")
			fmt.Println(e)
			return c, e
		}
		break
	}
	original := filepath.Join(c.Root, "Omsi.exe")
	bridge := bridgePath(u.dir)
	for _, view := range []int{64, 32} {
		v, e := newWindowsRegistry().Read(view, ifeoBridge, "Debugger")
		if e != nil {
			return c, e
		}
		if v.text() == `"`+bridge+`"` {
			p, e := newWindowsRegistry().Read(view, ifeoBridge, "FilterFullPath")
			if e != nil {
				return c, e
			}
			if !samePath(p.text(), original) {
				return c, fmt.Errorf("%s", localText(u.lang, "Desative esta bridge antes de mudar a pasta do OMSI.", "Deactivate this bridge before changing the OMSI folder.", "Deaktiviere diese Bridge, bevor du den OMSI-Ordner änderst."))
			}
		}
	}
	u.say("\nConfira a configuração:", "\nReview the configuration:", "\nPrüfe die Einstellungen:")
	fmt.Printf("OMSI: %s\nopenOMSI: %s\n", c.Root, c.OpenOMSI)
	if host, e := pluginHostPath(c); e == nil {
		fmt.Printf("%s: %s\n", localText(u.lang, "Auxiliar BCS a preparar na ativação", "BCS helper to prepare during activation", "BBS-Hilfsprogramm, das bei der Aktivierung bereitgestellt wird"), host)
	}
	if !u.yes("Salvar? [s/N]", "Save? [y/N]", "Speichern? [j/N]") {
		return readInstalledConfig(u.dir), nil
	}
	if e := saveInstalledConfig(u.dir, c); e != nil {
		return c, e
	}
	u.say("Configuração salva. Use a opção 2 para ativar. Depois, inicie a viagem pelo BCS.", "Configuration saved. Use option 2 to activate. Then start your trip through BCS.", "Einstellungen gespeichert. Wähle Option 2 zum Aktivieren. Starte die Fahrt anschließend über BBS.")
	return c, nil
}

func logSetup(dir, action string, e error) {
	f, err := os.OpenFile(filepath.Join(appDir(dir), "setup-v1.1.3.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s v%s by %s %s: %v\r\n", time.Now().Format(time.RFC3339), bridgeVersion, bridgeAuthor, action, e)
}
func adminAction(dir, action string) error {
	c := readInstalledConfig(dir)
	if action != "activate" && action != "deactivate" {
		return fmt.Errorf("unknown admin action")
	}
	if !isElevated() {
		return fmt.Errorf("%s", localText(c.Language, "É necessária permissão de administrador.", "Administrator permission is required.", "Administratorrechte sind erforderlich."))
	}
	if running, e := gamesRunning(); e != nil {
		return e
	} else if running {
		return fmt.Errorf("%s", localText(c.Language, "Feche OMSI e openOMSI antes de mudar a ativação.", "Close OMSI and openOMSI before changing activation.", "Schließe OMSI und openOMSI, bevor du die Aktivierung änderst."))
	}
	bridge := bridgePath(dir)
	if action == "activate" {
		if _, e := verifyPackage(dir); e != nil {
			return fmt.Errorf("%s: %w", localText(c.Language, "Falha na integridade do pacote", "Package integrity failed", "Die Prüfung der Dateiintegrität ist fehlgeschlagen"), e)
		}
		if e := validateConfig(c, dir); e != nil {
			return e
		}
		d, e := activateRegistryWithHost(newWindowsRegistry(), c, dir, bridge)
		if e == nil {
			fmt.Printf("%s: %s\n", localText(c.Language, "Auxiliar BCS preparado", "BCS helper prepared", "BBS-Hilfsprogramm bereitgestellt"), d.Path)
			logSetup(dir, fmt.Sprintf("plugin-helper path=%s created=%t", d.Path, d.Created), nil)
		}
		return e
	}
	return deactivateRegistryWithHost(newWindowsRegistry(), bridge)
}
func (u *setupUI) changeActivation(action string) error {
	if action == "activate" {
		fmt.Println(usageNotices(u.lang))
		u.say("\nA opção 2 ativa o openOMSI para esta instalação. Depois, abra o BCS e inicie a viagem normalmente. Para voltar ao OMSI original (inclusive pela Steam), use a opção 3.", "\nOption 2 enables openOMSI for this installation. Then open BCS and start your trip normally. To use original OMSI again (including Steam launches), use option 3.", "\nOption 2 aktiviert openOMSI für diese Installation. Öffne anschließend BBS und starte die Fahrt wie gewohnt. Mit Option 3 kehrst du zum originalen OMSI zurück, auch beim Start über Steam.")
		c := readInstalledConfig(u.dir)
		if host, e := pluginHostPath(c); e == nil {
			fmt.Printf("%s: %s\n", localText(u.lang, "A ativação prepara uma cópia do auxiliar BCS", "Activation prepares a copy of the BCS helper", "Bei der Aktivierung wird eine Kopie des BBS-Hilfsprogramms bereitgestellt"), host)
		}
		if !u.yes("Ativar ou atualizar esta instalação? [s/N]", "Activate or update this installation? [y/N]", "Diese Installation aktivieren oder aktualisieren? [j/N]") {
			return nil
		}
	} else {
		if !u.yes("Desativar a bridge desta pasta? [s/N]", "Deactivate this package's bridge? [y/N]", "Die Bridge aus diesem Ordner deaktivieren? [j/N]") {
			return nil
		}
	}
	if isElevated() {
		e := adminAction(u.dir, action)
		logSetup(u.dir, action, e)
		return e
	}
	u.say("O Windows pedirá permissão de administrador para esta alteração.", "Windows will request administrator permission for this change.", "Windows fragt für diese Änderung nach Administratorrechten.")
	return elevateSetup(action, u.lang)
}

func (u *setupUI) status(c Config) error {
	fmt.Printf("\nOpenOMSI BCS Bridge v%s - by %s\nOMSI: %s\nopenOMSI: %s\n", bridgeVersion, bridgeAuthor, c.Root, c.OpenOMSI)
	fmt.Print(pluginHostDiagnostics(c, u.dir))
	if c.Multiplayer {
		fmt.Printf("Multiplayer: %s / %s\n", c.CompanyID, c.PlayerName)
	} else {
		u.say("Multiplayer da empresa: desativado", "Company multiplayer: disabled", "Firmen-Multiplayer: deaktiviert")
	}
	for _, view := range []int{64, 32} {
		v, e := newWindowsRegistry().Read(view, ifeoBridge, "Debugger")
		if e != nil {
			return e
		}
		p, e := newWindowsRegistry().Read(view, ifeoBridge, "FilterFullPath")
		if e != nil {
			return e
		}
		flag, e := newWindowsRegistry().Read(view, ifeoParent, "UseFilter")
		if e != nil {
			return e
		}
		n, _ := flag.number()
		if v.Present {
			fmt.Printf("%d-bit: %s\n  %s\n", view, localText(u.lang, "Registrada", "Registered", "Registriert"), v.text())
			if v.text() != `"`+bridgePath(u.dir)+`"` {
				u.say("Registrada por outra pasta do pacote.", "Registered by another package folder.", "Die Registrierung gehört zu einem anderen Paketordner.")
			}
			if n != 1 {
				u.say("ATENÇÃO: o filtro não está ativo no Windows.", "NOTE: the filter is not enabled in Windows.", "ACHTUNG: Der Filter ist in Windows nicht aktiviert.")
			}
			fmt.Println("  OMSI:", p.text())
		} else {
			fmt.Printf("%d-bit: %s\n", view, localText(u.lang, "Desativada", "Inactive", "Deaktiviert"))
		}
	}
	return nil
}
func (u *setupUI) collect(c Config) error {
	u.say("O ZIP pode conter caminhos locais e identificadores da conta presentes nos logs. Revise antes de compartilhar. Nada será enviado automaticamente.", "The ZIP may contain local paths and account identifiers from game logs. Review before sharing. Nothing is uploaded automatically.", "Die ZIP-Datei kann lokale Pfade und Kontokennungen aus den Spielprotokollen enthalten. Prüfe sie vor dem Weitergeben. Es wird nichts automatisch hochgeladen.")
	if !u.yes("Criar ZIP de diagnóstico? [s/N]", "Create a diagnostic ZIP? [y/N]", "Eine Diagnose-ZIP-Datei erstellen? [j/N]") {
		return nil
	}
	path, e := collectLogs(u.dir, c)
	if e == nil {
		u.say("ZIP criado:", "ZIP created:", "ZIP-Datei erstellt:")
		fmt.Println(path)
	}
	return e
}
func (u *setupUI) verify() error {
	n, e := verifyPackage(u.dir)
	if e != nil {
		return e
	}
	fmt.Printf("%s: %d\n", localText(u.lang, "Arquivos conferidos por SHA-256", "Files checked with SHA-256", "Mit SHA-256 geprüfte Dateien"), n)
	u.say("Todos os arquivos listados estão intactos. SHA-256 confere integridade; não é um resultado de antivírus.", "All listed files are intact. SHA-256 checks integrity; it is not an antivirus result.", "Alle aufgeführten Dateien sind unverändert. SHA-256 prüft die Dateiintegrität; es ist kein Ergebnis einer Virenprüfung.")
	return nil
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dir := packageRoot()
	// The elevated helper performs one fixed operation and exits. The graphical
	// parent owns the UAC request, completion message and error display.
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--admin=") {
		action := strings.TrimPrefix(os.Args[1], "--admin=")
		err := adminAction(dir, action)
		logSetup(dir, action, err)
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 1 || (len(os.Args) == 2 && os.Args[1] == "--gui-smoke") {
		smoke := len(os.Args) == 2
		if err := runSetupGUI(dir, smoke); err != nil {
			logSetup(dir, "graphical-setup", err)
			if !smoke {
				guiMessageBox(0, err.Error(), 0x10)
			}
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--cli" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}
	prepareConsole()
	c := readInstalledConfig(dir)
	u := &setupUI{bufio.NewScanner(os.Stdin), c.Language, dir}
	fmt.Printf("OpenOMSI BCS Bridge v%s - by %s\n", bridgeVersion, bridgeAuthor)
	if len(os.Args) > 1 {
		var e error
		switch os.Args[1] {
		case "--deactivate":
			e = u.changeActivation("deactivate")
		case "--collect":
			e = u.collect(c)
		case "--verify":
			e = u.verify()
		case "--company-admin":
			e = setupChangesAllowed(u.lang)
			if e == nil {
				_, e = u.manageCompanyProfile(c)
			}
		default:
			e = fmt.Errorf("unknown option")
		}
		if e != nil {
			fmt.Println(e)
		}
		_, _ = u.line("Pressione Enter para fechar", "Press Enter to close", "Zum Schließen Enter drücken", "")
		if e != nil {
			os.Exit(1)
		}
		return
	}
	if c.Root == "" || c.OpenOMSI == "" {
		var e error
		c, e = u.configure(c)
		if e != nil {
			u.say("A configuração não foi concluída. Você pode tentar novamente no menu.", "Configuration was not completed. You can retry from the menu.", "Die Einrichtung wurde nicht abgeschlossen. Du kannst sie im Menü erneut starten.")
			fmt.Println(e)
		}
	} else {
		fmt.Println(usageNotices(u.lang))
	}
	for {
		u.say("\nPara começar: 1 = pastas e idioma, depois 2 = ativar. No ponto final: F9 → espere 2 segundos → finalize no BCS.", "\nGetting started: 1 = folders and language, then 2 = activate. At the last stop: F9 → wait 2 seconds → finish in BCS.", "\nErste Schritte: 1 = Ordner und Sprache, danach 2 = aktivieren. An der Endhaltestelle: F9 → mindestens 2 Sekunden warten → Fahrt in BBS abschließen.")
		u.say("\n1 - Configurar pastas e idioma\n2 - Ativar / atualizar\n3 - Desativar\n4 - Conferir estado\n5 - Coletar logs\n6 - Conferir integridade\n7 - Abrir tutorial\n8 - Configurar / desativar multiplayer da empresa\n9 - Criar / atualizar perfil da empresa (administrador)\n0 - Sair", "\n1 - Configure folders and language\n2 - Activate / update\n3 - Deactivate\n4 - Check status\n5 - Collect logs\n6 - Verify integrity\n7 - Open tutorial\n8 - Configure / disable company multiplayer\n9 - Create / update company profile (administrator)\n0 - Exit", "\n1 - Ordner und Sprache einstellen\n2 - Aktivieren / aktualisieren\n3 - Deaktivieren\n4 - Status prüfen\n5 - Protokolle sammeln\n6 - Dateiintegrität prüfen\n7 - Anleitung öffnen\n8 - Firmen-Multiplayer einrichten / deaktivieren\n9 - Firmenprofil erstellen / aktualisieren (Administrator)\n0 - Beenden")
		s, e := u.line("Opção", "Option", "Option", "")
		if e != nil {
			return
		}
		switch s {
		case "0":
			return
		case "1":
			c, e = u.configure(c)
		case "2":
			e = u.changeActivation("activate")
		case "3":
			e = u.changeActivation("deactivate")
		case "4":
			e = u.status(c)
		case "5":
			e = u.collect(c)
		case "6":
			e = u.verify()
		case "7":
			e = openDocument(filepath.Join(dir, "TUTORIAL.html"))
		case "8", "9":
			var running bool
			running, e = gamesRunning()
			if e == nil && running {
				e = fmt.Errorf("%s", localText(u.lang, "Feche o jogo antes de alterar o multiplayer.", "Close the game before changing multiplayer.", "Schließe das Spiel, bevor du Multiplayer änderst."))
			}
			if e == nil {
				if s == "8" {
					c, e = u.configureCompany(c)
				} else {
					_, e = u.manageCompanyProfile(c)
				}
			}
		default:
			u.say("Escolha uma opção do menu.", "Choose a menu option.", "Wähle eine Option aus dem Menü.")
		}
		if e != nil {
			u.say("A operação não foi concluída:", "The operation was not completed:", "Der Vorgang wurde nicht abgeschlossen:")
			fmt.Println(e)
		}
	}
}
