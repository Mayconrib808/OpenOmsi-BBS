package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (u *setupUI) configureCompany(old Config) (Config, error) {
	u.say("\n1 - Configurar empresa e entrada automática\n2 - Desativar multiplayer (manter a bridge)", "\n1 - Configure company and automatic joining\n2 - Disable multiplayer (keep the bridge)", "\n1 - Firma und automatischen Beitritt einrichten\n2 - Multiplayer deaktivieren (Bridge behalten)")
	mode, err := u.line("Opção", "Option", "Option", "1")
	if err != nil {
		return old, err
	}
	c := old
	if mode == "2" {
		c.Multiplayer = false
	} else if mode == "1" {
		u.say("O administrador fornece um arquivo JSON ou um link HTTPS para o perfil da empresa. Esta configuração fica salva para as próximas viagens.", "The administrator provides a JSON file or HTTPS company-profile link. This setting is saved for future trips.", "Der Administrator stellt eine JSON-Datei oder einen HTTPS-Link zum Firmenprofil bereit. Die Einstellung bleibt für weitere Fahrten gespeichert.")
		source, err := u.line("Arquivo ou link do perfil", "Profile file or link", "Profildatei oder Link", c.CompanyProfile)
		if err != nil {
			return old, err
		}
		source = strings.Trim(source, `"`)
		if !strings.Contains(source, "://") && !filepath.IsAbs(source) {
			source = filepath.Join(u.dir, source)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		p, err := loadCompanyProfile(ctx, source, appDir(u.dir), companyHTTPClient())
		cancel()
		if err != nil {
			return old, err
		}
		name, err := u.line("Seu nome no multiplayer", "Your multiplayer name", "Dein Multiplayer-Name", c.PlayerName)
		if err != nil {
			return old, err
		}
		if !companyText(name, 32) || strings.Contains(name, "|") {
			return old, fmt.Errorf("use a name with 1-32 characters, without control characters or |")
		}
		fmt.Printf("\n%s (%s): %d %s\n", p.CompanyName, p.CompanyID, len(p.Sessions), localText(u.lang, "sessão(ões) cadastrada(s)", "registered session(s)", "registrierte Sitzung(en)"))
		if p.Clock != nil {
			now, err := companyNow(*p.Clock, time.Now())
			if err != nil {
				return old, err
			}
			fmt.Printf("%s: %s (%s, %+d min)\n", localText(u.lang, "Relógio da empresa agora", "Company clock now", "Aktuelle Firmenzeit"), now.Format("2006-01-02 15:04:05"), p.Clock.TimeZone, p.Clock.ShiftMinutes)
			u.say("Confira se esse relógio corresponde ao mostrado no BCS. Sessões com data company acompanham esse relógio automaticamente; uma data manual diferente na bridge impede a entrada.", "Check that this clock matches BCS. Sessions dated company follow this clock automatically; a different manual bridge date prevents joining.", "Prüfe, ob diese Uhr mit BCS übereinstimmt. Sitzungen mit Datum company folgen ihr automatisch; ein anderes manuelles Bridge-Datum verhindert den Beitritt.")
		}
		u.say("Ao iniciar uma viagem no BBS, a ponte procurará uma sessão com mapa, data e horário compatíveis. Uma sessão já deve estar hospedada.", "Starting a BBS trip will find a session with a compatible map, date and clock. A session must already be hosted.", "Beim Start einer BBS-Fahrt wird eine Sitzung mit passender Karte, Datum und Uhrzeit gesucht. Sie muss bereits gehostet sein.")
		if !u.yes("Salvar e ativar multiplayer? [s/N]", "Save and enable multiplayer? [y/N]", "Speichern und Multiplayer aktivieren? [j/N]") {
			return old, nil
		}
		importCtx, importCancel := context.WithTimeout(context.Background(), 6*time.Second)
		c, _, err = enrollCompany(importCtx, u.dir, source, name, c)
		importCancel()
		if err != nil {
			return old, err
		}
	} else {
		return old, fmt.Errorf("choose 1 or 2")
	}
	if err = saveInstalledConfig(u.dir, c); err != nil {
		return old, err
	}
	u.say("Configuração salva. Inicie a viagem pelo BBS normalmente. Você pode desativar o multiplayer na opção 8.", "Configuration saved. Start the trip normally through BBS. You can disable multiplayer with option 8.", "Einstellung gespeichert. Starte die Fahrt wie gewohnt über BBS. Multiplayer lässt sich mit Option 8 deaktivieren.")
	return c, nil
}

func companyInputAsset(root, input string) (string, error) {
	input = strings.Trim(strings.TrimSpace(input), `"`)
	if filepath.IsAbs(input) {
		var err error
		input, err = filepath.Rel(root, input)
		if err != nil {
			return "", err
		}
	}
	input = filepath.ToSlash(strings.ReplaceAll(input, `\`, "/"))
	if !validCompanyAsset(input) {
		return "", fmt.Errorf("choose a game asset path inside OMSI 2")
	}
	return input, nil
}

func (u *setupUI) createCompany(c Config) (string, error) {
	if !filepath.IsAbs(c.Root) {
		return "", fmt.Errorf("configure the OMSI 2 folder first (option 1)")
	}
	u.say("\nEste assistente cria o perfil com os links e hashes dos seus arquivos. Ele não inclui nem envia os mods. Cada sessão precisa de um anfitrião ou servidor openOMSI já configurado.", "\nThis wizard creates a profile with links and hashes of your files. It does not include or upload mods. Each session needs an already configured openOMSI host or server.", "\nDer Assistent erstellt ein Profil mit Links und Hashes deiner Dateien. Mods werden weder eingebunden noch hochgeladen. Jede Sitzung benötigt einen eingerichteten openOMSI-Host oder -Server.")
	p := CompanyProfile{SchemaVersion: 1, OpenOMSIVersion: multiplayerGameVersion, Protocol: multiplayerProtocol}
	var err error
	for {
		p.CompanyID, err = u.line("Identificador da empresa (ex.: transfort-br; sem espaços)", "Company ID (e.g. transfort-br; no spaces)", "Firmen-ID (z. B. transfort-br; ohne Leerzeichen)", "")
		if err != nil {
			return "", err
		}
		if companyIDPattern.MatchString(p.CompanyID) {
			break
		}
		u.say("Use de 1 a 64 caracteres: letras minúsculas, números, - ou _. Comece com uma letra ou número. Exemplo: transfort-br. O nome visível será pedido a seguir.", "Use 1-64 characters: lowercase letters, digits, - or _. Start with a letter or digit. Example: transfort-br. The display name is requested next.", "Nutze 1-64 Zeichen: Kleinbuchstaben, Ziffern, - oder _. Beginne mit einem Buchstaben oder einer Ziffer. Beispiel: transfort-br. Der Anzeigename folgt danach.")
	}
	p.CompanyName, err = u.line("Nome da empresa", "Company name", "Firmenname", "")
	if err != nil {
		return "", err
	}
	if !companyText(p.CompanyName, 120) {
		return "", fmt.Errorf("invalid company name")
	}
	for {
		session := CompanySession{ID: fmt.Sprintf("session-%d", len(p.Sessions)+1), ClockToleranceSec: 180}
		session.Name, err = u.line("Nome da sessão", "Session name", "Sitzungsname", session.ID)
		if err != nil {
			return "", err
		}
		mapInput, err := u.line("Arquivo global.cfg do mapa (caminho completo ou maps/Nome/global.cfg)", "Map global.cfg (full path or maps/Name/global.cfg)", "global.cfg der Karte (vollständiger Pfad oder maps/Name/global.cfg)", "")
		if err != nil {
			return "", err
		}
		session.MapFile, err = companyInputAsset(c.Root, mapInput)
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(companyAssetKey(session.MapFile), "maps/") || !strings.HasSuffix(companyAssetKey(session.MapFile), "/global.cfg") {
			return "", fmt.Errorf("choose maps/<map>/global.cfg")
		}
		if _, err = companyAssetPath(c.Root, session.MapFile); err != nil {
			return "", err
		}
		session.MapName, err = u.line("Nome do mapa mostrado no BBS", "Map name shown in BBS", "In BBS angezeigter Kartenname", filepath.Base(filepath.Dir(filepath.FromSlash(session.MapFile))))
		if err != nil {
			return "", err
		}
		for {
			session.Date, err = u.line("Data do mundo da sessão (YYYY-MM-DD; company para sincronizar com o BCS)", "Session world date (YYYY-MM-DD; company to sync with BCS)", "Weltdatum der Sitzung (YYYY-MM-DD; company für BCS-Synchronisierung)", time.Now().Format("2006-01-02"))
			if err != nil {
				return "", err
			}
			if strings.EqualFold(session.Date, "company") {
				session.Date = "company"
				if p.Clock == nil {
					for {
						shift, err := u.line("Mudança de horário da empresa no BCS (horas inteiras, de -24 a 24; ex.: -8)", "Company time shift in BCS (whole hours, -24 to 24; e.g. -8)", "BCS-Zeitverschiebung der Firma (ganze Stunden, -24 bis 24; z. B. -8)", "0")
						if err != nil {
							return "", err
						}
						hours, parseErr := strconv.Atoi(shift)
						if parseErr != nil || hours < -24 || hours > 24 {
							u.say("Digite o número de horas mostrado nas definições da empresa, por exemplo -8. O valor deve ser um número inteiro entre -24 e 24.", "Enter the hours shown in the company settings, for example -8. Use a whole number between -24 and 24.", "Gib die Stunden aus den Firmeneinstellungen ein, zum Beispiel -8. Nutze eine ganze Zahl zwischen -24 und 24.")
							continue
						}
						p.Clock = &CompanyClock{TimeZone: "Europe/Berlin", ShiftMinutes: hours * 60}
						break
					}
				}
				break
			}
			date, parseErr := time.Parse("2006-01-02", session.Date)
			if parseErr == nil && date.Format("2006-01-02") == session.Date {
				break
			}
			u.say("Use uma data válida como 2026-10-06, ou company para acompanhar a data e o horário da empresa automaticamente.", "Use a valid date such as 2026-10-06, or company to follow the company's date and time automatically.", "Nutze ein gültiges Datum wie 2026-10-06 oder company für das automatische Firmendatum und die Firmenzeit.")
		}
		session.ServerURL, err = u.line("Endereço HTTP(S) do servidor (porta web, sem /status)", "Server HTTP(S) address (web port, without /status)", "HTTP(S)-Serveradresse (Web-Port, ohne /status)", "")
		if err != nil {
			return "", err
		}
		session.ServerURL = strings.TrimRight(session.ServerURL, "/")
		mapFolder := filepath.ToSlash(filepath.Dir(filepath.FromSlash(session.MapFile)))
		files, err := snapshotCompanyFolder(c.Root, mapFolder)
		if err != nil {
			return "", err
		}
		mapPackage := CompanyPackage{ID: session.ID + "-map", Name: session.MapName, Files: files, Folders: []string{mapFolder}}
		mapPackage.Version, err = u.line("Versão do mapa", "Map version", "Kartenversion", "1")
		if err != nil {
			return "", err
		}
		mapPackage.DownloadURL, err = u.line("Link HTTPS do mapa (Enter se não houver)", "Map HTTPS download page (Enter if unavailable)", "HTTPS-Downloadseite der Karte (Enter, falls nicht vorhanden)", "")
		if err != nil {
			return "", err
		}
		p.Packages = append(p.Packages, mapPackage)
		session.RequiredPackages = append(session.RequiredPackages, mapPackage.ID)
		busCount := 0
		busFolders := map[string]bool{}
		for {
			input, err := u.line("Ônibus .bus da frota (caminho completo ou Vehicles/Pasta/Onibus.bus; 0 para terminar)", "Fleet .bus file (full path or Vehicles/Folder/Bus.bus; 0 to finish)", ".bus-Datei der Flotte (vollständiger Pfad oder Vehicles/Ordner/Bus.bus; 0 zum Beenden)", "")
			if err != nil {
				return "", err
			}
			if input == "0" {
				if busCount == 0 {
					return "", fmt.Errorf("add at least one fleet bus")
				}
				break
			}
			bus, err := companyInputAsset(c.Root, input)
			if err != nil || !strings.HasPrefix(companyAssetKey(bus), "vehicles/") || !strings.HasSuffix(companyAssetKey(bus), ".bus") {
				return "", fmt.Errorf("choose a Vehicles/.../*.bus file")
			}
			if _, err = companyAssetPath(c.Root, bus); err != nil {
				return "", err
			}
			folder := filepath.ToSlash(filepath.Dir(filepath.FromSlash(bus)))
			if busFolders[companyAssetKey(folder)] {
				u.say("Essa pasta já está na frota; todos os modelos .bus dela estão incluídos.", "This folder is already in the fleet; all its .bus models are included.", "Dieser Ordner gehört bereits zur Flotte; alle enthaltenen .bus-Modelle sind eingeschlossen.")
				continue
			}
			files, err := snapshotCompanyFolder(c.Root, folder)
			if err != nil {
				return "", err
			}
			busCount++
			busFolders[companyAssetKey(folder)] = true
			pkg := CompanyPackage{ID: fmt.Sprintf("%s-bus-%d", session.ID, busCount), Files: files, Folders: []string{folder}}
			pkg.Name, err = u.line("Nome do pacote do ônibus", "Bus package name", "Name des Buspakets", filepath.Base(filepath.FromSlash(folder)))
			if err != nil {
				return "", err
			}
			pkg.Version, err = u.line("Versão do ônibus e das pinturas", "Bus and repaint version", "Version von Bus und Repaints", "1")
			if err != nil {
				return "", err
			}
			pkg.DownloadURL, err = u.line("Link HTTPS do ônibus (Enter se não houver)", "Bus HTTPS download page (Enter if unavailable)", "HTTPS-Downloadseite des Busses (Enter, falls nicht vorhanden)", "")
			if err != nil {
				return "", err
			}
			p.Packages = append(p.Packages, pkg)
			session.RequiredPackages = append(session.RequiredPackages, pkg.ID)
		}
		u.say("Cadastre também pastas de dependências externas usadas pelo mapa ou ônibus (Sceneryobjects, Splines, Texture etc.).", "Also register external dependency folders used by the map or buses (Sceneryobjects, Splines, Texture etc.).", "Hinterlege auch externe Abhängigkeitsordner der Karte oder Busse (Sceneryobjects, Splines, Texture usw.).")
		for n := 1; ; n++ {
			input, err := u.line("Pasta adicional (caminho relativo à pasta OMSI 2; 0 para terminar)", "Additional folder (relative to OMSI 2; 0 to finish)", "Zusätzlicher Ordner (relativ zu OMSI 2; 0 zum Beenden)", "0")
			if err != nil {
				return "", err
			}
			if input == "0" {
				break
			}
			folder, err := companyInputAsset(c.Root, strings.TrimRight(input, `/\`)+"/asset.cfg")
			if err != nil {
				return "", err
			}
			folder = filepath.ToSlash(filepath.Dir(filepath.FromSlash(folder)))
			files, err := snapshotCompanyFolder(c.Root, folder)
			if err != nil {
				return "", err
			}
			pkg := CompanyPackage{ID: fmt.Sprintf("%s-extra-%d", session.ID, n), Files: files, Folders: []string{folder}}
			pkg.Name, err = u.line("Nome da dependência", "Dependency name", "Name der Abhängigkeit", folder)
			if err != nil {
				return "", err
			}
			pkg.Version, err = u.line("Versão da dependência", "Dependency version", "Version der Abhängigkeit", "1")
			if err != nil {
				return "", err
			}
			pkg.DownloadURL, err = u.line("Link HTTPS (Enter se não houver)", "HTTPS download page (Enter if unavailable)", "HTTPS-Downloadseite (Enter, falls nicht vorhanden)", "")
			if err != nil {
				return "", err
			}
			p.Packages = append(p.Packages, pkg)
			session.RequiredPackages = append(session.RequiredPackages, pkg.ID)
		}
		p.Sessions = append(p.Sessions, session)
		if err = validateCompanyProfile(p); err != nil {
			return "", err
		}
		if !u.yes("Adicionar outra sessão? [s/N]", "Add another session? [y/N]", "Weitere Sitzung hinzufügen? [j/N]") {
			break
		}
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	if len(data) > companyProfileLimit {
		return "", fmt.Errorf("profile exceeds size limit")
	}
	dir := filepath.Join(u.dir, "Companies")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, p.CompanyID+".json")
	if _, err = os.Stat(path); err == nil {
		return "", fmt.Errorf("profile already exists: %s; edit it or choose another ID", path)
	}
	if err = writeSetupAtomic(path, append(data, '\n')); err != nil {
		return "", err
	}
	u.say("Perfil criado. Compartilhe este JSON, ou hospede o JSON em um endereço HTTPS e compartilhe o link. Os jogadores configuram esse arquivo/link uma vez na opção 8. Para alterar links e sessões depois, edite o perfil; para arquivos novos, gere os hashes novamente.", "Profile created. Share this JSON, or host the JSON at an HTTPS address and share its link. Players configure that file/link once with option 8. Edit the profile to change links or sessions; generate hashes again when assets change.", "Profil erstellt. Teile die JSON-Datei oder hoste sie unter einer HTTPS-Adresse und teile den Link. Spieler richten Datei/Link einmal mit Option 8 ein. Links und Sitzungen lassen sich im Profil bearbeiten; bei geänderten Dateien müssen die Hashes neu erzeugt werden.")
	if p.Clock != nil {
		u.say("Para sincronizar o servidor com esse relógio, o anfitrião deve iniciar CompanyHost.exe com este perfil. Confira primeiro se a hora calculada corresponde ao BCS.", "To synchronize the server to this clock, the host must start CompanyHost.exe with this profile. First check that the calculated clock matches BCS.", "Für die Serversynchronisierung muss der Gastgeber CompanyHost.exe mit diesem Profil starten. Prüfe zuerst, ob die berechnete Zeit mit BCS übereinstimmt.")
	}
	fmt.Println(path)
	return path, nil
}

func (u *setupUI) manageCompanyProfile(c Config) (string, error) {
	u.say("\n1 - Criar perfil novo\n2 - Atualizar os hashes do perfil existente\n3 - Atualizar inventário e hashes (adicionar/remover arquivos)\n0 - Voltar", "\n1 - Create a new profile\n2 - Update an existing profile's hashes\n3 - Update inventory and hashes (add/remove files)\n0 - Back", "\n1 - Neues Profil erstellen\n2 - Hashes eines vorhandenen Profils aktualisieren\n3 - Inventar und Hashes aktualisieren (Dateien hinzufügen/entfernen)\n0 - Zurück")
	mode, err := u.line("Opção", "Option", "Option", "1")
	if err != nil {
		return "", err
	}
	switch mode {
	case "1":
		return u.createCompany(c)
	case "2":
		return u.refreshCompanyProfile(c)
	case "3":
		return u.refreshCompanyProfile(c, true)
	case "0":
		return "", nil
	default:
		return "", fmt.Errorf("choose 1, 2, 3 or 0")
	}
}

func (u *setupUI) refreshCompanyProfile(c Config, inventory ...bool) (string, error) {
	if !filepath.IsAbs(c.Root) {
		return "", fmt.Errorf("configure the OMSI 2 folder first (option 1)")
	}
	u.say("Feche o BCS e o CompanyHost antes de atualizar. Esta ação do administrador registra o conteúdo instalado agora como referência da empresa; os jogadores continuam usando a checagem de arquivos.", "Close BCS and CompanyHost before updating. This administrator action records the currently installed content as the company's reference; players continue to use file validation.", "Schließe BBS und CompanyHost vor der Aktualisierung. Diese Administratoraktion registriert die aktuell installierten Inhalte als Firmenreferenz; die Dateiprüfung für Spieler bleibt aktiv.")
	source, err := u.line("Caminho do perfil JSON existente", "Existing JSON profile path", "Pfad des vorhandenen JSON-Profils", c.CompanyProfile)
	if err != nil {
		return "", err
	}
	source = strings.Trim(strings.TrimSpace(source), `"`)
	if strings.Contains(source, "://") || source == "" {
		return "", fmt.Errorf("choose the administrator's local JSON file")
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(u.dir, source)
	}
	stat, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() || stat.Size() > companyProfileLimit {
		return "", fmt.Errorf("choose a regular JSON profile within the size limit")
	}
	before, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	profile, err := loadCompanyProfile(context.Background(), source, u.dir, nil)
	if err != nil {
		return "", err
	}
	u.say("Conferindo os arquivos cadastrados; isso pode levar alguns minutos.", "Checking the declared files; this may take a few minutes.", "Registrierte Dateien werden geprüft; dies kann einige Minuten dauern.")
	var updated CompanyProfile
	var changed []string
	if len(inventory) > 0 && inventory[0] {
		u.say("Revisão do inventário: + adiciona, - remove, ~ atualiza o conteúdo. Nenhum arquivo do jogo será apagado.", "Inventory review: + adds, - removes, ~ updates content. No game files will be deleted.", "Inventarprüfung: + fügt hinzu, - entfernt, ~ aktualisiert Inhalte. Spieldateien werden nicht gelöscht.")
		updated, changed, err = refreshCompanyInventory(c.Root, profile)
	} else {
		updated, changed, err = refreshCompanyHashes(c.Root, profile)
	}
	if err != nil {
		return "", err
	}
	if len(changed) == 0 {
		u.say("O perfil já corresponde aos arquivos instalados. Nada foi alterado.", "The profile already matches the installed files. Nothing changed.", "Das Profil entspricht bereits den installierten Dateien. Keine Änderung.")
		return source, nil
	}
	fmt.Printf("%s: %d\n", localText(u.lang, "Alterações propostas no perfil", "Proposed profile changes", "Vorgeschlagene Profiländerungen"), len(changed))
	for _, path := range changed {
		fmt.Println(path)
	}
	if !u.yes("Registrar esses arquivos como a referência da empresa? [s/N]", "Record these files as the company's reference? [y/N]", "Diese Dateien als Firmenreferenz registrieren? [j/N]") {
		return "", nil
	}
	// Verify the proposed reference again after the owner's review. An
	// active BCS process must not silently change files while they approve it.
	all := updated.Sessions[0]
	all.RequiredPackages = nil
	for _, pkg := range updated.Packages {
		all.RequiredPackages = append(all.RequiredPackages, pkg.ID)
	}
	if problems := checkCompanyPackagesWithOriginals(c.Root, updated, all, nil); len(problems) != 0 {
		return "", fmt.Errorf("assets changed during review; try again with BCS and CompanyHost closed: %s", problems[0].Detail)
	}
	backup, err := saveCompanyProfileRefresh(source, before, updated)
	if err != nil {
		return "", err
	}
	u.say("Perfil atualizado. Empresa, links, sessões e relógio foram preservados. Reinicie o CompanyHost com esse mesmo JSON; compartilhe o perfil atualizado com os jogadores.", "Profile updated. Company, links, sessions and clock were preserved. Restart CompanyHost with the same JSON; share the updated profile with players.", "Profil aktualisiert. Firma, Links, Sitzungen und Uhr wurden beibehalten. Starte CompanyHost mit derselben JSON-Datei neu; teile das aktualisierte Profil mit den Spielern.")
	fmt.Printf("%s: %s\n", localText(u.lang, "Cópia do perfil anterior", "Previous profile backup", "Sicherung des vorherigen Profils"), backup)
	return source, nil
}

func saveCompanyProfileRefresh(path string, expected []byte, profile CompanyProfile) (string, error) {
	if err := validateCompanyProfile(profile); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return "", err
	}
	if len(data)+1 > companyProfileLimit {
		return "", fmt.Errorf("profile exceeds size limit")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(current, expected) {
		return "", fmt.Errorf("the profile changed during review; reload it before updating")
	}
	backup := path + ".backup-" + time.Now().UTC().Format("20060102-150405.000000000")
	file, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	_, err = file.Write(expected)
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := writeSetupAtomic(path, append(data, '\n')); err != nil {
		return "", err
	}
	return backup, nil
}
