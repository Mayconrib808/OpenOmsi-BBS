# Validation and limits

## Exact released build (existing evidence, re-read for this audit)

Native Windows run [41157142308](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41157142308), job `native`, completed successfully. Its source archive SHA-256 and source tree are recorded in `provenance.json`.

| Check actually executed | Result |
| --- | --- |
| Windows release build of `omsi-app` and native `--version` | Passed |
| `ai_traffic::response::tests` | 8 passed |
| `people::inspector::patrol_tests` | 1 passed |
| `tolerant_stop_box_tests` | 2 passed |
| `one_entry_one_exit_with_separate_rear_leaf_can_open_for_alighting` | 1 passed |
| `omsi-app bcs_events::` | 14 passed |
| Standalone international inspector name test (also present in the 14 app tests) | 1 passed |
| Generated police/fire geometry axis and actual model loading | Passed |

There were 27 test executions here, including the repeated name test, not a fresh run of every workspace regression. The simulation commands filtered out hundreds of other tests. These focused tests do not cover all dialogue timers, every cabin topology, visual appearance or guest traffic density.

The Bridge packaging run 41159417197 also reported passing Bridge regressions, update/changelog Windows UI checks and unchanged complete hosts/P14. Those are packaging evidence, not additional native gameplay tests.

## Checks newly performed for this source delivery

- Downloaded the exact native artifact; verified `openomsi-source.tar.gz` against its recorded SHA-256.
- Reproduced the published source tree byte-for-byte, including Windows line endings in six files written by the layering script.
- Exposed that exact tree in a normal Git commit; the materialized CI-only commit is no longer the only reference to these sources.
- Partitioned the complete official-base delta into five file-disjoint patches, retaining binary patch records and licenses.
- Applied every patch in order to a temporary Git index at the official base and compared the final Git tree with the artifact tree.
- Verified every source file SHA-256 and blob identity; verified no file is omitted or assigned to two parts.
- Read the initial and follow-up reviews of upstream PRs #3427, #3428 and #3429. Findings and remaining work are in `REVIEW.md`.

Run `python3 REVIEW-2.2.2-HOTFIX2/verify.py` to repeat the extraction/integrity checks. `verification.json` records their result; it does not certify gameplay or performance.

## Still not established

No new Rust compilation was run locally for this documentation-only export; the recorded successful Windows build belongs to the exact code tree included here. No zero-bug, zero-allocation or zero-FPS-impact claim is made. Real-map busy stops, articulated/addon buses, long avenues with separated LAN peers, disabled-option RNG equivalence, VR/mobile visuals, GPU/VRAM growth, p95/p99 frame time and a complete two-PC BCS journey remain to be validated.
