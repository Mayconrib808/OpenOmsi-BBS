from pathlib import Path
import base64
import hashlib
import io
import json
import zipfile

support = Path(__file__).resolve().parent
root = support.parents[2]
game = root / 'game'
payload = json.loads((support / 'overlay.json').read_text())
data = base64.b64decode(payload['zip_base64'], validate=True)
if hashlib.sha256(data).hexdigest() != payload['zip_sha256']:
    raise RuntimeError('Driver sharing source archive hash differs')
with zipfile.ZipFile(io.BytesIO(data)) as archive:
    if set(archive.namelist()) != set(payload['files']):
        raise RuntimeError('Unexpected driver sharing source entries')
    for name in archive.namelist():
        path = Path(name)
        if path.is_absolute() or '..' in path.parts:
            raise RuntimeError('Invalid source path')
        content = archive.read(name)
        if hashlib.sha256(content).hexdigest() != payload['files'][name]:
            raise RuntimeError('Driver sharing source file differs: ' + name)
        target = game / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
out = root / 'out'
out.mkdir(exist_ok=True)
(out / 'driver-share-source-sha256.json').write_text(json.dumps(payload['files'], indent=2) + '\n')
print('Applied driver sharing source with verified hashes.')
