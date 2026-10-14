"""Reconstruct the approved 2.1.8 updater and its original-key signed manifest."""
from pathlib import Path
import base64, hashlib, json

root = Path(__file__).resolve().parent
meta = json.loads((root / 'download.json').read_text())
data = b''.join(p.read_bytes() for p in sorted((root / 'payload').glob('part-*.bin')))
assert meta['version'] == '2.1.8' and meta['automatic_channel_updated'] is True
assert len(data) == meta['size'] and data[:2] == b'MZ'
assert hashlib.sha256(data).hexdigest() == meta['sha256']
assert meta['url'] == 'https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/' + meta['sha256'] + '.exe'
output = Path('update-service/assets/packages')
output.mkdir(parents=True, exist_ok=True)
(output / (meta['sha256'] + '.exe')).write_bytes(data)
envelope = json.loads((root / 'stable.json').read_text())
release = json.loads(base64.b64decode(envelope['payload']))
assert release['version'] == meta['version'] and release['sequence'] == meta['sequence']
assert release['installer'] == {k: meta[k] for k in ('url', 'sha256', 'size')}
# publication.mjs verifies the original Ed25519 trust key before deployment.
Path('update-service/assets/stable.json').write_bytes((root / 'stable.json').read_bytes())
print('Verified automatic update package:', meta['version'], meta['sha256'], len(data))
