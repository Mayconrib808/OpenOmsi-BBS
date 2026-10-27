# OpenOMSI modified source for upstream review

This tree contains the reconstructed OpenOMSI game source used by native build
0.3.45-coop.15-2.2.1.8. It does not include the external BCS Bridge application,
Setup, HostAgent, update service, signing keys, or proprietary BCS JAR.
Native game hooks that were present in the published game remain unchanged
so this is an accurate source export, not a newly edited variant.

Base: official openOMSI-org/openOMSI 0.3.45,
commit dd515ba1d917bcb9c9b0d1ba24f6964b27f76ba0.

changes-vs-official-0.3.45.patch is the cumulative game-only diff, including
added files. Applying it to the exact official commit was verified to
reproduce every file in source-sha256.json. This review folder is additional
packaging documentation and is excluded from that source inventory.

Features span cooperative driving/conductor controls and handover,
dynamic dedicated-server loading, passenger umbrellas and hailing,
passenger connections, double-horn greetings, and native admin clock controls.
Refer to the patch and tests for exact implementation and scope.

Build on Windows with the upstream prerequisites (stable Rust/MSVC and
dependencies documented in the original README):

    $env:OPENOMSI_VERSION="0.3.45-coop.15-2.2.1.8"
    $env:OPENOMSI_COOP_GAME_ONLY="1"
    $env:TAURI_CONFIG='{"version":"0.3.45"}'
    cargo build --locked --release -p omsi-app -p omsi-launcher-core

The published build also compiled the generic 32-bit plugin host:

    rustup target add i686-pc-windows-msvc
    cargo build --locked --release --target i686-pc-windows-msvc -p omsi-plugin --bin omsi-plugin-host

Original upstream licensing, authorship and notices are retained.
This export validates source reconstruction and the patch roundtrip;
it does not claim a fresh compilation or paired multiplayer qualification.
In particular, client-owned connecting passengers returning to the
authoritative multiplayer host remain a documented limitation.

GitHub automation files are preserved under UPSTREAM-REVIEW/upstream-workflows as inert review material. For the canonical source inventory, map these back to .github/workflows. Game sources are unchanged.
