# Build the 3.0 candidate

Use **Go 1.23.2** and Python 3.10+; Node.js 22 for directory tests. Build steps do not need BCS credentials or a copy of its proprietary JAR. No Rust recompilation is needed because the official server stays unmodified.

```sh
cd multiplayer3/source
go test -race ./...
go vet ./...
cd ../..
node --test multiplayer3/relay/test/*.test.js
python multiplayer3/scripts/recover_preview_server.py
python multiplayer3/scripts/build_preview.py --server multiplayer3/server --output build-v3 --sequence 2026101402 --source-commit YOUR_COMMIT_SHA
```

The recovery script downloads the official server archive and verifies every hash against its committed provenance. The builder cross-compiles Windows amd64 Setup/adapter/updater and the 386 BCS helper, validates PE resources, includes source and guides, and generates package/executable checksums. Ordinary Go compilation retains debugging information. No executable packer or obfuscator is used.

The complete `-teste.zip` is playable without Go, Python or Node installed. The `-arquivos-atualizacao.zip` and `Atualizar-Multiplayer-*.exe` contain only integration components. They preserve configured games, maps, invitations and preferences. An update from an old 3.x preview will download a current official server when needed; an old Recommended bundle is upgraded automatically through the verified cache, without reinstallation.

Optional local interoperability audit of the exact official BCS 5.0.2.0 JAR:

```sh
python multiplayer3/scripts/audit_bcs.py --jar PATH_TO_YOUR_BCS_JAR
python -m pip install lupa
python multiplayer3/scripts/audit_bcs.py --jar PATH_TO_YOUR_BCS_JAR --simulate
```

The audit never changes the JAR, emits no vendor code and does not authenticate to BCS. Lua simulation uses a mocked simulator API; see the validation record for its limits. Do not add a vendor JAR, decompiled files, private company settings or publication/signing credentials to this repository.

GitHub Actions runs the source tests on Windows and Linux, builds the candidate, records scanner evidence and publishes a prerelease. The final WebDisk release still needs the live two-PC checks and exact-hash antivirus review in `VALIDATION_3.md`.
