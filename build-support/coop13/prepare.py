"""Apply Coop13 fixes after the exact cumulative published Coop12 source."""
from pathlib import Path
import subprocess,json,hashlib
support=Path(__file__).resolve().parent
repo=support.parents[1]
root=repo.parent
game=root/'game'
subprocess.run(['python',str(repo/'build-support/coop12/prepare.py')],check=True)
sha=lambda b:hashlib.sha256(b).hexdigest()
spec=json.loads((support/'source-sha256.json').read_text())
for name,pin in spec.items():
 data=(game/name).read_bytes().replace(b'\r\n',b'\n')
 assert sha(data)==pin['before'], 'Published Coop12 base differs: '+name
 (game/name).write_bytes(data)
subprocess.run(['git','apply','--binary','--whitespace=nowarn','-'],cwd=game,input=(support/'fixes.patch').read_bytes(),check=True)
for name,pin in spec.items():
 data=(game/name).read_bytes().replace(b'\r\n',b'\n')
 (game/name).write_bytes(data)
 assert sha(data)==pin['after'], 'Coop13 result differs: '+name
out=root/'out'
out.mkdir(exist_ok=True)
(out/'coop13-source.json').write_text(json.dumps(dict(upstream='e874276e1bfa22896deb1e00adab0ed7d4f3ce2f',protocol=15,version='0.3.40-coop.13',patch_sha256=sha((support/'fixes.patch').read_bytes()),files=spec),indent=2)+'\n')
print('Applied Coop13 camera, ownership/cabin, mouse, audio and paint fixes on exact Coop12 baseline.')
