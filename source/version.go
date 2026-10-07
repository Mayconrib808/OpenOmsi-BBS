package main

import (
	"strings"
	"time"
)

const bridgeVersion = "2.0.0-dev.4"
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

// Display required BCS/BBS setup separately from the known map issue.
// The bridge does not change the user's BCS/BBS settings or map assets.
func usageNotices(lang string) string {
	return bugReportNotice(lang) + localText(lang,
		"\nOBRIGATÓRIO NO BCS/BBS: Definições > Definições avançadas > OMSI.\nDeixe DESMARCADO \"Iniciar o OMSI mais depressa\" antes de iniciar qualquer viagem com a bridge.\n\nPROBLEMA CONHECIDO: Berlin-Spandau não está carregando corretamente no openOMSI 0.2.0; ruas/trechos podem faltar. Esta versão não corrige o mapa. Para jogar nele, desative a bridge (opção 3) e use o OMSI original. Desmarcar a opção acima não é uma correção para Berlin-Spandau.\n",
		"\nREQUIRED IN BCS/BBS: Settings > Advanced settings > OMSI.\nLeave \"Start OMSI faster\" UNCHECKED before starting any trip with the bridge.\n\nKNOWN ISSUE: Berlin-Spandau is not loading correctly in openOMSI 0.2.0; roads/sections may be missing. This version does not fix the map. To play it, deactivate the bridge (option 3) and use original OMSI. Unchecking the setting above is not a fix for Berlin-Spandau.\n",
		"\nERFORDERLICH IN BBS: Einstellungen > Erweiterte Einstellungen > OMSI.\nLasse \"OMSI schneller starten\" DEAKTIVIERT (Häkchen entfernen), bevor du eine Fahrt mit der Bridge startest.\n\nBEKANNTES PROBLEM: Berlin-Spandau wird in openOMSI 0.2.0 nicht korrekt geladen; Straßen oder Kartenabschnitte können fehlen. Diese Version behebt das Kartenproblem nicht. Deaktiviere die Bridge (Option 3) und nutze das originale OMSI für diese Karte. Das Deaktivieren der obigen Einstellung behebt das Problem mit Berlin-Spandau nicht.\n")
}

// The public contact is shown in the selected language; no reports are sent automatically.
func bugReportNotice(lang string) string {
	return localText(lang,
		"\nBUGS — CONTATO NO DISCORD\nQualquer bug deve ser enviado pelo Discord para: .zmaycon.\nATENÇÃO: o ponto (.) no começo e o ponto no final fazem parte do nick. Inclua os dois pontos ao adicionar o contato e enviar uma mensagem. Copie o nick completo exatamente como mostrado: com os pontos nas duas pontas.\nDescreva o que aconteceu e informe o mapa e a linha. Inclua o ZIP de logs criado pela opção 5 do Setup, de preferência enquanto o problema estiver acontecendo.\n",
		"\nBUG REPORTS — DISCORD CONTACT\nReport any bug on Discord to: .zmaycon.\nIMPORTANT: The dot (.) at the start and the dot at the end are part of the username. Include BOTH dots when adding this contact and sending a message. Copy the complete username exactly as shown, including both dots.\nDescribe what happened and include the map and line. Attach the log ZIP created with Setup option 5, preferably while the issue is happening.\n",
		"\nFEHLER MELDEN — DISCORD-KONTAKT\nMelde jeden Fehler über Discord an: .zmaycon.\nWICHTIG: Der Punkt (.) am Anfang und der Punkt am Ende gehören zum Benutzernamen. Beide Punkte sind nötig, um den Kontakt hinzuzufügen und eine Nachricht zu senden. Kopiere den vollständigen Namen genau wie angegeben, einschließlich beider Punkte.\nBeschreibe das Problem und gib Karte und Linie an. Füge die mit Setup-Option 5 erstellte Protokoll-ZIP-Datei bei, möglichst während das Problem auftritt.\n")
}
