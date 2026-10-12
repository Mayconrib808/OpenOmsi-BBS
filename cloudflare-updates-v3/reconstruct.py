import io,base64,gzip,hashlib,json,struct,sys,os,shutil,subprocess,tempfile,zipfile,zlib
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
# Recover complete launcher/helper entries; the truncated third entry duplicates the launcher.
entries={}
dates={}
offset=start
while partial[offset:offset+4]==b'PK\x03\x04':
 header=struct.unpack_from('<4s5H3I2H',partial,offset)
 _,needed,flags,method,dostime,dosdate,crc,csize,usize,nlen,xlen=header
 name=partial[offset+30:offset+30+nlen].decode()
 dates[name]=(1980+(dosdate>>9),(dosdate>>5)&15,dosdate&31,dostime>>11,(dostime>>5)&63,(dostime&31)*2)
 body=offset+30+nlen+xlen
 if body+csize>len(partial):break
 raw=zlib.decompress(partial[body:body+csize],-15) if method==8 else partial[body:body+csize]
 if len(raw)!=usize or zlib.crc32(raw)&0xffffffff!=crc:raise SystemExit('Recovered entry CRC mismatch')
 entries[name]=raw
 offset=body+csize
launcher=entries['OpenOMSI-BBS/PeDePeAdapter/openomsi.exe']
helper=entries['OpenOMSI-BBS/compat/omsi-plugin-host32.exe']
entries['Setup.exe']=launcher
receipt=json.loads((repo/'multiplayer3'/'server'/'bbs-server.json').read_text())
manifest={'product':'bbs-openomsi-multiplayer','version':'3.0.2-dev.6','sequence':2026101206,'channel':'preview','source_commit':'','client_default':'0.2.29','update_origin':'https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev','native_server':receipt,'files':{'Setup.exe':hashlib.sha256(launcher).hexdigest(),'OpenOMSI-BBS/PeDePeAdapter/openomsi.exe':hashlib.sha256(launcher).hexdigest(),'OpenOMSI-BBS/compat/omsi-plugin-host32.exe':hashlib.sha256(helper).hexdigest()},'public_release_ready':False,'pending':['next_trip_supported_interface','3x_update_feed_publication','real_bcs_multiplayer_tests','company_map_weather_passenger_settings']}
entries['package.json']=json.dumps(manifest,indent=2).encode()
buffer=io.BytesIO()
with zipfile.ZipFile(buffer,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as archive:
 for name,data in sorted(entries.items()):
  info=zipfile.ZipInfo(name,dates.get(name,dates['Setup.exe']))
  info.compress_type=zipfile.ZIP_DEFLATED
  info.external_attr=(0o100644 if name=='package.json' else 0o100755)<<16
  archive.writestr(info,data,compresslevel=9)
payload=buffer.getvalue()
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
actual=build_preview.digest(exe)
print('RECOVERED_UPDATER_SHA256='+actual+' SIZE='+str(exe.stat().st_size),flush=True)
if exe.stat().st_size!=meta['size'] or actual!=meta['sha256']:raise SystemExit('Recovered wrapper requires a new publisher-signed manifest')
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
