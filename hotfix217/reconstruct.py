"""Reconstruct the exact locally compiled updater before Cloudflare deployment."""
from pathlib import Path
import base64, hashlib, json

root = Path(__file__).resolve().parent
envelope = json.loads((root / 'stable.json').read_text())
release = json.loads(base64.b64decode(envelope['payload']))
parts = sorted((root / 'payload').glob('part-*.bin'))
data = b''.join(p.read_bytes() for p in parts)
expected = release['installer']
assert release['version'] == '2.1.7'
assert len(data) == expected['size']
assert hashlib.sha256(data).hexdigest() == expected['sha256']
assert data[:2] == b'MZ'
(root / 'Atualizar-Bridge.exe').write_bytes(data)
Path('update-service/assets/stable.json').write_bytes((root / 'stable.json').read_bytes())
print('Verified updater:', release['version'], expected['sha256'], len(data))
