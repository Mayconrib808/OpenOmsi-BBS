# Historical native helper: unresolved provenance

The historical v1.1.2 ZIP supplied a native `app/compat/omsi-plugin-host32.exe`, retained from earlier bridge packages. **That binary is not in this repository and is not bundled by its build script.**

Recorded identification:

- File size: **8,704 bytes**.
- Format: Windows PE32 x86 console executable.
- SHA-256: `b26af8f2fc55f272ae89f0dcd02c423c26fb758daf24e860960bd2002c82b2d4`.
- Historical record: retained from v0.5.1 / v0.4.6.10; described there as a custom compatibility build.

The available package did not contain the custom helper's complete source or build environment. Its author, modifications and exact licence coverage cannot be established solely from that record. The original package's assertion that it was a custom compatibility build is a record of what that package said, not an independent provenance verification.

The included MIT-licensed Rust protocol reference is an exact upstream reference and is not this helper's source. A hash identifies bytes; it is not evidence of authorship, licensing, safety or compatibility. The MIT licence does **not** require publishing source, so missing source alone is not proof of a licence violation.

## Why the source build is incomplete

The three Go programs still expect a compatible 32-bit plugin host. They can be built and their portable logic can be tested without one. A fresh checkout cannot activate a working integration simply by compiling those programs.

The historical Go code stages a host next to the original `Omsi.exe` so the vendor plugin can resolve its startup marker relative to the host executable. The recorded historical helper was said to target the BCS plugin specifically. A generic upstream host is not assumed to be a drop-in replacement.

## Resolving the dependency

For a future complete release, establish the exact provenance and licence of a compatible host, or implement/build a documented replacement from appropriately licensed source. Retain the applicable notices, document any custom changes and verify real Windows plugin startup, panel operation and trip evaluation. Do not infer redistribution permission from a similarly named upstream component.

Current upstream reference: [openOMSI `omsi-plugin` at commit 538ad31b2a2c664cb0726db2547bf6238411eedf](https://github.com/openOMSI-Project/openOMSI/tree/538ad31b2a2c664cb0726db2547bf6238411eedf/crates/omsi-plugin).
