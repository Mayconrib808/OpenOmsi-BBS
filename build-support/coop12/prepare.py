"""Apply all cumulative Coop features to the pinned 0.3.40/P15 upstream tree."""
from pathlib import Path
import hashlib, json, subprocess

support = Path(__file__).resolve().parent
root = support.parents[2]
game = root / 'game'
base = 'e874276e1bfa22896deb1e00adab0ed7d4f3ce2f'
assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=game, text=True).strip() == base
spec = json.loads((support / 'source-sha256.json').read_text())
sha = lambda b: hashlib.sha256(b).hexdigest()
for name, item in spec.items():
    p = game / name
    if item['before'] is None:
        assert not p.exists(), name
    else:
        data = p.read_bytes().replace(b'\r\n', b'\n')
        assert sha(data) == item['before'], 'Unexpected upstream source: ' + name
        p.write_bytes(data)
subprocess.run(['git', 'apply', '--binary', '--whitespace=nowarn', '-'], cwd=game,
               input=(support / 'cumulative.patch').read_bytes(), check=True)
for name, item in spec.items():
    assert sha((game / name).read_bytes()) == item['after'], name
out = root / 'out'
out.mkdir(exist_ok=True)
(out / 'coop12-source.json').write_text(json.dumps(dict(upstream=base, protocol=15,
    patch_sha256=sha((support / 'cumulative.patch').read_bytes()), files=spec), indent=2) + '\n')
print('Coop12: complete cumulative 0.3.40/P15 source, native startup clock fix, two horn taps.')
