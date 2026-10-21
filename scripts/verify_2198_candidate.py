"""Verify and exercise the immutable 2.1.9.8 installer delivered to the owner.

The update is not rebuilt for publication: the automatic channel gets exactly
the same reviewed bytes as the manual package, including its seven components.
"""
import hashlib
import io
import json
from pathlib import Path
import struct
import zipfile

ROOT = Path(__file__).resolve().parents[1]
candidate = ROOT / 'release-candidates' / '2.1.9.8'
metadata = json.loads((candidate / 'candidate.json').read_text(encoding='utf-8'))
# The connected Git provider limits each request to 16 MiB. Three immutable
# binary blobs retain the reviewed installer without storing any credential.
parts = [candidate / ('Atualizar-Bridge.exe.part' + str(i)) for i in range(3)]
data = b''.join(p.read_bytes() for p in parts) if all(p.is_file() for p in parts) else (candidate / 'Atualizar-Bridge.exe').read_bytes()
assert metadata['version'] == '2.1.9.8'
assert metadata['sequence'] == 2026102101
assert len(data) == metadata['size'] == 21862400
assert hashlib.sha256(data).hexdigest() == metadata['sha256'] == '7b026ea86b88acef6e18c2be6c8c4f198e954610ee5e23834f86ecdbfb7035e7'
(candidate / 'Atualizar-Bridge.exe').write_bytes(data)

matches = []
pos = 0
while (pos := data.find(b'PK\x05\x06', pos)) >= 0:
    if pos + 22 <= len(data):
        end = pos + 22 + struct.unpack_from('<H', data, pos + 20)[0]
        size, offset = struct.unpack_from('<II', data, pos + 12)
        start = pos - size - offset
        if start >= 0 and end <= len(data) and data[start:start + 4] == b'PK\x03\x04':
            try:
                with zipfile.ZipFile(io.BytesIO(data[start:end])) as z:
                    if set(z.namelist()) == set(metadata['components']):
                        matches.append(data[start:end])
            except zipfile.BadZipFile:
                pass
    pos += 4
assert len(matches) == 1, 'Installer must contain one exact seven-component payload'
output = ROOT / 'build' / 'verified-2198'
output.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(io.BytesIO(matches[0])) as z:
    assert z.testzip() is None
    for name, digest in metadata['components'].items():
        b = z.read(name)
        assert hashlib.sha256(b).hexdigest() == digest, name
        pe = struct.unpack_from('<I', b, 60)[0]
        assert b[:2] == b'MZ' and b[pe:pe + 4] == b'PE\x00\x00', name
        assert struct.unpack_from('<H', b, pe + 4)[0] == 0x14c, name
        target = output / name
        assert target.resolve().is_relative_to(output.resolve()), name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(b)

with zipfile.ZipFile(output / 'Defender-Input.zip', 'w', compression=zipfile.ZIP_DEFLATED) as scan:
    manifest = metadata['sha256'] + '  Atualizar-Bridge.exe\n'
    scan.writestr('Atualizar-Bridge.exe', data)
    for name, digest in metadata['components'].items():
        scan.write(output / name, name)
        manifest += digest + '  ' + name + '\n'
    scan.writestr('docs/SHA256.txt', manifest)
scan_path = output / 'Defender-Input.zip'
scan_path.with_suffix('.zip.sha256').write_text(hashlib.sha256(scan_path.read_bytes()).hexdigest() + '  Defender-Input.zip\n', encoding='ascii')
print('Verified immutable 2.1.9.8 installer and seven Windows components:', metadata['sha256'])
