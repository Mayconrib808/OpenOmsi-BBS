from pathlib import Path
import hashlib,json,subprocess
support=Path(__file__).resolve().parent
root=support.parents[2]
game=root/'game'
spec=json.loads((support/'source-sha256.json').read_text())
sha=lambda data:hashlib.sha256(data).hexdigest()
for name,item in spec.items():
    p=game/name
    if item['before'] is None:
        if p.exists():raise ValueError('Unexpected existing source: '+name)
    else:
        data=p.read_bytes().replace(b'\r\n',b'\n')
        if sha(data)!=item['before']:raise ValueError('Source baseline differs: '+name)
        p.write_bytes(data)
patch=(support.parent/'greetings/greetings.patch').read_bytes().replace(b'\r\n',b'\n')
subprocess.run(['git','apply','--whitespace=nowarn','-'],input=patch,cwd=game,check=True)
for name,item in spec.items():
    p=game/name
    data=p.read_bytes().replace(b'\r\n',b'\n')
    if sha(data)!=item['after']:raise ValueError('Patched source differs: '+name)
    p.write_bytes(data)
out=root/'out';out.mkdir(exist_ok=True)
(out/'greetings-source-sha256.json').write_text(json.dumps(spec,indent=2)+'\n')
print('Applied oncoming bus greetings and preserved dynamic server streaming.')
