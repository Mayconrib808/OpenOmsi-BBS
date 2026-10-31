# Passengers and speech / Passageiros e falas

Closed-door requests, missed-stop reactions, optional cosmetic 22% exit thanks, real exit detection, stop tolerance and path recovery. Cumulative hailing, umbrella render support and onward connections are included where they share these complete source files; they are not all new Hotfix 2 features.

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full corrected developer-review source based on 2.2.2 Hotfix 2; this revision is later than the automatic-update installer. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

- [crates/omsi-app/src/app_events/frame/people.rs](../../crates/omsi-app/src/app_events/frame/people.rs)
- [crates/omsi-app/src/app_events/frame/people/dialog.rs](../../crates/omsi-app/src/app_events/frame/people/dialog.rs)
- [crates/omsi-app/src/app_events/frame/people/dialog_tests.rs](../../crates/omsi-app/src/app_events/frame/people/dialog_tests.rs)
- [crates/omsi-app/src/view_sync/people.rs](../../crates/omsi-app/src/view_sync/people.rs)
- [crates/omsi-app/src/view_sync/people/animation.rs](../../crates/omsi-app/src/view_sync/people/animation.rs)
- [crates/omsi-app/src/view_sync/people/extras.rs](../../crates/omsi-app/src/view_sync/people/extras.rs)
- [crates/omsi-app/src/view_sync/people/extras/geometry.rs](../../crates/omsi-app/src/view_sync/people/extras/geometry.rs)
- [crates/omsi-app/src/view_sync/people/extras/water.rs](../../crates/omsi-app/src/view_sync/people/extras/water.rs)
- [crates/omsi-launcher-core/src/passenger_animation_tests.rs](../../crates/omsi-launcher-core/src/passenger_animation_tests.rs)
- [crates/omsi-launcher-core/src/passenger_defaults.rs](../../crates/omsi-launcher-core/src/passenger_defaults.rs)
- [crates/omsi-sim/src/human/bones.rs](../../crates/omsi-sim/src/human/bones.rs)
- [crates/omsi-sim/src/human/gesture.rs](../../crates/omsi-sim/src/human/gesture.rs)
- [crates/omsi-sim/src/human/hail_look.rs](../../crates/omsi-sim/src/human/hail_look.rs)
- [crates/omsi-sim/src/human/hail_motion.rs](../../crates/omsi-sim/src/human/hail_motion.rs)
- [crates/omsi-sim/src/human/mod.rs](../../crates/omsi-sim/src/human/mod.rs)
- [crates/omsi-sim/src/people.rs](../../crates/omsi-sim/src/people.rs)
- [crates/omsi-sim/src/people/avatar.rs](../../crates/omsi-sim/src/people/avatar.rs)
- [crates/omsi-sim/src/people/bodies.rs](../../crates/omsi-sim/src/people/bodies.rs)
- [crates/omsi-sim/src/people/buses.rs](../../crates/omsi-sim/src/people/buses.rs)
- [crates/omsi-sim/src/people/dialog.rs](../../crates/omsi-sim/src/people/dialog.rs)
- [crates/omsi-sim/src/people/mirror.rs](../../crates/omsi-sim/src/people/mirror.rs)
- [crates/omsi-sim/src/people/model.rs](../../crates/omsi-sim/src/people/model.rs)
- [crates/omsi-sim/src/people/money.rs](../../crates/omsi-sim/src/people/money.rs)
- [crates/omsi-sim/src/people/pax.rs](../../crates/omsi-sim/src/people/pax.rs)
- [crates/omsi-sim/src/people/pax/tests.rs](../../crates/omsi-sim/src/people/pax/tests.rs)
- [crates/omsi-sim/src/people/pax_stops.rs](../../crates/omsi-sim/src/people/pax_stops.rs)
- [crates/omsi-sim/src/people/pax_task.rs](../../crates/omsi-sim/src/people/pax_task.rs)
- [crates/omsi-sim/src/people/pax_tick.rs](../../crates/omsi-sim/src/people/pax_tick.rs)
- [crates/omsi-sim/src/people/reaction_options_tests.rs](../../crates/omsi-sim/src/people/reaction_options_tests.rs)
- [crates/omsi-sim/src/people/reactions.rs](../../crates/omsi-sim/src/people/reactions.rs)
- [crates/omsi-sim/src/people/spawn.rs](../../crates/omsi-sim/src/people/spawn.rs)
- [crates/omsi-sim/src/people/tests.rs](../../crates/omsi-sim/src/people/tests.rs)
- [crates/omsi-sim/src/view_events.rs](../../crates/omsi-sim/src/view_events.rs)
- [launcher/ui/js/pages/passenger-settings.js](../../launcher/ui/js/pages/passenger-settings.js)
