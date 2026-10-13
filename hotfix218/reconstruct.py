"""Publish a verified manual download while preserving the signed 2.1.7 channel.

The original 2.1 signing key is required before stable.json can announce 2.1.8.
Do not replace that trust key or publish an unsigned stable manifest.
"""
from pathlib import Path
import hashlib, json

root = Path(__file__).resolve().parent
meta = json.loads((root / 'download.json').read_text())
data = b''.join(p.read_bytes() for p in sorted((root / 'payload').glob('part-*.bin')))
assert meta['version'] == '2.1.8' and meta['automatic_channel_updated'] is False
assert len(data) == meta['size'] and data[:2] == b'MZ'
assert hashlib.sha256(data).hexdigest() == meta['sha256']
assert meta['url'] == 'https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/' + meta['sha256'] + '.exe'
output = Path('update-service/assets/packages')
output.mkdir(parents=True, exist_ok=True)
(output / (meta['sha256'] + '.exe')).write_bytes(data)
print('Verified manual download:', meta['version'], meta['sha256'], len(data))
