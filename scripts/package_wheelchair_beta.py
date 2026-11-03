"""Package the native accessibility beta. Never changes the stable update feed."""
from __future__ import annotations
import argparse, hashlib, json, shutil, subprocess, zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / 'coop-assets' / 'wheelchair-beta'

def package(binary: Path, output: Path) -> Path:
    binary = binary.resolve(strict=True)
    if binary.read_bytes()[:2] != b'MZ':
        raise ValueError('A real Windows native executable is required')
    output.mkdir(parents=True, exist_ok=True)
    stage = output / 'OpenOMSI-Cadeirantes-Beta1'
    stage.mkdir(exist_ok=True)
    shutil.copytree(ASSETS, stage, dirs_exist_ok=True)
    shutil.copy2(binary, stage / 'openomsi.exe')
    steam = binary.with_name('steam_api64.dll')
    if not steam.is_file():
        steam = ROOT / 'assets/steam_redist/steam_api64.dll'
    shutil.copy2(steam, stage / 'steam_api64.dll')
    for name in ['LICENSE', 'TRADEMARKS.md']:
        shutil.copy2(ROOT / name, stage / name)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    manifest = {
        'version': 'wheelchair-beta.1', 'engine': '0.3.47-coop.16',
        'source_commit': commit,
        'source_url': f'https://github.com/Mayconrib808/OpenOmsi-BBS/tree/{commit}',
        'native_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
        'simulation_tests': 'CI cargo test --locked --release -p omsi-sim --lib',
        'native_startup': 'CI openomsi.exe --help',
        'visual_gameplay_verified': False,
        'multiplayer_verified': False,
        'supported_profile': '2007 Marcopolo Torino OF 1722M (measured initial profile)',
        'stable_updater_modified': False,
    }
    (stage / 'beta-manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    sources = output / 'OpenOMSI-Cadeirantes-Beta1-fontes.zip'
    subprocess.run(['git', 'archive', '--format=zip', '-o', str(sources.resolve()), 'HEAD'], cwd=ROOT, check=True)
    archive = output / 'OpenOMSI-Cadeirantes-Beta1-Windows-x64.zip'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as z:
        for p in sorted(stage.rglob('*')):
            if p.is_file(): z.write(p, p.relative_to(output))
    shutil.rmtree(stage)
    hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in [archive, sources]}
    (output / 'SHA256SUMS').write_text(''.join(f'{h}  {n}\n' for n,h in hashes.items()), encoding='ascii')
    print(json.dumps({'package': str(archive), 'source_commit': commit, 'hashes': hashes}))
    return archive

if __name__ == '__main__':
    a = argparse.ArgumentParser(description=__doc__)
    a.add_argument('--binary', type=Path, required=True)
    a.add_argument('--out', type=Path, required=True)
    args = a.parse_args()
    package(args.binary, args.out)
