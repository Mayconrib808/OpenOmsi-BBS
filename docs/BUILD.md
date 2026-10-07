# Build the complete bridge

Install **Go 1.23.2** and **Python 3.10+** and put Go on PATH. The fixed Go version preserves the facade layout; other toolchains are not assumed compatible. CGO is disabled and no Go modules are downloaded. End users need none of these tools.

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

Windows: `py -3 scripts/build.py` or `source/BUILD.cmd`. On this development branch the default output is `dist/OpenOmsi.+.BBS.2.0.0-dev.2/`, its complete ZIP and a ZIP checksum. Existing outputs are preserved; use `--output NEW_PATH` to build elsewhere. CI names its artifact from `source/version.go`; the release publisher skips development versions.

The script tests the bridge and new host, runs vet, compiles the Windows x86 host first, hashes it, and injects that digest into Setup/launcher. It compiles the bridge, Setup, facade and CompanyHost components and verifies the facade's three BCS RVAs against the writable 4 MiB backing in matching stripped/unstripped builds. It then includes source, config example, tutorial, credits and licences, generates the complete inventory/SHA-256 manifest, and tests package integrity and actual helper staging/removal.

For the approved v1.1.3 release, Go's embedded build ID is restored from `docs/releases/v1.1.3-buildids.json`. Linux and Windows builds differed only in the first two components of this non-executable metadata string. After restoring it, the **entire binary** must match its approved SHA-256; any additional difference fails the build. No code or executable layout is changed. The host already uses an empty build ID and is identical across the two environments.

The host uses explicit source files, `-trimpath`, `-buildvcs=false` and an empty linker build ID so identical source/toolchain produces identical helper bytes across local checkout paths. This keeps exact-hash helper recognition stable between rebuilds.

`--check-only` performs the same assembly/checks in a temporary directory without retaining a ZIP. Use the explicit build entry points: the parent source directory contains four separate main programs, so `go build .` is inappropriate.

The 2.0 development build has no live-approved executable hash file; its compiled code is intentionally different. Package integrity and native tests still run. Binary approval and promotion to a release require the pending integration tests in [MULTIPLAYER.md](MULTIPLAYER.md).

## Native Windows checks

On Windows, the script also compiles an original synthetic C DLL and exercises the actual PE32 host: stdcall callbacks, startup root/environment, export flags, float and Unicode string writes, triggers, same-thread calls, message pumping, finalization, EOF cleanup and rejection of other DLL names. CompanyHost native process tests also verify that closing its Windows Job Object terminates both the owned server and a simulated tunnel child.

For these development tests only, install Visual Studio C++ x86 tools and the Windows SDK. The GitHub Windows runner already supplies them. Neither the C toolchain nor the compiled fixture DLL is shipped in the runtime ZIP. Linux can cross-compile the full runtime ZIP; native checks run on Windows CI.

Portable helper tests use inert PE fixtures and a memory Registry. An optional historical-hash test reads only ignored `local-only/omsi-plugin-host32.exe`; that input is never packaged. No arbitrary downloaded helper is needed for normal builds.

## CI and distribution

The pinned Source checks workflow runs Linux checks and Windows full packaging/native tests. A successful Windows job uploads **OpenOMSI-BCS-Bridge-v<version>**, containing the runtime ZIP and checksum, for 30 days. Its repository-content permissions remain read-only.

The separate **Publish tested release** workflow runs after successful main-branch push checks from this same repository. It downloads that exact CI run’s artifact, verifies the ZIP/internal manifest and the four approved v1.1.3 executable hashes, then uploads the ZIP/checksum as a draft and publishes an experimental prerelease. Its publication job alone has contents-write and actions-read permissions. Pull-request artifacts cannot trigger publication, and an existing version is preserved. A matching file under `docs/releases/` is required for release notes and approved binary hashes. Repository visibility is not changed. The persistent download is on [Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v1.1.3).

A ZIP supplies this bridge, not OMSI 2, BCS/BBS, openOMSI or game assets. Preserve the included licences/notices. A complete package and passing automated tests do not establish live trip evaluation or vendor authorisation.
