#!/usr/bin/env python3
"""Verify an exact native source export and the complete, file-disjoint patch series."""
from pathlib import Path
from datetime import datetime, timezone
import os, re, subprocess, json, hashlib, tempfile, stat

HERE=Path(__file__).resolve().parent
ROOT=HERE.parent
def git(*args,env=None):
    return subprocess.check_output(['git',*args],cwd=ROOT,env=env,stderr=subprocess.STDOUT)
def sha(b): return hashlib.sha256(b).hexdigest()
def sections(b):
    blocks=re.findall(rb'^diff --git .*?(?=^diff --git |\Z)',b,flags=re.S|re.M)
    assert b''.join(blocks)==b,'unparsed patch content'
    result={}
    for block in blocks:
        path=block.split(b'\n',1)[0].decode().split(' b/',1)[1]
        assert path not in result,('duplicate patch path',path)
        result[path]=block
    return result

meta=json.loads((HERE/'provenance.json').read_text())
files=json.loads((HERE/'source-files.json').read_text())
exact=meta['exact_source_commit']; expected=meta['exact_source_tree']
assert git('rev-parse',exact+'^{tree}').decode().strip()==expected
for entry in files:
    p=ROOT/entry['path']
    data=p.read_bytes()
    assert len(data)==entry['size'] and sha(data)==entry['sha256'],('source bytes changed',entry['path'])
    blob=hashlib.sha1(b'blob '+str(len(data)).encode()+b'\0'+data).hexdigest()
    assert blob==entry['blob_sha1'],('blob changed',entry['path'])
    if os.name!='nt':
        assert bool(p.stat().st_mode&stat.S_IXUSR)==(entry['mode']=='100755'),('file mode changed',entry['path'])

listed=(HERE/'series.txt').read_text().splitlines()
assert listed==[p['path'] for p in meta['patches']]
complete=sections((HERE/'all-native-vs-official-0.3.46.patch').read_bytes())
partition={}
for part in meta['patches']:
    raw=(HERE/part['path']).read_bytes()
    assert sha(raw)==part['sha256'] and len(raw)==part['size'],('patch changed',part['path'])
    members=sections(raw)
    assert len(members)==part['files']
    assert not partition.keys()&members.keys(),('overlapping feature files',part['group'])
    partition.update(members)
assert complete==partition,'feature partitions omit or change a complete file diff'

try:git('cat-file','-e',meta['official_base']+'^{commit}')
except subprocess.CalledProcessError:
    raise SystemExit('Official base object missing. Run: git fetch https://github.com/openOMSI-org/openOMSI.git '+meta['official_base'])
with tempfile.TemporaryDirectory(prefix='openomsi-review-index-') as temp:
    env=os.environ.copy();env['GIT_INDEX_FILE']=str(Path(temp)/'index')
    git('read-tree',meta['official_base'],env=env)
    for part in meta['patches']:
        git('apply','--cached','--whitespace=nowarn',str(HERE/part['path']),env=env)
    reproduced=git('write-tree',env=env).decode().strip()
assert reproduced==expected,('patch series does not reproduce the published source',reproduced,expected)
result=dict(verified_at_utc=datetime.now(timezone.utc).isoformat(),source_files=len(files),
    changed_files=len(partition),patches=len(meta['patches']),exact_source_commit=exact,
    expected_source_tree=expected,reproduced_source_tree=reproduced,
    source_sha256s_verified=True,patches_file_disjoint=True,patch_roundtrip_passed=True,
    new_rust_compilation_performed=bool(meta.get("revision_build_record",{}).get("windows_build_passed")),
    windows_build_run=meta.get("revision_build_record",{}).get("run_id"),gameplay_tested=False,fps_or_vram_measured=False)
(HERE/'verification.json').write_text(json.dumps(result,indent=2)+'\n',encoding='utf-8')
print(json.dumps(result,indent=2))
