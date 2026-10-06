# Changelog

This file distinguishes the imported software version from subsequent repository preparation. Earlier full runtime ZIPs are not represented as cleared GitHub releases.

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
