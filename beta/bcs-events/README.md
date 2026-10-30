# BCS event capture beta 0.1.4

Passive first stage for native visual BCS occurrences. The Java agent observes an already-running `system.q` thread and writes local JSON snapshots plus an NDJSON change history. It does not create 3D scenes or connect the stream to OpenOMSI yet.

The event manager, premium popup and phone controller were mapped from the user-provided BCS JAR. Production packages contain only observer code and complete reviewed sets of three class fingerprints, never vendor classes or the original vendor JAR. The agent checks those fingerprints before reading private state. Version 0.1.1 supports both supplied BCS builds (9 and 10). All three classes must match the same reviewed profile; unknown or mixed builds disable observation. The reviewed update changes only system/M references to system/O in the event manager and popup bytecode; the observed fields and event conditions are unchanged.

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

The agent reads private fields inside the BCS JVM. Read-only behavior and synthetic tests do not establish PeDePe approval or guarantee against account sanctions. No official authorization for this observer has been confirmed.

## Startup diagnostics (0.1.2)

The original launcher wrapper set JAVA_TOOL_OPTIONS before launching the official executable, whose PE manifest requires administrator elevation. The real launcher/elevation path was not exercised by the original synthetic JVM tests. A user reported no files after an event; provided BCS logs show the event thread, but no observer thread in the available thread dump. This does not establish the exact cause.

The new wrapper elevates before configuring the scoped environment, runs an explicit javaagent preflight with the installed BCS JRE and java -version, logs startup failures, refuses a detected existing BCS Java session, and checks the actual launched JVM acknowledgement. The agent writes its startup status synchronously. Preflight output is separate from BCS captures and is never considered proof of a live BCS connection. The status checker requires a recent snapshot and a live PID.

Windows tests use a synthetic native launcher and fixture JVM to cover inheritance, startup acknowledgement, live status, and an intentionally dropped environment. They do not run the vendor launcher or exercise interactive UAC, which remains a real-machine validation boundary. The diagnostic-only entry point does not launch vendor code or access a BCS account.

## Inspector capture (0.1.4)

This upgrade adds independent inspector fields to the existing schema-1 stream; emergency event IDs and completion semantics are unchanged. The packaged launcher retains the shipped 0.1.3 support for installation beside the vendor JAR or in its own subfolder.

Reviewed mappings for the supplied BCS 9/10 builds:

- `system.M` (9) / `system.O` (10), `aE:int`: selected inspector ID. `aF:String`: editable display name. `S:short`: available inspections. The `aC/aD/aE` getters return these fields; the trip-selection success callback in `p.E` sets the ID/name after a successful server response. Selection alone is not proof of a passenger already aboard or of an active OpenOMSI trip.
- `system.q.g:o.f`: inspection panel, separate from emergency popup `h`. The inspection-start button in `l.i` calls `system.q.e`, which creates this panel. A new panel increments `inspection_instance`, including consecutive inspections with the same selected inspector.
- `o.f.n:int`: number checked; the inspection worker increments it when processing passengers. `a:AtomicBoolean`: pause flag; `i:AtomicBoolean`: stopped flag; `r:AtomicBoolean`: report-submitted flag.
- `inspection_phase`: `idle`, `requested` (panel exists; no passenger checked yet), `checking`, `paused`, `stopped`, or `closed` (panel disappeared). These phases report observations, not invented timers. `stopped`, report submitted, and disappearance do **not** establish successful completion; cancellations can follow the same paths. `inspection_outcome` stays `unknown`.
- The reviewed inspector DTO has no sex or human-model field. Names are editable, so `inspector_gender` stays `unknown` and `inspector_appearance_source` explains the missing mapping. No sex is inferred from a name.

Inspector reads require matching additional fingerprints of the corresponding Spieldaten class, `o.f`, `l.i`, `p.E`, and inspector DTO. Unknown inspector code disables this capability while leaving the independently verified emergency reader available. No vendor callback, setter, network service, UI getter, or inspection action is invoked. No vendor bytecode is rewritten or distributed.

Run `python beta/bcs-events/test_inspector.py` for isolated JVM fixtures covering both state layouts, absence/selection, repeated inspections, checking, pause, stop, closure, editable names, unchanged state, and rejection of an inspector-only fingerprint mismatch. These fixtures do not execute vendor BCS or validate a live inspection. The existing ambulance tester is synthetic and cannot validate this capture.

To validate live: close BCS, replace the existing beta capture files with 0.1.4, launch `Iniciar-Beta.bat`, choose an inspector for a normal company trip, check `Abrir-Capturas.bat`, then use the actual BCS start-inspection control once allowed. Save both JSON and NDJSON after the inspection. No random emergency occurrence or full-length trip is required merely to confirm the selected inspector and requested/working phases. The native 0.2.4 game does not yet consume these inspector fields or create a dedicated inspector character.
