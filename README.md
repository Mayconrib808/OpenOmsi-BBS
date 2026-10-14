<p align="center"><img src="docs/assets/multiplayer3-banner.svg" alt="BBS+OpenOmsi Multiplayer 3.0" width="100%"></p>

[Português](README.pt-BR.md) · [English](README.md) · [Deutsch](README.de.md) · [Website](https://mayconrib808.github.io/OpenOmsi-BBS/)

# BBS+OpenOmsi Multiplayer 3.0

**Use openOMSI multiplayer for your Bus Company Simulator trips.** BCS/BBS chooses the trip and receives its driving data. openOMSI runs the shared world, so drivers on the same map can encounter one another.

> **Release candidate: 3.0.3-rc.1.** This candidate targets BCS 5.0.2.0 and protocol-7 openOMSI. The bundled official server is 0.2.50; the minimum official client is 0.2.48. 0.2.47 was not officially released. A complete two-PC trip remains the final gameplay check.

**[Download the complete Windows ZIP](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v3.0.3-rc.1)** · [SHA-256 / source / release notes](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v3.0.3-rc.1)

## Quick start

1. Extract the complete ZIP to a permanent folder.
2. Close BCS, open Setup.exe and choose your language.
3. Confirm OMSI 2, create a company room or paste its invitation, and save.
4. Click Configure BCS. In BCS verify OpenOMSI (Beta) and OpenOMSI-BBS/PeDePeAdapter.
5. Start and finish trips through BCS. Use its native Next trip flow with the game open.

[Read the full player and host guide](docs/MULTIPLAYER_3.en.md)

## What changes in 3.0

- A company invitation routes each trip to its map session. The first driver on each map hosts automatically; joining peers share that world.
- Berlin and Carrão City get separate servers and tunnels. A direct Berlin server link only points to Berlin. Use the company invitation for map routing.
- Next trip uses the native BCS 5.0.2.0 plugin/command interface. Original vendor Lua is forwarded unchanged; no BCS JAR patch is applied.
- Unmodified official server, protocol-7 private-bus adapter, player fuel restoration, fresh BCS game.log and local diagnostic ZIP.
- Windows; PT-BR, English and German; solo switch, backups and verified Cloudflare 3.x updates.

A host PC must stay on and connected while peers use its map. Missing peer buses may use stand-ins. Map/mod dependencies remain the player’s responsibility.

## Source, validation and antivirus

Source is included and published. Builds use ordinary Go compilation without packers or obfuscation, with SHA-256 manifests and product/version metadata. The executable is unsigned; no zero-detection VirusTotal claim is made without a report for the final hash.

[Validation](docs/VALIDATION_3.md) · [2.1 → 3.0](docs/FEATURES_2_TO_3.md) · [Antivirus](docs/ANTIVIRUS_3.md) · [Build](docs/BUILD_3.md) · [Issues](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose)

Discord: **`.zmaycon.`**

Independent community project by Mayconrib808. Not an official PeDePe/openOMSI product or endorsement.

Source: [MIT](LICENSE). Third-party software retains its own license. The ZIP contains the integration source and official openOMSI server; OMSI 2, BCS, vendor plugins, maps, buses, credentials and publisher keys are not redistributed.
