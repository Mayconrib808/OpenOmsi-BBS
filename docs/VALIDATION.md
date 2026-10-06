# Validation scope — v1.1.3

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

## Live Windows trip — 2026-10-06

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

## Release integrity

The four executable hashes in [the approved binary manifest](releases/v1.1.3-binaries.sha256) come from the package used for this live test. Publication takes the ZIP from a successful Linux/Windows Source checks run, verifies its external checksum, its full internal manifest and those four hashes, uploads both release assets as a draft, checks the uploaded assets and publishes an experimental prerelease. Documentation and packaging additions change the ZIP checksum while executable identity is preserved. Private repository visibility is retained.

Cross-platform comparison found only 39–40 changed bytes per Setup/launcher/facade binary, entirely inside Go's embedded build-ID string. Restoring the recorded ID made each whole-file SHA-256 identical to the live-tested executable; the plugin host already matched without changes. The build restores this metadata before package tests and rejects any remaining whole-file difference. Details are in [the build guide](BUILD.md).

No vendor approval or malware certification is claimed.
