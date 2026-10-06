# Validation of the repository source import

Validated locally on **2026-10-06**, using a Linux x64 host and the verified **Go 1.23.2** toolchain.

| Check | Result |
| --- | --- |
| Portable tests | **49 top-level tests and 72 nested subcases passed** |
| Optional historical-helper removal test | **1 top-level test skipped** because the excluded historical binary was not supplied |
| `go vet` on the portable source/test lists | Passed |
| Windows x86 Setup, launcher and facade compilation | All three compiled successfully |
| Facade stripped/unstripped layout comparison | Passed; all three expected BCS RVAs fit within this program's writable backing |
| Runtime Go import comparison | Runtime Go files remain byte-for-byte unchanged from the v1.1.2 package; fixture-related test code was adapted |
| Included openOMSI reference and licence | Matched the exact upstream files at reference commit `538ad31b2a2c664cb0726db2547bf6238411eedf` |
| Repository content | No prebuilt executables, vendor binaries, game/DLC assets, captured account logs or real driver fixtures included |

This record does **not** report a real Windows session, UAC exercise, BCS panel/plugin connection, passenger test, remuneration comparison, vendor approval or malware certification. GitHub Actions is configured to run source checks on Linux and Windows; its results are separate from these local checks.

The source import is incomplete for runtime use because the native plugin host is excluded. See [helper provenance](HELPER_PROVENANCE.md) and [real Windows checks](TEST_ON_WINDOWS.md).
