"""Stage an already tested Windows package for the isolated Cloudflare 3.x feed."""
import base64,hashlib,json,shutil
from pathlib import Path
root=Path(__file__).resolve().parent
build=root.parent/'hotfix-build'
info=json.loads((build/'BUILD-INFO.json').read_text())
envelope=json.loads((root/'preview.json').read_text())
release=json.loads(base64.b64decode(envelope['payload']))
assert info['version']==release['version']=='3.0.2-dev.7'
assert info['product']==release['product']=='bbs-openomsi-multiplayer'
assert info['sequence']==release['sequence']==2026101207
updater=build/f"Atualizar-Multiplayer-{info['version']}.exe"
data=updater.read_bytes()
sha=hashlib.sha256(data).hexdigest()
assert sha==info['updater_sha256']==release['installer']['sha256']
assert len(data)==info['updater_size']==release['installer']['size']
out=root/'public'
if out.exists():shutil.rmtree(out)
(out/'packages').mkdir(parents=True)
(out/'packages'/f'{sha}.exe').write_bytes(data)
(out/'preview.json').write_bytes((root/'preview.json').read_bytes())
full=build/f"BBS-OpenOmsi-Multiplayer-{info['version']}-teste.zip"
data=full.read_bytes()
downloads=out/'downloads';downloads.mkdir()
parts=[]
for n,offset in enumerate(range(0,len(data),12*1024*1024)):
 name='full.part'+str(n)
 (downloads/name).write_bytes(data[offset:offset+12*1024*1024])
 parts.append('/downloads/'+name)
(out/'downloads.json').write_text(json.dumps({'name':full.name,'size':len(data),'sha256':hashlib.sha256(data).hexdigest(),'parts':parts}))
print('STAGED_VERSION='+info['version']+' UPDATER_SHA256='+sha+' FULL_ZIP_SHA256='+hashlib.sha256(data).hexdigest())
