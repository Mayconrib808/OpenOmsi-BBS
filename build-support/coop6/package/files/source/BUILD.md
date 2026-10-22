# openOMSI 0.3.38-coop.6

Extract `openomsi-coop6-source.zip`. The archive contains the corresponding
source for the delivered game, based on upstream v0.3.38 commit
866c60de268f3fc8737287b8e2d13993c49b1f1f, with cooperative changes through Coop 6.

Windows requires Rust stable MSVC, Visual Studio C++ Build Tools, Windows SDK
and CMake. In the extracted source directory, set OPENOMSI_VERSION to
0.3.38-coop.6 and OPENOMSI_COOP_GAME_ONLY to 1, then run:

```text
cargo test --locked --manifest-path tools/coop-check/Cargo.toml
cargo build --locked --release --target x86_64-pc-windows-msvc -p omsi-app --bin openomsi
```

The executable is `target/x86_64-pc-windows-msvc/release/openomsi.exe`. Retain the
matching support DLLs. Place the PCM cue next to the game under
`assets/sounds/coop_turnstile_coin.wav`.

The portable opener and support binaries come from the working Coop 5 package;
their hashes and the new game hash are recorded in BUILD-INFO.json. The opener
requires no administrator rights and changes its environment only for its own
process tree. Its ordinary updater opt-out preserves this experimental fork;
it does not configure Windows security software.

The runtime package adds no automatic source-copy script or installer.
Source build scripts remain part of the corresponding source archive.

See validation/ for build, cooperative regression and antivirus evidence.
