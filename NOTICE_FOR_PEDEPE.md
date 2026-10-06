# Technical notice for PeDePe

This document describes the imported v1.1.2 bridge implementation. It is published documentation, not a message sent to PeDePe and not evidence of permission from PeDePe.

## Independent project and distribution

Maintainer: **Mayconrib808**. The bridge is an experimental independent compatibility project. It is not a PeDePe product and carries no claimed endorsement, approval or official support.

The repository contains bridge source, synthetic fixtures and one attributed openOMSI MIT protocol reference. It does not distribute or modify a distributed copy of any PeDePe JAR, DLL, OPL, executable, image, sound, account log or proprietary source. The old custom plugin-host binary is not included while its provenance is unresolved.

## Local operation

The original BCS/BBS installation remains responsible for its normal login, licensing, online communications, trip allocation and remuneration. The Go bridge code has no direct HTTP/network client, credential handling or implementation of the BCS server protocol. A separately loaded vendor plugin or simulator can have its own behaviour, which is outside that claim.

The bridge:

1. Reads a local trip log and local timetable/vehicle metadata to select the simulator launch arguments.
2. Uses an explicitly scoped Windows Image File Execution Options redirect to start its launcher when the selected original `Omsi.exe` is invoked. It does not replace that original executable.
3. Exposes its own legacy compatibility windows and memory layout for the local interface expected by the inspected BCS/BBS version. The interface is an implementation target, not a PeDePe-documented or approved API.
4. Translates counters saved by openOMSI, preserves the starting local profile, and mirrors updated data into **`Drivers/bbs.odr`**. This write can affect the evaluation that BCS/BBS consumes; the bridge therefore does not claim to leave all BCS data unchanged.
5. In a complete runtime installation, stages a separately supplied plugin host under the bridge-specific filename `OpenOMSI_BCS_PluginHost32.exe`. BCS's `bbs.start` marker is only checked for existence, not created, read for content or deleted by the bridge.

There is no implementation here for bypassing purchases, Premium, account authentication or server-side checks. The counter translation is intended to preserve saved simulator data; it is not an assurance that every gameplay event or anti-cheat expectation is represented correctly. PeDePe's service rules remain applicable.

## Limits and contact

Compatibility does not establish authorisation. No statement guarantees acceptance by PeDePe, completeness of telemetry or identical remuneration to an original OMSI session.

Technical or rights concerns can be brought to this repository's maintainer through an Issue without attaching proprietary code or account data, or through the existing project Discord contact **`.zmaycon.`**. The maintainer can review the affected file and its provenance. No automatic removal promise or external message has been made on a rights holder's behalf.

Official PeDePe information: [website](https://pedepe.de/), [business contact](https://pedepe.de/onlineshop/kontakt), [support community](https://community.pedepe.de/forum/). These links are vendor references, not authorisations for this bridge.
