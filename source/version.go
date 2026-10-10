package main

import (
	"strings"
	"time"
)

const bridgeVersion = "2.1.0"
const bridgeAuthor = "Mayconrib808"

// Large maps can take several minutes on first load. All startup watchers share one limit.
const startupTimeout = 15 * time.Minute

func normalizeLanguage(s string) string {
	s = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", "-")
	switch s {
	case "en", "en-us", "en-gb":
		return "en"
	case "de", "de-de", "de-at", "de-ch":
		return "de"
	default:
		return "pt"
	}
}

func localText(lang, pt, en, de string) string {
	switch normalizeLanguage(lang) {
	case "en":
		return en
	case "de":
		return de
	default:
		return pt
	}
}

func affirmativeAnswer(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "s", "sim", "y", "yes", "j", "ja":
		return true
	default:
		return false
	}
}

// PeDePe added native openOMSI support on 2026-10-09. The preferred 2.1 flow
// uses PeDePeAdapter/openomsi.exe as the OpenOMSI path in BBS. The historical
// IFEO bridge remains in the package only as a compatibility fallback.
func usageNotices(lang string) string {
	return bugReportNotice(lang) + localText(lang,
		"\nMODO RECOMENDADO NO BCS/BBS: selecione OpenOMSI (Beta) e use a pasta PeDePeAdapter deste pacote como caminho do OpenOMSI. O BBS passa mapa, ônibus, horário, motorista e finalização diretamente; a 2.1.0 acrescenta somente o multiplayer da empresa quando necessário. Não use F9 para finalizar no modo nativo.\n\nMODO LEGADO: a ativação antiga por Omsi.exe continua disponível apenas como fallback. Nesse modo, mantenha \"Iniciar o OMSI mais depressa\" desmarcado e siga o procedimento legado de finalização.\n",
		"\nRECOMMENDED BCS/BBS MODE: select OpenOMSI (Beta) and use this package's PeDePeAdapter folder as the OpenOMSI path. BBS supplies the map, bus, time, driver and trip completion directly; 2.1.0 only adds company multiplayer when needed. Do not use F9 to finish a native-mode trip.\n\nLEGACY MODE: the old Omsi.exe interception remains only as a fallback. In that mode keep \"Start OMSI faster\" unchecked and use the legacy completion procedure.\n",
		"\nEMPFOHLENER BBS-MODUS: Wähle OpenOMSI (Beta) und den Ordner PeDePeAdapter dieses Pakets als OpenOMSI-Pfad. BBS übergibt Karte, Bus, Zeit, Fahrer und Fahrtabschluss direkt; 2.1.0 ergänzt nur bei Bedarf den Firmen-Multiplayer. Im nativen Modus ist F9 zum Beenden nicht erforderlich.\n\nLEGACY-MODUS: Die alte Umleitung über Omsi.exe bleibt nur als Fallback erhalten. Dort \"OMSI schneller starten\" deaktiviert lassen und den alten Abschlussablauf verwenden.\n")
}

// The public contact is shown in the selected language; no reports are sent automatically.
func bugReportNotice(lang string) string {
	return localText(lang,
		"\nBUGS — CONTATO NO DISCORD\nQualquer bug deve ser enviado pelo Discord para: .zmaycon.\nATENÇÃO: o ponto (.) no começo e o ponto no final fazem parte do nick. Inclua os dois pontos ao adicionar o contato e enviar uma mensagem. Copie o nick completo exatamente como mostrado: com os pontos nas duas pontas.\nDescreva o que aconteceu e informe o mapa e a linha. Inclua o ZIP de logs criado pela opção 5 do Setup, de preferência enquanto o problema estiver acontecendo.\n",
		"\nBUG REPORTS — DISCORD CONTACT\nReport any bug on Discord to: .zmaycon.\nIMPORTANT: The dot (.) at the start and the dot at the end are part of the username. Include BOTH dots when adding this contact and sending a message. Copy the complete username exactly as shown, including both dots.\nDescribe what happened and include the map and line. Attach the log ZIP created with Setup option 5, preferably while the issue is happening.\n",
		"\nFEHLER MELDEN — DISCORD-KONTAKT\nMelde jeden Fehler über Discord an: .zmaycon.\nWICHTIG: Der Punkt (.) am Anfang und der Punkt am Ende gehören zum Benutzernamen. Beide Punkte sind nötig, um den Kontakt hinzuzufügen und eine Nachricht zu senden. Kopiere den vollständigen Namen genau wie angegeben, einschließlich beider Punkte.\nBeschreibe das Problem und gib Karte und Linie an. Füge die mit Setup-Option 5 erstellte Protokoll-ZIP-Datei bei, möglichst während das Problem auftritt.\n")
}
