package main

import (
	"errors"
	"strings"
	"sync"
)

var languageState = struct {
	sync.RWMutex
	value string
}{value: "pt-BR"}

func language() string {
	languageState.RLock()
	defer languageState.RUnlock()
	return languageState.value
}
func validLanguage(v string) bool { return v == "pt-BR" || v == "en" || v == "de" }
func setLanguage(v string) {
	if validLanguage(v) {
		languageState.Lock()
		languageState.value = v
		languageState.Unlock()
	}
}
func loadLanguage(dir string) bool {
	s := readPreferences(dir)
	if !validLanguage(s.Language) {
		return false
	}
	setLanguage(s.Language)
	return true
}
func saveLanguage(dir, v string) error {
	if !validLanguage(v) {
		return errors.New("idioma inválido")
	}
	if e := writePreferences(dir, v, nil); e != nil {
		return e
	}
	setLanguage(v)
	return nil
}

var translations = map[string][3]string{
	"errorbcsgamelog":   {"Não foi possível abrir o game.log necessário para o BCS", "Could not open the game.log required by BCS", "Die für BCS erforderliche game.log konnte nicht geöffnet werden"},
	"errorreadyplugin":  {"Não foi possível preparar o aviso de carregamento para o BCS", "Could not prepare the BCS loading signal", "Das Ladesignal für BCS konnte nicht vorbereitet werden"},
	"client":            {"Cliente openOMSI", "openOMSI client", "openOMSI-Client"},
	"clientautomatic":   {"Atualizar automaticamente", "Update automatically", "Automatisch aktualisieren"},
	"clientmanual":      {"Manual — usar meu executável", "Manual — use my executable", "Manuell — eigene Datei verwenden"},
	"companyclock":      {"Horário da empresa (obrigatório)", "Company clock (required)", "Firmenzeit (erforderlich)"},
	"clockinstructions": {"No BCS, abra Empresa → Definições e consulte “Mudança de horário”. Informe o valor da sua empresa abaixo.", "In BCS, open Company → Settings and find “Time shift”. Enter your company's value below.", "Öffne in BCS Firma → Einstellungen und suche die Zeitverschiebung. Trage unten den Wert deiner Firma ein."},
	"clockexample":      {"Exemplo das definições da empresa no BCS", "Example from BCS company settings", "Beispiel aus den BCS-Firmeneinstellungen"},
	"bcsclocklabel":     {"Mudança de horário:", "Time shift:", "Zeitverschiebung:"},
	"hours":             {"Horas", "Hours", "Stunden"},
	"clockexamplehint":  {"Neste exemplo, informe -8. Sua empresa pode ter outro valor: confira antes de salvar.", "In this example, enter -8. Your company may use a different value: check before saving.", "In diesem Beispiel: -8. Deine Firma kann einen anderen Wert haben; prüfe ihn vor dem Speichern."},
	"clockplaceholder":  {"Preencha o valor do BCS", "Enter the BCS value", "BCS-Wert eingeben"},
	"timezonehint":      {"Relógio usado como referência no BCS, por exemplo Europe/Berlin. A mudança de horário será aplicada sobre ele.", "The reference clock used by BCS, for example Europe/Berlin. The time shift is applied to this clock.", "Referenzuhr von BCS, zum Beispiel Europe/Berlin. Die Zeitverschiebung wird darauf angewendet."},
	"title":             {productName, productName, productName},
	"continue":          {"Continuar", "Continue", "Weiter"}, "save": {"Salvar e continuar", "Save and continue", "Speichern und fortfahren"},
	"intro": {"Configure uma vez. Nas próximas viagens, a abertura continua automaticamente.", "Set up once. Future trips launch automatically.", "Einmal einrichten. Weitere Fahrten starten automatisch."},
	"root":  {"Pasta do OMSI 2", "OMSI 2 folder", "OMSI-2-Ordner"}, "detected": {"OMSI 2 detectado automaticamente", "OMSI 2 detected automatically", "OMSI 2 automatisch erkannt"},
	"player": {"Nome do jogador", "Player name", "Spielername"}, "join": {"Entrar em uma empresa existente", "Join an existing company", "Einer bestehenden Firma beitreten"},
	"create":   {"Criar sala multiplayer para minha empresa", "Create a multiplayer room for my company", "Multiplayer-Raum für meine Firma erstellen"},
	"roomnote": {"Esta sala conecta sua empresa no multiplayer. Ela não cria nem altera empresas dentro do BCS.", "This room connects your company in multiplayer. It does not create or change companies in BCS.", "Dieser Raum verbindet eure Firma im Multiplayer. Er erstellt oder ändert keine Firma im BCS."},
	"invite":   {"Convite da empresa", "Company invitation", "Firmeneinladung"}, "invitehint": {"Cole o link enviado pelo criador da sala.", "Paste the link shared by the room creator.", "Den Link des Raumerstellers einfügen."},
	"name": {"Nome da empresa", "Company name", "Firmenname"}, "timezone": {"Relógio de referência", "Reference time zone", "Referenzzeitzone"},
	"offset":     {"Diferença do horário da empresa (horas)", "Company clock offset (hours)", "Zeitverschiebung der Firma (Stunden)"},
	"offsethint": {"Copie o número de “Mudança de horário” da sua empresa no BCS. Use 0 somente se o BCS indicar 0; não use o fuso UTC do seu país.", "Copy your BCS company's “Time shift” number. Use 0 only if BCS shows 0; do not enter your country's UTC offset.", "Übernimm die Zeitverschiebung deiner BCS-Firma. Nutze 0 nur, wenn BCS 0 zeigt; nicht die UTC-Zeitzone deines Landes."},
	"server":     {"Servidor openOMSI", "openOMSI server", "openOMSI-Server"}, "recommended": {"Recomendada automaticamente (ônibus livres)", "Automatic recommended version (unrestricted buses)", "Automatisch empfohlene Version (freie Buswahl)"},
	"latest": {"Atualizar automaticamente (última oficial compatível)", "Automatic updates (latest compatible official release)", "Automatisch aktualisieren (neueste kompatible offizielle Version)"}, "manual": {"Escolher versão", "Choose a version", "Version auswählen"},
	"version":    {"Número da versão", "Version number", "Versionsnummer"},
	"serverhint": {"Atualiza quando seu PC abre uma nova sessão como host. Sessões abertas são preservadas. Se a versão oficial não permitir ônibus livres, usa o servidor compatível incluído e registra o motivo no log.", "Updates when your PC starts a new hosted session. Existing sessions continue. If the official release cannot allow unrestricted buses, the bundled compatible server is used and the reason is logged.", "Aktualisiert beim Start einer neuen Sitzung als Host. Laufende Sitzungen bleiben erhalten. Ohne freie Buswahl wird der mitgelieferte kompatible Server verwendet; der Grund steht im Log."},
	"advanced":   {"Opções avançadas", "Advanced options", "Erweiterte Optionen"}, "game": {"openomsi.exe escolhido", "Selected openomsi.exe", "Gewählte openomsi.exe"},
	"gamehint": {"Automático verifica a versão oficial mais recente antes de cada viagem pelo BCS. O download fica em cache. Manual usa o executável escolhido, sem atualizar.", "Automatic checks the latest official release before each BCS trip. Downloads are cached. Manual keeps your chosen executable without updating.", "Automatisch prüft vor jeder BCS-Fahrt die neueste offizielle Version. Downloads werden gespeichert. Manuell verwendet die gewählte Datei ohne Updates."},
	"edit":     {"Atualizar o nome e horário da sala que criei (mapas vazios)", "Update the name and clock of my room (empty maps)", "Name und Uhrzeit meines Raums ändern (leere Karten)"},
	"language": {"Idioma", "Language", "Sprache"}, "saved": {"Configuração salva!", "Settings saved!", "Einstellungen gespeichert!"},
	"created": {"Sala criada! Envie este link para os membros da sua empresa.", "Room created! Share this link with your company members.", "Raum erstellt! Diesen Link an die Mitglieder deiner Firma senden."},
	"copy":    {"Copiar convite", "Copy invitation", "Einladung kopieren"}, "copied": {"Convite copiado", "Invitation copied", "Einladung kopiert"},
	"play": {"Continuar para o jogo", "Continue to the game", "Weiter zum Spiel"}, "done": {"Pode fechar esta tela. A abertura da viagem está continuando.", "You can close this page. Your trip is now launching.", "Diese Seite kann geschlossen werden. Die Fahrt wird jetzt gestartet."},
	"bcspath":        {"Se a configuração automática não reconhecer sua versão do BCS, selecione esta pasta em OpenOMSI Path dentro do BCS:", "If automatic setup cannot recognize your BCS version, select this folder under OpenOMSI Path in BCS:", "Falls die automatische Einrichtung deine BCS-Version nicht erkennt, diesen Ordner im BCS unter OpenOMSI Path auswählen:"},
	"hostnote":       {"O PC do primeiro jogador precisa continuar ligado enquanto hospeda.", "The first player's PC must stay on while hosting.", "Der PC des ersten Spielers muss beim Hosten eingeschaltet bleiben."},
	"clientdownload": {"Preparando o cliente openOMSI. O primeiro download pode levar alguns minutos.", "Preparing the openOMSI client. The first download may take a few minutes.", "Der openOMSI-Client wird vorbereitet. Der erste Download kann einige Minuten dauern."},
	"host":           {"Primeiro jogador: preparando o servidor do mapa.", "First player: preparing this map's server.", "Erster Spieler: Der Kartenserver wird vorbereitet."},
	"guest":          {"Entrando na sessão existente da empresa e do mapa.", "Joining the existing company and map session.", "Der bestehenden Sitzung der Firma und Karte wird beigetreten."},
	"error":          {"Não foi possível continuar. Confira o log em OpenOMSI-BBS/logs e tente novamente.", "Could not continue. Check the log in OpenOMSI-BBS/logs and try again.", "Fortfahren nicht möglich. Das Protokoll unter OpenOMSI-BBS/logs prüfen und erneut versuchen."},
	"errorinvite":    {"O convite está inválido ou a sala não está disponível. Abra Setup.exe para corrigir o convite. O serviço central precisa estar atualizado para a 3.0.", "The invitation is invalid or the room is unavailable. Open Setup.exe to correct it. The central service must be updated for 3.0.", "Die Einladung ist ungültig oder der Raum nicht verfügbar. Die Einladung über Setup.exe korrigieren. Der zentrale Dienst muss für 3.0 aktualisiert sein."},
	"errornetwork":   {"Não foi possível acessar o serviço. Confira a conexão e tente novamente.", "The service could not be reached. Check your connection and try again.", "Der Dienst ist nicht erreichbar. Die Verbindung prüfen und erneut versuchen."},
	"errorversion":   {"A versão selecionada não confirma os recursos necessários, incluindo ônibus livres. Abra Setup.exe, escolha Recomendada e tente novamente. Seu convite continua válido.", "The selected version does not confirm the required features, including unrestricted buses. Open Setup.exe, choose Recommended, and retry. Your invitation is still valid.", "Die gewählte Version bestätigt die benötigten Funktionen einschließlich freier Buswahl nicht. Setup.exe öffnen, Empfohlen wählen und erneut versuchen. Die Einladung bleibt gültig."},
	"errordownload":  {"Não foi possível preparar o download ou verificar os arquivos. Confira a conexão e o log. A versão anterior do cache foi preservada.", "The download or file verification could not be completed. Check your connection and the log. The previous cached version was preserved.", "Download oder Dateiprüfung konnten nicht abgeschlossen werden. Verbindung und Protokoll prüfen. Die vorherige gespeicherte Version wurde beibehalten."},
	"errorclock":     {"O horário informado é inválido. Confira a timezone e a diferença em horas.", "The clock settings are invalid. Check the time zone and offset in hours.", "Die Zeitangaben sind ungültig. Zeitzone und Verschiebung in Stunden prüfen."},
	"errorfiles":     {"Confira as pastas e a permissão de gravação. O OMSI 2 original precisa estar instalado. Consulte os detalhes no log.", "Check the folders and write permission. The original OMSI 2 must be installed. See the log for details.", "Ordner und Schreibrechte prüfen. Das originale OMSI 2 muss installiert sein. Einzelheiten stehen im Protokoll."},
	"errorplayer":    {"Informe um nome de jogador válido.", "Enter a valid player name.", "Einen gültigen Spielernamen eingeben."},
	"errorowner":     {"A chave do criador da sala não está disponível neste PC, ou há mapas ativos. Só o criador pode editar a sala com os mapas parados.", "The room creator's key is unavailable on this PC, or maps are active. Only the creator can edit the room with all maps stopped.", "Der Schlüssel des Raumerstellers fehlt auf diesem PC oder Karten sind aktiv. Nur der Ersteller kann den Raum ändern, wenn alle Karten gestoppt sind."},
	"errorclosed":    {"A configuração foi fechada sem salvar. Inicie a viagem novamente para configurar.", "Setup was closed without saving. Start the trip again to set it up.", "Die Einrichtung wurde ohne Speichern geschlossen. Die Fahrt erneut starten, um sie einzurichten."},
}

func init() {
	for key, value := range map[string][3]string{
		"previewwarning":        {"Versão de desenvolvimento para testes. Próxima viagem ainda depende de uma integração permitida pelo BCS.", "Development version for testing. Next trip still requires an integration permitted by BCS.", "Entwicklungsversion zum Testen. Die nächste Fahrt erfordert noch eine vom BCS erlaubte Integration."},
		"setupheading":          {"Sua empresa, seu multiplayer", "Your company, your multiplayer", "Deine Firma, dein Multiplayer"},
		"bcsheading":            {"Conectar ao BCS", "Connect to BCS", "Mit BCS verbinden"},
		"bcshint":               {"Salve a empresa, feche o BCS e clique em Configurar BCS. A pasta correta será selecionada com backup das configurações.", "Save the company, close BCS and click Configure BCS. The correct folder will be selected with a settings backup.", "Firma speichern, BCS schließen und BCS einrichten anklicken. Der richtige Ordner wird gewählt und die Einstellungen werden gesichert."},
		"configurebcs":          {"Configurar BCS", "Configure BCS", "BCS einrichten"},
		"inspectbcs":            {"Verificar estado", "Check status", "Status prüfen"},
		"bcsnotchecked":         {"Estado ainda não verificado.", "Status has not been checked yet.", "Status noch nicht geprüft."},
		"bcsconfigured":         {"BCS configurado para esta instalação.", "BCS is configured for this installation.", "BCS ist für diese Installation eingerichtet."},
		"bcsneedsconfiguration": {"O BCS ainda aponta para outra pasta. Clique em Configurar BCS.", "BCS still points to another folder. Click Configure BCS.", "BCS verweist noch auf einen anderen Ordner. BCS einrichten anklicken."},
		"bcsmustsave":           {"Salve os dados da empresa antes de configurar o BCS.", "Save your company settings before configuring BCS.", "Vor der BCS-Einrichtung die Firmeneinstellungen speichern."},
		"manualbcs":             {"Minha versão do BCS não foi reconhecida", "My BCS version was not recognized", "Meine BCS-Version wurde nicht erkannt"},
		"copypath":              {"Copiar pasta", "Copy folder", "Ordner kopieren"},
		"copiedpath":            {"Pasta copiada", "Folder copied", "Ordner kopiert"},
		"readyhint":             {"Agora conecte o BCS e conclua a configuração. O convite permanece salvo para as próximas viagens.", "Now connect BCS and finish setup. Your invitation stays saved for future trips.", "Jetzt BCS verbinden und Einrichtung abschließen. Die Einladung bleibt für weitere Fahrten gespeichert."},
		"donemanual":            {"Pode fechar esta tela e iniciar a viagem pelo BCS.", "You can close this page and start your trip in BCS.", "Diese Seite schließen und die Fahrt im BCS starten."},
		"finishsetup":           {"Concluir configuração", "Finish setup", "Einrichtung abschließen"},
		"updatesheading":        {"Atualizações e idioma", "Updates and language", "Updates und Sprache"},
		"automaticupdates":      {"Atualizações automáticas", "Automatic updates", "Automatische Updates"},
		"automaticupdatehint":   {"Verifica novas versões ao abrir. Você confirma a instalação. Desativar mantém a busca manual disponível.", "Checks for new versions on opening. You confirm installation. Manual checks remain available when disabled.", "Prüft beim Öffnen auf neue Versionen. Du bestätigst die Installation. Bei Deaktivierung bleibt die manuelle Suche verfügbar."},
		"savepreferences":       {"Salvar preferências", "Save preferences", "Einstellungen speichern"},
		"preferencessaved":      {"Preferências salvas.", "Preferences saved.", "Einstellungen gespeichert."},
		"checkupdates":          {"Buscar atualizações", "Check for updates", "Nach Updates suchen"},
		"installupdate":         {"Atualizar agora", "Update now", "Jetzt aktualisieren"},
		"installupdatehint":     {"Feche o BCS antes de instalar. Empresa, jogador e opções serão preservados.", "Close BCS before installing. Company, player and preferences will be preserved.", "BCS vor der Installation schließen. Firma, Spieler und Einstellungen bleiben erhalten."},
		"updatenotchecked":      {"Atualizações ainda não verificadas.", "Updates have not been checked yet.", "Noch nicht nach Updates gesucht."},
		"updatechecking":        {"Buscando atualização…", "Checking for updates…", "Updates werden gesucht…"},
		"updatecurrent":         {"Você já está na versão mais recente deste canal.", "You have the latest version in this channel.", "Du hast bereits die neueste Version dieses Kanals."},
		"updatefound":           {"Nova versão disponível:", "New version available:", "Neue Version verfügbar:"},
		"updateerror":           {"Não foi possível verificar ou instalar a atualização. Tente novamente. Seus dados foram preservados.", "The update could not be checked or installed. Try again. Your data was preserved.", "Das Update konnte nicht geprüft oder installiert werden. Erneut versuchen. Deine Daten bleiben erhalten."},
		"updatepreview":         {"O canal de atualização da 3.0 ainda não foi publicado. O canal estável da 2.x continua separado.", "The 3.0 update channel has not been published yet. The stable 2.x channel remains separate.", "Der Update-Kanal für 3.0 wurde noch nicht veröffentlicht. Der stabile 2.x-Kanal bleibt getrennt."},
		"updateinstalling":      {"Atualização iniciada", "Update started", "Update gestartet"},
		"updateinstallhint":     {"A configuração será reaberta após a instalação.", "Setup will reopen after installation.", "Die Einrichtung wird nach der Installation wieder geöffnet."},
		"independent":           {"Projeto independente. Não é um produto oficial da PeDePe.", "Independent project. Not an official PeDePe product.", "Unabhängiges Projekt. Kein offizielles PeDePe-Produkt."},
		"errorbcsbusy":          {"Feche o BCS e tente novamente. Não alterei suas configurações.", "Close BCS and try again. Your settings were not changed.", "BCS schließen und erneut versuchen. Deine Einstellungen wurden nicht verändert."},
		"errorbcsformat":        {"Não reconheci as configurações desta instalação do BCS. Confira a pasta do OMSI ou use a opção de copiar a pasta nesta tela.", "This BCS installation's settings were not recognized. Check the OMSI folder or use the copy-folder option on this page.", "Die Einstellungen dieser BCS-Installation wurden nicht erkannt. Den OMSI-Ordner prüfen oder den Ordner über diese Seite kopieren."},
		"errorbcscheck":         {"Não foi possível confirmar que o BCS está fechado. Reinicie o computador ou configure a pasta pelo próprio BCS.", "Could not confirm that BCS is closed. Restart the computer or configure the folder within BCS.", "Es konnte nicht bestätigt werden, dass BCS geschlossen ist. Den Computer neu starten oder den Ordner im BCS einstellen."},
	} {
		translations[key] = value
	}
}

func tr(key string) string {
	a, ok := translations[key]
	if !ok {
		return key
	}
	switch language() {
	case "en":
		return a[1]
	case "de":
		return a[2]
	}
	return a[0]
}
func friendlyError(e error) string {
	if e == nil {
		return ""
	}
	var startup hostStartupError
	if errors.As(e, &startup) { return startup.Error() }
	s := strings.ToLower(e.Error())
	if strings.Contains(s, "bcs running") {
		return tr("errorbcsbusy")
	}
	if strings.Contains(s, "bcs process check") {
		return tr("errorbcscheck")
	}
	if strings.Contains(s, "bcs settings") || strings.Contains(s, "bcs encoding") {
		return tr("errorbcsformat")
	}
	var he httpError
	if errors.As(e, &he) {
		if he.Code == 404 {
			return tr("errorinvite")
		}
		if he.Code == 409 {
			return tr("errorowner")
		}
	}
	for _, c := range []struct {
		parts []string
		key   string
	}{{[]string{"ônibus livres", "restringe ônibus", "protocolo 7"}, "errorversion"}, {[]string{"convite", "unknown company"}, "errorinvite"}, {[]string{"horário", "clock", "timezone"}, "errorclock"}, {[]string{"download", "sha-256", "cache", "zip"}, "errordownload"}, {[]string{"nome", "informe seu nome"}, "errorplayer"}, {[]string{"criador", "chave"}, "errorowner"}, {[]string{"network", "serviço", "connection", "diretório"}, "errornetwork"}, {[]string{"arquivo", "caminho", "pasta", "permission", "access is denied"}, "errorfiles"}, {[]string{"sem salvar"}, "errorclosed"}} {
		for _, part := range c.parts {
			if strings.Contains(s, part) {
				return tr(c.key)
			}
		}
	}
	return tr("error")
}

func init() {
	translations["controlsheading"] = [3]string{"Controles do programa", "Program controls", "Programmsteuerung"}
	translations["controlhint"] = [3]string{"Multiplayer desligado mantém o openOMSI em modo solo na próxima viagem. Desativar o programa exige o BCS fechado e faz o BCS voltar ao OMSI original. A configuração é preservada. Sessões já abertas não são interrompidas.", "Disabling multiplayer keeps openOMSI in solo mode for the next trip. Disable the program with BCS closed to restore original OMSI launching. Settings are preserved. Running sessions are not interrupted.", "Ohne Multiplayer läuft openOMSI bei der nächsten Fahrt allein. BCS schließen und das Programm deaktivieren, um wieder das originale OMSI zu starten. Einstellungen bleiben erhalten. Laufende Sitzungen werden nicht unterbrochen."}
	translations["multiplayeron"] = [3]string{"Multiplayer ativado", "Multiplayer enabled", "Multiplayer aktiviert"}
	translations["multiplayeroff"] = [3]string{"Multiplayer desativado: modo solo", "Multiplayer disabled: solo mode", "Multiplayer deaktiviert: Einzelspieler"}
	translations["disablemultiplayer"] = [3]string{"Desativar multiplayer", "Disable multiplayer", "Multiplayer deaktivieren"}
	translations["enablemultiplayer"] = [3]string{"Ativar multiplayer", "Enable multiplayer", "Multiplayer aktivieren"}
	translations["disableprogram"] = [3]string{"Desativar programa", "Disable program", "Programm deaktivieren"}
	translations["enableprogram"] = [3]string{"Reativar programa", "Reactivate program", "Programm reaktivieren"}
	translations["controlsaved"] = [3]string{"Preferência salva. Será aplicada na próxima viagem.", "Preference saved. Applies to the next trip.", "Einstellung gespeichert. Gilt für die nächste Fahrt."}
	translations["programoff"] = [3]string{"Programa desativado. O BCS volta a abrir o OMSI original.", "Program disabled. BCS launches original OMSI again.", "Programm deaktiviert. BCS startet wieder das originale OMSI."}
}

func init() {
	translations["viewrelease"] = [3]string{"Ver release 3.x e download", "View 3.x release and download", "3.x-Release und Download ansehen"}
}

func init() {
	translations["updatecurrent"] = [3]string{"Nenhuma atualização 3.x mais recente foi publicada no canal consultado.", "No newer 3.x update is published in this channel.", "In diesem Kanal ist kein neueres 3.x-Update veröffentlicht."}
}

func init() {
	translations["editconfiguration"] = [3]string{"Ver ou editar configuração", "View or edit configuration", "Konfiguration ansehen oder bearbeiten"}
	translations["errordetails"] = [3]string{"Ver detalhes do erro", "View error details", "Fehlerdetails anzeigen"}
}

func init() {
	translations["error"] = [3]string{"Não foi possível continuar. Abra Ver detalhes do erro abaixo.", "Could not continue. Open View error details below.", "Vorgang fehlgeschlagen. Unten Fehlerdetails anzeigen öffnen."}
}

func init() {
	translations["previewwarning"] = [3]string{"Candidata 3.0: BCS 5.0.2.0 e openOMSI 0.2.48+. Valide uma viagem completa com dois jogadores antes do lançamento definitivo.", "3.0 release candidate: BCS 5.0.2.0 and openOMSI 0.2.48+. Validate a complete two-player trip before the final release.", "3.0-Release-Kandidat: BCS 5.0.2.0 und openOMSI 0.2.48+. Vor der endgültigen Veröffentlichung eine vollständige Fahrt mit zwei Spielern prüfen."}
	translations["serverhint"] = [3]string{"O servidor usa protocolo 7. Novas sessões podem baixar a versão oficial; a opção Recomendada usa a 0.2.50 incluída. Cada mapa tem sua própria sessão e seu próprio túnel.", "The server uses protocol 7. New sessions can download the official release; Recommended uses the bundled 0.2.50. Each map has its own session and tunnel.", "Der Server nutzt Protokoll 7. Neue Sitzungen können die offizielle Version laden; Empfohlen nutzt die enthaltene 0.2.50. Jede Karte hat ihre eigene Sitzung und ihren eigenen Tunnel."}
	translations["errorversion"] = [3]string{"Cliente e servidor precisam usar protocolo 7 (openOMSI 0.2.48 ou superior). A 0.2.47 não foi publicada oficialmente. No Setup, use cliente automático e servidor Recomendado 0.2.50, ou versões manuais compatíveis.", "Client and server need protocol 7 (openOMSI 0.2.48 or later). 0.2.47 was not officially released. In Setup use the automatic client and Recommended 0.2.50 server, or compatible manual versions.", "Client und Server benötigen Protokoll 7 (openOMSI ab 0.2.48). 0.2.47 wurde nicht offiziell veröffentlicht. Automatischen Client und empfohlenen Server 0.2.50 oder kompatible manuelle Versionen wählen."}
	translations["diagnostics"] = [3]string{"Diagnóstico e suporte", "Diagnostics and support", "Diagnose und Support"}
	translations["diagnosticsdownload"] = [3]string{"Coletar logs", "Collect logs", "Protokolle sammeln"}
	translations["diagnosticshint"] = [3]string{"Gera um ZIP local com os logs do programa. Revise antes de compartilhar. Nada é enviado automaticamente. Discord: .zmaycon. (com os dois pontos).", "Creates a local ZIP of program logs. Review before sharing. Nothing is uploaded automatically. Discord: .zmaycon. (include both dots).", "Erstellt eine lokale ZIP mit Programmprotokollen. Vor dem Teilen prüfen. Kein automatischer Upload. Discord: .zmaycon. (mit beiden Punkten)."}
}
