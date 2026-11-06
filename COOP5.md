# openOMSI 0.3.38-coop.5 — clicks through the displayed cashier camera

The walking simulation updates `App::camera` with the avatar's eyes. The old
frame order calculated hover immediately afterward; the cooperative camera was
applied later, for drawing. Moving to a cashier camera could therefore show a
door or steering tooltip over the tickets and cash. The cooperative picker also
read that mutable camera instead of retaining the one used for the picture.

The cooperative view now snapshots its camera and bus body pose together, after
applying the selected camera, its saved offset and look. Hover, mouse press and
HTML pointer drag obtain their rays from that snapshot. Remapping from the
drawn pose to the current simulation pose is retained. An absent or different
bus snapshot cannot fall back to an unrelated avatar view. Per-frame hover now
runs after the displayed camera and bus pose have been established.

Native ticket-block hits and mesh cash/door/flap controls use the existing
selection and owner-authoritative action paths. COOP1 protocol 14, conductor
permissions, transaction checks, ticket/change panel buttons and the driver's
own mouse state are unchanged.

## Verification

The native regression harness compiles the production render-camera projection,
scene unprojection, `App::coop_cursor_ray`, `App::coop_drawn_view` and paper
ticket hit method in a lightweight window shell. Generated bus geometry tests
ticket, change and flap targeting with an avatar looking elsewhere, an adjusted
camera, changed FOV and bus motion/rotation between drawing and input. The
shell stands in for window/GPU setup and the camera assignment stage; the
camera selection/capture, projection, ray routing and geometry are real code.
An additional check refuses absent or other-bus snapshots. Existing script,
drag/release, permission and network regressions remain enabled.

    cargo test --locked --manifest-path tools/coop-check/Cargo.toml

The package retains the coop.4.1 MSVC build method and support binaries. See
COOP4_1.md for build background and `scripts/build-coop-msvc-cross.sh` to build.
Tests with generated fixtures do not replace a Windows multiplayer test with
the user's installed bus assets. No Defender scan or graphical Windows run is
available in the build environment.
