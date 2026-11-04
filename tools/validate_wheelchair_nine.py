#!/usr/bin/env python3
"""Run the actual 2.2.3 wheelchair detector/controller against nine supplied OMSI add-ons.

Uses the add-on files themselves (not synthetic unit-test fixtures). A successful
result verifies discovery and a CPU-only controller cycle, NOT visual/gameplay
compatibility, passenger AI, collision with moving riders, or multiplayer.
"""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import sys
import tempfile
from zipfile import ZipFile, BadZipFile

CASES = [
    ("Articulado Scania", ("Articulado Scania.zip",), "Mi4_K310UA_F_ZF.bus"),
    ("Articulado Mi4 Voith", ("Articulado(2).zip",), "Mi4_UA_F_Voith.bus"),
    ("Articulado Mi4 ZF", ("Articulado(2).zip",), "Mi4_UA_F_ZF.bus"),
    ("Torino 2007 VW T2 BH", ("Torino 2007.zip",), "Torino_2007_VW_T2_BH.bus"),
    ("Torino 2025", ("Torino 2025.zip",), "TS_1726-AR_Manual.bus"),
    ("Torino S Rio Ita", ("Torino S.zip",), "Torino_S_Rio_Ita.bus"),
    ("VIP I 3 portas", ("Vip 1 3 portas.zip",), "Vip 1 OF 1721 Manual.bus"),
    ("VIP I 4 portas", ("Vip 1 4 portass.zip", "Vip 1 4 portas.zip"), "Vip 1 OF 1721 Manual.bus"),
    ("VIP III Volkswagen", ("Vip iii Wolks.zip",), "Vip 3.bus"),
]

def zip_for(folder: Path, names: tuple[str, ...]) -> Path | None:
    files = {p.name.casefold(): p for p in folder.iterdir() if p.is_file()}
    return next((files[n.casefold()] for n in names if n.casefold() in files), None)

def extract_addon(archive: Path, dest: Path) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    with ZipFile(archive) as zip_file:
        total = 0
        for entry in zip_file.infolist():
            raw = entry.filename.replace("\\", "/")
            rel = PurePosixPath(raw)
            components = [p for p in rel.parts if p not in ("", ".")]
            if not components or any(part == ".." for part in components):
                continue
            if raw.startswith("/") or ":" in components[0] or entry.is_dir():
                continue
            if components[0].casefold() == "vehicles":
                components.pop(0)
            if not components:
                continue
            # No hidden symlink extraction or special files.
            if (entry.external_attr >> 16) & 0o170000 == 0o120000:
                continue
            total += entry.file_size
            if total > 4_000_000_000:
                raise ValueError("Archive uncompressed size exceeds safety limit")
            target = dest.joinpath(*components)
            target.parent.mkdir(parents=True, exist_ok=True)
            with zip_file.open(entry) as source, target.open("wb") as output:
                shutil.copyfileobj(source, output)

def locate_bus(base: Path, filename: str) -> Path:
    matches = [p for p in base.rglob("*.bus") if p.name.casefold() == filename.casefold()]
    if not matches:
        # OMSI add-ons sometimes use an uppercase extension.
        matches = [p for p in base.rglob("*") if p.is_file() and
                   p.name.casefold() == filename.casefold()]
    if len(matches) != 1:
        raise ValueError(f"expected one {filename}, found {len(matches)}: {matches[:5]}")
    return matches[0]

def verdict(data: dict) -> tuple[str, str]:
    discovery = data.get("detection") or {}
    profile = discovery.get("profile")
    if not profile:
        reasons = discovery.get("reasons") or []
        return "BLOQUEADO", "; ".join(map(str, reasons[:6])) or "nenhum perfil validado"
    exercise = data.get("native_exercise") or {}
    if exercise.get("error"):
        return "FALHOU", str(exercise["error"])
    if exercise.get("skipped"):
        return "PENDENTE_NO_JOGO", str(exercise["skipped"])
    controller = exercise.get("controller") or {}
    boarding = controller.get("boarding")
    alighting = controller.get("alighting")
    step = controller.get("max_step_metres")
    interlocked = controller.get("interlocked_after_stow")
    if boarding == "Riding" and alighting == "Finished" and isinstance(step, (float, int)) and step <= 0.012 and interlocked is False:
        return "CICLO_CPU_APROVADO", f"embarque={boarding}, desembarque={alighting}, salto={step:.4f} m"
    return "FALHOU", f"embarque={boarding}, desembarque={alighting}, salto={step}, travado={interlocked}"

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archives", type=Path, required=True,
                        help="directory with the original ZIP files supplied for the nine models")
    parser.add_argument("--probe", type=Path, required=True,
                        help="wheelchair_probe.exe from candidate artifact, or Linux wheelchair_probe")
    parser.add_argument("--game-root", type=Path, required=True,
                        help="base OMSI 2 installation directory for shared data/scripts")
    parser.add_argument("--out", type=Path, default=Path("wheelchair-nine-report.json"))
    args = parser.parse_args()
    if not args.probe.is_file() or not args.game_root.is_dir() or not args.archives.is_dir():
        parser.error("the probe, OMSI 2 installation and ZIP directory must exist")
    records: list[dict] = []
    with tempfile.TemporaryDirectory(prefix="openomsi-wheelchair-nine-") as temp:
        staging = Path(temp)
        unpacked: dict[str, Path] = {}
        for title, archives, bus_name in CASES:
            archive = zip_for(args.archives, archives)
            if archive is None:
                records.append({"model": title, "status": "ARQUIVO_AUSENTE", "detail": ", ".join(archives)})
                continue
            try:
                key = str(archive.resolve())
                if key not in unpacked:
                    directory = staging / f"addon-{len(unpacked)}" / "Vehicles"
                    extract_addon(archive, directory)
                    unpacked[key] = directory
                bus_file = locate_bus(unpacked[key], bus_name)
                env = dict(os.environ)
                env["WHEELCHAIR_EXERCISE"] = "1"
                run = subprocess.run(
                    [str(args.probe.resolve()), str(args.game_root.resolve()), str(bus_file.resolve())],
                    capture_output=True, text=True, encoding="utf-8", errors="replace",
                    timeout=240, env=env)
                if run.returncode:
                    raise RuntimeError(f"probe exit={run.returncode}; stderr={run.stderr[-1000:]}")
                data = json.loads(run.stdout)
                status, detail = verdict(data)
                records.append({
                    "model": title, "status": status, "detail": detail,
                    "archive": archive.name, "bus_file": str(bus_file.relative_to(unpacked[key])),
                    "discovery_ms": data.get("discovery_ms"),
                    "profile": (data.get("detection") or {}).get("profile"),
                    "reasons": (data.get("detection") or {}).get("reasons"),
                    "controller": (data.get("native_exercise") or {}).get("controller"),
                })
            except (ValueError, RuntimeError, OSError, BadZipFile, subprocess.TimeoutExpired, json.JSONDecodeError) as e:
                records.append({"model": title, "status": "ERRO", "detail": str(e)})
    payload = {"kind": "CPU-only real-add-on detection and controller simulation",
               "gameplay_verified": False, "models": records}
    args.out.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")
    print("| Modelo | Resultado | Diagnóstico |")
    print("|---|---|---|")
    for item in records:
        detail = str(item["detail"]).replace("|", "/").replace("\n", " ")[:250]
        print(f"| {item['model']} | {item['status']} | {detail} |")
    print(f"\nRelatório completo: {args.out.resolve()}")
    print("AVISO: ciclo CPU aprovado não comprova a operação visual dentro do OMSI.")
    return 0 if len(records) == 9 and all(x["status"] == "CICLO_CPU_APROVADO" for x in records) else 1

if __name__ == "__main__":
    sys.exit(main())
