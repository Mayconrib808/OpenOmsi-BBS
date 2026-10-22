"""Restore the exact signed distribution from pinned, approved build artifacts."""
import base64
import hashlib
import io
import json
import re
import shutil
import struct
import sys
import zipfile
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
OUT = ROOT / 'prepared-publication'
PARTS = OUT / 'assets/packages'
RECIPE_SHA256 = '1be14eadedd0e7aafdd526ecaf3c22e6ede3a61a91ba211652746da3490b76b0'
OLD_UPDATER = '67741c25f265d829b310cdaedff1f625725eeddbdce9fa977181bcbcd88220db'
NEW_UPDATER = 'c734faca3808127012892b6045a1995c03ecda8abf3cf7242b13a3034449662d'

def sha(data):
    return hashlib.sha256(data).hexdigest()

def check(data, spec, label):
    if len(data) != spec['size'] or sha(data) != spec['sha256']:
        raise ValueError('Exact bytes differ: ' + label)
    return data

def restore(spec, sources):
    out = bytearray()
    for operation in spec['operations']:
        if operation[0] == 'literal':
            out.extend(base64.b64decode(operation[1], validate=True))
        elif operation[0] == 'copy':
            _, name, offset, size = operation
            if offset < 0 or size < 0 or offset + size > len(sources[name]):
                raise ValueError('Invalid copy range')
            out.extend(sources[name][offset:offset+size])
        else:
            raise ValueError('Invalid restoration operation')
        if len(out) > spec['size']:
            raise ValueError('Oversized restoration')
    return check(bytes(out), spec, 'restored distribution')

def verify_coop(data, spec):
    check(data, spec, 'Coop archive')
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        seen = set()
        for item in archive.infolist():
            if not item.filename.startswith(spec['prefix']) or item.is_dir():
                raise ValueError('Unexpected Coop archive entry')
            relative = item.filename[len(spec['prefix']):]
            if relative.startswith('/') or '..' in Path(relative).parts:
                raise ValueError('Unsafe archive entry')
            key = 'OpenOmsi Coop 0.3.38/' + relative
            if key in seen or key not in spec['files'] or sha(archive.read(item)) != spec['files'][key]:
                raise ValueError('Coop file integrity differs: ' + relative)
            seen.add(key)
        if seen != set(spec['files']):
            raise ValueError('Missing Coop files')

def main():
    encoded = b''.join(p.read_bytes() for p in sorted((HERE/'recipe').glob('part-*.b64')))
    if sha(encoded) != RECIPE_SHA256:
        raise ValueError('Recipe integrity differs')
    recipe = json.loads(zlib.decompress(base64.b64decode(encoded, validate=True)))
    if recipe['schema'] != 1:
        raise ValueError('Unsupported recipe')
    coop6 = json.loads((HERE/'coop6.json').read_text())
    coop7 = json.loads((HERE/'coop7.json').read_text())
    old_parts = [(ROOT/'approved-base/coop-assets'/(part['sha256']+'.bin')).read_bytes() for part in coop6['parts']]
    for data, spec in zip(old_parts, coop6['parts']):
        check(data, spec, 'approved Coop 6 part')
    sources = {
        'old-updater':(ROOT/'approved-base/Atualizar-Bridge.exe').read_bytes(),
        'old-coop':b''.join(old_parts),
        'game-exe':(ROOT/'approved-game/openomsi.exe').read_bytes(),
        'game-source':(ROOT/'approved-game/openomsi-coop7-source.zip').read_bytes(),
    }
    for name in ['game-exe','game-source']:
        compressor = zlib.compressobj(9,zlib.DEFLATED,-15)
        sources['deflate-'+name] = compressor.compress(sources[name])+compressor.flush()
    for name,data in sources.items():
        check(data, recipe['sources'][name], name)
    updater = restore(recipe['updater'], sources)
    coop = restore(recipe['coop'], sources)
    if sha(updater) != NEW_UPDATER or updater[:2] != b'MZ':
        raise ValueError('Unexpected updater identity')
    verify_coop(sources['old-coop'], coop6)
    verify_coop(coop, coop7)
    PARTS.mkdir(parents=True,exist_ok=True)
    (PARTS/(OLD_UPDATER+'.exe')).write_bytes(sources['old-updater'])
    (PARTS/(NEW_UPDATER+'.exe')).write_bytes(updater)
    for data,part in zip(old_parts,coop6['parts']):
        (PARTS/(part['sha256']+'.bin')).write_bytes(data)
    offset = 0
    for part in coop7['parts']:
        data = check(coop[offset:offset+part['size']], part, 'Coop 7 part')
        (PARTS/(part['sha256']+'.bin')).write_bytes(data)
        offset += part['size']
    if offset != len(coop):
        raise ValueError('Incomplete Coop 7 split')
    for name in ['coop6.json','coop7.json','verificar-antes.mjs','verificar-online.mjs','worker-assets.mjs','wrangler.jsonc']:
        shutil.copyfile(HERE/name,OUT/name)
    shutil.copyfile(HERE/'stable.json',OUT/'assets/stable.json')
    (OUT/'Coop7.zip').write_bytes(coop)
    report = {'version':'2.2.0.1','sequence':2026102202,'updater_sha256':sha(updater),'coop7_sha256':sha(coop),'preserved_previous_version':'2.2.0','native_game_build_run':39775741405,'native_game_source_commit':'b6949d797997f453457bd9cc0601032c698c2cf5','exact_bytes_verified':True,'two_player_bcs_gameplay_tested':False}
    (OUT/'publication-report.json').write_text(json.dumps(report,indent=2)+'\n')
    print('Exact signed updater and Coop 7 restored; all files verified. Previous updater and Coop 6 retained.')

if __name__ == '__main__':
    main()
