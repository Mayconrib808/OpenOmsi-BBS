"""Layer the bounded passenger/audio hotfix over the exact published 2.2.1.7 source."""
from pathlib import Path
import hashlib,json,subprocess
root=Path('game');support=Path('support');here=support/'build-support/passenger-hotfix2218'
subprocess.run(['python',str((support/'build-support/coop15-0345/prepare.py').resolve())],check=True)
spec=json.loads((here/'source.json').read_text())
def digest(p):return hashlib.sha256(p.read_text(encoding='utf-8').replace('\r\n','\n').encode()).hexdigest() if p.exists() else None
for name,wanted in spec['baseline_files_sha256'].items():assert digest(root/name)==wanted,('Published baseline differs',name)
patch=(here/'changes.patch').resolve()
assert hashlib.sha256(patch.read_bytes()).hexdigest()==spec['patch_sha256']
subprocess.run(['git','apply','--check',str(patch)],cwd=root,check=True)
subprocess.run(['git','apply',str(patch)],cwd=root,check=True)
for name,wanted in spec['result_files_sha256'].items():assert digest(root/name)==wanted,('Hotfix result differs',name)
subprocess.run(['git','diff','--check'],cwd=root,check=True)
print('Verified published 2.2.1.7 baseline and exact 2.2.1.8 passenger/audio overlay.')
