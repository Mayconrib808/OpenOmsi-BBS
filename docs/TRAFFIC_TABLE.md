# The traffic table

Every AI vehicle of the host's world is one row of a table, under an id of its own. The
traffic simulation writes the table after each of its steps; what goes over the LAN to the
clients is read from it, and a client holds its own copy and draws its cars from that. The code is
`crates/omsi-net/src/traffic_table/` (no game, no GPU: a dedicated server uses it as it is),
the host's writer is `omsi-app/src/traffic/table.rs`, the client's side of the frame is
`omsi-app/src/lan_world.rs`. LAN protocol 15.

## What a row holds

| Part | Contents | Changes |
| --- | --- | --- |
| id | `TrafficId`: slot (12 bits) and generation (8 bits, 1..255) | never for a life |
| kind | car, bus, rail, motorcycle, truck, other | never |
| look | vehicle file (relative to a content root), repaint index and name; shared by rows, told once per client | never |
| meta | line and destination of a timetable bus (`#<row of the depot file>`) | when the bus is given a new trip |
| row | position (cm, world coordinates), heading (360/4096 deg), pitch, bank (0.1 deg), speed (0.05 m/s), front wheel angle (0.5 deg), indicators, brake lights, headlights, stop state (boarding / leaving) | every step |

The row is held in the units it travels in (`Row`), so two rows are equal exactly when they
would be drawn the same and the host's row is bit for bit what a client decodes. Wheels,
doors, lamps and the sounds are the client's own scripts' work from these numbers, as before.

An id is given by the table (`IdPool`). A slot a vehicle leaves rests for 30 s
(`REUSE_DELAY`) and comes back with the next generation: a delayed datagram or a client that
has not heard of the vehicle's end can never take the next vehicle in that slot for the old
one. Table full (4096 rows): the vehicle is simply not told. The simulation's own car number
stays the simulation's (`TrafficTable::id_of(sim_id)`); where another channel names a car (its
script context, its sounds, the riders of a timetable bus in `WORLD`) it names it by the
table id.

## Who reads what

* The simulation writes (`Traffic::write_table`, called from `Traffic::after_step`; a client's
  mirror does not write). Collisions with the player, the AI's decisions and the timetable
  buses' passengers stay the host's, from the simulation's own state: nothing reads the table
  back into the simulation.
* The host's clients are told from the table (`ClientSync`).
* A client's `ClientTable` is what its mirror cars are made and posed from (`lan_world`:
  `add_and_remove_cars`, `place_cars`).
* The game's own pictures and sounds on the host (and in single player) still read the
  simulation's vehicles: they are the vehicle instances whose scripts the table's numbers
  are taken from, and drawing them from a second copy would double their scripts. The wire
  and the client's drawing are tested without a game by `Loopback` (both ends in one
  process through a lossy, late, reordering network: `traffic_table/tests.rs`).

## The wire

Two datagrams, both bit-packed like the other binary ones (`wire.rs` has the layout).

* `TRAF` (0xBA), host to client, ten snapshots a second: new looks, spawns (id, look, kind
  and the whole row), line and destination changes, removals, and row updates. A snapshot is
  cut into datagrams of at most 1180 bytes (below the MTU of the VPNs), each of its own: a
  whole row (110 bits, centimetres from an anchor near the client: 21 bits reach 5 km), or a
  delta against a row the client acknowledged (a few bits when nothing changed, about 6-7
  bytes a moving car).
* `TACK` (0xBB), client to host: the newest datagram heard and which of the 32 before it, and up
  to 8 troubles (a row it cannot use as a delta, a vehicle it does not know).

### Acknowledged state, not repeated state

What a datagram carried is the client's from the acknowledgement on. So

* a standing vehicle costs nothing once acknowledged (a sign of life every 8 s);
* what is not acknowledged (the datagram was lost, or its answer) is told again with the
  row's state of that moment, deltas always against the last row acknowledged - a lost
  datagram never makes a later one unreadable;
* spawns, looks, line changes and removals are told every 0.35 s until acknowledged. A removal
  carries the snapshot it was made in and a spawn the one it was made in: a late copy of an old
  spawn after a removal (or of an old removal after a new spawn) is ignored;
* a client that loses a row (or never had it) says so in its next answer and the host starts
  that vehicle over; a host that starts a client's table anew changes the epoch byte, and
  the client starts over too.

### Range, priority, budget

A client is told of the cars within 650 m of its vehicle (they leave at 700 m, so none comes
and goes on the edge) and, per snapshot, of those due: within 200 m ten times a second,
within 400 m five, beyond three. A budget of 40 000 bytes a second per client
(`TRAFFIC_BYTES_PER_SECOND`) holds; what does not fit waits, the longest-waiting nearest first.
`OMSI_DEBUG_LAN=1` logs rows told whole / as changes, rows deferred by the budget, answers heard.

### Drawing

A client draws the cars 160 ms behind the host (`PlayClock`), between two rows by linear
blending (angles the short way round). Past the newest row it guesses on from the speed and the
turn the vehicle was making, for at most 400 ms, and not at all for a vehicle that stands. A
jump of more than 30 m is a teleport, no gliding. The indicators, brake lights and stop state
switch past the middle of the two rows.

## Compatibility

Protocol 15. A game of protocol 14 and one of 15 turn each other away at the handshake with
the usual message ("both players need the same version"): `WORLD` no longer carries cars and
`DESC|c|…` is gone. The 0xBA and 0xBB first bytes are taken (no text message starts with them).
