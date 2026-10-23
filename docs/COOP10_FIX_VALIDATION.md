# Coop 10 — driver authority, avatars and walking LAN worlds

**Baseline: Bridge 2.2.0.3-hotfix.2 (commit 49e76d5). Status: source candidate only. Not released, not validated in two-player gameplay.**

## Confirmed reports / sources
- Coop 9 `0.3.38-coop.9`, Brigadeiro V2: the owner can only accept a driver request while seated; handing off changes to the passenger view without relocating the avatar correctly.
- Driver share is revoked within seconds; the requesting player may see the original driver seated and inherit a camera rather than occupy the driver's position.
- In F1 of another player's bus the original driver figure blocks the view.
- The selected human `.hum` appearance is inconsistent until the bus is removed or a participant reconnects.
- In player-hosted LAN, cars/pedestrians are streamed near players with vehicles; a player walking after removing the bus shows 0 local AI.
- BCS must remain attached to the original owner and no second bus may be spawned.

## Candidate source changes on this branch (NOT VERIFIED)
- `build-support/coop10/prepare.py` post-processes the pinned Coop 9 cumulative source.
- Owner relinquishes the driver seat through the actual on-foot state before the view changes. The bus and BCS stay with the owner.
- Pending driver requests remain authorizable through Cooperative when the owner is out of their seat.
- A local `LanInput.hide_driver_for` flag suppresses the *render* of the remote driver only in the chosen F1 camera. Other clients and exterior views remain unaffected.
- Remote selected human figures refresh despite temporary vehicle stand-ins, avoiding a character being locked to vehicle lifecycle.
- Player-hosted LAN world anchors include walkers, not only poses with a vehicle; host remains authoritative for cars and pedestrians.

## Incomplete / blocked from release
1. **Core priority:** physically place the approved visiting avatar in the driver's position on all clients; correctly animate the original owner standing/leaving, prevent overlap, and keep F1 tied to the vehicle camera.
2. Verify input messages actually drive the owned vehicle; transfer the destination display, doors and wheel commands; remove spurious revocations without weakening input-token, speed and timeout safety checks.
3. Validate full human-content discovery and permissioned download of missing .hum/texture/mesh dependencies into an isolated cache, without copying paid/private assets or replacing originals.
4. Simulate/stream traffic and passenger interaction around walking players, including a *walking host* and a conductor far from the owner bus; check dedicated-server behavior is preserved.
5. Add regression tests for entering/leaving a driver's seat, owner authorization from inside/outside, F1/F2/exterior cameras, late join, reconnect, skin changes, host rotation, LAN latency and BCS continuity.
6. Compile for Windows x64, pass native regressions and Defender, and observe a real two-PC player-hosted session before publishing through Bridge Buscar atualizações.

## Acceptance criteria for publication
- Grant with owner seated, standing in bus or walking outside; no teleport to rear and no duplicate driver.
- Exactly one controller/one bus/one BCS career; returning wheel works even if owner is standing, and disconnection applies safe brake.
- Visitor visually sits at the driver's actual position, owns controls, and can update route display; owner retains his own avatar.
- F1 hides the overlapping driver's figure **for the local camera only**, F2 and external views show correct figures.
- Character changes replicate immediately; late joiners receive selected figure, cache permitted assets or use a safe fallback.
- Cars and people load around *every walking player*, not only host/vehicle, with bounded server CPU/RAM/bandwidth and no duplicated AI.
- Existing ticket/change/turnstile features and protocol-14 sessions keep functioning.
