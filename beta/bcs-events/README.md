# BCS event capture beta 0.1.0

Passive first stage for native visual BCS occurrences. The Java agent observes an already-running `system.q` thread and writes local JSON snapshots plus an NDJSON change history. It does not create 3D scenes or connect the stream to OpenOMSI yet.

The event manager, premium popup and phone controller were mapped from the user-provided BCS JAR. Production packages contain only observer code and three class fingerprints, never vendor classes or the original vendor JAR. The agent checks those fingerprints before reading private state. A changed BCS build disables observation until its mapping has been reviewed again.

## Mapped fields

- `system.q.h`: active popup (`o.e`); only the premium BCS popup `o.j` is supported by this beta.
- `o.j.l`: event ID; `o.j.n`: popup countdown; `o.j.p`: completion flag.
- `system.q.j`: manager event ID; `system.q.l`: requested service; `system.q.m`: internal service-ready deadline.
- Phone controller callbacks map 2 to police, 3 to medical, 5 to fire department, 6 to dispatch and 7 to the custom phone number.

`service_delay_elapsed` only means the BCS deadline passed. `completion_observed` additionally requires the popup completion flag and the manager resetting its event ID. Neither field claims that a native emergency vehicle exists or has arrived.

A new `event_instance` is assigned when the active popup changes, including consecutive occurrences with the same event ID. The event is considered visible only once its actual popup exists, rather than when the random generator merely chooses a candidate ID.

## Build and test

A JDK 17+ is used only to build and test. Agent bytecode targets Java 8.

```sh
python beta/bcs-events/test_probe.py
python beta/bcs-events/build_probe.py --vendor-jar /path/to/the-reviewed/busbetrieb-simulator.jar --output /path/to/output
```

Do not generate a new production fingerprint profile for an unreviewed BCS update: matching obfuscated names and field types alone does not prove their meaning remained unchanged.

The JVM tests use synthetic classes and cover event/service transitions, a wrong-service call, actual completion versus timer expiry, repeated same-ID occurrences, unchanged fixture state, paths containing spaces and rejection of an unsupported class fingerprint. They never execute the supplied BCS application.

The Windows launcher scripts scope `JAVA_TOOL_OPTIONS` to the launched BCS process tree. No BCS JAR, stable Bridge executable, host, server, reward, event schedule or global Java setting is modified. The first real-user BCS capture remains required to validate launcher/JRE compatibility and the live mapping.

## User workflow

Extract the generated `Beta-Eventos-BCS` folder inside the BCS directory, close BCS and launch `Iniciar-Beta.bat`. After a real occurrence and service call, use `Abrir-Capturas.bat` and inspect the JSON/NDJSON files. These captures contain event IDs, service states, process IDs and times; no account credentials or remote upload are collected.

Following successful real captures, the next stage is a Bridge/native event protocol and corresponding passenger/vehicle scenes. Vehicle assets, placement and routing, treatment timing and host-authoritative multiplayer synchronization remain separate implementation work.
