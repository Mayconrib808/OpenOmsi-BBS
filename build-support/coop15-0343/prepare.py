from pathlib import Path
import hashlib,json,subprocess
root=Path('game')
info=json.loads(Path('support/build-support/coop15-0343/source.json').read_text())
patch=Path('support/build-support/coop15-0343/cumulative.patch')
assert hashlib.sha256(patch.read_bytes()).hexdigest()==info['patch_sha256']
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()==info['upstream_commit']
subprocess.run(['git','apply','--check',str(patch.resolve())],cwd=root,check=True)
subprocess.run(['git','apply',str(patch.resolve())],cwd=root,check=True)
assert not subprocess.check_output(['git','diff','--check'],cwd=root,text=True)
print('Genuine upstream 0.3.43 plus all previous Coop/passenger/fuel corrections and street-facing hail arm applied.')
