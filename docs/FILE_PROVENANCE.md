# File provenance

Development input: `OpenOMSI_BCS_Bridge_v1.1.2_by_Mayconrib808.zip`. Import and v1.1.3 preparation: **2026-10-06**. The old ZIP is not a cleared public release.

| Material | Origin and handling | Licence/status |
| --- | --- | --- |
| Original bridge Go code | v1.1.2 source imported; v1.1.3 updates helper deployment/rollback, version and runtime file names | Root MIT |
| New `source/pluginhost/` | Go implementation authored for this project; protocol/ABI adapted from exact MIT upstream reference | Root MIT plus preserved openOMSI copyright/permission notice |
| Host C fixture and Python native tests | Original synthetic DLL and test harness | Root MIT; no vendor code; fixture binary never packaged |
| Driver/session/helper tests and `source/testdata/` | Synthetic profiles/sessions, inert PE fixtures; optional historical hash check uses an excluded local input | Root MIT; no real account/session/vehicle data |
| `source/reference/openomsi-plugin-v020.rs` | Exact upstream library at commit 538ad31b2a2c664cb0726db2547bf6238411eedf | MIT, 2026 usonskyyyy |
| `source/reference/OPENOMSI_LICENSE.txt` | Exact upstream notice at that commit | Preserved verbatim |
| Build scripts, offline tutorial, README and guides | Written for this project | Root MIT |
| `docs/GO_LICENSE.txt`, `GO_TOOLCHAIN_NOTICES.txt` | Go 1.23.2 licence and earlier attribution inventory | Existing notices retained; not relicensed |
| Runtime executables in generated ZIP | Built from the accompanying source by pinned toolchain | Above code licences plus Go runtime notices |
| Inventory and SHA-256 files | Generated from the package assembled by the build script | Integrity metadata, not rights/safety proof |

Excluded from repository and new distribution: historical host binary, proprietary game/vendor executables or plugins, BCS screenshot, maps/buses/DLCs, real profiles/logs, diagnostic ZIPs and personal config. Go executables are generated for distribution, not committed as source files.

This provenance record does not establish PeDePe authorisation. See [the technical notice](../NOTICE_FOR_PEDEPE.md).
