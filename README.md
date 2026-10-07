<p align="center">
  <img src="docs/assets/readme-banner.png" alt="OpenOmsi + BBS — your BBS trips, powered by openOMSI" width="100%">
</p>

<p align="center">
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v1.1.3"><img alt="Version 1.1.3" src="https://img.shields.io/badge/version-1.1.3-f47f30?style=for-the-badge"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/actions/workflows/source-checks.yml"><img alt="Linux and Windows build checks" src="https://img.shields.io/github/actions/workflow/status/Mayconrib808/OpenOmsi-BBS/source-checks.yml?branch=main&style=for-the-badge&label=build"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases"><img alt="Windows package" src="https://img.shields.io/badge/platform-Windows-388bfd?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/github/license/Mayconrib808/OpenOmsi-BBS?style=for-the-badge&color=78a81d"></a>
</p>

<p align="center">
  <b><a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases">Releases</a></b> ·
  <a href="#installation">Quick start</a> ·
  <a href="#documentation">Documentation</a> ·
  <a href="SUPPORT.md">Support</a> ·
  <a href="README.pt-BR.md">Português do Brasil</a>
</p>

**OpenOmsi + BBS** connects **openOMSI** to **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)** on Windows. Pick your trip in BBS and launch openOMSI with the bus and timetable prepared by the bridge.

> [!WARNING]
> **Experimental community project.** Version 1.1.3 has passed automated checks and a real BBS trip. Other map/bus combinations and complete evaluation/payment parity still need testing. See the [validation record](docs/VALIDATION.md).

> [!IMPORTANT]
> You need your own installed **OMSI 2**, **BCS/BBS** and **openOMSI**. The games, paid addons and original BBS plugin are installed separately.

## Releases

**[Download OpenOmsi + BBS 1.1.3.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip)**

| Package | Version | Download |
| --- | --- | --- |
| Complete Windows package | **1.1.3 · experimental** | [OpenOmsi + BBS 1.1.3.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip) |
| SHA-256 verification | 1.1.3 | [Checksum](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip.sha256) |
| Release history and notes | Published versions | [Browse Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases) |

The complete ZIP includes **Setup.exe**, the bridge launcher, compatibility facade, automatically installed 32-bit helper, offline tutorial, source, credits and licenses. **No separate helper download, Go or Python installation is needed to play.**

Choose the named Windows ZIP under **Assets**. GitHub's automatic “Source code” archives contain source files and need a build first.

## What the bridge provides

| Feature | What you get |
| --- | --- |
| **One setup tool** | Configure paths, activate the bridge and restore original OMSI launches. |
| **Start from BBS** | Use the usual BBS trip workflow to open the prepared openOMSI session. |
| **Trip integration** | Connect the locally installed BBS plugin and translate timetable, stop and ticket data. |
| **Local diagnostics** | Generate a diagnostic ZIP from Setup option 5 when something needs investigation. |

## Installation

The tested reference is **Windows x64 + openOMSI 0.2.0 + BCS/BBS 5.0.0.1**. Check compatibility before replacing openOMSI with a newer build.

1. Install **OMSI 2**, **BCS/BBS** and [**openOMSI**](https://github.com/openOMSI-Project/openOMSI/releases) separately.
2. Download the complete bridge ZIP and **extract everything into a permanent folder**.
3. With the games closed, run **Setup.exe** and configure the two game paths.
4. Choose **2 — Activate / update** and accept the Windows permission request.
5. In BCS/BBS, leave **“Start OMSI faster” unchecked**. In openOMSI, disable synchronisation with the real-time clock.
6. Start your trip normally from BBS.

At the last stop: **F9 → wait at least two seconds → finish in BCS/BBS while openOMSI remains open**.

To return to original OMSI, close the games and use Setup option **3**. Open **TUTORIAL.html** from the extracted ZIP for the illustrated offline guide. [Detailed installation guide →](docs/INSTALL.md)

## Tested trip

A real Windows trip on **6 October 2026** used **Carrão City**, line **2201**, tour **02**, departure **09:20**, with **Caio Apache VIP I OF 1721 manual**. The original BBS plugin loaded, the timetable aligned, six stops and eight tickets were saved, BBS received the evaluation and completed the trip, and openOMSI exited normally.

That result covers this tested combination. The known **Berlin-Spandau loading issue in openOMSI 0.2.0** remains documented. [Validation details and remaining checks →](docs/VALIDATION.md)

## Documentation

| Guide | Use it for |
| --- | --- |
| [Installation](docs/INSTALL.md) | Paths, activation and restoring original OMSI |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | Startup, plugin panel and timetable problems |
| [Support and diagnostics](SUPPORT.md) | Reporting a problem and reviewing diagnostic files |
| [Windows test guide](docs/TEST_ON_WINDOWS.md) | Reproducible integration checks |
| [Build from source](docs/BUILD.md) | Toolchain, package assembly and native tests |
| [Runtime effects](docs/RUNTIME_EFFECTS.md) | Files and Windows settings used by the bridge |
| [Credits and licenses](CREDITS.md) | Contributors, upstream work and notices |
| [Accessibility](ACCESSIBILITY.md) | Supported interfaces and reporting barriers |

<details>
<summary><b>How the bridge works and what it writes</b></summary>

The included `Omsi.exe` is this project's compatibility facade, not the original game executable. The helper loads only the user's locally installed original `bbs.dll`. No proprietary executables, DLLs, JARs, plugins, maps, buses, DLC assets or real account logs are distributed.

Original proprietary executables, JARs and DLLs are not patched or replaced. The bridge **does write local data**: it updates `Drivers/bbs.odr`, creates its own runtime files, deploys its helper and configures a scoped Windows Registry launch redirect. See [runtime effects](docs/RUNTIME_EFFECTS.md) and [the technical notice](NOTICE_FOR_PEDEPE.md).

Automatic dates use the local Windows date. Local counter translation can influence the vendor's evaluation; complete telemetry and payment parity are not promised.

</details>

## Contributing

Issues, fixes, documentation and reproducible test results are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

For a source check, install **Go 1.23.2** and **Python 3.10+**, then run:

```sh
python3 scripts/build.py --check-only
```

To assemble the complete ZIP, run `python3 scripts/build.py` (Windows: `py -3 scripts/build.py` or `source/BUILD.cmd`). Output goes to `dist/`. [Full build instructions →](docs/BUILD.md)

## Support

[Report a bug](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Suggest a feature](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Security policy](SECURITY.md)

Contact **Mayconrib808** on Discord: **`.zmaycon.`** (both dots). Setup option **5** creates local diagnostics; [review them before sharing](SUPPORT.md). Nothing is uploaded automatically.

## Credits and license

Maintained and tested by **Mayconrib808**. Implementation and review assisted by **OpenAI Codex**, credited as a development tool.

Original code, documentation and project artwork: [**MIT**](LICENSE). The helper adapts openOMSI's MIT protocol/ABI with **copyright 2026 usonskyyyy** and the [original notice](source/reference/OPENOMSI_LICENSE.txt) preserved. Go runtime notices accompany the binaries. See [all credits](CREDITS.md) and [third-party notices](THIRD_PARTY_NOTICES.md).

This is an independent project. No affiliation, endorsement, approval or official support is claimed from PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH or Valve.
