# Cooperative regression checks

Run `cargo test --manifest-path tools/coop-check/Cargo.toml` from the source root.
The harness compiles the actual picking, passenger geometry and protocol modules
with the real simulation/model/geometry crates. The unchanged ray broad phase
is included from omsi-scene by build.rs, avoiding its window/GPU dependencies.
The installed-bus test requires OMSI content and is normally ignored.
These checks do not execute the full frame pipeline, a Windows app or a game.
