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
    elif sha(p.read_bytes())!=item['before']:raise ValueError('Source baseline differs: '+name)
subprocess.run(['git','apply','--whitespace=nowarn',str(support/'greetings.patch')],cwd=game,check=True)
for name,item in spec.items():
    if sha((game/name).read_bytes())!=item['after']:raise ValueError('Patched source differs: '+name)
out=root/'out';out.mkdir(exist_ok=True)
(out/'greetings-source-sha256.json').write_text(json.dumps(spec,indent=2)+'\n')
print('Applied oncoming bus greetings and preserved dynamic server streaming.')
