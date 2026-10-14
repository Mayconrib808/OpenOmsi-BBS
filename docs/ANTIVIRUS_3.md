# Antivirus and source transparency — 3.0

The integration is built from the published Go source. It does not use an executable packer, obfuscator, renamed suspicious imports or a detector-specific build. Product, author, icon and version metadata are present. Keeping normal debug information makes the binaries easier to inspect; it is not a guarantee of scanner acceptance.

The included openOMSI server is the **unmodified official 0.2.50** release. Its upstream archive and file hashes are recorded in `multiplayer3/server/bbs-server.json`. The downloaded Cloudflare tunnel is also an official release pinned to its published SHA-256.

## What the program does

Setup opens a local configuration page, writes its own configuration and installs its own 32-bit helper beside OMSI 2 for the user's existing BCS plugin. It updates the BCS path/settings after a backup, reads the selected saved situation, launches the selected game, forwards the original vendor Lua and supervises the server/tunnel processes it starts. It exports logs only when requested. No private signing seed, publisher token, BCS credentials, game assets or proprietary JAR is included.

## Current evidence

- ZIP and executable SHA-256 sidecars identify the exact candidate.
- `SHA256SUMS.txt` inside the complete package identifies its files.
- Source and the build command are included; the GitHub build publishes its source commit and toolchain version.
- The Windows workflow records a Microsoft Defender scan when available, including findings or an unavailable scanner. A detection blocks the release job.
- Authenticode is **unsigned** unless a later release explicitly records a real signing certificate. A self-signed certificate is not presented as trusted publisher signing.
- **No VirusTotal verdict for the final candidate is claimed here.** Search the exact file hash or submit the public candidate, and preserve the report URL and scan date. A new build has a new hash and requires its own check.

Do not disable antivirus or add blanket exclusions to play. Investigate a warning on the exact file. VirusTotal aggregates vendors' verdicts; it does not issue or change those verdicts itself. A persistent false-positive report must be raised with the engine vendor, with the report and matching source/build evidence.

Sources: [VirusTotal false-positive contacts](https://docs.virustotal.com/docs/false-positive-contacts), [Microsoft developer resources](https://learn.microsoft.com/en-us/defender-endpoint/developer-resources), [Microsoft file submission](https://www.microsoft.com/en-us/wdsi/filesubmission).
