# BBS+OpenOmsi Multiplayer 3.0.3-rc.1

Bring BCS 5.0.2.0 trips into openOMSI multiplayer, with automatic company/map sessions.

- Protocol 7, official unmodified 0.2.50 server, no fallback to old protocol 6.
- Private-bus adapter and an independent tunnel for every map, ported from the recent 2.1 corrections.
- Native BCS next-trip command and acknowledgment; no JAR patch.
- Restore selected player fuel, retain original Lua telemetry and dashboard content.
- Fresh BCS game.log capture and readiness after the real player bus exists.
- PT-BR/EN/DE Setup and offline guide, local diagnostics, saved preferences and verified 3.x updates.

Download the named **-teste.zip** Windows package under Assets, extract everything and run Setup.exe with BCS closed. Choose a company invitation and configure BCS to launch **OpenOMSI-BBS/PeDePeAdapter**.

This is a release candidate. Complete a real two-PC trip, evaluation and next-trip check before promoting the final WebDisk release. A VirusTotal report for the exact release hashes is not yet recorded. No zero-detection claim or vendor endorsement is made.

0.2.47 was not officially released. Use official clients 0.2.48+ with protocol 7, including 0.2.50. A company invitation routes maps; a direct Berlin server URL only joins Berlin.
