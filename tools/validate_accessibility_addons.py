"""Run the CPU accessibility probe on supplied add-ons without redistributing them."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--probe", type=Path, required=True)
    parser.add_argument("--game-root", type=Path, required=True)
    parser.add_argument("--addons", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    rows = []
    for bus in sorted(args.addons.rglob("*.bus")):
        completed = subprocess.run(
            [str(args.probe.resolve()), str(args.game_root.resolve()), str(bus.resolve())],
            env=dict(os.environ, WHEELCHAIR_EXERCISE="1"),
            capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=180,
        )
        row = {"bus": bus.relative_to(args.addons).as_posix(),
               "definition_sha256": hashlib.sha256(bus.read_bytes()).hexdigest()}
        if completed.returncode:
            row.update(status="load_failed", error=completed.stderr[-2000:])
        else:
            data = json.loads(completed.stdout)
            detection = data.get("detection") or {}
            profiles = detection.get("profiles", [])
            exercises = data.get("bay_exercises", [])
            passed = bool(profiles) and len(exercises) == len(profiles) and all(
                exercise["exercise"].get("controller", {}).get("boarding") == "Riding"
                and exercise["exercise"].get("controller", {}).get("alighting") == "Finished"
                and exercise["exercise"].get("controller", {}).get("interlocked_after_stow") is False
                for exercise in exercises
            )
            row.update(
                status="controller_cycle_passed" if passed else "pending" if profiles else "access_blocked",
                profiles=profiles, bay_exercises=exercises, reasons=detection.get("reasons", []),
                geometry_family=detection.get("family"), discovery_ms=data.get("discovery_ms"),
                script_errors=data.get("script_errors"),
                missing_sections=[line for line in completed.stderr.splitlines() if line.startswith("Missing articulated section")],
            )
        rows.append(row)
        print(f"{row['status']}: {row['bus']}", flush=True)
    report = {
        "schema": 1, "bridge_version": "2.2.3", "base_release": "2.2.2.4-hotfix.5",
        "scope": "CPU equipment macro and wheelchair controller fixtures; no renderer, full passenger demand journey or paired gameplay",
        "gameplay_visual_tested": False, "paired_gameplay_tested": False,
        "probe_sha256": hashlib.sha256(args.probe.read_bytes()).hexdigest(),
        "summary": {status: sum(row["status"] == status for row in rows) for status in
                    ["controller_cycle_passed", "pending", "access_blocked", "load_failed"]},
        "vehicles": rows,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"]))


if __name__ == "__main__":
    main()
