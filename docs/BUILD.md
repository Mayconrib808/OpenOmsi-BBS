# Building and checking the source

Install **Go 1.23.2** and **Python 3.10 or later**. Put Go's `bin` folder on `PATH`. The Go version is fixed because the legacy facade relies on the memory layout produced by that toolchain; a newer compiler is not assumed compatible. CGO is disabled, and the bridge uses the standard library without downloaded Go modules.

## Checks without games or a native helper

From the repository root on Linux/macOS:

```sh
python3 scripts/build.py --check-only
```

On Windows:

```powershell
py -3 scripts/build.py --check-only
```

The script runs the imported tests with the native Go target, runs `go vet`, cross-compiles the three Windows x86 programs and compares stripped/unstripped facade sections. It confirms that BCS's three expected RVAs fall inside the facade's own writable backing region. Build/check outputs are temporary in this mode.

There are multiple main functions in `source/`, so **do not run `go build .`**. The build script supplies the explicit source lists for Setup, the launcher and the facade.

Tests use fictitious fixtures, an in-memory Registry implementation and temporary directories. Their results do not validate UAC, a visible BCS panel, plugin startup, real map passengers or online evaluation. See [Windows checks](TEST_ON_WINDOWS.md).

Helper staging, ownership, diagnostics and rollback tests use an inert synthetic PE header. One explicitly optional test checks removal of the exact historical helper; it is skipped in a fresh checkout. A maintainer who independently verifies a local copy can place it in ignored `local-only/omsi-plugin-host32.exe` to run that exact-hash test. The build script never packages that local copy. A successful unit test is not permission to redistribute it.

## Development components

```sh
python3 scripts/build.py
```

The Windows equivalent is `py -3 scripts/build.py` or `source\BUILD.cmd`. The default destination is `dist/source-build`. An existing output directory is not overwritten; choose another with `--output` when needed.

The output contains Setup, the launcher, the compatibility facade and accompanying source/notices. **The native plugin host and complete runtime packaging are absent.** A fresh source build cannot be activated as a working bridge. Do not publish this directory as a complete v1.1.2 release or copy proprietary dependencies into it.

The older full package's `PACKAGE_FILES.txt`/`SHA256.txt` cannot verify these new build bytes. A future complete release needs its own manifest, all required files, verified helper provenance and live integration validation. The hash-generation utility in `source/tools/release_tools.go` remains available for that release work.

## CI

The checked-in GitHub Actions workflow runs the same `--check-only` path on Ubuntu and Windows with Go 1.23.2. Both referenced actions are pinned to exact upstream commits. Workflow permissions are limited to reading repository contents; it does not publish releases or upload game/user diagnostics.
