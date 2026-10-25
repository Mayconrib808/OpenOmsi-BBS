"""Keep the complete released P15 host. Change only the audited clock retry immediate.
Corresponding full P15 source is unavailable (host-provenance.json). Do not rebuild
these hosts from the older source directory. Every other code byte is conserved.
"""
import hashlib,json,struct
from pathlib import Path
SPECS={
 'HostAgent.exe':dict(sha='1a5d856e9c77a8ae13ccce50c9a0dc82bce742dce566bc5b3ec1306b1d8804b4',function=0x341430,versions=[4117722,4144852,4189014]),
 'CompanyHost.exe':dict(sha='90bf9c7eeda4f03f9d22417df48a25d5216887004f68f014a818c90b023d96f1',function=0x314950,versions=[3932503])}

def patch(name,original):
 s=SPECS[name];assert hashlib.sha256(original).hexdigest()==s['sha'], 'Complete P15 baseline differs: '+name
 b=bytearray(original);f=s['function']
 # pending && !synced, increment failures, compare with 2, signed jl to continuation.
 before=bytes.fromhex('0fb64f2884c9746784c075678b4f2c41894f2c83f9020f8cb1000000')
 assert b[f+0x4cd:f+0x4e9]==before
 assert b[f+0x540:f+0x547]==bytes.fromhex('c7472c00000000'), 'Synchronized sample must reset failures'
 # Confirm this is the branch producing the exact observed error, via its PE address.
 pe=struct.unpack_from('<I',b,0x3c)[0];opt=pe+24;base=struct.unpack_from('<I',b,opt+28)[0];ss=struct.unpack_from('<H',b,pe+20)[0]
 def file_offset(va):
  for i in range(struct.unpack_from('<H',b,pe+6)[0]):
   o=opt+ss+40*i;_,rva,size,raw=struct.unpack_from('<4I',b,o+8)
   if base+rva<=va<base+rva+size:return raw+va-base-rva
  raise AssertionError('Unmapped error address')
 assert b[f+0x4e9:f+0x4eb]==bytes.fromhex('8d05')
 a=file_offset(struct.unpack_from('<I',b,f+0x4eb)[0]);error=b'the server accepted clock commands but did not synchronize; stopping the session'
 assert b[a:a+len(error)]==error
 b[f+0x4e2]=36 # 36 consecutive five-second samples, instead of two; finite recovery.
 allowed={f+0x4e2}
 for a in s['versions']:
  assert b[a:a+7]==b'2.2.0.3';b[a:a+7]=b'2.2.0.8';allowed.add(a+6)
 changed={i for i,(x,y) in enumerate(zip(original,b)) if x!=y}
 assert changed==allowed and len(b)==len(original)
 report=dict(original_sha256=s['sha'],patched_sha256=hashlib.sha256(b).hexdigest(),changed_offsets=sorted(changed),clock_retry_limit=36,all_other_bytes_preserved=True,complete_p15_baseline=True,full_host_source_recompiled=False)
 return bytes(b),report

if __name__=='__main__':
 import argparse
 p=argparse.ArgumentParser();p.add_argument('directory',type=Path);a=p.parse_args();report={}
 for name in SPECS:
  path=a.directory/name;b,r=patch(name,path.read_bytes());path.write_bytes(b);report[name]=r
 (a.directory/'complete-host-clock-patch.json').write_text(json.dumps(report,indent=2)+'\n')
 print('Complete P15 hosts conserved; only audited clock retry immediate and same-length version identifiers changed.')
