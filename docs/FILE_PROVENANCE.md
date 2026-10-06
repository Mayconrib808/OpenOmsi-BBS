# File provenance for the repository import

Import date: **2026-10-06**. Source package name: `OpenOMSI_BCS_Bridge_v1.1.2_by_Mayconrib808.zip`. This name identifies the private development input; the ZIP itself is not committed or offered as a cleared release.

| Material | Source and handling | Licence/status |
| --- | --- | --- |
| Runtime `.go` files and `source/tools/release_tools.go` | Imported unchanged from the bridge's included source | Original bridge implementation; root MIT notice |
| Driver/session/helper test code | Imported with synthetic values and helper fixtures; historical exact-hash removal is an optional local test | Root MIT notice; no runtime logic changes |
| `source/testdata` | Freshly authored fictitious driver records and session text | Root MIT notice; no real account, session, vehicle or company data |
| `source/reference/openomsi-plugin-v020.rs` | Upstream protocol reference, independently compared to the exact reference commit | MIT, usonskyyyy; original notice preserved |
| `source/reference/OPENOMSI_LICENSE.txt` | Compared to upstream licence at the same reference commit | Original notice preserved verbatim |
| Configuration example | Empty installation paths; comment corrected to v1.1.2 | Root MIT notice |
| Build scripts, README, translated README and `.md` guides | Written for the repository import | Root MIT notice |
| Go attribution inventory | Earlier package's compiler notices retained; helper attribution claim removed | Existing third-party notices, not relicensed by root MIT |
| `docs/GO_LICENSE.txt` | Go 1.23.2 toolchain licence | Existing Go notice, not relicensed by root MIT |

Excluded: all prebuilt executables; the custom native helper; BCS setting screenshot; proprietary game/DLC/vendor components; real driver fixtures; historical real-session log; diagnostic ZIPs; user configuration.

The import review establishes what is included and how it is attributed. It does not establish vendor authorisation or certify the functionality of proprietary components. See [the technical notice](../NOTICE_FOR_PEDEPE.md) for the local compatibility method.
