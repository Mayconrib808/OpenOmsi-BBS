# BBS+OpenOmsi Multiplayer 3.0.3-rc.1

The public candidate uses the unmodified official openOMSI 0.2.50 server and protocol-7 gateway/tunnel behavior ported from 2.1.8. Player fuel restoration is ported from 2.1.7. BCS 5.0.2.0 next-trip commands and acknowledgments are native; no vendor JAR patch is used.

Build: `python scripts/build_preview.py --server server --output dist --sequence 2026101401`.
Tests: `cd source && go test -race ./... && go vet ./...`; `node --test relay/test/*.test.js`.

The audit script accepts the user's locally installed official BCS JAR and optionally simulates its original Lua. The JAR and Lua are never included in the candidate.

Full two-PC gameplay and the VirusTotal report on the exact final hashes remain release qualification. Consult the project validation record before calling a final release fully tested.
