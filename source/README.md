# Source layout

Use [scripts/build.py](../scripts/build.py) for explicit build entry points. The parent directory contains three main programs; `pluginhost/` is the fourth standalone program.

| Files | Role |
| --- | --- |
| `setup_main.go`, `setup_windows.go`, `setup_files.go`, `registry.go` | Setup, scoped activation/deactivation, integrity and diagnostics |
| `main.go`, `timetable.go`, `launch_checks.go` | Local trip selection, timetable/clock checks and simulator launch |
| `company.go`, `multiplayer.go` | Company profiles, asset inventories, server preflight, joining and startup guard |
| `company_setup.go`, `setup_ui.go` | One-time player binding and administrator profile wizard |
| `compat.go`, `facade_memory.go` | Own legacy compatibility windows and memory |
| `driver.go`, `session.go`, `process_windows.go` | Saved-counter translation and trip/process lifecycle |
| `plugin_host.go` | Stage/update the bundled source-backed helper, preserve unknown files, rollback/removal |
| `pluginhost/` | Windows x86 stdcall DLL host and openOMSI wire protocol; original mock DLL test source |
| `config.go`, `paths.go`, `version.go`, `diagnostics.go` | Config, paths, version/languages and local diagnostics |
| `*_test.go`, `testdata/` | Synthetic automated checks |
| `tools/release_tools.go` | Facade PE layout and hash tooling |
| `reference/` | Exact openOMSI MIT protocol library and notice |

Original code: [MIT](../LICENSE). Host protocol adaptation retains the [openOMSI notice](reference/OPENOMSI_LICENSE.txt). Preserve [third-party notices](../THIRD_PARTY_NOTICES.md). The bridge updates `Drivers/bbs.odr` and a scoped Registry filter; see [runtime effects](../docs/RUNTIME_EFFECTS.md). Vendor service rules remain separate from these source licences.
