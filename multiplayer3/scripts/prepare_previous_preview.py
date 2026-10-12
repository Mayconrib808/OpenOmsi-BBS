import io,base64,gzip,hashlib,json,struct,sys,os,shutil,subprocess,tempfile,zipfile,zlib
from pathlib import Path
root=Path(__file__).resolve().parents[2]/"cloudflare-updates-v3"
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

launcher_sha=hashlib.sha256(launcher).hexdigest()
helper_sha=hashlib.sha256(helper).hexdigest()
print('PREVIOUS_PREVIEW_HASHES='+json.dumps({'launcher':launcher_sha,'helper':helper_sha}))
(repo/'multiplayer3/source/previous_development_generated.go').write_text('package main\nfunc init(){\npreviousV3Adapters["'+launcher_sha+'"]=true\npreviousPluginHosts["'+helper_sha+'"]=true\n}\n')
