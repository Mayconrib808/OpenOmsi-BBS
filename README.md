# openOMSI BBS Bridge

An experimental, unofficial bridge between **openOMSI** and **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)**, maintained by **Mayconrib808**.

[Português do Brasil](README.pt-BR.md) · [Installation](docs/INSTALL.md) · [Build](docs/BUILD.md) · [Credits](CREDITS.md)

**v1.1.3 builds a complete bridge ZIP.** Setup, launcher, compatibility facade and the new 32-bit plugin host are built from the included source. The host is bundled and deployed automatically by Setup: users do not download another helper or install development tools. The earlier host with unresolved provenance is excluded.

## Installation

Install legitimate OMSI 2, BCS/BBS and **openOMSI 0.2.0 Windows x64** separately. Extract the complete bridge ZIP into a permanent folder, run **Setup.exe**, configure the two game paths and choose **2 — Activate / update**. Read the bundled offline **TUTORIAL.html**.

In BCS/BBS, leave **“Start OMSI faster” unchecked**. Disable real-time clock synchronisation in openOMSI. At the last stop: **F9 → wait at least two seconds → finish in BCS/BBS while openOMSI remains open**. Setup option **3** returns launches to original OMSI.

The local compatibility target is **BCS/BBS 5.0.0.1**. The historical package records a Berlin-Spandau loading problem in openOMSI 0.2.0; the bridge does not fix it. Automatic dates use the local Windows date. Updates, map loading, plugin panels and trip evaluation need live checks; see [validation](docs/VALIDATION.md).

## Independent project and runtime effects

No affiliation, endorsement, approval or official support is claimed from PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH or Valve.

No proprietary game/BCS executables, DLLs, JARs, plugins, maps, buses, DLC assets or real account logs are distributed. The host loads only the user's locally installed original `bbs.dll`. The included Omsi.exe is our own compatibility facade, not a copy of the original game executable.

**Original proprietary executables/JARs/DLLs are not patched or replaced. Local data is written:** the bridge updates `Drivers/bbs.odr`, creates its own runtime files, stages its own helper in the OMSI folder and configures a scoped Windows Registry launch redirect. A blanket claim that it changes no PeDePe files would be inaccurate. See [runtime effects](docs/RUNTIME_EFFECTS.md) and [the technical notice](NOTICE_FOR_PEDEPE.md).

Credits and licences do not constitute PeDePe authorisation or guarantee acceptance under BCS/BBS service rules. Local counter translation can influence the vendor's evaluation; complete telemetry and payment parity are not promised.

## Development and downloadable build

With **Go 1.23.2** and **Python 3.10+**:

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

On Windows use `py -3 scripts/build.py` or `source/BUILD.cmd`. The complete folder, ZIP and checksum are produced in `dist/`. End users need neither Go nor Python. Tests, vet, four Windows builds, facade layout and package integrity are checked. Windows native DLL tests also require Visual Studio C++ x86 tools for developers only.

[GitHub Actions](https://github.com/Mayconrib808/OpenOmsi-BBS/actions) runs Linux and Windows checks and attaches the complete ZIP/checksum to each successful Windows run as **OpenOMSI-BCS-Bridge-v1.1.3**. This repository's visibility and GitHub authentication still apply to downloads. It does not automatically publish a public Release. Passing CI does not replace a live BCS trip.

## Licence and support

Original code/documentation: [MIT](LICENSE). The Go host adapts openOMSI's MIT protocol/ABI; **copyright 2026 usonskyyyy** and the [original notice](source/reference/OPENOMSI_LICENSE.txt) are preserved. Go runtime notices accompany the binaries. See [third-party notices](THIRD_PARTY_NOTICES.md) and [all credits](CREDITS.md).

Coordination, requirements, testing and maintenance: **Mayconrib808**. Implementation and review assisted by **OpenAI Codex**, acknowledged as a development tool.

Issues or Discord **`.zmaycon.`** (both dots). Setup option 5 creates local diagnostics; [review their contents before sharing](SUPPORT.md). No automatic upload.
