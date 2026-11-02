"""Extract only authenticated P16 companions; never replace complete legacy hosts."""
import argparse, hashlib, io, json, shutil, zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HOSTS = {
    'HostAgent.exe': '697aaf26a7fe1fa355c6385a1f609875c56cb5d8385bee35f2aaaba203a55d75',
    'CompanyHost.exe': '45ac05a46126ec2e870d1310929639ae9c6843c7b0127cb5743da50fc134e4f4',
}
sha = lambda b: hashlib.sha256(b).hexdigest()

def stage(candidate, output, server_output=None):
    report = json.loads((candidate / 'build-report.json').read_bytes())
    spec = json.loads((candidate / 'coop-spec.json').read_bytes())
    assert report['p16_host_hashes'] == HOSTS
    assert report['complete_hosts_unchanged'] and report['new_p16_host_ui_tested']
    controls = json.loads((candidate / 'native-p16-controls.json').read_bytes())
    assert controls['status'] == 'passed' and controls['unrelated_config_preserved']
    chunks = []
    for part in spec['parts']:
        data = (candidate / 'coop-assets' / (part['sha256'] + '.bin')).read_bytes()
        assert len(data) == part['size'] and sha(data) == part['sha256']
        chunks.append(data)
    data = b''.join(chunks)
    assert len(data) == spec['size'] and sha(data) == spec['sha256'] == report['coop_archive_sha256']
    output.mkdir(parents=True, exist_ok=True)
    files = {}
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        for name, digest in HOSTS.items():
            rel = 'app/hosts-p16/' + name
            blob = archive.read(spec['prefix'] + rel)
            assert sha(blob) == digest == spec['files'][rel]
            path = output / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(blob)
            files[rel] = digest
        if server_output:
            for rel in spec['files']:
                if rel.startswith('app/server-dinamico-p16/') and (rel.endswith('.dll') or rel.endswith('/openomsi.exe')):
                    blob = archive.read(spec['prefix'] + rel)
                    assert sha(blob) == spec['files'][rel]
                    path = server_output / Path(rel).name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(blob)
    for path in (ROOT / 'build-support/p16-host-repair').iterdir():
        if path.is_file(): shutil.copy2(path, output / path.name)
    shutil.copy2(ROOT / 'LICENSE', output / 'LICENSE')
    proof = output / 'validacao'
    proof.mkdir(exist_ok=True)
    shutil.copy2(candidate / 'native-p16-controls.json', proof / 'native-p16-controls.json')
    manifest = dict(repair='P16-host-manager-20261102', files=files,
                    source_commit='742df3b8deb31910cd35ed1e08f43c0759ccf633',
                    source_url='https://github.com/Mayconrib808/OpenOmsi-BBS/tree/742df3b8deb31910cd35ed1e08f43c0759ccf633',
                    candidate_run_id=41538116149, candidate_artifact_id=13184639904,
                    legacy_hosts_replaced=False, native_runtime_replaced=False,
                    actual_host_ui_tested=True, user_gameplay_tested=False)
    (output / 'P16-HOST-REPAIR.json').write_text(json.dumps(manifest, indent=2)+'\n')

if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('candidate', type=Path)
    p.add_argument('output', type=Path)
    p.add_argument('--server-output', type=Path)
    a = p.parse_args()
    stage(a.candidate, a.output, a.server_output)
