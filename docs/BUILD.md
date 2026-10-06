# Build the complete bridge

Install **Go 1.23.2** and **Python 3.10+** and put Go on PATH. The fixed Go version preserves the facade layout; other toolchains are not assumed compatible. CGO is disabled and no Go modules are downloaded. End users need none of these tools.

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

Windows: `py -3 scripts/build.py` or `source/BUILD.cmd`. The default output is `dist/OpenOMSI_BCS_Bridge_v1.1.3_by_Mayconrib808/`, its complete ZIP and a ZIP checksum. Existing outputs are preserved; use `--output NEW_PATH` to build elsewhere.

The script tests the bridge and new host, runs vet, compiles the Windows x86 host first, hashes it, and injects that digest into Setup/launcher. It compiles all three other components and verifies the facade's three BCS RVAs against the writable 4 MiB backing in matching stripped/unstripped builds. It then includes source, config example, tutorial, credits and licences, generates the complete inventory/SHA-256 manifest, and tests package integrity and actual helper staging/removal.

The host uses explicit source files, `-trimpath`, `-buildvcs=false` and an empty linker build ID so identical source/toolchain produces identical helper bytes across local checkout paths. This keeps exact-hash helper recognition stable between rebuilds.

`--check-only` performs the same assembly/checks in a temporary directory without retaining a ZIP. Use the explicit build entry points: the parent source directory contains three separate main programs, so `go build .` is inappropriate.

## Native Windows checks

On Windows, the script also compiles an original synthetic C DLL and exercises the actual PE32 host: stdcall callbacks, startup root/environment, export flags, float and Unicode string writes, triggers, same-thread calls, message pumping, finalization, EOF cleanup and rejection of other DLL names.

For these development tests only, install Visual Studio C++ x86 tools and the Windows SDK. The GitHub Windows runner already supplies them. Neither the C toolchain nor the compiled fixture DLL is shipped in the runtime ZIP. Linux can cross-compile the full runtime ZIP; native checks run on Windows CI.

Portable helper tests use inert PE fixtures and a memory Registry. An optional historical-hash test reads only ignored `local-only/omsi-plugin-host32.exe`; that input is never packaged. No arbitrary downloaded helper is needed for normal builds.

## CI and distribution

The pinned GitHub workflow runs Linux checks and Windows full packaging/native tests. A successful Windows job uploads **OpenOMSI-BCS-Bridge-v1.1.3**, containing the runtime ZIP and checksum, for 30 days. Repository authentication/visibility applies. Permissions remain read-only for repository contents; it does not publish a Release or change repository visibility.

A ZIP supplies this bridge, not OMSI 2, BCS/BBS, openOMSI or game assets. Preserve the included licences/notices. A complete package and passing automated tests do not establish live trip evaluation or vendor authorisation.
