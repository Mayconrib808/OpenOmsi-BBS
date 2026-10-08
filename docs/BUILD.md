# Build the complete bridge

Install **Go 1.23.2** and **Python 3.10+** and put Go on PATH. The fixed Go version preserves the facade layout; other toolchains are not assumed compatible. CGO is disabled and no Go modules are downloaded. End users need none of these tools.

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

Windows: `py -3 scripts/build.py` or `source/BUILD.cmd`. The default output is `dist/OpenOmsi.+.BBS.<version>/`, its complete ZIP and a ZIP checksum; the version comes from `source/version.go`. Existing outputs are preserved; use `--output NEW_PATH` to build elsewhere. CI names its artifact from that same version; the release publisher skips development versions.

The script tests the bridge and new host, runs vet, compiles the Windows x86 host first, hashes it, and injects that digest into Setup/launcher. It compiles the bridge, Setup, facade and CompanyHost components and verifies the facade's three BCS RVAs against the writable 4 MiB backing in matching stripped/unstripped builds. It then includes source, config example, tutorial, credits and licences, generates the complete inventory/SHA-256 manifest, and tests package integrity and actual helper staging/removal.

For the approved v1.1.3 release, Go's embedded build ID is restored from `docs/releases/v1.1.3-buildids.json`. Linux and Windows builds differed only in the first two components of this non-executable metadata string. After restoring it, the **entire binary** must match its approved SHA-256; any additional difference fails the build. No code or executable layout is changed. The host already uses an empty build ID and is identical across the two environments.

The host uses explicit source files, `-trimpath`, `-buildvcs=false` and an empty linker build ID so identical source/toolchain produces identical helper bytes across local checkout paths. This keeps exact-hash helper recognition stable between rebuilds.

`--check-only` performs the same assembly/checks in a temporary directory without retaining a ZIP. Use the explicit build entry points: the parent source directory contains four separate main programs, so `go build .` is inappropriate.

The 2.0 development build has no live-approved executable hash file; its compiled code is intentionally different. Package integrity and native tests still run. Binary approval and promotion to a release require the pending integration tests in [MULTIPLAYER.md](MULTIPLAYER.md).

## Native Windows checks

For the 2.0.2 development Setup, `scripts/setup_resources.py` builds a Windows x86 COFF resource object from `source/resources/setup.ico` and `setup-banner.bmp` using Python's standard library. Setup is compiled from its explicit file list in an isolated directory so Go links that resource object without including the other main programs. A generated local module with a fixed import path keeps temporary directory names out of the executable; it has no external dependencies. The build verifies the actual PE bitmap/icon bytes. No resource compiler, Go dependency or image library is added to the normal build.

Windows CI switches the real native controls through Portuguese, English and German, verifies that typed fields survive, checks optional-multiplayer field enablement and loads both window icons and the bitmap. It uploads actual window captures as **Setup-Windows-previews** for visual review. Normal Setup does not take screenshots. Language selection is saved when the user chooses **Salvar e ativar / Save and activate / Speichern / aktivieren**.

On Windows, the script also compiles an original synthetic C DLL and exercises the actual PE32 host: stdcall callbacks, startup root/environment, export flags, float and Unicode string writes, triggers, same-thread calls, message pumping, finalization, EOF cleanup and rejection of other DLL names. CompanyHost native process tests also verify that closing its Windows Job Object terminates both the owned server and a simulated tunnel child.

The 2.0.3-dev.2 plugin host is also compiled in an isolated fixed-path module to link standard icon and VERSIONINFO resources, produced by `scripts/plugin_host_resources.py` with the same COFF writer. Its normal Go build ID, symbols and debug information are retained; only the Windows GUI subsystem flag is passed to its linker. Both exact linked resource bytes and the Windows version.dll view of the metadata are verified. A separate reconstruction with the dev.1 build flags must reproduce that published helper's hash, proving that the protocol source is unchanged. Full-package tests install the actual dev.1 bytes and check successful replacement, activation rollback and deactivation with the new helper. Resource metadata is not a digital signature.

For these development tests only, install Visual Studio C++ x86 tools and the Windows SDK. The GitHub Windows runner already supplies them. Neither the C toolchain nor the compiled fixture DLL is shipped in the runtime ZIP. Linux can cross-compile the full runtime ZIP; native checks run on Windows CI.

Portable helper tests use inert PE fixtures and a memory Registry. An optional historical-hash test reads only ignored `local-only/omsi-plugin-host32.exe`; that input is never packaged. No arbitrary downloaded helper is needed for normal builds.

## CI and distribution

The pinned Source checks workflow runs Linux checks and Windows full packaging/native tests. A successful Windows job uploads **OpenOMSI-BCS-Bridge-v<version>**, containing the runtime ZIP and checksum, for 30 days. Its repository-content permissions remain read-only.

For dev.2, the Windows job scans the final ZIP and all six executables with updated Microsoft Defender and active real-time/cloud protection. Scan inputs are outside the hosted workspace, with Internet-zone marking and exact ZIP/manifest hashes checked. A detection, scanner error or unavailable cloud blocks the job and development publication; evidence is uploaded as **Defender-package-report**. The development publisher requires the successful scan step, complete manifest/source identity and linked helper resources. This is an automated scan of the candidate, not a Microsoft false-positive ruling or a substitute for testing the player's original download path; see [ANTIVIRUS.md](ANTIVIRUS.md).

The separate **Publish tested release** workflow runs after successful main-branch push checks from this same repository. It downloads that exact CI run’s artifact, verifies the ZIP/internal manifest and the five approved executable hashes for the selected release, then uploads the ZIP/checksum as a draft and publishes the versioned release. Its publication job alone has contents-write and actions-read permissions. Pull-request artifacts cannot trigger publication, and an existing version is preserved. A matching file under `docs/releases/` is required for release notes and approved binary hashes. Repository visibility is not changed. The persistent download is on [Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v2.0.1).

A ZIP supplies this bridge, not OMSI 2, BCS/BBS, openOMSI or game assets. Preserve the included licences/notices. A complete package and passing automated tests do not establish live trip evaluation or vendor authorisation.

The Windows check also creates the native graphical Setup controls and runs the actual metadata probe against SHA-256-pinned official OpenOMSI 0.2.0 and 0.2.11 player/server builds. These checks do not load a map or certify a BCS trip, next-trip button or two-player gameplay.

The dev.2 regression suite covers missing/existing CompanyHost configurations, map situation companions versus real timezone assets, BBS original-backup identity with UTF-8/UTF-16 lists, strict administrator review, inventory additions and duplicate folder reports. Its BBS fixtures are synthetic; a successful check does not establish that every changed Solaris script on a player's installation has a matching recorded original.
