package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		u.say("Ao iniciar uma viagem no BBS, a ponte procurará uma sessão com mapa, data e horário compatíveis. Uma sessão já deve estar hospedada.", "Starting a BBS trip will find a session with a compatible map, date and clock. A session must already be hosted.", "Beim Start einer BBS-Fahrt wird eine Sitzung mit passender Karte, Datum und Uhrzeit gesucht. Sie muss bereits gehostet sein.")
		if !u.yes("Salvar e ativar multiplayer? [s/N]", "Save and enable multiplayer? [y/N]", "Speichern und Multiplayer aktivieren? [j/N]") {
			return old, nil
		}
		c.Multiplayer, c.CompanyProfile, c.CompanyID, c.PlayerName = true, source, p.CompanyID, name
	} else {
		return old, fmt.Errorf("choose 1 or 2")
	}
	if oldBytes, err := os.ReadFile(configPath(u.dir)); err == nil {
		if err = writeSetupAtomic(configPath(u.dir)+".backup", oldBytes); err != nil {
			return old, err
		}
	}
	if err = writeSetupAtomic(configPath(u.dir), encodeConfig(c)); err != nil {
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
	p.CompanyID, err = u.line("Identificador da empresa (letras minúsculas, números, - ou _)", "Company ID (lowercase letters, digits, - or _)", "Firmen-ID (Kleinbuchstaben, Ziffern, - oder _)", "")
	if err != nil {
		return "", err
	}
	if !companyIDPattern.MatchString(p.CompanyID) {
		return "", fmt.Errorf("invalid company ID")
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
		session.Date, err = u.line("Data do mundo da sessão (YYYY-MM-DD)", "Session world date (YYYY-MM-DD)", "Weltdatum der Sitzung (YYYY-MM-DD)", time.Now().Format("2006-01-02"))
		if err != nil {
			return "", err
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
			files, err := snapshotCompanyFolder(c.Root, folder)
			if err != nil {
				return "", err
			}
			busCount++
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
	fmt.Println(path)
	return path, nil
}
