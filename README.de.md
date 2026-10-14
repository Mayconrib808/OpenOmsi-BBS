<p align="center"><img src="docs/assets/multiplayer3-banner.svg" alt="BBS+OpenOmsi Multiplayer 3.0" width="100%"></p>

[Português](README.pt-BR.md) · [English](README.md) · [Deutsch](README.de.md) · [Website](https://mayconrib808.github.io/OpenOmsi-BBS/)

# BBS+OpenOmsi Multiplayer 3.0

**Nutze den Multiplayer von openOMSI für deine Busbetrieb-Simulator-Fahrten.** BBS/BCS wählt die Fahrt und erhält die Fahrdaten. openOMSI simuliert die gemeinsame Welt, sodass Fahrer auf derselben Karte einander begegnen können.

> **Release-Kandidat: 3.0.3-rc.2.** Dieser Kandidat nutzt BBS 5.0.2.0 und openOMSI mit Protokoll 7. Enthalten ist der offizielle Server 0.2.50; offizieller Client ab 0.2.48. 0.2.47 wurde nicht offiziell veröffentlicht. Eine vollständige Fahrt mit zwei PCs ist noch die abschließende Spielprüfung.

**[Vollständige Windows-ZIP herunterladen](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v3.0.3-rc.2)** · [SHA-256 / source / release notes](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v3.0.3-rc.2)

## Schnellstart

1. Die gesamte ZIP in einen dauerhaften Ordner entpacken.
2. BBS schließen, Setup.exe öffnen und Sprache wählen.
3. OMSI 2 bestätigen, Firmenraum erstellen oder Einladung einfügen und speichern.
4. BCS konfigurieren wählen. Im BBS OpenOMSI (Beta) und OpenOMSI-BBS/PeDePeAdapter prüfen.
5. Fahrten im BBS starten und beenden. Nächste Fahrt nativ bei geöffnetem Spiel nutzen.

[Vollständige Anleitung für Spieler und Host](docs/MULTIPLAYER_3.de.md)

## Neu in 3.0

- Eine Firmeneinladung führt jede Fahrt zur passenden Kartensitzung. Der erste Fahrer einer Karte hostet automatisch; weitere Fahrer teilen diese Welt.
- Berlin und Carrão City erhalten getrennte Server und Tunnel. Ein direkter Berlin-Serverlink führt nur nach Berlin. Für Kartenrouting die Firmeneinladung nutzen.
- Nächste Fahrt nutzt die native Schnittstelle von BBS 5.0.2.0. Das ursprüngliche PeDePe-Lua wird unverändert weitergegeben; kein JAR-Patch.
- Unveränderter offizieller Server, Busadapter für Protokoll 7, Kraftstoffübernahme, neue BCS-game.log und lokale Diagnose-ZIP.
- Windows; PT-BR, Englisch und Deutsch; Einzelspieler, Backups und geprüfte Cloudflare-Updates 3.x.

Der Host-PC muss eingeschaltet und online bleiben, solange Mitspieler die Karte nutzen. Fehlende Busse können als Ersatzmodell erscheinen. Karten und Abhängigkeiten müssen installiert sein.

## Quellcode, Prüfung und Antivirus

Quellcode ist enthalten und veröffentlicht. Normale Go-Kompilierung ohne Packer oder Verschleierung, mit SHA-256-Manifesten und Produkt-/Versionsdaten. Die EXE hat keine Authenticode-Signatur; ohne Bericht zum endgültigen Hash wird keine Null-Erkennungs-Garantie abgegeben.

[Validation](docs/VALIDATION_3.md) · [2.1 → 3.0](docs/FEATURES_2_TO_3.md) · [Antivirus](docs/ANTIVIRUS_3.md) · [Build](docs/BUILD_3.md) · [Issues](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose)

Discord: **`.zmaycon.`**

Unabhängiges Community-Projekt von Mayconrib808. Kein offizielles Produkt, keine Partnerschaft oder Bestätigung von PeDePe/openOMSI.

Source: [MIT](LICENSE). Third-party software retains its own license. The ZIP contains the integration source and official openOMSI server; OMSI 2, BCS, vendor plugins, maps, buses, credentials and publisher keys are not redistributed.
