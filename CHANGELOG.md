# Changelog

## 2.0.0-dev.2

- Added an explicit company reference timezone and BCS time shift, with portable timezone data and automatic company calendar selection.
- Added CompanyHost.exe to launch an independently installed OpenOMSI 0.2.0 server with the company clock and maintain it through local administrative clock corrections.
- Live sessions validate the shared company clock independently of each player's departure. Fixed-date profiles and manual historical choices remain supported.
- Setup explains/retries invalid company IDs and supports `company` dates with one-time BCS shift configuration.
- Real multiplayer rendering, boarding and BCS evaluation remain pending user testing.

## 2.0.0-dev.1 — development

- Optional company profiles loaded from a local JSON file or administrator HTTPS URL.
- Automatic selection of a compatible registered session when starting a BBS trip.
- Required asset hashes and full-folder inventories, with administrator-provided download links for missing or different bus/map/repaint packages.
- Server map, clock, capacity, version/protocol and declared-fleet checks; joined-world date/time confirmation before BBS readiness.
- Setup options 8 and 9 for player configuration and company-profile creation.
- Multiplayer disabled by default; CI artifacts only while real two-player BBS validation is pending.

This file distinguishes the imported software version from subsequent repository preparation. Earlier full runtime ZIPs are not represented as cleared GitHub releases.

## v1.1.3 — 2026-10-06

- Published the complete runtime ZIP and checksum as an experimental GitHub prerelease, with fixed download links in English and Portuguese. Publication verifies that all four executable hashes match the live-tested package.
- Recorded the completed Carrão City line 2201 / tour 02 live trip: timetable alignment, real bbs.dll startup, six stops, eight tickets, evaluation transfer and normal openOMSI shutdown.

- Replaced the excluded historical helper with an attributed MIT Go Windows x86 host implementing the pinned openOMSI protocol/ABI and documented BCS-specific startup environment.
- Bundled the helper automatically; users need no extra helper download. Added recognised-copy updates with rollback and exact-byte removal while preserving unknown files.
- Added protocol, lifecycle, idle-message-pump and native Windows stdcall mock-DLL tests, plus complete package integrity/staging tests.
- Builds now produce all four executables, the complete ZIP, tutorial in three languages, source, notices, inventory and checksums.
- Updated runtime/log filenames to v1.1.3 and added downloadable CI build artifacts. Original products/assets remain excluded.
- Automated validation is distinct from live BCS panel, game-trip and online evaluation checks; no vendor authorisation is claimed.

## Repository preparation — 2026-10-06

- Imported the v1.1.2 Go implementation without changing its runtime Go files.
- Added English and Brazilian Portuguese project documentation, MIT licensing for original bridge material, upstream notices and individual acknowledgements.
- Documented the `bbs.odr` write, the Registry launch redirect, bridge-owned runtime files and the limits of integration testing.
- Excluded the old prebuilt native helper, all prebuilt executables, the BCS interface screenshot and real diagnostic/game data.
- Replaced driver/session fixtures with freshly authored synthetic examples and updated the corresponding expected test values.
- Removed the test suite's dependency on a bundled native helper: general checks use a synthetic PE header, while exact historical-hash removal is an explicitly optional local test.
- Added repeatable portable tests, vetting, Windows cross-compilation and facade layout checks, plus a Linux/Windows CI workflow.
- No complete installable GitHub release is supplied by this import.

## v1.1.2 — imported package, 2026-10-06

- Added German setup messages and acceptance of `J`/`Ja` confirmations.
- Clarified that BCS/BBS's "Start OMSI faster" setting must remain unchecked.
- Highlighted the reported unresolved Berlin-Spandau loading issue under openOMSI 0.2.0.
- Kept the route matching, timetable-source checks, diagnostics and compatibility logic from the preceding implementation.
- Included the existing Discord support contact in setup messages.

## Earlier implementation context

v1.1.1 prioritised the physical final stop over the displayed terminus during route matching and expanded local timetable diagnostics. v1.1.0 reorganised setup, rejected ambiguous departure/source selection and refined the compatibility startup lifecycle. These summaries describe historical implementation intent; they do not constitute new live validation of those packages or universal map support.
