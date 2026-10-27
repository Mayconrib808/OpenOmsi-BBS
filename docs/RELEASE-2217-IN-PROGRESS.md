# Bridge 2.2.1.7 / openOMSI 0.3.45 — implementation and validation

Upstream target: dd515ba1d917bcb9c9b0d1ba24f6964b27f76ba0 (v0.3.45). Start from the complete published Coop15 source, retain all previous ports and apply the reproducible checked layer in build-support/coop15-0345. Do not substitute the stock executable.

## Implemented, pending full Windows release validation

- Researched hailing: direction fixed at gesture start relative to waiting pose, open relaxed hand, free-arm cone, smooth bounded head/body. Evidence and limits: build-support/passenger-hail-br/sources.md.
- Driver handover: suspend and hide the visitor's original vehicle outside the scene/physics lists; restore only on return. Original trip remains passive for BCS accounting. Authenticated authority announcement updates effective vehicle assignment, conductor routing and menus. Only current driver returns the wheel.
- Conductor: outside-panel mouse input passes to bus controls. Former owner can target its passive vehicle remotely. Delegated commands receive acknowledgements only after actual execution result, with receipts at both authorities. Ticket/payment desk context and cash actions travel to the passenger simulation authority; no second cash desk.
- Umbrellas: upward rays against real opaque and glass scenery triangles (thin roofs included); cached round-robin probes capped at eight per frame, real-time cover hysteresis, original close/carry/reopen animation. Camera collision keeps its own material and size policy.
- AI horn: preserve short complete recordings, sustain/release envelope for long recordings, optional recorded release, two 380 ms maximum presses separated by 160 ms, bounded normalization, priority, improved range and cabin low-pass. Verify perceptually with real buses before claiming acoustic realism.
- Weather: decode sky textures and generate cloud fields on one worker, retain previous sky until latest request completes; no synchronous cloud regeneration during runtime weather change. Official 0.3.45 also moves surface texture alpha analysis to preparation workers. Logs cannot attribute every stall to this work.
- Fuel: newest supplied BCS log already passed 291 litres / 97% through CLI and skipped automatic full-tank fill. Preserve original fuel priority and run actual Windows CLI fixtures again.
- Reconnect: ignore stale pre-WELCOME INFO until peer ID is assigned; retain post-bind identity rejection. Regression test passed with complete portable Bridge suite.
- Generic 32-bit plugin host chosen for non-BBS DLLs; BBS-only helper stays restricted.
- Setup: explicit verified Coop backup and restore, current-version allowlist, validation before writes and rollback. Portable Bridge regression suite passed, including stock-updater and tampering cases.
- Connections: game default 20%, slider 0–100% in Gameplay and lobby. 0 disables. Eligible people walk to a different nearby loaded stop with an available onward destination, prefer a stopped in-service bus, retain person/network ID and never reboard the bus they left. Up to two connections per person avoids indefinite circulation. Separate terminal metadata does not exist in old OMSI maps; eligibility uses stop layout and timetable facts, not station-name guesses. Dedicated simulation follows host settings.

## Research scope for connections

20% is a configurable initial gameplay parameter, not a measured national transfer rate. Counts of boarding events, discounted temporal integrations, metro-bus integrations and distinct people are different denominators. References: SPTrans, Relatório Integrado da Administração 2024, https://www.sptrans.com.br/relatorio-integrado-da-administracao-2024/ ; URBS, Com as novas integrações temporais, passageiro pode trocar de linha sem pagar nova passagem, https://www.urbs.curitiba.pr.gov.br/portal/com-as-novas-integracoes-temporais-passageiro-pode-trocar-de-linha-sem-pagar-nova-passagem/ . Both support transfers as ordinary travel, not one universal probability.

## Follow-up validation

All 12 arm/hailing/neck tests and the connection test pass locally. The arm IK transports its elbow plane with the reach direction and owns its smooth phase. This avoids an elbow-plane singularity during the transition. Driver-role menu actions are gated, current visiting drivers can authorize a conductor while standing, and dedicated status consumes authenticated sender-owned assignment metadata without granting simulation authority. Native type checks also pass. The checked follow-up source layer is reproduced and tested by Windows CI.

## Windows candidate verified

Native run: https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/40449138327 (source 9eb3f7ef92163dce9029886897bb215ccb83daef). All selected native suites passed: simulation including hailing/connections, network, LAN, launcher, driver/conductor, roof, audio and original 0.3.45 input tests. The actual game passed version/help/self-test and the generic 32-bit host was compiled.

Approved complete candidate: https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/40452388183 (source 2b03a649aaf19b51dfe919eb25dd53c02e84e0e8). Full portable Bridge tests passed on Windows, actual fuel CLI cases passed, actual rebuilt P15 HostAgent UI verified all six controls plus admin clock, and Setup GUI smoke verified backup controls. P14 and unrelated runtime assets were checked by SHA256. Default, managed host and cached company profile identify 0.3.45.

Installer SHA256: a8088e4c14ebc7d2b66bdb29038697061c9a7dc8ee66b650fe0cab74f5f0338c (24493056 bytes).
Coop archive SHA256: ed9f709adb88d57972ede3937c9228c3157ac1cea467d1a97266019e9b106c0f.

The original publisher key signed sequence 2026102701. Deployment is confirmed only after publish-2217 completes its independent signature, payload and live production checks. No actual paired-player gameplay or perceptual horn listening is claimed from these automated checks.
