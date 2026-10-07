# Validation scope — 2.0.1

This page distinguishes the observed OpenOMSI 0.2.0 trips from checks of the 2.0.1 bridge. A successful automated check establishes only the behavior exercised by that check. The current source checks and artifacts are listed in [GitHub Actions](https://github.com/Mayconrib808/OpenOmsi-BBS/actions).

## Observed company-multiplayer trip — 2026-10-07

Mayconrib808 supplied dedicated-server logs and a BCS completion screenshot during the 2.0 development test preceding this release. The OpenOMSI client/server used for that test were 0.2.0; it is not a live-game test of 0.2.11 or the new 2.0.1 graphical Setup.

| Item | Observed result |
| --- | --- |
| Map and bus | SP_Carrao City; Caio Apache Vip I OF 1721 REMAKE by Victor, Manual, EMTU repaint |
| Company/session | Transfort - BR; Transfort - Carrao City |
| Joining | Server identified MayconRib808, the bus, its position and the N407 destination |
| Timetable | N407 tour 03 assigned to the LAN player; the corresponding AI bus was removed from the road |
| Chat | Server received the player's message “Opa” |
| Shared passengers | Server recorded waiting passengers boarding the player's bus |
| Clock | CompanyHost reported synchronization on 2026-10-07; subsequent corrections continued |
| Connection | A timeout at 06:39:27 UTC was followed by reconnection at 06:39:55 and a WebSocket connection; sustained two-client reliability has not been established |
| Evaluation | BCS displayed successful N407 completion and 67%: 855/1280 points |
| Score details | Punctuality 320, safe driving 0, distance/tickets 390, passenger satisfaction 145; evaluation parity with original OMSI has not been established |
| Exit | The player deliberately chose Return to office; the server recorded a normal leave at 07:05:58 UTC and resumed the tour's later AI departures |

This confirms joining and integration for the observed session with one player. The completion screenshot does **not** test BCS's Next trip button: the player explicitly selected Return to office.

Real account logs, profiles, identifiers, proprietary timetables and screenshots are not distributed as test fixtures.

## 2.0.1 compatibility boundaries

| Area | Scope and remaining check |
| --- | --- |
| OpenOMSI versions | Selection is based on required executable capabilities and plugin interface, rather than an exact release allowlist |
| Official 0.2.11 | Its pinned source exposes protocol 6 and the same native plugin wire interface; a live BCS trip with that version is still pending |
| Multiplayer transport | Server preflight requires compatible protocol and valid map, fleet, player-limit and clock data; the actual game handshake can still reject a protocol mismatch |
| Persistent configuration | Regression checks cover user settings, imported company profiles and recovery across package updates; a fresh user-facing Windows setup still needs a live walkthrough |
| Night trips | Regression checks cover equivalent service-day times such as 01:40 and 25:40, exact trip matching and ambiguity refusal |
| Next trip | State/transition regression checks cannot establish that the real BCS button works; a consecutive live trip is required |
| Traffic penalties | Actual saved collision/driving counters are translated; red-light-specific penalties remain unavailable because the necessary authoritative telemetry is absent. See [traffic penalties](TRAFFIC_PENALTIES.md) |
| Two players | Chat, mutual bus visibility, movement, disconnect/reconnect and passenger boarding must be tested with two independent clients |
| Profile sharing | An accessible administrator-provided profile is required; local loopback addresses do not connect players on other computers |

Keep the company host running throughout the two-player test. Each player must use the administrator's declared content. Passing version/protocol preflight does not certify every map, bus, feature or BCS evaluation.

## Historical 1.1.3 source checks

Prepared on **2026-10-06** with Go 1.23.2. The build checks are reproducible through `scripts/build.py`; the exact GitHub run status is available in [Actions](https://github.com/Mayconrib808/OpenOmsi-BBS/actions).

| Check | Evidence/scope |
| --- | --- |
| Bridge tests | Synthetic driver/session data, timetable matching, ownership, rollback, diagnostic boundaries and helper upgrades |
| Host protocol tests | Fixed bytes following the pinned Rust format, truncated requests, state ordering, finalization and idle message-pump progress |
| Go vet | Bridge and host, including Windows x86 host source |
| Windows build | All four executables built from included source; no historical/vendor binary included |
| Facade layout | Stripped/unstripped sections match; all three expected BCS RVAs fit writable backing `[0x1C19E0,0x5C19E0)` |
| Full package | Inventory and SHA-256 manifest verified by actual Go package verifier; built helper activates/deactivates in temporary directories and memory Registry |
| Native Windows DLL tests | Actual PE32 host with original synthetic stdcall DLL; tests root/Steam process environment, flags, float/string/trigger ABI, Unicode, locked thread, Windows messages, finalization, EOF cleanup and non-BCS rejection |
| Attribution/content | Original MIT notices, exact openOMSI reference, Go notices and credit inventory included; no proprietary assets or real account fixtures |

Linux local builds run all checks except native Windows DLL execution. The Windows CI job executes those native checks before attaching a runtime ZIP. Optional exact historical-binary tests may be skipped when that excluded local input is absent.

## Historical live Windows trip — 2026-10-06

Mayconrib808 supplied a v1.1.3 diagnostic after a completed live trip. The evidence was reviewed locally; real account logs, driver profiles, account identifiers, proprietary timetable files and screenshots are excluded from distribution.

| Item | Observed result |
| --- | --- |
| Products | openOMSI 0.2.0 Windows x64; original locally installed BCS/BBS plugin |
| Map and bus | Carrão City; Caio Apache VIP I OF 1721 manual |
| Trip selection | Line 2201, tour 02, 09:20–09:30, Divisa de Ferraz → CPTM Guaianazes; matched 2201rota2 |
| Clock/timetable | Exact 09:20 match, already aligned, zero timetable offset; real-time synchronisation disabled |
| Startup | BCS acknowledged the start menu, openOMSI signalled ready, and the real bbs.dll loaded |
| Saved trip | Six stops, two early and none late; eight tickets for 25.90; openOMSI reported 1.86 km |
| Evaluation transfer | BCS evaluation payload matched the bridge-saved driver state; no new collision/injury counts in this trip |
| Completion | BCS confirmed the shift; the bridge requested graceful closure and openOMSI exited normally |

This establishes the main integration for **that session/map/bus combination**. It does not establish universal first-launch panel reliability, every map/date/Chrono/route, all UAC installations, online rule acceptance, complete telemetry, evaluation equivalence or remuneration parity. Low comfort/driving scores originated in the openOMSI session; their equivalence to original OMSI has not been established. Berlin-Spandau remains an independently documented map-loading limitation. Further checks are listed in [the Windows checklist](TEST_ON_WINDOWS.md).

## Historical 1.1.3 release integrity

The four executable hashes in [the approved binary manifest](releases/v1.1.3-binaries.sha256) come from the package used for this live test. Publication takes the ZIP from a successful Linux/Windows Source checks run, verifies its external checksum, its full internal manifest and those four hashes, uploads both release assets as a draft, checks the uploaded assets and publishes an experimental prerelease. Documentation and packaging additions change the ZIP checksum while executable identity is preserved. Private repository visibility is retained.

Cross-platform comparison found only 39–40 changed bytes per Setup/launcher/facade binary, entirely inside Go's embedded build-ID string. Restoring the recorded ID made each whole-file SHA-256 identical to the live-tested executable; the plugin host already matched without changes. The build restores this metadata before package tests and rejects any remaining whole-file difference. Details are in [the build guide](BUILD.md).

No vendor approval or malware certification is claimed.

## Verificações da 2.0.1

Em 2026-10-07, [Source checks #37598136666](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/37598136666) passou no Linux e no Windows para o commit `8977f5309e4ea73d517a516e337fd549f4dded3e`. Incluiu os testes de horários operacionais (linha 522, 01:40/25:40, offset zero), repetição de Schicht ID, transição de viagem, persistência de perfil, exportação HTTPS, criação nativa dos controles do Setup, DLL/message pump do auxiliar e integridade do pacote.

O probe real da ponte aceitou os quatro ZIPs oficiais com SHA-256 verificado: OpenOMSI 0.2.0 e 0.2.11, cliente e servidor Windows x64. Isso confirma os pré-requisitos de CLI/host; não carregou mapas nem executou viagens BCS nessas versões. Os cinco executáveis aprovados estão registrados em `v2.0.1-binaries.sha256`. O teste real de dois jogadores, o botão Próxima viagem e uma viagem completa com 0.2.11 continuam pendentes.
