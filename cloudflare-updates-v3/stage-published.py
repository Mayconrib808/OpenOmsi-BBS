"""Stage only exact, published and scanned release bytes for the 3.x feed."""
import base64,hashlib,json,shutil,urllib.request
from pathlib import Path

root=Path(__file__).resolve().parent
payload=(root/'release-v2.json').read_bytes()
release=json.loads(payload)
anchor=json.loads((root/'preview-v2.json').read_text())
assert base64.b64decode(anchor['payload'],validate=True)==payload
assert len(anchor['source_commit'])==40
expected=json.loads((root/'distribution.json').read_text())
assert release['version']==expected['version']=='3.0.4'
assert release['sequence']==2026101404
assert release['product']=='bbs-openomsi-multiplayer'
base='https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v'+release['version']+'/'
def fetch(name,sha,size):
 request=urllib.request.Request(base+name,headers={'User-Agent':'BBS-OpenOmsi-Multiplayer-Publisher/3.0.4'})
 with urllib.request.urlopen(request,timeout=120) as response:data=response.read(size+1)
 assert len(data)==size and hashlib.sha256(data).hexdigest()==sha,'Published artifact hash mismatch: '+name
 return data
installer=fetch('Atualizar-Multiplayer-'+release['version']+'.exe',release['installer']['sha256'],release['installer']['size'])
full=fetch(expected['name'],expected['sha256'],expected['size'])
public=root/'public'
if public.exists():shutil.rmtree(public)
(public/'packages').mkdir(parents=True)
(public/'packages'/(release['installer']['sha256']+'.exe')).write_bytes(installer)
for name in ['preview.json','preview-v2.json']:(public/name).write_bytes((root/name).read_bytes())
downloads=public/'downloads';downloads.mkdir()
parts=[]
for index,offset in enumerate(range(0,len(full),12*1024*1024)):
 name='full.part'+str(index);(downloads/name).write_bytes(full[offset:offset+12*1024*1024]);parts.append('/downloads/'+name)
(public/'downloads.json').write_text(json.dumps(dict(expected,parts=parts)))
print('Staged exact published 3.0.4 candidate and verified updater for the independent 3.x channel')
