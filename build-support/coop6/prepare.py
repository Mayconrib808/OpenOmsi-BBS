from pathlib import Path
import base64
import hashlib
import json
import shutil
import sys
import zipfile

support = Path(__file__).resolve().parent
root = support.parents[2]
game = root / 'game'
out = root / 'out'
out.mkdir(exist_ok=True)

if len(sys.argv) == 1:
    manifest = json.loads((support / 'overlay-sha256.json').read_text())
    for name, digest in manifest.items():
        data = (support / 'overlay' / name).read_bytes()
        if hashlib.sha256(data).hexdigest() != digest:
            raise RuntimeError(f'Overlay hash mismatch: {name}')
        target = game / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
    audio = base64.b64decode((support / 'coop_turnstile_coin.wav.b64').read_text())
    target = game / 'assets/sounds/coop_turnstile_coin.wav'
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(audio)
    (out / 'coop_turnstile_coin.wav').write_bytes(audio)
    (out / 'overlay-sha256.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(f'Applied {len(manifest)} reviewed source files and PCM audio.')
elif sys.argv[1] == 'package':
    with zipfile.ZipFile(out / 'openomsi-coop6-source.zip', 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(game.rglob('*')):
            if not path.is_file():
                continue
            relative = path.relative_to(game)
            if any(p in {'.git', 'target'} for p in relative.parts):
                continue
            archive.write(path, Path('openomsi-coop-source') / relative)
    files = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.iterdir()) if p.is_file() and p.suffix not in {'.log'}}
    (out / 'build-sha256.json').write_text(json.dumps(files, indent=2) + '\n')
    print('Packaged corresponding source and SHA256 manifest.')
else:
    raise SystemExit('Unknown preparation step')
