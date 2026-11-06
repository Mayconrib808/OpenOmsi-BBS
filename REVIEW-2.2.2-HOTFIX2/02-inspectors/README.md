# Inspectors / Fiscais

An inspector already inside the bus, a reserved passenger seat, appearance selection and international name data, physical aisle patrol, pause, return to the reserved seat and bounded retry on disconnected cabin paths.

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full corrected developer-review source based on 2.2.2 Hotfix 2; this revision is later than the automatic-update installer. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

- [crates/omsi-app/src/bcs_events/inspector.rs](../../crates/omsi-app/src/bcs_events/inspector.rs)
- [crates/omsi-app/src/bcs_events/names-data/GFDL-1.2.txt](../../crates/omsi-app/src/bcs_events/names-data/GFDL-1.2.txt)
- [crates/omsi-app/src/bcs_events/names-data/NOTICE.md](../../crates/omsi-app/src/bcs_events/names-data/NOTICE.md)
- [crates/omsi-app/src/bcs_events/names-data/nam_dict.txt.gz](../../crates/omsi-app/src/bcs_events/names-data/nam_dict.txt.gz)
- [crates/omsi-app/src/bcs_events/names-data/names.tsv](../../crates/omsi-app/src/bcs_events/names-data/names.tsv)
- [crates/omsi-app/src/bcs_events/names.rs](../../crates/omsi-app/src/bcs_events/names.rs)
- [crates/omsi-sim/src/people/inspector.rs](../../crates/omsi-sim/src/people/inspector.rs)
