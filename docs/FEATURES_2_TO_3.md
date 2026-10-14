# What carries forward from 2.1

| Working behavior from 2.1 | 3.0 treatment |
| --- | --- |
| BCS native OpenOMSI launch and PeDePeAdapter | Retained as the only recommended path. |
| Selected map, bus, timetable, HOF, repaint and original departure seconds | Retained through situation expansion and native service start. |
| Fuel from the actual player bus | Ported from the 2.1.7 correction, including AI/player distinction and preserving other startup variables. |
| Real Lua telemetry and third-party bus/dashboard plugins | Original PeDePe Lua forwarded byte for byte. Existing client content/plugins remain available. |
| Driver data and real counters | Retained with identity and concurrent-change checks; no fabricated evaluation. BCS 5.0.2.0 also evaluates its native Lua counters. |
| Next trip and automatic service/display | Uses the native 5.0.2.0 command and acknowledgment interface. The 2.1 JAR patch is not carried into public 3.0. |
| Protocol-7 company gateway and private buses | Ported transport, identity/authentication preservation and missing-model stand-ins. Official server remains unmodified. |
| Separate tunnel per map | Ported to avoid the upstream global tunnel cleanup terminating another map. |
| Backup, deactivate, solo play | Retained with explicit BCS configuration, multiplayer-only switch and restoration. |
| PT-BR, English and German | Retained in Setup, messages and guides. |
| Automatic/manual checks, update preference, integrity | Retained in the independent Cloudflare 3.x feed, with source-linked trust and signed legacy metadata. |
| Diagnostics and Discord contact | Local log ZIP added to the new Setup; `.zmaycon.` is shown with both dots. |
| DLSS/ReShade installation selected by user | Manual client selection keeps the chosen compatible executable. Graphics setup is separate. |

The 3.0 invitation identifies the company, then routes by map. It is different from a temporary direct server link. Host election and independent map sessions are the main multiplayer redesign.

Existing 2.x updates stay in their own channel. A 2.1 installation is not silently overwritten by the 3.x updater. Extract the 3.0 package, configure it once, and let BCS point to the new adapter. Existing 3.x test users can receive a verified in-place update while keeping their settings.
