from pathlib import Path, PurePosixPath
import subprocess,tarfile,hashlib,json,sys

artifact=Path(sys.argv[1]).resolve();root=Path(sys.argv[2]).resolve()
meta=json.loads((artifact/'SOURCE.json').read_text())
assert meta['build_commit']=='64db8910d135d123162ed90d772b06831909c9e2'
assert meta['run_id']==41157142308
assert meta['source_tree']=='990d5300185280c6f209d72ddaa0666a07f9d482'
assert meta['source_sha256']=='e83115d0e0523b8667f6d73e664e536273acd29f331f89ff071affc36e325bd8'
assert meta['native_sha256']=='2f911bc2308f5f8e84597ca917c5bc6fbf680e88816ead6dfc8d08adb9b7e220'
archive=artifact/'openomsi-source.tar.gz'
assert hashlib.sha256(archive.read_bytes()).hexdigest()==meta['source_sha256']
assert hashlib.sha256((artifact/'openomsi.exe').read_bytes()).hexdigest()==meta['native_sha256']
seen=set()
with tarfile.open(archive,'r:gz') as source:
    for member in source.getmembers():
        if member.isdir():continue
        assert member.isfile(),('unexpected archive entry',member.name)
        prefix,sep,name=member.name.partition('/')
        parts=PurePosixPath(name).parts
        assert prefix=='openomsi-222' and sep and name and not name.startswith('/')
        assert '..' not in parts and '.git' not in parts
        assert name not in seen
        seen.add(name)
        p=root/name
        p.parent.mkdir(parents=True,exist_ok=True)
        p.write_bytes(source.extractfile(member).read())
        p.chmod(member.mode)
assert len(seen)==1994,len(seen)
tracked=subprocess.check_output(['git','ls-files','-z'],cwd=root).split(b'\0')
for name in tracked:
    if name and name.decode() not in seen:(root/name.decode()).unlink()
subprocess.run(['git','add','--all'],cwd=root,check=True)
tree=subprocess.check_output(['git','write-tree'],cwd=root,text=True).strip()
assert tree==meta['source_tree'],(tree,meta['source_tree'])
print('Verified exact released source tree:',tree,'files:',len(seen))
