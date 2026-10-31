# BCS events / Eventos do BCS

Capture reader, event controller, fainting and drunk/aggressive actor poses, oil/fire/bin visuals, ambulance/police/fire dispatch, host-authoritative response messages and original fallback asset generator.

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full corrected developer-review source based on 2.2.2 Hotfix 2; this revision is later than the automatic-update installer. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

- [ci/bcs-capture-reader-tests.rs](../../ci/bcs-capture-reader-tests.rs)
- [crates/omsi-app/src/bcs_events/assets.rs](../../crates/omsi-app/src/bcs_events/assets.rs)
- [crates/omsi-app/src/bcs_events/dispatch.rs](../../crates/omsi-app/src/bcs_events/dispatch.rs)
- [crates/omsi-app/src/bcs_events/effects.rs](../../crates/omsi-app/src/bcs_events/effects.rs)
- [crates/omsi-app/src/bcs_events/feed.rs](../../crates/omsi-app/src/bcs_events/feed.rs)
- [crates/omsi-app/src/bcs_events/lifecycle.rs](../../crates/omsi-app/src/bcs_events/lifecycle.rs)
- [crates/omsi-app/src/bcs_events/mod.rs](../../crates/omsi-app/src/bcs_events/mod.rs)
- [crates/omsi-app/src/bcs_events/network.rs](../../crates/omsi-app/src/bcs_events/network.rs)
- [crates/omsi-app/src/bcs_events/posture.rs](../../crates/omsi-app/src/bcs_events/posture.rs)
- [crates/omsi-app/src/traffic/responder.rs](../../crates/omsi-app/src/traffic/responder.rs)
- [crates/omsi-sim/examples/bcs_models.rs](../../crates/omsi-sim/examples/bcs_models.rs)
- [crates/omsi-sim/src/ai_traffic/response.rs](../../crates/omsi-sim/src/ai_traffic/response.rs)
- [docs/beta/bcs-incidents.md](../../docs/beta/bcs-incidents.md)
- [docs/beta/emergency-models.json](../../docs/beta/emergency-models.json)
- [scripts/stage-bcs-police.py](../../scripts/stage-bcs-police.py)
- [tools/generate_bcs_assets.py](../../tools/generate_bcs_assets.py)
