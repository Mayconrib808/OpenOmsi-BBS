#!/bin/sh
# Build the cooperative game using the upstream Windows MSVC target/profile.
# Needs Rust, cargo-xwin, LLVM/clang-cl, CMake and Ninja.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
export OPENOMSI_VERSION="${OPENOMSI_VERSION:-0.3.38-coop.6}"
export OPENOMSI_COOP_GAME_ONLY=1
export XWIN_ARCH=x86_64
cargo xwin build --locked --release --target x86_64-pc-windows-msvc \
    -p omsi-app --bin openomsi
