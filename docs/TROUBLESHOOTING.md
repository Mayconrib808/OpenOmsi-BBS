# Troubleshooting

## Missing helper or package files

Extract the complete v1.1.3 ZIP, including `app/compat/omsi-plugin-host32.exe`; use Setup option 6 to check integrity and option 2 to activate. The helper is bundled automatically: no separate download is needed. A source checkout must first be built through `scripts/build.py`.

Unknown existing files at the bridge-specific helper path are preserved. A recognised historical helper is updated automatically. Close games before updating; deactivate through the previous package's Setup before moving folders. Do not create BCS's startup marker yourself. If an explicit development `OMSI_PLUGIN_HOST32` environment override points to an invalid file, remove that override so Setup can use the bundled default.

## Clock or departure mismatch

Disable real-time clock synchronisation in openOMSI. The bridge refuses ambiguous routes, unsupported offsets and timetable sources that could override the expected duty. `date=auto` uses the Windows local date; use `date=YYYY-MM-DD` in the bridge config only when the correct date is known. Available log data does not establish every company's calendar date.

## BCS panel missing or trip startup stalls

Confirm the original products are installed, **"Start OMSI faster" is unchecked**, the versions match and BCS is starting the trip normally. Wait for map loading, inspect the task switcher and try a windowed simulator session. Marker existence alone does not prove plugin readiness or a visible panel. Use local diagnostics if the failure persists.

## Berlin-Spandau has missing roads/sections

The historical v1.1.2 input records this as an unresolved openOMSI 0.2.0 map-loading problem. Deactivate the bridge and use original OMSI for that map. The BCS faster-startup setting is a separate requirement, not a map repair.

## Trip data missing at completion

At the last stop press F9, wait at least two seconds, and keep the simulator open while finishing in BCS/BBS. An incomplete write, changed driver identity or external profile edit can cause the bridge to retain previous data or suspend file mirroring. Do not force invented counters into a real profile.

## Compile/layout check fails

Use the exact reference Go **1.23.2** and the supplied source lists via `scripts/build.py`. The facade's memory-layout check is required; suppressing it can produce a program incompatible with the inspected BCS interface. Include the error text in a bridge issue.

Before sharing diagnostics, review [support and privacy guidance](../SUPPORT.md). Logs and selected timetable files are not automatically uploaded and may contain private or third-party material.
