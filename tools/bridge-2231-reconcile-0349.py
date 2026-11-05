#!/usr/bin/env python3
"""Reconcile exact v0.3.49 additions after the 0.3.47-based Bridge source merge.

Run ONLY after merging official 0784e3cd into the Bridge tree. Fails closed
when an expected local conflict shape changes. This is a source candidate,
not authorization to deploy the BCS updater without all release gates.
"""
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]

def replace_exact(path: str, old: str, new: str, expected: int = 1) -> None:
    target=ROOT/path
    source=target.read_text(encoding="utf-8")
    found=source.count(old)
    if found==0 and source.count(new)==expected:
        print(f'already integrated {path}: {expected} expected reconciliation(s)', flush=True)
        return
    if found!=expected:
        raise RuntimeError(f"{path}: expected {expected} occurrences, got {found} of {old!r}")
    target.write_text(source.replace(old,new),encoding="utf-8")
    print(f"merged {path}: {expected} precise reconciliation(s)", flush=True)

# Official P17 wire identity must never be overwritten by the Bridge P16 hunk.
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "pub const PROTOCOL: u32 = 16;",
    "pub const PROTOCOL: u32 = 17;",
)

# The v0.3.49 Traffic/Random module supersedes horn's local pseudo-random helper.
replace_exact(
    "crates/omsi-sim/src/ai_traffic/illegal_parking.rs",
    "use super::horn::draw;",
    "use super::random::draw;",
)

# Restore the 0.3.49 per-light AI traffic decision state while preserving
# the Bridge's bus-lane passing and incident vehicle fields.
replace_exact(
    "crates/omsi-sim/src/ai_traffic/model.rs",
    "    pub passing: Option<Passing>,\n",
    "    pub passing: Option<Passing>,\n"
    "    /// 0.3.49: current traffic light decision, including amber/red.\n"
    "    pub current_light: Option<(usize, usize)>,\n"
    "    pub amber_decision: f32,\n",
)
replace_exact(
    "crates/omsi-sim/src/ai_traffic/model.rs",
    "            passing: None,\n",
    "            passing: None,\n"
    "            current_light: None,\n"
    "            amber_decision: 0.0,\n",
)

# Restore official short-trip behavior without replacing Bridge's accessible
# passenger path, transfers, boarding and wheelchair code.
replace_exact(
    "crates/omsi-sim/src/people/pax_task.rs",
    "use super::pax_stops::entry_on_platform;\n",
    "use super::pax_stops::entry_on_platform;\n\n"
    "const SHORT_RIDE_KM: f32 = 3.0;\n"
    "pub(super) fn short_ride(dest: Option<&str>, ride_km: f32) -> bool {\n"
    "    dest.is_none() && ride_km > 0.0 && ride_km < SHORT_RIDE_KM\n"
    "}\n",
)

# P17 adds the SoundAcks category to an existing BCS-specific per-traffic
# rate-limit counter. Its last index is 12, so both storage and diagnostics
# must expand without clobbering the 12 earlier categories.
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "    drop_counts: [u32; 12],",
    "    drop_counts: [u32; 13],",
)
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "            drop_counts: [0; 12],",
    "            drop_counts: [0; 13],",
)
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "                p.drop_counts[11]\n",
    "                p.drop_counts[11],\n"
    "                p.drop_counts[12]\n",
)

# New upstream population tests must initialize the Bridge's network seat field.
replace_exact(
    "crates/omsi-sim/src/people/population_limit_tests.rs",
    "        remote: false,\n    };",
    "        remote: false,\n        remote_seat: None,\n    };",
    expected=2,
)

print("P17 reconciliation complete. Compile/test before considering release.",flush=True)
