# BBS+OpenOmsi Multiplayer 3.0.4

Hotfix candidate for the 3.0 multiplayer line, targeting BCS 5.0.2.0 and protocol-7 openOMSI clients.

- Fixes upgrades that stopped with **“arquivo de auxiliar BCS diferente; preservado”**. The Bridge-owned `OpenOMSI_BCS_PluginHost32_v3.exe` can now be migrated even when its old hash was not pre-registered; abnormal paths such as directories/symlinks are still refused. An unknown regular previous file is backed up before replacement.
- Simplifies company-clock setup: the reference timezone is no longer exposed to users. Setup keeps the BCS reference clock internally and requires only **Company time shift (hours)**.
- The clock instructions now explicitly tell users to copy the exact **Time shift / Mudança de horário / Zeitverschiebung** number from BCS. For example, if BCS shows `-8 Hours`, enter `-8`; do not calculate the difference between the PC clock and the game clock.
- Keeps existing 3.x invitations, company settings and preferences. No clean reinstall is required for a normal 3.0.3 → 3.0.4 update.
- Protocol 7, official unmodified 0.2.50 server, native BCS next-trip command, fuel restoration, diagnostics and PT-BR/EN/DE support remain unchanged from 3.0.3.

Download the named **-teste.zip** Windows package under Assets for a clean test installation, or use **Atualizar-Multiplayer-3.0.4.exe** to update an existing 3.x candidate with BCS closed.

This remains a release candidate. Complete a real two-PC trip, evaluation and next-trip check before promoting the final WebDisk release. A VirusTotal report for the exact release hashes is not yet recorded. No zero-detection claim or vendor endorsement is made.

0.2.47 was not officially released. Use official clients 0.2.48+ with protocol 7, including 0.2.50. A company invitation routes maps; a direct Berlin server URL only joins Berlin.
