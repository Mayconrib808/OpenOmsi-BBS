# openOMSI 0.3.38-coop.4.1 — Windows build revision

The game source and conductor-control corrections from coop.4 are preserved.
The Windows target changes from GNU/MinGW to MSVC, the target used by upstream's
Windows release workflow. This build is cross-compiled on Linux using cargo-xwin
and LLVM, rather than built on a Windows runner. Cargo.lock is unchanged.

The workspace release profile is used: opt-level 3, four codegen units, thin LTO
and debug level 1. MSVC symbols are emitted separately in a PDB; the PDB is not
required to run the game. No packer or executable protection wrapper is added.
The portable opener, upstream launchers, and supporting DLLs are retained.

## Reproduce

Install Rust 1.99.1 and its x86_64-pc-windows-msvc target, LLVM 20.1.2/clang-cl, CMake,
Ninja, and cargo-xwin 0.23.1, following their installation and license terms.
On distributions that provide only clang, a clang-cl symlink to that compiler
selects its standard CL driver mode. Run sh scripts/build-coop-msvc-cross.sh.
The actual SDK/tool versions and output hashes are recorded in the package's
source/BUILD-INFO.json. The existing MSVC build scripts can also build on Windows.

Run the conductor regressions with:

    cargo test --locked --manifest-path tools/coop-check/Cargo.toml

Generated bus fixtures test the actual script, picker and protocol paths.
They do not replace a live test with two players and the user's Caio Millennium
IV Scania bus. See COOP4.md for behavior and limitations.

## Security report

The user confirmed coop.3 downloads with Defender active. The coop.4 ZIP was
blocked as Trojan:Win32/MalUri.A!cl. Comparing the two releases showed that only
programa/openomsi.exe changed among EXE/DLL files. This narrows the regression,
but does not determine what triggered Defender or prove a false positive.

This revision is not antivirus certification. Defender status must be checked
on the new artifact. No antivirus settings are modified. The separate Enhanced+
GPU device-loss crash remains outside the conductor-control correction.

The source was recovered from coop.4's corresponding source archive, which
recorded original commit 25d7e7ca47cae38df522dbb296607223d0cddd0b. A new local
repository starts from those recovered files; that old history is not recreated.
