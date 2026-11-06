# Traffic improvements / Melhorias no trânsito

Street population weighted by physical lane length, separate population around LAN players, guest traffic activation, population diagnostics and retained emergency-priority and double-horn/light greeting support. Emergency dispatch itself is in part 01; network/menu glue is in part 00.

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full corrected developer-review source based on 2.2.2 Hotfix 2; this revision is later than the automatic-update installer. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

- [crates/omsi-app/src/traffic.rs](../../crates/omsi-app/src/traffic.rs)
- [crates/omsi-app/src/traffic/mirror.rs](../../crates/omsi-app/src/traffic/mirror.rs)
- [crates/omsi-app/src/traffic/population.rs](../../crates/omsi-app/src/traffic/population.rs)
- [crates/omsi-app/src/view_sync/traffic.rs](../../crates/omsi-app/src/view_sync/traffic.rs)
- [crates/omsi-app/src/view_sync/traffic_audio.rs](../../crates/omsi-app/src/view_sync/traffic_audio.rs)
- [crates/omsi-audio/src/greeting.rs](../../crates/omsi-audio/src/greeting.rs)
- [crates/omsi-sim/src/ai_traffic.rs](../../crates/omsi-sim/src/ai_traffic.rs)
- [crates/omsi-sim/src/ai_traffic/emergency.rs](../../crates/omsi-sim/src/ai_traffic/emergency.rs)
- [crates/omsi-sim/src/ai_traffic/greetings.rs](../../crates/omsi-sim/src/ai_traffic/greetings.rs)
- [crates/omsi-sim/src/ai_traffic/model.rs](../../crates/omsi-sim/src/ai_traffic/model.rs)
- [crates/omsi-sim/src/ai_traffic/tick.rs](../../crates/omsi-sim/src/ai_traffic/tick.rs)
