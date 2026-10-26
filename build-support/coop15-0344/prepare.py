"""Port the fully tested cumulative Coop15 layer onto the exact official 0.3.44 changes.

The official 0.3.44 release modifies only seven files relative to v0.3.43.
Replay the audited complete 0.3.43 layer first (125 files), then merge all
seven target files. Never substitute an unmodified official executable for
the cooperative build or replace the existing custom multiplayer code.
"""
from pathlib import Path
import json
import shutil
import subprocess

root = Path("game")
support = Path("support")
official = Path("official44")
spec = json.loads((support / "build-support/coop15-0344/source.json").read_text(encoding="utf-8"))

def revision(dir):
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=dir, text=True).strip()

assert revision(root) == spec["official_base_commit"], "The original 0.3.43 source moved"
assert revision(official) == spec["official_target_commit"], "Official 0.3.44 is not pinned"
# This script checks the original SHA-256 and applies the entire prior audited
# patch, including hand/neck, real handover, fuel, conductor and GUI tests.
subprocess.run(["python", str((support / "build-support/coop15-0343/prepare.py").resolve())], check=True)

unchanged_by_coop = [
    "CHANGELOG.md",
    "crates/omsi-app/src/frame_stages.rs",
    "crates/omsi-app/src/watchdog.rs",
    "crates/omsi-navigator/src/build.rs",
    "crates/omsi-render/src/pipelines/errors.rs",
]
assert sorted(spec["official_changed_paths"]) == sorted(unchanged_by_coop + [
    "crates/omsi-app/src/lib.rs", "crates/omsi-app/src/settings.rs"
])
for path in unchanged_by_coop:
    source = official / path
    dest = root / path
    if path != "crates/omsi-app/src/watchdog.rs":
        previous = subprocess.check_output(
            ["git", "show", "HEAD:" + path], cwd=root)
        # The cumulative layer is not allowed to have changed these sources:
        # replacing them must be identical to applying the genuine 0.3.44 diff.
        assert dest.read_bytes() == previous, "Coop layer unexpectedly edits " + path
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, dest)
    assert dest.read_bytes() == source.read_bytes()

# Preserve ALL additional cooperative modules while enabling official watchdog.
lib_path = root / "crates/omsi-app/src/lib.rs"
lib = lib_path.read_text(encoding="utf-8")
assert lib.count("mod view_sync;\n") == 1
assert "mod watchdog;" not in lib
lib = lib.replace("mod view_sync;\n", "mod view_sync;\nmod watchdog;\n", 1)
lib_path.write_text(lib, encoding="utf-8")

# Preserve passenger/fuel settings while adding the precise 0.3.44 safe-GPU
# defaults. This is the sole overlap in Settings with our cumulative patch.
settings_path = root / "crates/omsi-app/src/settings.rs"
settings = settings_path.read_text(encoding="utf-8")
old = """        self.mirror_size = self.mirror_size.min(256);
        if n >= 2 {
            self.shadows = false;"""
new = """        self.mirror_size = self.mirror_size.min(256);
        // Official 0.3.44: turn off expensive Enhanced effects after driver loss.
        self.ray_tracing_on = false;
        self.cloud_shadows = false;
        self.light_glow = false;
        self.lamp_shadows = false;
        if self.light_shadows == "all" {
            self.light_shadows = "street".into();
        }
        if n >= 2 {
            // Official 0.3.44: a second device loss falls back to Vanilla+.
            if graphics_mode(&self.graphics) == "enhanced" {
                self.graphics = "vanilla_plus".into();
            }
            self.light_shadows = "off".into();
            self.shadows = false;"""
assert settings.count(old) == 1, "0.3.44 safe-GPU merge anchor changed"
settings = settings.replace(old, new, 1)
settings_path.write_text(settings, encoding="utf-8")

# Verify that all official 0.3.44 additions were transferred, not just the
# version string. Source identity and patch proof stay independently pinned.
target_lib = (official / "crates/omsi-app/src/lib.rs").read_text(encoding="utf-8")
assert "mod watchdog;" in target_lib and lib.count("mod watchdog;") == 1
target_settings = (official / "crates/omsi-app/src/settings.rs").read_text(encoding="utf-8")
for expression in [
    "self.ray_tracing_on = false;", "self.cloud_shadows = false;",
    "self.light_glow = false;", "self.lamp_shadows = false;",
    'self.graphics = "vanilla_plus".into();',
    'self.light_shadows = "off".into();',
]:
    assert expression in target_settings and expression in settings, expression
for required in ["mod coop;", "mod coop_drive;", "mod coop_view;", "mod mouse_control;"]:
    assert required in lib, "Coop module regressed: " + required
for required in ["passenger_umbrellas", "passenger_hailing_maps", "ticket_chance_percent"]:
    assert required in settings, "Coop setting regressed: " + required
assert not subprocess.check_output(["git", "diff", "--check"], cwd=root, text=True)
print("Authentic upstream 0.3.44 changes merged with all cumulative Coop15, passenger and fuel fixes.")
