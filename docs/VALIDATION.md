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

**Not verified by these checks:** real BCS `bbs.dll` panel/startup, a full OMSI/openOMSI trip, UAC interaction on a player's installation, passenger behaviour on real maps, online rule acceptance, complete telemetry or remuneration parity. Those require the legitimate installed products and [live Windows validation](TEST_ON_WINDOWS.md). No vendor approval or malware certification is claimed.
