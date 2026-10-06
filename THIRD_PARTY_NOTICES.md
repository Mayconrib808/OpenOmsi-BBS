# Third-party notices

The root [MIT licence](LICENSE) applies to the project's original bridge code, synthetic fixtures and documentation. It does not relicense external products or override existing third-party notices.

## Included openOMSI protocol reference

- File: `source/reference/openomsi-plugin-v020.rs`.
- Upstream file: [`crates/omsi-plugin/src/lib.rs`](https://github.com/openOMSI-Project/openOMSI/blob/538ad31b2a2c664cb0726db2547bf6238411eedf/crates/omsi-plugin/src/lib.rs).
- Reference commit: `538ad31b2a2c664cb0726db2547bf6238411eedf`.
- Licence: **MIT**, copyright **(c) 2026 usonskyyyy**.
- Complete unmodified notice: [`source/reference/OPENOMSI_LICENSE.txt`](source/reference/OPENOMSI_LICENSE.txt).

This Rust file is documentation of the upstream protocol. It is not compiled into the Go programs and is not claimed to be the complete source of the excluded custom helper.

## Go runtime and standard library

The three Go bridge programs use the Go standard library with CGO disabled. Compiled binaries contain Go runtime/standard-library code. The reference build uses **Go 1.23.2**. Its main BSD-style licence is [Go LICENSE at go1.23.2](https://github.com/golang/go/blob/go1.23.2/LICENSE); a local copy is kept in [`docs/GO_LICENSE.txt`](docs/GO_LICENSE.txt).

The earlier package's more extensive compiler attribution inventory is preserved in [`docs/GO_TOOLCHAIN_NOTICES.txt`](docs/GO_TOOLCHAIN_NOTICES.txt). That inventory includes toolchain files and is not a claim that every listed component is linked into each bridge binary. Preserve the notices actually applicable to the toolchain and linked code when producing a distribution.

## Development-only tools

Python runs the local build scripts and is not bundled into a bridge binary. GitHub Actions references `actions/checkout` and `actions/setup-go` at fixed commit IDs. Their source is not copied into this repository. Their respective repositories and licences remain authoritative.

## External software and content

OMSI 2, BCS/BBS, Steam, openOMSI executables, map content, buses and add-ons are installed separately. This repository supplies no licence for those items. See [credits](CREDITS.md) for vendor/project acknowledgements.

The unresolved prebuilt native plugin host from the historical v1.1.2 package is excluded. Its rights are not inferred from the upstream Rust reference. See [helper provenance](docs/HELPER_PROVENANCE.md).
