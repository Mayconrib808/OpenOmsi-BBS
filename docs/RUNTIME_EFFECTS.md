# Local runtime effects

This describes the **v1.1.3 Go implementation and included source-backed host**. It is not an audit of a proprietary plugin's internal behaviour.

| Location or interface | Read or write | Purpose and limits |
| --- | --- | --- |
| Original OMSI 2 `Omsi.exe` | Used as the path matched by a Registry redirect; not replaced or patched | Activation redirects launches of the selected path to this bridge |
| BCS/BBS executable, JAR and plugin binaries | Not patched or distributed by the Go bridge | The bundled host loads the locally installed original plugin; no vendor file is bundled |
| `Drivers/bbs.odr` | Read, backed up locally and updated | Translates actual saved simulator counters; external changes suspend file mirroring |
| Bridge `app/bridge.ini` | Created/updated | Local paths, language and bridge options |
| Bridge-owned runtime files | Created/updated | Logs, driver snapshots, readiness/PID data, trip diagnostics and temporary timetable ZIPs |
| Installed map timetable and Chrono files | Read; installed source files not rewritten | A temporary ZIP can change what openOMSI loads for the selected duty during a session |
| openOMSI user settings | Read | Detects real-time clock conflicts; does not rewrite those settings |
| BCS/BBS settings | User changes required settings manually | The bridge displays the faster-startup requirement; it does not edit the setting |
| BCS `bbs.start` marker | Existence check only | Bridge does not create, inspect the contents of, copy or delete it |
| `OpenOMSI_BCS_PluginHost32.exe` in the OMSI folder | Setup stages the bundled helper copy | Recognised bridge bytes can be updated automatically with rollback; unknown files/links are preserved; removal checks exact bundled or historical hashes |
| Host process directory/environment | Process-local settings | Working/DLL directory set to original OMSI root, SteamAppId/SteamGameId=252530 only in this process; no permanent Steam or environment change |
| Windows Registry IFEO filter | Explicit activation/deactivation writes | Scoped to the selected original executable; UAC is requested; rollback/restoration logic preserves unrelated configuration |
| This bridge's own compatibility windows and memory | Created/updated | Supplies legacy local state for BCS/BBS; does not claim a vendor-approved API |
| Diagnostic ZIP requested by the user | Local archive created | Can contain private data and local timetable content; no automatic upload |

Registry scope:

```text
HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\
Image File Execution Options\Omsi.exe\OpenOMSI_BCS_Bridge
```

The Go code contains no direct BCS service client. Simulator/vendor-plugin network behaviour remains outside that statement. Saved ticket revenue and counters are translated, so they can influence BCS/BBS's own evaluation. The bridge does not calculate the service's payment and does not guarantee identical evaluation or complete telemetry.

Deactivation restores launch behaviour for the selected installation. It does not undo completed trips or claim to roll back server-side results. Keep the starting driver snapshot for local recovery analysis; avoid overwriting a profile changed by another process.
