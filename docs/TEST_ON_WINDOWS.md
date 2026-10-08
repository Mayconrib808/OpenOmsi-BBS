# Test the 2.0.3 dev1 changes

1. Extract the complete new ZIP and use its Setup to save/activate with the games closed. Keep BCS “Start OMSI faster” unchecked.
2. Repeat startup on a fast and slow map. Check that the BCS panel appears. Collect logs on a failure; compare the start-menu acknowledgment, native readiness and timetable transition. A hidden facade starting before the game is intentional; it supports early BCS probes.
3. Save at the terminal with F9, wait two seconds, complete in BCS, choose Next trip and select a fresh trip within 15 minutes. Repeat with selection after the old client has exited. Verify the new bus/map/line/direction/departure and that the completed evaluation stays intact.
4. Start a BCS trip with rain and compare precipitation, clouds and road wetness. Repeat with snow. A fresh map/date-matched `laststn.osn.owt` is required. The bridge logs an unavailable/old snapshot instead of assuming a forecast.
5. For multiplayer, update the host agent and deploy the updated relay. Start a closed map with rainy BCS weather, then join a second player. Both should see the shared host weather. A later client's sunny selection must not change the occupied world. After idle shutdown, a new wake can choose new weather. Manual servers retain their configured weather.

These checks require the separately installed BCS/OpenOMSI products. The automated Windows facade and synthetic-DLL tests do not replace them.

# Windows integration validation

Portable tests and PE checks exercise the bridge's local logic and build layout. They are not a live integration test. The v1.1.3 package includes the source-backed helper. The separately installed legitimate products are necessary for the following live checks. Native mock-DLL CI checks are documented in VALIDATION.md and do not establish these game results.

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
