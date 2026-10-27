package main

// Keep the graphical Setup's language choices aligned with the CLI and saved
// configuration. Native names remain recognizable before changing language.
var setupLanguages = []string{"pt", "en", "de"}
var setupLanguageNames = []string{"Português (Brasil)", "English", "Deutsch"}

func setupLanguageIndex(lang string) int {
	for i, code := range setupLanguages {
		if code == normalizeLanguage(lang) {
			return i
		}
	}
	return 0
}

var setupStrings = map[string][3]string{
	"server_copy":           {"Copiar link", "Copy link", "Link kopieren"},
	"server_copied":         {"Link de acesso copiado: ", "Join link copied: ", "Beitrittslink kopiert: "},
	"server_active_none":    {"Nenhum mapa com link ativo no momento.", "No map with an active join link right now.", "Derzeit keine Karte mit aktivem Beitrittslink."},
	"server_status":         {"Status do Servidor", "Server status", "Serverstatus"},
	"server_check":          {"Verificar se o servidor está online", "Check whether the server is online", "Serverstatus jetzt prüfen"},
	"server_checking":       {"Verificando...", "Checking...", "Wird geprüft..."},
	"server_standby":        {"Agente pronto (sob demanda)", "Agent ready (on demand)", "Agent bereit (auf Anfrage)"},
	"server_online":         {"Servidor online", "Server online", "Server online"},
	"server_starting":       {"Agente ativo; iniciando mapa", "Agent active; starting map", "Agent aktiv; Karte startet"},
	"server_agent_offline":  {"HostAgent offline", "HostAgent offline", "HostAgent offline"},
	"server_unavailable":    {"Não foi possível confirmar", "Could not confirm", "Bestätigung nicht möglich"},
	"server_unconfigured":   {"Host/empresa não configurado", "Host/company not configured", "Host/Firma nicht eingerichtet"},
	"server_no_maps":        {"Nenhum mapa disponível", "No map available", "Keine Karte verfügbar"},
	"bundled_coop":          {"Usar Coop incluído", "Use bundled Coop", "Coop auswählen"},
	"bundled_coop_selected": {"Coop incluído 0.3.45 selecionado. Use Salvar e ativar para aplicar.", "Bundled Coop 0.3.45 selected. Use Save and activate to apply.", "Enthaltener Coop 0.3.45 ausgewählt. Mit Speichern und aktivieren übernehmen."},
	"bundled_coop_missing":  {"A pasta OpenOmsi Coop 0.3.41 não foi encontrada. Conclua a atualização 2.2 ou extraia o pacote completo.", "The OpenOmsi Coop 0.3.41 folder was not found. Complete the 2.2 update or extract the full package.", "Der Ordner OpenOmsi Coop 0.3.41 fehlt. Das Update 2.2 abschließen oder das vollständige Paket entpacken."},
	"autostart":             {"Ligar o ônibus automaticamente", "Start the bus automatically", "Bus automatisch starten"},
	"autostart_hint":        {"Desmarcado: partida e IBIS manuais.", "Unchecked: start bus and set up IBIS manually.", "Ohne Häkchen: Bus und IBIS selbst vorbereiten."},
	"updates_automatic":     {"Buscar atualizações ao abrir", "Check for updates on startup", "Beim Öffnen nach Updates suchen"},
	"updates_manual_only":   {"Verificação automática desativada.", "Automatic checking disabled.", "Automatische Prüfung deaktiviert."},
	"coop_backup":           {"Backup do Cooperativo", "Back up Cooperative", "Koop sichern"},
	"coop_restore":          {"Restaurar backup", "Restore backup", "Sicherung wiederherstellen"},
	"coop_backup_work":      {"Conferindo e salvando a versão modificada...", "Verifying and saving the modified build...", "Modifizierte Version prüfen und sichern..."},
	"coop_backup_done":      {"Backup da versão modificada salvo.", "Modified build backed up.", "Modifizierte Version gesichert."},
	"coop_restore_work":     {"Restaurando a versão modificada...", "Restoring the modified build...", "Modifizierte Version wiederherstellen..."},
	"coop_restore_done":     {"Cooperativo restaurado. Seus mapas e configurações foram preservados.", "Cooperative restored. Your maps and settings were preserved.", "Koop wiederhergestellt. Karten und Einstellungen bleiben erhalten."},
	"coop_restore_confirm":  {"Restaurar os arquivos da versão modificada? Feche openOMSI, BCS e HostAgent. Mapas, ônibus e configurações pessoais serão preservados.", "Restore the modified build? Close openOMSI, BBS and HostAgent. Maps, buses and personal settings will be preserved.", "Modifizierte Version wiederherstellen? openOMSI, BBS und HostAgent schließen. Karten, Busse und eigene Einstellungen bleiben erhalten."},

	"updates_check":        {"Buscar atualizações", "Check for updates", "Nach Updates suchen"},
	"updates_install":      {"Atualizar agora", "Update now", "Jetzt aktualisieren"},
	"updates_checking":     {"Buscando atualizações...", "Checking for updates...", "Updates werden gesucht..."},
	"updates_current":      {"Você está atualizado.", "You are up to date.", "Du bist auf dem neuesten Stand."},
	"updates_available":    {"Versão %s disponível.", "Version %s available.", "Version %s verfügbar."},
	"updates_unavailable":  {"Não foi possível buscar agora.", "Could not check right now.", "Prüfung derzeit nicht möglich."},
	"updates_unconfigured": {"Distribuição aguardando configuração.", "Distribution awaiting configuration.", "Update-Dienst noch nicht eingerichtet."},
	"updates_confirm":      {"Instalar a versão %s? Feche BCS, openOMSI e o HostAgent. O Setup será reaberto ao concluir.", "Install version %s? Close BCS, openOMSI and HostAgent. Setup will reopen when finished.", "Version %s installieren? BBS, openOMSI und HostAgent schließen. Setup startet anschließend neu."},
	"updates_downloading":  {"Baixando e conferindo a atualização...", "Downloading and verifying the update...", "Update wird geladen und geprüft..."},
	"language":             {"Idioma / Language / Sprache", "Language / Idioma / Sprache", "Sprache / Language / Idioma"},
	"intro":                {"Configure uma vez.\r\nInicie suas viagens pelo BCS.", "Set up once.\r\nStart your trips from BCS/BBS.", "Einmal einrichten.\r\nFahrten über BBS starten."},
	"games":                {"1   Escolha os jogos", "1   Choose your games", "1   Spiele auswählen"},
	"root":                 {"Pasta do OMSI 2", "OMSI 2 folder", "OMSI-2-Ordner"},
	"executable":           {"Executável do OpenOMSI", "OpenOMSI executable", "OpenOMSI-Programmdatei"},
	"folder":               {"Escolher pasta...", "Browse folder...", "Ordner wählen..."},
	"file":                 {"Escolher arquivo...", "Browse file...", "Datei wählen..."},
	"company":              {"2   Multiplayer da empresa — opcional", "2   Company multiplayer — optional", "2   Firmen-Multiplayer — optional"},
	"enable":               {"Ativar multiplayer da empresa", "Enable company multiplayer", "Firmen-Multiplayer aktivieren"},
	"profile":              {"Perfil da empresa (JSON ou link HTTPS; só no multiplayer)", "Company profile (JSON or HTTPS link; multiplayer only)", "Firmenprofil (JSON oder HTTPS-Link; nur für Multiplayer)"},
	"player":               {"Seu nome (só no multiplayer)", "Your name (multiplayer only)", "Dein Name (nur für Multiplayer)"},
	"optional":             {"Sem multiplayer? Deixe a caixa desmarcada. Perfil e nome podem ficar vazios.", "Playing without multiplayer? Leave the box unchecked. Profile and name can be empty.", "Ohne Multiplayer? Kästchen deaktiviert lassen. Profil und Name dürfen leer bleiben."},
	"profile_hint":         {"Use o perfil do administrador. Ele será guardado; você não precisa editar o JSON.", "Use your administrator's profile. A copy is saved; you do not need to edit JSON.", "Nutze das Profil der Administration. Eine Kopie wird gespeichert; kein JSON-Bearbeiten nötig."},
	"save":                 {"Salvar e ativar", "Save and activate", "Speichern / aktivieren"},
	"deactivate":           {"Desativar ponte", "Deactivate bridge", "Bridge deaktivieren"},
	"status":               {"Conferir estado", "Check status", "Status prüfen"},
	"cancel":               {"Cancelar", "Cancel", "Abbrechen"},
	"ready":                {"Escolha os caminhos dos jogos e use Salvar e ativar. O multiplayer é opcional.", "Choose the game paths, then Save and activate. Multiplayer is optional.", "Spielpfade auswählen, dann Speichern / aktivieren. Multiplayer ist optional."},
	"logs":                 {"Coletar logs", "Collect logs", "Logs sammeln"},
	"tutorial":             {"Tutorial", "Tutorial", "Anleitung"},
	"host":                 {"Empresa / servidor", "Company / server", "Firma / Server"},
	"admin":                {"Perfil (administrador)", "Profile (administrator)", "Profil (Administration)"},
	"failed":               {"Não foi possível concluir: ", "Could not complete: ", "Vorgang fehlgeschlagen: "},
	"need_profile":         {"Selecione o perfil JSON ou cole o link HTTPS fornecido pelo administrador da empresa.", "Select the JSON profile or paste the HTTPS link provided by your company administrator.", "Wähle das JSON-Profil oder füge den HTTPS-Link deiner Firmenadministration ein."},
	"checking_games":       {"Conferindo os jogos... Aguarde. O Windows pedirá permissão para ativar a ponte.", "Checking the games... Please wait. Windows will request permission to activate the bridge.", "Spiele werden geprüft... Bitte warten. Windows fragt nach der Berechtigung zur Aktivierung."},
	"incompatible":         {"O OpenOMSI escolhido não passou na verificação de compatibilidade: %w", "The selected OpenOMSI failed the compatibility check: %w", "Die gewählte OpenOMSI-Version hat die Kompatibilitätsprüfung nicht bestanden: %w"},
	"package":              {"Pacote incompleto ou alterado. Extraia o ZIP completo novamente: %w", "Package incomplete or modified. Extract the complete ZIP again: %w", "Paket unvollständig oder verändert. Entpacke die vollständige ZIP erneut: %w"},
	"cancelled":            {"Verificação cancelada.", "Check cancelled.", "Prüfung abgebrochen."},
	"activated":            {"Tudo pronto! Abra o BCS e inicie sua viagem normalmente.", "All set! Open BCS/BBS and start your trip normally.", "Fertig! Öffne BBS und starte deine Fahrt wie gewohnt."},
	"server_required":      {"\r\nNo multiplayer, o servidor da empresa precisa estar aberto.", "\r\nFor multiplayer, the company server must be running.", "\r\nFür Multiplayer muss der Firmenserver laufen."},
	"close_games":          {"Feche o BCS, o OMSI, o OpenOMSI e o CompanyHost antes de alterar a configuração.", "Close BCS, OMSI, OpenOMSI and CompanyHost before changing settings.", "Schließe BBS, OMSI, OpenOMSI und CompanyHost, bevor du Einstellungen änderst."},
	"cancelling":           {"Cancelando a verificação... Se a ativação já começou, aguarde sua conclusão.", "Cancelling the check... If activation has started, wait for it to finish.", "Prüfung wird abgebrochen... Eine bereits begonnene Aktivierung bitte abwarten."},
	"pick_executable":      {"Escolha o openOMSI ou o abridor do Cooperativo", "Choose openOMSI or the Cooperative opener", "openOMSI oder den kooperativen Starter auswählen"},
	"executable_filter":    {"OpenOMSI / Cooperativo (*.exe)\x00*.exe\x00\x00", "OpenOMSI / Cooperative (*.exe)\x00*.exe\x00\x00", "OpenOMSI / Kooperativ (*.exe)\x00*.exe\x00\x00"},
	"pick_profile":         {"Escolha o perfil da empresa", "Choose the company profile", "Firmenprofil auswählen"},
	"profile_filter":       {"Perfil da empresa (*.json)\x00*.json\x00\x00", "Company profile (*.json)\x00*.json\x00\x00", "Firmenprofil (*.json)\x00*.json\x00\x00"},
	"confirm_deactivate":   {"Desativar a ponte e voltar ao OMSI original?", "Deactivate the bridge and return to original OMSI?", "Bridge deaktivieren und zum originalen OMSI zurückkehren?"},
	"deactivating":         {"Desativando a ponte... Aguarde a solicitação do Windows.", "Deactivating the bridge... Wait for the Windows prompt.", "Bridge wird deaktiviert... Warte auf die Windows-Abfrage."},
	"deactivated":          {"Ponte desativada. O OMSI original será usado nas próximas viagens.", "Bridge deactivated. Future trips will use original OMSI.", "Bridge deaktiviert. Künftige Fahrten verwenden das originale OMSI."},
	"checking_status":      {"Conferindo o estado...", "Checking status...", "Status wird geprüft..."},
	"confirm_logs":         {"Criar um ZIP de diagnóstico? Ele pode conter caminhos locais e identificadores presentes nos logs. Nada será enviado automaticamente.", "Create a diagnostic ZIP? It may contain local paths and identifiers from the logs. Nothing is uploaded automatically.", "Diagnose-ZIP erstellen? Sie kann lokale Pfade und Kennungen aus den Logs enthalten. Es wird nichts automatisch hochgeladen."},
	"collecting":           {"Coletando os logs...", "Collecting logs...", "Logs werden gesammelt..."},
	"zip_created":          {"ZIP criado: ", "ZIP created: ", "ZIP erstellt: "},
	"file_dialog_error":    {"Não foi possível abrir o seletor de arquivos (%d).", "Could not open the file picker (%d).", "Dateiauswahl konnte nicht geöffnet werden (%d)."},
	"pick_folder":          {"Escolha a pasta original do OMSI 2 (contém Omsi.exe)", "Choose the original OMSI 2 folder (contains Omsi.exe)", "Originalen OMSI-2-Ordner auswählen (enthält Omsi.exe)"},
	"local_folder":         {"Selecione uma pasta local do Windows.", "Select a local Windows folder.", "Wähle einen lokalen Windows-Ordner."},
	"busy":                 {"Uma operação está em andamento. Aguarde a conclusão ou use Cancelar.", "An operation is running. Wait for it to finish or use Cancel.", "Ein Vorgang läuft. Warte auf den Abschluss oder wähle Abbrechen."},
}

func setupText(lang, key string) string {
	texts, ok := setupStrings[key]
	if !ok {
		return key
	}
	return texts[setupLanguageIndex(lang)]
}
