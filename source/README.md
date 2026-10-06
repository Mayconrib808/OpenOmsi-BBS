# Source layout

The original runtime Go files were imported from bridge v1.1.2. Build entry points are separated because the directory contains three main functions; use [the build script](../scripts/build.py), not `go build .`.

| Files | Role |
| --- | --- |
| `setup_main.go`, `setup_windows.go`, `setup_files.go`, `registry.go` | Setup UI, scoped Registry activation/deactivation and package/diagnostic operations |
| `main.go`, `timetable.go`, `launch_checks.go` | Read local trip data, identify timetable sources, validate clock/source constraints and launch the separately installed simulator |
| `compat.go`, `facade_memory.go` | Legacy compatibility windows and this process's own memory layout |
| `driver.go` | Translate saved local driver data and mirror it with stability/conflict checks |
| `session.go`, `process_windows.go` | Current-shift completion and simulator/facade lifecycle |
| `plugin_host.go` | Deploy/identify an external compatible native helper; no helper binary is included here |
| `config.go`, `paths.go`, `version.go`, `diagnostics.go` | Configuration, paths, version/language messages and local diagnostic selection |
| `*_test.go`, `testdata/` | Automated checks and synthetic examples |
| `tools/release_tools.go` | PE layout comparison and manifest generation utility |
| `reference/` | Exact attributed openOMSI MIT protocol reference; not the missing custom helper source |

The root [MIT notice](../LICENSE) covers original bridge code. Preserve [third-party notices](../THIRD_PARTY_NOTICES.md). BCS/OMSI service rights and product licences remain separate. The interface is a local compatibility implementation, not claimed vendor-approved integration.

The driver mirror writes `Drivers/bbs.odr` and the activation writes a scoped Registry filter. See [runtime effects](../docs/RUNTIME_EFFECTS.md) before attempting real use. See [helper provenance](../docs/HELPER_PROVENANCE.md) for the unresolved runtime dependency.
