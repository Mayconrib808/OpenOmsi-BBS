"""Hash-checked full vehicle authority candidate; cumulative hotfix must precede it."""
from pathlib import Path
import hashlib,json,subprocess,sys
support=Path(__file__).resolve().parent
root=support.parents[2];game=root/'game'
sha=lambda b:hashlib.sha256(b).hexdigest()
mode=sys.argv[1] if len(sys.argv)>1 else 'game'
assert mode in ('game','server','server15')
applied={}
for group in (('shared','game') if mode=='game' else (('p15',) if mode=='server15' else ('shared',))):
 spec=json.loads((support/(group+'-source.json')).read_text())
 for name,item in spec.items():
  p=game/name
  if item['before'] is None:assert not p.exists(),name
  else:
   data=p.read_bytes().replace(b'\r\n',b'\n');assert sha(data)==item['before'],'Unexpected source baseline: '+name;p.write_bytes(data)
 subprocess.run(['git','apply','--whitespace=nowarn','-'],cwd=game,input=(support/(group+'.patch')).read_bytes(),check=True)
 for name,item in spec.items():assert sha((game/name).read_bytes().replace(b'\r\n',b'\n'))==item['after'],name
 applied.update(spec)
out=root/'out';out.mkdir(exist_ok=True)
(out/'coop11-source.json').write_text(json.dumps(applied,indent=2)+'\n')
print('Prepared full vehicle authority candidate; Windows multiplayer gameplay remains unverified.')
