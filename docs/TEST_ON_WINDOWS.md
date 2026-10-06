# Windows integration validation

Portable tests and PE checks exercise the bridge's local logic and build layout. They are not a live integration test. A complete compatible runtime package with verified helper provenance and legitimate dependencies is necessary for the following checks.

| Area | Evidence to collect |
| --- | --- |
| Activation/deactivation | Scoped Registry redirect only; original executable remains unchanged; original launch restored after deactivation |
| Startup | Correct map, bus and repaint; plugin starts; panel appears; bridge does not create/delete BCS's marker |
| Timetable | Correct route endpoints and exact departure; passengers appear as expected for the real map duty; source conflicts produce a diagnostic rather than a wrong trip |
| Time | Real-time sync conflict is detected; known manual/automatic date behaviour matches documentation |
| Driver data | Starting snapshot retained; saved counts/ratings translated faithfully; repeated reads do not double count; external changes suspend file mirroring |
| Completion | F9 save settles before finishing; current shift completion is recognised; simulator/facade lifecycle ends as intended |
| Languages | Portuguese/English/German prompts, path entry, UAC handoff and confirmations |
| Diagnostics | Only intended local files collected; account data and paid content are reviewed before sharing |

Record exact software versions, map and trip details. Do not report passing these tests unless those checks were actually executed. The earlier package's statement about a successful older-version trip is not validation of this repository build. Online acceptance and complete telemetry cannot be established by unit tests alone.
