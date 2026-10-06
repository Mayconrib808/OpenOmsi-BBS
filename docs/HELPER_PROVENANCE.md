# Plugin host provenance

## Source-backed replacement in v1.1.3

The package builds `app/compat/omsi-plugin-host32.exe` from **`source/pluginhost/`** using Go 1.23.2, Windows x86, CGO disabled. It is bundled automatically. Setup deploys it as `OpenOMSI_BCS_PluginHost32.exe` beside the original game executable. No separate user download is required.

Original replacement implementation: **Mayconrib808 project, 2026**, assisted by OpenAI Codex; root MIT licence. Its wire protocol, DLL ABI and frame order are adapted from the MIT openOMSI reference at commit **538ad31b2a2c664cb0726db2547bf6238411eedf**, copyright **2026 usonskyyyy**. File headers and the complete original permission notice in `source/reference/OPENOMSI_LICENSE.txt` retain that attribution. The exact upstream Rust library is included as a reference; it is not compiled into the Go host.

Documented BCS-specific adaptations:

- Only the installed `bbs.dll` under the original OMSI `Plugins` directory is accepted; no vendor DLL is copied or distributed.
- The host must be deployed in the original OMSI root for executable-relative plugin paths. It sets its own working directory and DLL search directory to that root.
- `SteamAppId`/`SteamGameId=252530` are set in this host process only, with no permanent environment/configuration change.
- Stdcall plugin calls remain on one locked Windows thread; Windows messages run while the parent pipe is idle.
- Binary stdout follows the pinned START/FRAME/FINALIZE protocol; diagnostics use stderr. UTF-16 variables, optional procedures, EOF cleanup and finalization are handled.
- The host does not create, read or delete `bbs.start`. A synthetic test DLL inspects its own test marker to validate placement.

Build tooling hashes the newly built host and injects that digest into Setup and the launcher. Those programs recognise the exact bundled bytes for deactivation. A recognised old bridge-specific copy can be updated automatically, with rollback on failed activation. Unknown files and links are preserved.

Automated protocol/unit tests and native Windows tests use original fictitious fixtures. They do not prove the proprietary BCS plugin's panel, online acceptance or trip evaluation; see [validation](VALIDATION.md).

## Excluded historical binary

The old v1.1.2 input package retained an earlier custom executable with unresolved exact authorship/build provenance:

| Identification | Value |
| --- | --- |
| Size | 8,704 bytes |
| Format | Windows PE32 x86 console |
| SHA-256 | `b26af8f2fc55f272ae89f0dcd02c423c26fb758daf24e860960bd2002c82b2d4` |
| Historical package record | v0.5.1 / v0.4.6.10 custom compatibility build |

That binary is **not in this repository or in the new ZIP**. Its hash is retained solely to recognise an already installed old bridge-specific copy for update/removal. A hash is not proof of authorship, licence, safety or compatibility. The MIT licence does not require publishing source; missing source alone is not proof of a violation.

[Exact upstream reference](https://github.com/openOMSI-Project/openOMSI/tree/538ad31b2a2c664cb0726db2547bf6238411eedf/crates/omsi-plugin).
