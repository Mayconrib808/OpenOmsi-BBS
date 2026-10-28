# BCS visual incidents: first native implementation

Status: source implementation only. No new game executable or release is provided by
this commit. Rust/Cargo are unavailable in the current editing environment, so the
native compilation and Rust tests have not been executed. Do not describe these
scenes as validated in-game.

## Implemented

- An explicitly enabled local reader for the already validated capture protocol.
  Enable with `OPENOMSI_BCS_EVENTS_BETA=1`. It reads
  `%LOCALAPPDATA%/OpenOmsi-BBS/EventosBeta`, or `OPENOMSI_BCS_EVENTS_DIR`.
- Reads run on a worker every 250 ms with file-size and entry-count bounds. Startup
  preflight subdirectories and NDJSON histories are never replayed as current events.
  Compatible schema-1 snapshots must contain a matching PID and a real observation
  timestamp within 15 seconds (at most two seconds ahead). Two compatible source
  processes are treated as ambiguous, not assigned by recency.
- Event 4 selects one seated, local passenger in the player's bus. The selection stays
  with that person, never switching the illness to another passenger when they leave.
- A reversible torso/head slump blends in while the passenger is unwell. It supports
  original and procedural animation, preserving the seat, legs, passenger counters,
  tickets and BCS state. It is an initial seated collapse, not a floor fall or treatment
  animation.
- Scene stages distinguish the medical call (service 3), elapsed BCS delay and actual
  completion flags. No timer claims a vehicle has arrived.
- Scenes clear on stale/missing sources, leaving the bus, changing map/vehicle UID,
  entering multiplayer or borrowing another player's bus. This first implementation
  is disabled in LAN sessions pending an authoritative incident protocol.
- Plugins may listen to `bcs_incident(event_id, instance, stage, passenger)` for scene
  intent. This is an additive event, not a change to BCS's official plugin.

The floating camera and published umbrella-water fix are preserved in this branch's
baseline. HostAgent, CompanyHost, native release binaries and update feeds are untouched.

## Remaining before a playable visual beta

1. Compile the native integration and run its focused Rust tests. Validate the passenger
   pose in an actual bus with the original and enhanced animation modes.
2. Integrate an ambulance asset with the existing AI road network, emergency priority,
   collision checks and parking/stop control. No vehicle is spawned by this commit.
3. Implement arrival, local treatment/transfer presentation and departure. A BCS event
   completing before visual arrival needs an explicit presentation policy; do not
   change BCS's result, accounting or event scheduler to make animations fit.
4. Validate a real medical event/call/completion capture and connect multiplayer only
   after its authority, passenger identity and replicated vehicle lifecycle are defined.

Ticket inspectors, police scenes and fire scenes are outside this first slice.

## Focused tests supplied

`bcs_events::feed::tests` checks Java Instant timestamps, leap days, stale/future
observations and non-compatible sources. `bcs_events::lifecycle::tests` covers wrong
service calls, medical requests, deadlines versus completion, event 34 having no fainting
scene, repeated occurrences, bus changes, paused selection and source loss. They are
source tests, not executed test results in this environment.

When a Rust toolchain is available:

```sh
cargo check --locked -p omsi-app
cargo test --locked -p omsi-app --lib bcs_events::
```
