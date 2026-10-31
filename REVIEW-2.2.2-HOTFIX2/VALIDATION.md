# Validation of the corrected developer-source revision

The tested native code commit is `44d2e919b505133be401869552c108b03ec48a70`, tree `43b7d5ffa5b2e3d4c7901399011f21658b6790c0`. [Windows build and regression run](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41169027280) builds the real `omsi-app` executable with the compiled translation catalog, invokes its `--version`, then runs the checks below before publishing the code branch. The source archive and executable SHA-256 are recorded in [revision-build.json](revision-build.json).

| Check | Scope |
| --- | --- |
| Official upstream locale validator | 19 new keys, 573 added translations, zero errors/warnings; no existing translations changed |
| Launcher executable schema tests | Seven real, unique toggles, names/help, restart notice and preserved transfer slider |
| `omsi-cfg optional_features::` | Independent off defaults, explicit false, disabled connections and BCS capture requirements |
| `omsi-sim people::` | Full passenger selection, including existing doors/AI-stop/inspector tests and new reaction identity/disabled RNG behavior |
| `omsi-launcher-core passenger_` | Passenger settings migration and persistence regressions |
| `omsi-app app_events::frame::people::dialog::tests` | Actual compiled native and launcher tables, every supported language/alias, 116 caption translations, distinct reactions |
| `omsi-app settings::` | Settings suite, 128 toggle combinations across both writers, old/partial configurations and all translated controls/help |
| `omsi-app traffic::population::optional_density_tests` | Baseline while disabled, zero/sparse roads and bounded extra scale |
| `omsi-app bcs_events::` | Existing capture, event/controller, lifecycle, names and visual-model regressions |

Commands, individual test names and counts are retained in the Actions job log and test evidence artifact. This table identifies the selections actually run; it does not imply every workspace test or real gameplay was run. Earlier trial runs exposed stale object-ID-based stop assertions. The final tests challenge selected-route priority for AI/player buses and nearest-stop fallback in both distance orderings; no production selection code was changed to satisfy those tests. The final Windows run must pass before publication.

## Delivery integrity

`verify.py` checks every source file against its SHA-256 and Git blob, checks patch hashes and file-disjoint coverage, applies all five patches to a temporary Git index at official 0.3.46, and compares the resulting tree to the **tested revised native tree**. [verification.json](verification.json) records the reconstruction. This protects against omitted files or inconsistent patches; compilation evidence is the separate Windows CI run.

The original released Hotfix 2 code/archive metadata remains in [provenance.json](provenance.json), including native run 41157142308, its tree/hash and original publication sequence 2026103003. This revision is not represented as the identical shipped tree and has not been published into Buscar atualizações. Bridge/HostAgent/CompanyHost/P14 were outside this native-source change.

## Gameplay and performance still pending

No interactive Windows trip, two-PC BCS journey, articulated/addon door matrix, full-bus caption stress test, FPS/p95/p99 measurement, VR/mobile appearance or GPU/VRAM profiling is claimed. The disabled fast paths and bounded calculations are implemented and covered where described; allocation-free active operation or zero performance impact is not established. See [REVIEW.md](REVIEW.md) for the remaining findings.
