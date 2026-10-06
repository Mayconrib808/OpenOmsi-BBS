# openOMSI BBS Bridge

An experimental, unofficial compatibility bridge between **openOMSI** and **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)**, maintained by **Mayconrib808**.

[Português do Brasil](README.pt-BR.md) · [Installation](docs/INSTALL.md) · [Build from source](docs/BUILD.md) · [Credits](CREDITS.md)

**Repository status: v1.1.2 source import. This repository does not provide a complete, ready-to-play release.** The three Go programs can be compiled and tested. The earlier package's custom 32-bit plugin-host executable is excluded while its precise source and redistribution provenance remain unresolved. See [helper provenance](docs/HELPER_PROVENANCE.md).

## Independence and third-party rights

This project is not affiliated with, endorsed by, sponsored by, or supported by PeDePe GbR, the openOMSI Project, MR-Software GbR, Aerosoft GmbH, or Valve. Product names identify compatibility targets; they do not imply approval.

The repository includes independently developed bridge code, synthetic test fixtures, and an attributed MIT-licensed openOMSI protocol reference. It includes no proprietary OMSI 2 or BCS/BBS executables, DLLs, JARs, plugins, maps, buses, DLC assets, or account logs. Install the original products separately and comply with their own licences and service terms.

**The bridge does not patch or replace the proprietary OMSI 2 or BCS/BBS executable files. It does write runtime data:** it translates and mirrors the local `Drivers/bbs.odr` profile, creates bridge-owned files, and configures a scoped Windows Registry launch redirect when activated. Consequently, a claim that it "changes no PeDePe files" would be inaccurate. The distinction is explained in [runtime effects](docs/RUNTIME_EFFECTS.md) and the [technical notice for PeDePe](NOTICE_FOR_PEDEPE.md).

No statement in this repository represents a PeDePe authorisation or a guarantee of compliance with BCS/BBS online rules. Compatibility support and permission from a service provider are separate matters.

## What the bridge implements

- Reads the locally installed BCS/BBS trip log to identify the selected map, route, tour, departure, vehicle and repaint when those fields are available.
- Launches the separately installed openOMSI with matching arguments and a compatibility facade for the legacy local interface expected by BCS/BBS.
- Aligns a uniquely identified departure using a temporary timetable ZIP when needed, without rewriting the installed map timetable.
- Translates saved openOMSI driver counters into the local profile and compatibility-memory layout expected by BCS/BBS.
- Provides Portuguese, English and German setup messages, scoped activation/deactivation, integrity checks and user-requested diagnostic collection.

This is not a universal compatibility guarantee. Ambiguous departures or conflicting timetable sources are rejected. Panel behaviour, passenger loading, trip evaluation and telemetry coverage require real Windows integration tests.

## Requirements and known limits

The imported code targets **Windows**, **openOMSI 0.2.0 Windows x64** and the locally inspected **BCS/BBS 5.0.0.1** layout. The bridge facade and plugin host are 32-bit. Version updates can change this interface.

You need separately installed legitimate copies of OMSI 2 and BCS/BBS, openOMSI, and a compatible plugin host with verified rights. The host is not included here. Go is needed only for development.

Before using a complete compatible package, leave **"Start OMSI faster"** unchecked in BCS/BBS and disable real-time clock synchronisation in openOMSI. At the final stop, save with **F9**, wait at least **two seconds** for the save, then finish in BCS/BBS while the simulator remains open.

The imported package reports incomplete Berlin-Spandau loading under openOMSI 0.2.0. This bridge does not fix that map issue. Automatic calendar selection uses the Windows local date because the available BCS log does not reliably supply the company date.

## Development

With **Go 1.23.2** and **Python 3.10+** installed:

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

On Windows, use `py -3 scripts/build.py` or `source/BUILD.cmd`. Checks run the existing portable tests and `go vet`, compile the three Windows programs, and verify the facade memory layout. A normal build produces **development-only components** in `dist/source-build`; it does not bundle the missing helper or create an installable release ZIP.

GitHub Actions runs the same checks on Linux and Windows, using the fixed reference Go version. Passing these checks does not establish live BCS/BBS integration.

## Licence, credits and support

Original bridge code and repository documentation are under [MIT](LICENSE). The included openOMSI reference retains its [own copyright and MIT notice](source/reference/OPENOMSI_LICENSE.txt). Go runtime/toolchain notices are recorded in [third-party notices](THIRD_PARTY_NOTICES.md). Those licences do not cover third-party products, marks or assets.

Project coordination, requirements and integration testing: **Mayconrib808**. Implementation and repository preparation were assisted by **OpenAI Codex**. See [all credits](CREDITS.md).

Report bridge problems in this repository's Issues, or contact the maintainer through the existing project Discord handle **`.zmaycon.`**, including both dots. Follow [support guidance](SUPPORT.md) before sharing diagnostics. Support for this bridge is independent of the original software vendors.
