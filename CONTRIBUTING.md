# Contributing

Use [the build guide](docs/BUILD.md) and run `python3 scripts/build.py --check-only` before submitting a change. Runtime changes should be explained with the affected version, trigger, resulting behaviour and relevant validation. UI and BCS integration require a real Windows check in addition to unit tests.

Submit only original code or code whose provenance and licence you can document. Retain applicable copyright and licence notices. Do not include vendor binaries, decompiled proprietary source, paid content, credentials, real account logs or personal driver files. Use synthetic fixtures for reproducible tests.

Changes must not implement purchase/authentication/Premium bypasses or invented trip counters. Do not label a reverse-engineered compatibility interface as a vendor-approved API. Credit actual contributions in [CREDITS.md](CREDITS.md) without suggesting endorsements.

The unresolved historical helper is not an approved vendored dependency. Any proposed replacement should come with its exact source, build steps, applicable notices and a Windows integration result. [The provenance record](docs/HELPER_PROVENANCE.md) describes what is presently unknown.
