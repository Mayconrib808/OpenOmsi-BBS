"""Package cumulative reproducible Coop13 source overlays, never native binaries or credentials."""
from pathlib import Path
from zipfile import ZipFile, ZIP_DEFLATED
ROOT=Path(__file__).resolve().parents[1]
paths=[
 'build-support/coop6','build-support/driver-share','build-support/character',
 'build-support/greetings','build-support/greetings-hotfix','build-support/greetings-server',
 'build-support/coop10','build-support/coop11','build-support/coop12','build-support/coop13',
 'source/dynamic_host_update.go','source/company_gateway.go',
 'source/bridge_update.go','.github/workflows/coop13-p15.yml'
]
output=ROOT/'out/openomsi-coop13-source.zip'
output.parent.mkdir(parents=True,exist_ok=True)
readme="""OpenOMSI Coop13 reproducible source overlays, not a proprietary or precompiled package.
Upstream: https://github.com/openOMSI-org/openOMSI
Pinned upstream bases:
 game and dedicated P15 (0.3.40) e874276e1bfa22896deb1e00adab0ed7d4f3ce2f
 server P14 (0.3.39) 9eaf95d4caafabe041edcdfc37931b5259c30c8a
 server P15 (0.3.40) e874276e1bfa22896deb1e00adab0ed7d4f3ce2f
For Coop13, apply only build-support/coop13/prepare.py to the pinned 0.3.40 tree.
The complete Windows recipe is .github/workflows/coop13-p15.yml,
at the pinned repository commit from the validated GitHub Actions run.
This ZIP contains patches and scripts, not upstream source contents. Full
corresponding sources are the pinned public upstream tree plus these overlays.
"""
with ZipFile(output,'w',ZIP_DEFLATED,compresslevel=9) as z:
 z.writestr('README-SOURCE.txt',readme)
 for rel in paths:
  p=ROOT/rel
  if p.is_file():
   z.write(p,rel)
  else:
   for f in sorted(p.rglob('*')):
    if f.is_file() and all(q not in ('__pycache__','target','.git') for q in f.relative_to(p).parts):
     if f.suffix.lower() not in ('.exe','.dll','.pdb','.obj','.pyc','.zip'):
      z.write(f,f.relative_to(ROOT).as_posix())
print(str(output))
