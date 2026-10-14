# BBS+OpenOmsi Multiplayer 3.0

Fahre deine Fahrten aus dem **Busbetrieb-Simulator (BBS/BCS)** im **Multiplayer von openOMSI**. BBS wählt und bewertet die Fahrt; openOMSI simuliert die gemeinsame Welt. Fahrer auf derselben Karte können den Bussen der anderen begegnen.

**3.0.3-rc.2 ist ein Release-Kandidat.** Vor der endgültigen WebDisk-Veröffentlichung eine vollständige Fahrt mit zwei PCs prüfen.

## Voraussetzungen und Einrichtung

Windows 10/11 x64, ein eigener installierter OMSI 2 und **BBS 5.0.2.0**, die Karte, ihre Abhängigkeiten und der eigene Bus. Client und Server benötigen **Protokoll 7**, eingeführt mit **openOMSI 0.2.48**. 0.2.47 wurde nicht offiziell veröffentlicht. Enthalten ist der unveränderte offizielle **Server 0.2.50**.

1. Die gesamte ZIP in einen dauerhaften Ordner entpacken. BBS schließen und **Setup.exe** öffnen.
2. Sprache wählen und den ursprünglichen OMSI-2-Ordner bestätigen.
3. Firmenraum erstellen oder die Einladung der Firma einfügen. Spielernamen eintragen.
4. Beim Erstellen Referenzzeitzone und **Zeitverschiebung** aus den BBS-Firmeneinstellungen übernehmen. Bei Bedarf ausdrücklich null eingeben.
5. Speichern und bei geschlossenem BBS **BCS konfigurieren** wählen. Zuerst wird ein Einstellungsbackup erstellt.
6. Im BBS **OpenOMSI (Beta)** und **OpenOMSI-BBS/PeDePeAdapter** prüfen. Falls die automatische Einrichtung nicht möglich ist, den Adapterordner im Setup kopieren und im BBS auswählen.
7. Die Fahrt im BBS starten. Erster Client-/Tunnel-Download und Kartenladen können etwas dauern.

Den echten Client im Setup auswählen. Im BBS wird der Adapter ausgewählt. Automatische Downloads prüfen den offiziellen SHA-256. Unter Erweiterte Optionen → Client → Manuell bleibt eine kompatible eigene Installation nutzbar, auch 0.2.50 mit DLSS/ReShade. Grafikeffekte werden separat eingerichtet.

## Host und Karten

Die **Firmeneinladung** führt jede Fahrt zur passenden Kartensitzung. Der erste Fahrer einer Karte startet einen unabhängigen lokalen Server; weitere Fahrer dieser Karte treten ihm bei. Berlin und Carrão City haben getrennte Server und Tunnel. Fahrer auf unterschiedlichen Karten sehen einander nicht. Ein direkter `trycloudflare.com`-Link zu Berlin führt nur nach Berlin; für unterschiedliche Karten die Firmeneinladung im Setup nutzen.

Wenn der Host sein Spiel schließt, läuft der dedizierte Prozess weiter, solange Mitspieler bleiben. Sein PC muss eingeschaltet und online bleiben. Ausschalten beendet die Verbindung. Beim nächsten Beitritt kann ein anderer Host gewählt werden; der laufende Weltzustand wird nicht nahtlos übertragen. Leere Sitzungen schließen nach einer Wartezeit; die Einladung bleibt gültig.

Jeder fährt seinen eigenen Bus. Fehlende Modelle können auf anderen PCs durch Ersatzmodelle dargestellt werden. Für das genaue Modell denselben Bus und dieselbe Lackierung installieren. Keine Karten, privaten Busse oder kostenpflichtigen DLC-Dateien werden mitgeliefert.

## Abschluss und nächste Fahrt

BBS übergibt Bus, Situation, Dienst, Lackierung und Wetter. Die Integration bereitet den Dienst vor und übernimmt den gespeicherten Kraftstoff des Spielerbusses. Zielanzeige und Fahrplan benötigen passende Karten-, HOF- und Busskripte.

BBS 5.0.2.0 verfügt über eine native Befehlsschnittstelle mit Rückmeldung für **Nächste Fahrt**. Das unveränderte PeDePe-Plugin wird weitergegeben und wählt den nächsten Dienst im laufenden Spiel. Die JAR-Datei wird nicht gepatcht.

Die Fahrt über den **nativen BBS-Ablauf** beenden und openOMSI bis zur Ergebnisbestätigung geöffnet lassen. Die alten F9-/Zwei-Sekunden-Anweisungen gehören zur Legacy-Integration 2.x. Danach im BBS **Nächste Fahrt** wählen und Dienst sowie Zielanzeige vor der Abfahrt prüfen.

## Updates und Hilfe

Integrationsupdates nutzen den getrennten geprüften **Cloudflare-Kanal 3.x**. Automatische und manuelle Suche sind verfügbar. Vor der Installation BBS schließen; Firma und Spielereinstellungen bleiben erhalten. Client und Server haben eigene Updateoptionen. Empfohlen nutzt den enthaltenen Server 0.2.50; Manuell erhält die Auswahl. Kein stiller Rückfall auf Protokoll 6.

Multiplayer deaktivieren erhält Solo-Fahrten in openOMSI. Programm bei geschlossenem BBS deaktivieren stellt den Start des ursprünglichen OMSI wieder her. Bestehende Sitzungen werden nicht unterbrochen.

Im Setup **Protokolle sammeln**, die ZIP prüfen und Karte, Bus/HOF und Fahrt über [Issues](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) oder Discord **`.zmaycon.`** melden (beide Punkte gehören zum Namen). Kein automatischer Upload.

Das [Validierungsprotokoll](VALIDATION_3.md) trennt automatische Prüfungen von ausstehenden Spiel- und Antivirusprüfungen. Unabhängiges Community-Projekt von Mayconrib808; kein offizielles Produkt oder eine Bestätigung von PeDePe/openOMSI.
