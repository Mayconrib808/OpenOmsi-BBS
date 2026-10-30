# BCS visual incidents: native medical response beta

The native integration passes `cargo check --locked -p omsi-app --lib` on Linux.
This branch does not yet provide a Windows executable or an in-game validated release.
The floating camera and published umbrella folding-water fix are preserved in the
baseline. HostAgent, CompanyHost, stable binaries and update feeds are untouched.

## Medical scene

Enable `OPENOMSI_BCS_EVENTS_BETA=1`. The reader uses
`%LOCALAPPDATA%/OpenOmsi-BBS/EventosBeta`, or `OPENOMSI_BCS_EVENTS_DIR`.
It reads the working capture agent's existing schema; replacing that agent is not
required. Reads occur on a worker every 250 ms, with size/count limits. Current
snapshots require matching filename/PID, schema 1, an observation no older than
15 seconds, and at most two seconds ahead. Preflight folders and NDJSON history
are not replayed. Multiple compatible source processes are treated as ambiguous.

Event 4 selects one seated, local passenger of the player's bus. A reversible
head/torso slump blends in without changing the seat or leg anchor. This is a
seated collapse; no floor fall, medic or stretcher animation is implemented.
While unwell, the selected local rider keeps their seat through ordinary
alighting stops and continues following the bus world transform. Resolution of the illness or loss of its source/context releases that hold so
the normal passenger routine can resume.

When service 3 (ambulance) is requested and the bus is stopped:

1. The controller finds an unobstructed, same-direction road stop 16–50 m behind
   the bus, near its ground level. The current target is 30 m behind the bus.
2. It searches for a connected road start 160–480 m away, hidden from the camera.
   The dispatched model uses the normal AI body, rendering, sound and road graph.
   It gets emergency priority per instance and retains normal collision checks.
3. Arrival requires the actual body within 1.5 m horizontally and 2 m vertically
   of the stop, moving under 0.3 m/s. It is checked again after physics, so an
   impact invalidates arrival. BCS delays do not establish arrival.
4. The patient stays unwell if BCS closes its popup before the ambulance arrives.
   Completion is latched only from the captured BCS completion flag. Transfer
   requires five consecutive seconds physically on scene, that completion flag,
   and the player's bus below 1 km/h.
5. Transfer removes this one native passenger through the existing seat/desk
   release and render-retirement paths. The native current onboard count therefore
   decreases by one. It does not write BCS counters, rewards, scheduler, account,
   ticket totals or results. No medic carrying the passenger is simulated.
6. The ambulance's stop is released and it drives away with normal traffic. It is
   retired only while hidden, rather than popping out beside the bus.

Without a valid model or clear connected route, dispatch retries every three
seconds. It does not fabricate arrival or remove the patient on a timer. Scenes
cancel after source loss, the selected passenger leaving their seated activity,
leaving/changing bus, map/vehicle UID change, moving
more than 8 m after dispatch, or 180 simulated seconds. A cancelled ambulance is
released to drive away; reused vehicle IDs on a new map cannot remove an
unrelated car. Sessions in LAN mode and borrowed buses are excluded until an
authoritative incident protocol exists.

The additive plugin event `bcs_incident(event_id, instance, stage, passenger)`
reports capture/controller intent. Actual dispatch, physical arrival and transfer
are logged separately; this event is not proof of physical arrival.

## Standalone visual test

The manual harness sets `OPENOMSI_BCS_EVENTS_VISUAL_TEST=1` only for its own
game process. It also disables plugins and starts with `--riders 4`. The game
accepts this hold only in a seeded standalone run, with no server, LAN host or
LAN join. Seeded seated passengers wait aboard before the medical scene starts,
including at an out-of-service stop; this avoids normal alighting emptying the
test bus before the tester presses the event button. Once a patient is selected,
the ordinary single-patient responder hold takes over. Normal BCS trips and LAN
sessions do not receive the test hold.

## Emergency models

The loader consumes `bcs-emergency-vehicles.json` in the original OMSI root.
Each role may explicitly map a relative `.ovh` or `.bus` file. Without a mapping:

| Role | Curated installed model |
| --- | --- |
| Ambulance | `Vehicles/MB_T1/ai_mb_t1_rtw.ovh` |
| Police | `Vehicles/MIPA/MIPA_AI_BMW_F31/MIPA_AI_BMW_F31_pol_active.ovh` |
| Fire | `Vehicles/RO_Roman_Diesel/Roman_apca.bus` |

See `emergency-models.json` for attribution and original archive hashes. The
original BMW and Roman archives were downloaded and their actual definitions,
scripts and sound entries inspected. The BMW is the selected police fallback;
only its dedicated dispatched instance receives the service role. An ordinary
traffic kind or a vague filename never classifies other cars as police.

Custom mappings can supply `en_route`, `on_scene` and `departing` maps of numeric
script variables. Curated BMW/Roman signal controls apply only to their exact
known paths and roles; explicit maps override those presets. Controls are fed
after the model script so an always-active script cannot restore priority/siren
on departure. Native visual/sound validation of the original models, including
the Roman beacon shutdown appearance, remains pending.

Example (copy only if you need to override the installed models):

```json
{
  "ambulance": {"model": "Vehicles/MB_T1/ai_mb_t1_rtw.ovh"},
  "police": {"model": "Vehicles/MIPA/MIPA_AI_BMW_F31/MIPA_AI_BMW_F31_pol_active.ovh"},
  "fire": {"model": "Vehicles/RO_Roman_Diesel/Roman_apca.bus"}
}
```

The source repository contains no third-party vehicle bytes. BMW files may be
staged unmodified with `scripts/stage-bcs-police.py`, preserving the author README
and provenance. The Roman package is supported as a separately installed package;
it is not redistributed here. A missing model leaves that service's visual
response unavailable, with a log message, rather than using a civilian car or
another emergency service. The dispatcher currently implements the medical
scene only; police, fire and ticket-inspector scenes are not complete.

## Focused validation

Fourteen focused tests passed locally on Linux with Rust 1.99.1 on 2026-10-28: application compilation
passed, all three physical response simulation tests passed, the medical seat-hold
regression passed (including normal alighting after cancellation), and all ten
`bcs_events::` tests passed. The local compiler required a single codegen unit;
the app test crate used optimization level 0. Incremental compilation was disabled
for the final application verification after a corrupted local compiler cache.
The original BMW staging helper
also copied and hash-verified 48 original files and preserved author attribution.
These results do not establish Windows or in-game compatibility.

```sh
cargo check --locked -p omsi-app --lib
cargo test --locked -p omsi-sim --lib ai_traffic::response::
cargo test --locked -p omsi-app --lib bcs_events::
```

Before a playable Windows beta, validate the medical capture/call/completion,
passenger pose in both animation modes and through alighting stops,
original ambulance mesh and siren,
spawn/stop placement on actual maps, and arrival/departure in game. A successful
headless simulation or compile does not validate these visuals.

### Short BCS medical calls (beta 0.2.2)

Responders now enter on a connected 10–18 m approach to a road stop roughly 14 m
behind the bus, already moving. The route is built backwards from the stop, so
a nearby origin cannot require a kilometre-long detour. Nearby creation can be
visible when a hidden origin is unavailable. Actual road clearance, braking and
physical arrival still apply. Generic end-of-trip cleanup preserves the response
route until arrival; previously it erased the route within 150 m and the response
controller stopped the car immediately. BCS completion adds no five-second hold.
A stalled approach records the AI stop reason and expires after 30 s.
