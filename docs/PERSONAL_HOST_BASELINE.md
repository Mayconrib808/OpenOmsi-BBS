# Host baseline for subsequent releases

Maycon requires later updates to retain the host features already delivered in his personal edition: dynamic native P14 server 0.3.39-bbs-stream.1 (SHA-256 8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240), saved installation directory, company/map/METAR/clock/population settings, private bus transfers, and HostAgent/CompanyHost protocol 6/7/14/15 compatibility. Reusing the older public HostAgent binaries does not integrate the separate dynamic server payload. Verify the actual package contents before calling a release cumulative.

This branch builds a host-only personal repair on the exact released 2.2.0.2 source, with the existing dynamic native executable unchanged. It must preserve installed player/game files (including Coop 8 appearance and driver-sharing changes), independent custom/P15 servers and all user settings. Never ship an older full personal Coop 6 bundle over newer game files. Unknown/newer host components must not be overwritten automatically.

The current task authorizes only delivery of a personal ZIP. It does not authorize changing the public update channel. A future public integration must explicitly carry these requirements forward and validate its own native artifacts.
