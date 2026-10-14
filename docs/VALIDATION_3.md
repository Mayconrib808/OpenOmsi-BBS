# 3.0.3-rc.3 — validation record

Date: 14 October 2026. This record is about the new multiplayer integration, not a claim that every map, bus or graphics configuration has been played.

| Check | Evidence / status |
| --- | --- |
| Official BCS 5.0.2.0 JAR | SHA-256 and native launch/command contract passed; [audit receipt](releases/v3.0.3-rc.3-bcs-audit.json). Original file unchanged. |
| Original BCS Lua plugin | Executed against a mocked simulator API: stale command ignored, vehicle required, pause avoided, command once, next trip selected, 62-field telemetry and acknowledgment passed. |
| Source | Go tests with race detector and vet; Windows amd64/386 cross-compilation. Windows execution passed in GitHub Actions, including the old-preview upgrade regression. |
| Multiplayer transports | UDP and WebSocket, fragmentation/ping, private bus names, opaque binary packets, protocol-7 challenges/reconnect fields and player ID ownership checked. |
| Map routing | Company/map leases tested concurrently; separate maps have separate ownership, listeners and tunnels. No cross-map world sharing. |
| BCS launch | Real child-process tests cover missing/invalid inherited output, direct fresh game.log capture, original Lua forwarding and readiness only after a vehicle exists. |
| Fuel | Player selection with AI vehicles present, UTF-16, decimal comma, zero fuel, missing values and unrelated startup assignments checked. |
| Setup | Chromium smoke test passed in PT-BR, EN and DE; manual update checks and saved automatic preferences checked without a real company. |
| Diagnostics | Only owned bounded logs are included; invitations, addresses and known secrets are removed. Review remains necessary before sharing. |
| Runtime provenance | Bundled official openOMSI 0.2.50 server ZIP and each file match upstream SHA-256; no binary patch. Cloudflare tunnel release is pinned to its official digest. |
| Update trust | Existing 3.x signing key recovered and matched to the embedded public key. Trust, tampering rejection, transactional replacement and configuration preservation checked. |
| Live two-PC BCS trip | **Pending on the candidate:** joining, seeing peers, completion/evaluation, next trip/table/display, host exit with peer remaining, Berlin and Carrão City. |
| Antivirus | Binaries have source and hashes, ordinary Go compilation and product metadata. No executable packer or obfuscator. Authenticode is unsigned. The first runner had RPC error 0x800106ba. The prior rc.2 run scanned its ZIP and every packaged EXE/DLL successfully. The rc.3 workflow additionally scans the standalone updater; its [exact release report](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v3.0.3-rc.3/DEFENDER-REPORT.json) is published with the candidate. This is a Defender result on that runner. A VirusTotal report for the final exact hashes is still required before making a zero-detection claim. |

## Release qualification

Use the release candidate for those live checks. Record the precise integration/client/server versions, BCS version, map, bus/HOF and result. Promote a final release after confirming the playable flow and investigating any scanner detections on the exact release bytes.

An automated test of packet forwarding is not a rendered two-player game. The BCS plugin simulation is not a BCS account/login/payment test. No vendor binary, credentials or account data are shipped as test fixtures.
