# Repository instructions

## Complete P15 host programs

The `source/` HostAgent/CompanyHost implementation is older than the complete P15 binaries shipped since 2.2.0.3. `build-support/bridge2203/host-provenance.json` explicitly records `corresponding_host_source_available: false`. Compiling those host source files removed existing controls in 2.2.0.8 Hotfix 2. Do not repeat that build path for a release.

Preserve the complete released HostAgent.exe and CompanyHost.exe unless their corresponding full source has first been recovered and all features verified. The exact original hashes and minimal clock-recovery patch are audited in `scripts/patch_complete_host_clock.py`. Do not broaden binary patches or ignore its baseline/instruction checks. HostAgent's base identifier 2.2.0.8 and installer version 2.2.0.8.3 intentionally differ.

Any host replacement must run `scripts/test_complete_host_ui.py` against the actual shipped Windows executable before publication. Required controls: voice, radio, voice range, public server listing, machine identifier requirement, and per-map administrator password. Verify map switching, Save/reopen, explicit false values, and preservation of unrelated server/company/keys. General Go tests alone do not verify the complete host UI. Do not claim user settings were restored merely because fixture tests passed.

Retain all previous Coop driver-transfer/camera/skin/audio fixes and the distinct P14 and P15 servers. A host fix must not silently rebuild or rename the native game. Compare actual executable and audio hashes. Preserve the 60-minute map startup allowance and calendar/timetable handling unless a scoped, tested change is necessary.
