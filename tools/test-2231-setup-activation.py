#!/usr/bin/env python3
"""Reproduce the Setup activation integrity gate on the exact published 2.2.3.1 ZIP."""
from pathlib import Path
import sys, json, shutil, hashlib, zipfile, subprocess, tempfile, os

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
import build,setup_resources

approved=ROOT/'approved-new'
runtime=ROOT/'activation-runtime'
spec=json.loads((approved/'coop-spec.json').read_text(encoding='utf-8'))
archive=approved/'OpenOMSI-BCS-Bridge-2.2.3.1-FULL.zip'
assert archive.is_file(), 'Missing approved published archive'
sha=lambda b:hashlib.sha256(b).hexdigest()
with zipfile.ZipFile(archive) as z:
  assert 'docs/SHA256.txt' not in z.namelist(), 'Unexpected legacy SHA256 in original archive'
  for entry in z.infolist():
    if entry.is_dir():continue
    dest=(runtime/entry.filename).resolve()
    assert dest.is_relative_to(runtime.resolve()),'unsafe published archive entry'
    dest.parent.mkdir(parents=True,exist_ok=True)
    with z.open(entry) as r,dest.open('wb') as w:shutil.copyfileobj(r,w)

# Build the exact source-stamped Go manifest that ships in the 2.2.3.1 updater.
prefix='Bridge-2.2.3.1/'
origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
manifest='package main\n\nconst bundledCoopOrigin = '+json.dumps(origin)+'\nvar bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:'+json.dumps(spec['sha256'])+',Size:'+str(spec['size'])+',Prefix:'+json.dumps(prefix)+',\nParts:[]coopDistributionPart{\n'
manifest+=''.join('{SHA256:'+json.dumps(p['sha256'])+',Size:'+str(p['size'])+'},\n' for p in spec['parts'])+'},\nFiles:map[string]string{\n'
manifest+=''.join(json.dumps(n)+':'+json.dumps(v)+',\n' for n,v in sorted(spec['files'].items()))+'},}\n'

pinned=['PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe','app/compat/omsi-plugin-host32.exe']
for name in pinned:
  assert (runtime/name).is_file(),f'Missing required Bridge binary: {name}'
pins=':'.join(sha((runtime/n).read_bytes()) for n in pinned)
game='OpenOmsi Coop 0.3.41/programa/openomsi.exe'
assert sha((runtime/game).read_bytes())==spec['files'][game], 'Published game does not match signed manifest'
assert build.VERSION=='2.2.3.1','Wrong Setup source version'

with tempfile.TemporaryDirectory(prefix='setup2231-') as folder:
  work=Path(folder)
  for source in build.PROGRAMS['Setup.exe']:
    if source=='coop_bundle_manifest.go':
      (work/source).write_text(manifest,encoding='utf-8')
    else:
      shutil.copy2(build.SOURCE/source,work/source)
  (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n',encoding='utf-8')
  resources=setup_resources.prepare(work,build.SOURCE/'resources')
  flags='-s -w -H=windowsgui -X=main.bundledPluginHostSHA256='+sha((runtime/'app/compat/omsi-plugin-host32.exe').read_bytes())
  flags+=' -X=main.activationPinnedRuntimeSHA256='+pins
  flags+=' -X=main.bridgeUpdateOrigin='+origin+' -X=main.bridgeUpdateSequence=2026110501'
  env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GO111MODULE='on',GOTOOLCHAIN='local',GOEXPERIMENT='')
  executable=runtime/'Setup.exe'
  subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(executable.resolve()),'.'],cwd=work,env=env,check=True)
  setup_resources.verify_executable(executable,resources)

print('Running actual patched Windows Setup.exe --verify-activation on published package',flush=True)
subprocess.run([str((runtime/'Setup.exe').resolve()),'--verify-activation'],cwd=runtime,check=True,timeout=150)

# Tampering a P17 game binary MUST continue to fail closed.
target=runtime/game
with target.open('r+b') as output:
  output.seek(64)
  byte=output.read(1)
  output.seek(64)
  output.write(bytes([byte[0]^1]))
result=subprocess.run([str((runtime/'Setup.exe').resolve()),'--verify-activation'],cwd=runtime,timeout=150)
if result.returncode==0:raise RuntimeError('CORRUPTED game was incorrectly accepted')
print('PASS: verified published Coop and Bridge runtime; corrupted native game rejected',flush=True)
