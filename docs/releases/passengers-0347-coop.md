# Selected 0.3.47 passenger integration for Bridge 2.2.2.3-test.1

Origin: openOMSI-org/openOMSI tag v0.3.47, commit
8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6; merged PRs #3427/#3428/#3429.
The native Coop base remains 0.3.46. This is a scoped integration, not a claim
that all unrelated 0.3.47 engine, networking or dependency changes were merged.

The original gesture, wrist, gaze, full-bus reaction gate, capacity snapshot,
visual bus matching, connection choice and loaded-ground/collision walk check
replace the earlier custom implementations. Their original regression tests and
MIT attribution are retained. Umbrellas keep their separate existing rendering;
the upstream hail uses the other arm only when the umbrella already occupies one.
Winter clothes rendering and code are removed. The abandoned setting is inert,
even if an old settings.cfg saved it enabled. No winter assets are required.

First read of settings without passenger_features_0347=1 enables hailing,
full-bus reaction, onward connections and Natural animation, on every map.
Both writers save that marker; subsequent explicit opt-outs survive restart.
The connection percentage remains the user's bounded value, default 20%.
Other optional features remain independently off by default.

The upstream single-player-only application gate is replaced with the existing
Coop ownership adapter. Only the owner simulates alighting/choice/walking; a
mirror never draws connection randomness. A client requires a host advertising
return/ack support. It retains ownership while walking and waiting until ACK;
the host validates the mapped stop, exact onward service and observed former
rider, adopts once, and retries acknowledgements for repeated frames.
The optional i1 descriptor suffix carries lifetime transfer count and previous
bus. Client grant restores that history, host adoption keeps count 1 or 2 rather
than resetting to 1. Missing legacy history conservatively consumes the limit;
invalid values are rejected. The first four descriptor fields and P15 packet
protocol are unchanged. Matching new client and host are required for the full
feature. P14/complete HostAgent/CompanyHost are not rebuilt.

Validation includes upstream synthetic gesture/capacity/0-20-100%/identity/
service/ground tests, retained Coop authority/handback regressions, descriptor
compatibility, native/launcher migration and restart, and actual launcher schema.
Rendered appearance, real-map walking and paired multiplayer gameplay still
require manual validation. Compilation success is not visual approval.
