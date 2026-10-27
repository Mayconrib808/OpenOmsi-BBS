"""Reproduce genuine 0.3.45 plus the complete published Coop15 and new fixes."""
from pathlib import Path
import hashlib,json,subprocess
root=Path('game');support=Path('support');official=Path('official45')
here=support/'build-support/coop15-0345'
spec=json.loads((here/'source.json').read_text())
def revision(p):return subprocess.check_output(['git','rev-parse','HEAD'],cwd=p,text=True).strip()
assert revision(root)==spec['base_commit']
assert revision(official)==spec['official_target_commit']
subprocess.run(['python',str((support/'build-support/coop15-0344/prepare.py').resolve())],check=True)
for name,digest in spec['baseline_files_sha256'].items():
 data=(root/name).read_text(encoding='utf-8').replace('\r\n','\n').encode()
 assert hashlib.sha256(data).hexdigest()==digest,('Preserved source changed',name)
patch=(here/'changes.patch').resolve()
assert hashlib.sha256(patch.read_bytes()).hexdigest()==spec['patch_sha256']
subprocess.run(['git','apply','--check',str(patch)],cwd=root,check=True)
subprocess.run(['git','apply',str(patch)],cwd=root,check=True)
for name in spec['official_files_exact']:
 assert (root/name).read_text(encoding='utf-8').replace('\r\n','\n')==(official/name).read_text(encoding='utf-8').replace('\r\n','\n'),name
assert not subprocess.check_output(['git','diff','--check'],cwd=root,text=True)
print('Genuine 0.3.45 merged, complete Coop15 retained, researched hailing and new corrections applied.')
