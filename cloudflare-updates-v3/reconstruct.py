import base64,gzip,hashlib,json,struct,sys,os,shutil,subprocess,tempfile,zipfile,zlib
from pathlib import Path
root=Path(__file__).resolve().parent
repo=root.parent
meta=json.loads((root/'package.json').read_text())
compressed=base64.b64decode(''.join(p.read_text() for p in sorted((root/'payload').glob('*.b64'))))
partial=zlib.decompressobj(31).decompress(compressed)
needle=b'OpenOMSI-BBS/PeDePeAdapter/openomsi.exe'
pos=-1
start=-1
while True:
 pos=partial.find(needle,pos+1)
 if pos<0:break
 if pos>=30 and partial[pos-30:pos-26]==b'PK\x03\x04':start=pos-30;break
if start<0:raise SystemExit('Cannot recover embedded payload')
end=partial.find(b'PK\x05\x06',start)
if end<0:raise SystemExit('Embedded payload footer was not recovered')
comment=struct.unpack_from('<H',partial,end+20)[0]
payload=partial[start:end+22+comment]
source=repo/'multiplayer3'/'source'
updater=source/'updater'
(updater/'update_payload.zip').write_bytes(payload)
(updater/'payload_generated.go').write_text('package main\nimport _ "embed"\n//go:embed update_payload.zip\nvar embeddedPayload []byte\nfunc init(){payload=embeddedPayload}\n')
sys.path.insert(0,str(repo/'multiplayer3'/'scripts'))
import build_preview
out=root/'public'
(out/'packages').mkdir(parents=True,exist_ok=True)
exe=out/'packages'/(meta['sha256']+'.exe')
obj,resources,fields=build_preview.metadata(updater,'3.0.2-dev.6','amd64',build_preview.PRODUCT+' updater','Atualizar-Multiplayer.exe')
build_preview.compile_exe('go',updater,'amd64',exe,'-s -w -H=windowsgui',resources,fields)
obj.unlink(missing_ok=True)
if exe.stat().st_size!=meta['size'] or build_preview.digest(exe)!=meta['sha256']:raise SystemExit('Recovered updater differs from locally verified bytes')
(out/'preview.json').write_bytes((root/'preview.json').read_bytes())
with tempfile.TemporaryDirectory() as temp:
 stage=Path(temp)
 with zipfile.ZipFile(updater/'update_payload.zip') as archive:
  for name in archive.namelist():
   if name=='package.json':dest=stage/'OpenOMSI-BBS'/'build.json'
   else:dest=stage/name
   dest.parent.mkdir(parents=True,exist_ok=True)
   dest.write_bytes(archive.read(name))
 subprocess.run([sys.executable,str(repo/'multiplayer3'/'scripts'/'recover_preview_server.py')],check=True)
 shutil.copytree(repo/'multiplayer3'/'server',stage/'OpenOMSI-BBS'/'server')
 for name in ['source','scripts','relay']:
  shutil.copytree(repo/'multiplayer3'/name,stage/'OpenOMSI-BBS'/name,ignore=shutil.ignore_patterns('*.syso','*.pyc','__pycache__','update_payload.zip','payload_generated.go'))
 for name in ['LICENSE','LEIA-ME.txt','README-en.txt','LESEN-de.txt','DESENVOLVIMENTO.md']:
  shutil.copy2(repo/'multiplayer3'/name,stage/'OpenOMSI-BBS'/name)
 full=root/'full.zip'
 build_preview.archive(full,{p.relative_to(stage).as_posix():p for p in stage.rglob('*') if p.is_file()})
 data=full.read_bytes()
 downloads=out/'downloads';downloads.mkdir(exist_ok=True)
 parts=[]
 for i,offset in enumerate(range(0,len(data),12*1024*1024)):
  name='full.part'+str(i);(downloads/name).write_bytes(data[offset:offset+12*1024*1024]);parts.append('/downloads/'+name)
 (out/'downloads.json').write_text(json.dumps({'name':'BBS-OpenOmsi-Multiplayer-3.0.2-dev.6-teste.zip','size':len(data),'sha256':hashlib.sha256(data).hexdigest(),'parts':parts}))
 full.unlink()
print('Recovered the exact locally verified updater and prepared complete ZIP.')
