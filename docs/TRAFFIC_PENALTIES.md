# Traffic penalties in OpenOmsi + BBS 2.0.1

**A penalidade específica por avançar o sinal vermelho ainda não está disponível.** A ponte continua transmitindo os contadores de colisões e a avaliação de condução que o OpenOMSI realmente salvou. Essa avaliação não permite descobrir se houve uma infração de semáforo.

## What is supported

The bridge reads the simulator's saved personnel `.odr`, translates its rating order, and publishes the same collision counters and driving penalty to the BCS personnel file and facade memory. BCS remains responsible for calculating the evaluation and remuneration. Pressing F9 publishes the current simulator data; a change is accepted only after two identical complete reads.

The openOMSI driving penalty `P` is a proportion between zero and one; the corresponding driving rating is `100 * (1 - P)`. In the verified 0.2.11 source, collisions, strong acceleration or turns, and repeated changes between throttle and brake can increase `P`. Distance driven decreases it. A low driving rating is therefore not evidence of a red-light offence, and the bridge does not turn it into one.

The diagnostic `driver-state-v1.1.3.txt` retains its historical filename and now includes the saved collision counters, `P`, the corresponding driving rating, and the red-light limitation. These are saved personnel values, not a live signal detector.

## Why the existing BCS detector is unavailable

The BCS JAR supplied for the earlier local integration contains `AmpelIntegration.java` (`n/a.class`) and `res/ampelSpeicher.cfg`. That detector reads the original OMSI runtime map, tile, object-instance and path arrays. Its configuration starts from `Omsi.exe + 0x461588`; a path object's signal state is read at offset `0x1a0`. It also needs the actual bus position and live simulation time. This finding applies to that supplied JAR, not every BCS installation.

The compatibility facade supplies the BCS driver memory and schedule/menu handshake. It does not contain the running OpenOMSI map/path tree. Copying those original OMSI addresses into the facade cannot provide the real signal states.

OpenOMSI 0.2.11 simulates traffic-light phases internally, but its verified plugin interface does not export a player's red-light crossing counter or event, or the controlling lane, stop line and live signal state needed to reconstruct one reliably. The plugin's player-position data and its generic `crash` or `pedestrian` events are insufficient. The bridge does not infer an offence from nearby lamp colour, an assumed signal cycle, or a score change.

## Evidence from the official source

OpenOMSI tag `v0.2.11` resolves to commit `0874dac1ff8051a8484de1781af4e17cdf6d21bc`. Links below are pinned to that exact version:

- [Personnel format and rating semantics](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-content/src/driver.rs): `Driver::save`, `check_ratings` and `driving_percent`; the format contains collision counters and ratings, with no red-light counter.
- [Career tracking and saved penalties](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-app/src/career.rs): `Career::tick`, `penalise`, `crashed`, `driving_rating` and `save`.
- [Published plugin game state](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-app/src/plugins.rs): `game_info` exports position, schedule and collision information but no controlling lane or signal state.
- [Plugin interface and event contract](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-plugin/src/lib.rs): `PluginIo::info`, `others` and `events`.
- [Plugin documentation](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/docs/PLUGINS.md): documented game state, `crash`, `pedestrian` and `stops_skipped` events.
- [Actual career/event integration](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-app/src/app_events.rs): the frame records collisions and pedestrian hits and sends the corresponding plugin events.
- [Internal traffic-light simulation](https://github.com/openOMSI-Project/openOMSI/blob/0874dac1ff8051a8484de1781af4e17cdf6d21bc/crates/omsi-app/src/traffic.rs): traffic controllers and `light_vars` belong to the simulator's internal world.

These sources establish the missing interface in the inspected version. They do not establish that all future or custom OpenOMSI builds have the same limitation.

## What would make red-light detection possible

### Rechecked for 2.0.2 preparation

On 2026-10-07, the current official release was rechecked at **OpenOMSI 0.2.14**, commit [`37d9e61b90c6dcbeecb7913011b70995262791d7`](https://github.com/openOMSI-Project/openOMSI/tree/37d9e61b90c6dcbeecb7913011b70995262791d7). Its [`game_info` / plugin adapter](https://github.com/openOMSI-Project/openOMSI/blob/37d9e61b90c6dcbeecb7913011b70995262791d7/crates/omsi-app/src/plugins.rs), [career state](https://github.com/openOMSI-Project/openOMSI/blob/37d9e61b90c6dcbeecb7913011b70995262791d7/crates/omsi-app/src/career.rs) and [plugin contract](https://github.com/openOMSI-Project/openOMSI/blob/37d9e61b90c6dcbeecb7913011b70995262791d7/docs/PLUGINS.md) still provide no authoritative player red-light crossing event/counter.

The internal multiplayer [`LightState`](https://github.com/openOMSI-Project/openOMSI/blob/37d9e61b90c6dcbeecb7913011b70995262791d7/crates/omsi-net/src/world.rs) contains a crossing object's ID, controller time and held state. This is part of game-world synchronization, not a supported bridge detector: it does not itself identify a player's lane, stop-line crossing or applicable red signal. Adding a nearby-light heuristic or copying OMSI memory offsets would not establish a correct BBS offence.

Consequently, **2.0.2-dev.1 does not implement red-light penalties**. The investigation establishes an upstream/interface dependency; it is not an in-game BBS validation of OpenOMSI 0.2.14. The existing 0.2.0/0.2.11 compatibility probes and live-validation scope remain as documented.

A future integration needs an authoritative event from the simulator, or a documented interface exposing the player's controlling lane, stop-line crossing and signal state at that instant. It should identify each crossing so an event is counted once, and distinguish red, red/yellow, yellow, dark signals, reversing, teleporting and reconnecting. Multiplayer checks must use the host's actual shared light state.

After that interface exists, the bridge can be connected to BCS's supported detector/evaluation path and tested with actual passages on red and green. Neither a new upstream implementation nor a BCS red-light result has been validated for 2.0.1. The bridge does not create fines or guessed violation counts.

## Validation included in the bridge

Regression tests verify that saved collision counters and driving penalty reach both BCS memory and the personnel-file fallback unchanged. They also verify that real penalty recovery with distance is accepted without inventing collision or red-light events. This tests data translation; an in-game BCS red-light test remains unavailable until the required telemetry exists.
