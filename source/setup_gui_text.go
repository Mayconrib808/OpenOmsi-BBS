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
	"language":           {"Idioma / Language / Sprache", "Language / Idioma / Sprache", "Sprache / Language / Idioma"},
	"intro":              {"Configure uma vez.\r\nInicie suas viagens pelo BCS.", "Set up once.\r\nStart your trips from BCS/BBS.", "Einmal einrichten.\r\nFahrten über BBS starten."},
	"games":              {"1   Escolha os jogos", "1   Choose your games", "1   Spiele auswählen"},
	"root":               {"Pasta do OMSI 2", "OMSI 2 folder", "OMSI-2-Ordner"},
	"executable":         {"Executável do OpenOMSI", "OpenOMSI executable", "OpenOMSI-Programmdatei"},
	"folder":             {"Escolher pasta...", "Browse folder...", "Ordner wählen..."},
	"file":               {"Escolher arquivo...", "Browse file...", "Datei wählen..."},
	"company":            {"2   Multiplayer da empresa — opcional", "2   Company multiplayer — optional", "2   Firmen-Multiplayer — optional"},
	"enable":             {"Ativar multiplayer da empresa", "Enable company multiplayer", "Firmen-Multiplayer aktivieren"},
	"profile":            {"Perfil da empresa (JSON ou link HTTPS; só no multiplayer)", "Company profile (JSON or HTTPS link; multiplayer only)", "Firmenprofil (JSON oder HTTPS-Link; nur für Multiplayer)"},
	"player":             {"Seu nome (só no multiplayer)", "Your name (multiplayer only)", "Dein Name (nur für Multiplayer)"},
	"optional":           {"Sem multiplayer? Deixe a caixa desmarcada. Perfil e nome podem ficar vazios.", "Playing without multiplayer? Leave the box unchecked. Profile and name can be empty.", "Ohne Multiplayer? Kästchen deaktiviert lassen. Profil und Name dürfen leer bleiben."},
	"profile_hint":       {"Use o perfil do administrador. Ele será guardado; você não precisa editar o JSON.", "Use your administrator's profile. A copy is saved; you do not need to edit JSON.", "Nutze das Profil der Administration. Eine Kopie wird gespeichert; kein JSON-Bearbeiten nötig."},
	"save":               {"Salvar e ativar", "Save and activate", "Speichern / aktivieren"},
	"deactivate":         {"Desativar ponte", "Deactivate bridge", "Bridge deaktivieren"},
	"status":             {"Conferir estado", "Check status", "Status prüfen"},
	"cancel":             {"Cancelar", "Cancel", "Abbrechen"},
	"ready":              {"Escolha os caminhos dos jogos e use Salvar e ativar. O multiplayer é opcional.", "Choose the game paths, then Save and activate. Multiplayer is optional.", "Spielpfade auswählen, dann Speichern / aktivieren. Multiplayer ist optional."},
	"logs":               {"Coletar logs", "Collect logs", "Logs sammeln"},
	"tutorial":           {"Tutorial", "Tutorial", "Anleitung"},
	"host":               {"Empresa / servidor", "Company / server", "Firma / Server"},
	"admin":              {"Perfil (administrador)", "Profile (administrator)", "Profil (Administration)"},
	"failed":             {"Não foi possível concluir: ", "Could not complete: ", "Vorgang fehlgeschlagen: "},
	"need_profile":       {"Selecione o perfil JSON ou cole o link HTTPS fornecido pelo administrador da empresa.", "Select the JSON profile or paste the HTTPS link provided by your company administrator.", "Wähle das JSON-Profil oder füge den HTTPS-Link deiner Firmenadministration ein."},
	"checking_games":     {"Conferindo os jogos... Aguarde. O Windows pedirá permissão para ativar a ponte.", "Checking the games... Please wait. Windows will request permission to activate the bridge.", "Spiele werden geprüft... Bitte warten. Windows fragt nach der Berechtigung zur Aktivierung."},
	"incompatible":       {"O OpenOMSI escolhido não passou na verificação de compatibilidade: %w", "The selected OpenOMSI failed the compatibility check: %w", "Die gewählte OpenOMSI-Version hat die Kompatibilitätsprüfung nicht bestanden: %w"},
	"package":            {"Pacote incompleto ou alterado. Extraia o ZIP completo novamente: %w", "Package incomplete or modified. Extract the complete ZIP again: %w", "Paket unvollständig oder verändert. Entpacke die vollständige ZIP erneut: %w"},
	"cancelled":          {"Verificação cancelada.", "Check cancelled.", "Prüfung abgebrochen."},
	"activated":          {"Tudo pronto! Abra o BCS e inicie sua viagem normalmente.", "All set! Open BCS/BBS and start your trip normally.", "Fertig! Öffne BBS und starte deine Fahrt wie gewohnt."},
	"server_required":    {"\r\nNo multiplayer, o servidor da empresa precisa estar aberto.", "\r\nFor multiplayer, the company server must be running.", "\r\nFür Multiplayer muss der Firmenserver laufen."},
	"close_games":        {"Feche o OMSI, o OpenOMSI e o CompanyHost antes de alterar a configuração.", "Close OMSI, OpenOMSI and CompanyHost before changing settings.", "Schließe OMSI, OpenOMSI und CompanyHost, bevor du Einstellungen änderst."},
	"cancelling":         {"Cancelando a verificação... Se a ativação já começou, aguarde sua conclusão.", "Cancelling the check... If activation has started, wait for it to finish.", "Prüfung wird abgebrochen... Eine bereits begonnene Aktivierung bitte abwarten."},
	"pick_executable":    {"Escolha openomsi.exe", "Choose openomsi.exe", "openomsi.exe auswählen"},
	"executable_filter":  {"OpenOMSI (openomsi.exe)\x00openomsi.exe\x00Executáveis (*.exe)\x00*.exe\x00\x00", "OpenOMSI (openomsi.exe)\x00openomsi.exe\x00Executables (*.exe)\x00*.exe\x00\x00", "OpenOMSI (openomsi.exe)\x00openomsi.exe\x00Programme (*.exe)\x00*.exe\x00\x00"},
	"pick_profile":       {"Escolha o perfil da empresa", "Choose the company profile", "Firmenprofil auswählen"},
	"profile_filter":     {"Perfil da empresa (*.json)\x00*.json\x00\x00", "Company profile (*.json)\x00*.json\x00\x00", "Firmenprofil (*.json)\x00*.json\x00\x00"},
	"confirm_deactivate": {"Desativar a ponte e voltar ao OMSI original?", "Deactivate the bridge and return to original OMSI?", "Bridge deaktivieren und zum originalen OMSI zurückkehren?"},
	"deactivating":       {"Desativando a ponte... Aguarde a solicitação do Windows.", "Deactivating the bridge... Wait for the Windows prompt.", "Bridge wird deaktiviert... Warte auf die Windows-Abfrage."},
	"deactivated":        {"Ponte desativada. O OMSI original será usado nas próximas viagens.", "Bridge deactivated. Future trips will use original OMSI.", "Bridge deaktiviert. Künftige Fahrten verwenden das originale OMSI."},
	"checking_status":    {"Conferindo o estado...", "Checking status...", "Status wird geprüft..."},
	"confirm_logs":       {"Criar um ZIP de diagnóstico? Ele pode conter caminhos locais e identificadores presentes nos logs. Nada será enviado automaticamente.", "Create a diagnostic ZIP? It may contain local paths and identifiers from the logs. Nothing is uploaded automatically.", "Diagnose-ZIP erstellen? Sie kann lokale Pfade und Kennungen aus den Logs enthalten. Es wird nichts automatisch hochgeladen."},
	"collecting":         {"Coletando os logs...", "Collecting logs...", "Logs werden gesammelt..."},
	"zip_created":        {"ZIP criado: ", "ZIP created: ", "ZIP erstellt: "},
	"file_dialog_error":  {"Não foi possível abrir o seletor de arquivos (%d).", "Could not open the file picker (%d).", "Dateiauswahl konnte nicht geöffnet werden (%d)."},
	"pick_folder":        {"Escolha a pasta original do OMSI 2 (contém Omsi.exe)", "Choose the original OMSI 2 folder (contains Omsi.exe)", "Originalen OMSI-2-Ordner auswählen (enthält Omsi.exe)"},
	"local_folder":       {"Selecione uma pasta local do Windows.", "Select a local Windows folder.", "Wähle einen lokalen Windows-Ordner."},
	"busy":               {"Uma operação está em andamento. Aguarde a conclusão ou use Cancelar.", "An operation is running. Wait for it to finish or use Cancel.", "Ein Vorgang läuft. Warte auf den Abschluss oder wähle Abbrechen."},
}

func setupText(lang, key string) string {
	texts, ok := setupStrings[key]
	if !ok {
		return key
	}
	return texts[setupLanguageIndex(lang)]
}
