#!/usr/bin/env python3
"""Package the checked 0.3.52 game, launcher and P17 hosts over Hotfix 1."""
from pathlib import Path
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import zipfile

import build
import build_bridge_update
import setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED

ROOT = Path(__file__).resolve().parents[1]
SRC = build.SOURCE
VERSION = '2.2.3.3'
SEQUENCE = 2026110701
ORIGIN = 'https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
COOP = 'OpenOmsi Coop 0.3.41'
NATIVE = '0.3.52-coop.17-candidate'
UPSTREAM = '41d69ab19043656b49d6cad0c34f85f3178503b8'
BASE_SHA = 'bd6b6b3ce5dd14a0e13f41031b02feb66ac7d763fbf87492e92e1fdfb2f249bf'
BASE_COOP = '8c95bba13b1e8b1bedb7a9ae9cd15835f2cadba08e2dfa97f7cf838590db1afc'
OFFICIAL_ZIP = '9ea07150defe034beda044190945a6405c97323b36e8f3b98d2c238fe53e1658'
FIDELITYFX = (
    'amd_fidelityfx_loader_dx12.dll', 'amd_fidelityfx_upscaler_dx12.dll',
    'amd_fidelityfx_loader_vk.dll', 'amd_fidelityfx_upscaler_vk.dll',
    'LICENSE-FidelityFX.md', 'THIRD-PARTY-FidelityFX.md',
)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def file_sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as source:
        for chunk in iter(lambda: source.read(1 << 20), b''):
            digest.update(chunk)
    return digest.hexdigest()


def js(path):
    return json.loads(path.read_text(encoding='utf-8-sig'))


def run(args, **kwargs):
    print('RUN', args[0], args[1:3], flush=True)
    subprocess.run(args, check=True, **kwargs)


def write_distribution(spec):
    text = 'package main\n\nconst bundledCoopOrigin = ' + json.dumps(ORIGIN) + '\n'
    text += 'var bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:' + json.dumps(spec['sha256'])
    text += ',Size:' + str(spec['size']) + ',Prefix:' + json.dumps(spec['prefix']) + ',\nParts:[]coopDistributionPart{\n'
    text += ''.join('{SHA256:' + json.dumps(p['sha256']) + ',Size:' + str(p['size']) + '},\n' for p in spec['parts'])
    text += '},\nFiles:map[string]string{\n'
    text += ''.join(json.dumps(n) + ':' + json.dumps(h) + ',\n' for n, h in sorted(spec['files'].items())) + '},}\n'
    (SRC / 'coop_bundle_manifest.go').write_text(text, encoding='utf-8')


def main():
    parser = argparse.ArgumentParser()
    for name in ('approved', 'native', 'output', 'official'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--native-run', type=int, required=True)
    args = parser.parse_args()
    approved, native, out = args.approved.resolve(), args.native.resolve(), args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    assert build.VERSION == VERSION
    assert subprocess.check_output(['go', 'version'], text=True).split()[2] == 'go1.23.2'
    old, spec = js(approved / 'build-report.json'), js(approved / 'coop-spec.json')
    installer = (approved / 'Atualizar-Bridge.exe').read_bytes()
    assert old['version'] == '2.2.3.2-hotfix.1' and old['sequence'] == 2026110602
    assert sha(installer) == old['updater_sha256'] == BASE_SHA
    assert spec['sha256'] == old['coop_archive_sha256'] == BASE_COOP
    payload = embedded_payload(installer)
    assert set(payload) == set(EXPECTED)
    assert all(sha(contents) == old['components_sha256'][name] for name, contents in payload.items())
    archive = out / 'approved-coop.zip'
    with archive.open('wb') as output:
        for part in spec['parts']:
            data = (approved / 'coop-assets' / (part['sha256'] + '.bin')).read_bytes()
            assert len(data) == part['size'] and sha(data) == part['sha256']
            output.write(data)
    assert archive.stat().st_size == spec['size'] and file_sha(archive) == BASE_COOP
    runtime = out / 'runtime'
    with zipfile.ZipFile(archive) as bundle:
        assert set(bundle.namelist()) == {spec['prefix'] + name for name in spec['files']}
        for name, digest in spec['files'].items():
            dest = (runtime / name).resolve()
            assert dest.is_relative_to(runtime) and not Path(name).is_absolute()
            dest.parent.mkdir(parents=True, exist_ok=True)
            with bundle.open(spec['prefix'] + name) as source, dest.open('wb') as target:
                shutil.copyfileobj(source, target, 1 << 20)
            assert file_sha(dest) == digest, name
    archive.unlink()
    for name, data in payload.items():
        target = runtime / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)

    meta = js(native / 'BUILD-INFO.json')
    commit = meta['source_commit']
    assert len(commit) == 40 and all(c in '0123456789abcdef' for c in commit)
    assert meta['version'] == NATIVE and meta['upstream_commit'] == UPSTREAM and meta['protocol'] == 17
    assert meta['validation'] == 'compiled-and-startup-tested'
    assert int(meta['run_id']) == args.native_run
    assert 'exit code: 0' in (native / 'native-self-test.txt').read_text(encoding='utf-8-sig')
    game = (native / 'openomsi.exe').read_bytes()
    assert game[:2] == b'MZ' and sha(game) == meta['native_sha256']
    steam = file_sha(native / 'steam_api64.dll')
    for folder in (runtime / COOP / 'programa', runtime / 'app/server-dinamico-p17'):
        assert file_sha(folder / 'steam_api64.dll') == steam, 'Unexpected Steam ABI change'
        (folder / 'openomsi.exe').write_bytes(game)
    launcher_files = {
        'openomsi-launcher-app.exe': meta['desktop_sha256'],
        'openomsi-launcher.exe': meta['cli_sha256'],
    }
    for name, digest in launcher_files.items():
        candidate = native / name
        assert file_sha(candidate) == digest
        shutil.copy2(candidate, runtime / COOP / 'programa' / name)

    # The official 0.3.52 Windows release bundles the MIT-listed FSR 3 DLLs.
    # Pull just those six allowlisted files from the hash-pinned official ZIP.
    official = args.official.resolve()
    assert file_sha(official) == OFFICIAL_ZIP
    added_fx = []
    with zipfile.ZipFile(official) as dist:
        for name in FIDELITYFX:
            entries = [p for p in dist.namelist() if Path(p).name == name]
            assert len(entries) == 1, name
            relative = COOP + '/programa/' + name
            with dist.open(entries[0]) as inp, (runtime / relative).open('wb') as dst:
                shutil.copyfileobj(inp, dst, 1 << 20)
            added_fx.append(relative)

    host_dir = runtime / 'app/hosts-p17'
    env_host = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0',
                    GOTOOLCHAIN='local', GO111MODULE='off', GOEXPERIMENT='')
    host_hashes = {}
    for name in ('HostAgent.exe', 'CompanyHost.exe'):
        flags = '-s -w' + (' -H=windowsgui' if name == 'HostAgent.exe' else '')
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags,
             '-o', str(host_dir / name), *build.PROGRAMS[name]], cwd=SRC, env=env_host)
        host_hashes[name] = file_sha(host_dir / name)
    host_manifest = runtime / 'docs/HOST-PESSOAL-2.2.0.3.json'
    info = js(host_manifest)
    info['version'] = VERSION
    for item in info['files']:
        item['sha256'] = file_sha(runtime / item['path'])
    host_manifest.write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')

    source = native / 'openomsi-source.tar.gz'
    assert file_sha(source) == meta['source_sha256']
    added = [COOP + '/source/openomsi-2.2.3.3-source.tar.gz',
             COOP + '/source/SOURCE-2.2.3.3.json', COOP + '/CHANGELOG-2.2.3.3.md']
    shutil.copy2(source, runtime / added[0])
    provenance = dict(bridge_version=VERSION, version=NATIVE, source_commit=commit, commit=commit,
                      run_id=args.native_run, protocol=17, upstream_commit=UPSTREAM, native_sha256=meta['native_sha256'],
                      source_sha256=meta['source_sha256'], windows_gameplay_tested=False, paired_gameplay_tested=False)
    (runtime / added[1]).write_text(json.dumps(provenance, indent=2) + '\n', encoding='utf-8')
    shutil.copy2(ROOT / 'docs/releases/2.2.3.3.md', runtime / added[2])
    marker_path = runtime / 'app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json'
    marker = js(marker_path)
    marker.update(bridge_line=VERSION, source_commit=commit, native_version=NATIVE,
                  native_sha256=meta['native_sha256'], version=NATIVE, protocol=17,
                  openomsi_version='0.3.52', upstream_commit=UPSTREAM)
    marker_path.write_text(json.dumps(marker, indent=2) + '\n', encoding='utf-8')
    info_path = runtime / COOP / 'source/BUILD-INFO.json'
    info = js(info_path)
    info.update(version=NATIVE, bridge_version=VERSION, game_sha256=meta['native_sha256'],
                native_candidate_run=args.native_run, native_candidate_commit=commit, native_build_commit=commit,
                dynamic_p17_sha256=meta['native_sha256'], dynamic_p17_version=NATIVE,
                launcher_sha256=meta['desktop_sha256'], launcher_build_commit=commit, upstream_commit=UPSTREAM)
    info_path.write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')
    checksums = runtime / COOP / 'SHA256.json'
    checksums.write_text(json.dumps({p.relative_to(runtime / COOP).as_posix(): file_sha(p)
                                    for p in sorted((runtime / COOP).rglob('*')) if p.is_file() and p != checksums}, indent=2) + '\n')
    changed = {COOP + '/programa/openomsi.exe', 'app/server-dinamico-p17/openomsi.exe',
               'app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json', COOP + '/source/BUILD-INFO.json', COOP + '/SHA256.json',
               'docs/HOST-PESSOAL-2.2.0.3.json'}
    changed.update(COOP + '/programa/' + name for name in launcher_files)
    changed.update('app/hosts-p17/' + name for name in host_hashes)
    for name, digest in spec['files'].items():
        if name not in changed:
            assert file_sha(runtime / name) == digest, ('Unrelated runtime changed', name)

    files = {name: file_sha(runtime / name) for name in set(spec['files']) | set(added) | set(added_fx)}
    prefix = 'Bridge-2.2.3.3/'
    archive = out / 'coop.zip'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
        for name in sorted(files):
            bundle.write(runtime / name, prefix + name)
    assert archive.stat().st_size <= 512 << 20
    assert sum((runtime / n).stat().st_size for n in files) <= 1536 << 20
    parts, partdir = [], out / 'coop-assets'
    partdir.mkdir()
    with archive.open('rb') as source:
        while data := source.read(16 << 20):
            digest = sha(data)
            (partdir / (digest + '.bin')).write_bytes(data)
            parts.append(dict(sha256=digest, size=len(data)))
    new_spec = dict(files=files, parts=parts, prefix=prefix, sha256=file_sha(archive), size=archive.stat().st_size)
    archive.unlink()
    (out / 'coop-spec.json').write_text(json.dumps(new_spec, indent=2) + '\n')
    write_distribution(new_spec)

    run(['python', str(ROOT / 'scripts/test_complete_host_ui.py'), str(host_dir), '--protocol', '17', '--large-inventory',
         '--server', str(runtime / 'app/server-dinamico-p17/openomsi.exe'), '--report', str(out / 'native-p17-controls.json')])
    assert js(out / 'native-p17-controls.json')['status'] == 'passed'
    env = dict(os.environ, GOOS='windows', GOARCH='386', GO386='sse2', CGO_ENABLED='0', GO111MODULE='on', GOTOOLCHAIN='local', GOEXPERIMENT='')
    pinned = ['PeDePeAdapter/openomsi.exe', 'app/OpenOMSI_BCS_Bridge.exe', 'app/compat/Omsi.exe', 'app/compat/omsi-plugin-host32.exe']
    with tempfile.TemporaryDirectory(prefix='bbs-2233-') as folder:
        work = Path(folder)
        for name in build.PROGRAMS['Setup.exe']:
            shutil.copy2(SRC / name, work / name)
        (work / 'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
        resources = setup_resources.prepare(work, SRC / 'resources')
        flags = '-s -w -H=windowsgui -X=main.bundledPluginHostSHA256=' + sha(payload['app/compat/omsi-plugin-host32.exe'])
        flags += ' -X=main.activationPinnedRuntimeSHA256=' + ':'.join(sha(payload[n]) for n in pinned)
        flags += ' -X=main.bridgeUpdateOrigin=' + ORIGIN + ' -X=main.bridgeUpdateSequence=' + str(SEQUENCE)
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags, '-o', str(runtime / 'Setup.exe'), '.'], cwd=work, env=env)
        setup_resources.verify_executable(runtime / 'Setup.exe', resources)
    run([str(runtime / 'Setup.exe'), '--verify-activation'], cwd=runtime, timeout=90)
    stale = runtime / 'docs/SHA256.txt'
    stale.write_text(old['components_sha256']['Setup.exe'] + '  Setup.exe\n' + ''.join(old['components_sha256'][n] + '  ' + n + '\n' for n in pinned))
    run([str(runtime / 'Setup.exe'), '--verify-activation'], cwd=runtime, timeout=90)
    stale.unlink()
    game_path = runtime / COOP / 'programa/openomsi.exe'
    with game_path.open('r+b') as stream:
        stream.seek(64); original = stream.read(1); stream.seek(64); stream.write(bytes([original[0] ^ 1]))
    assert subprocess.run([str(runtime / 'Setup.exe'), '--verify-activation'], cwd=runtime, timeout=90).returncode != 0
    with game_path.open('r+b') as stream:
        stream.seek(64); stream.write(original)
    for folder in (runtime / COOP / 'programa', runtime / 'app/server-dinamico-p17'):
        run([str(folder / 'openomsi.exe'), '--self-test'], cwd=folder, timeout=90)
    run([str(runtime / 'Setup.exe'), '--gui-smoke'], cwd=runtime, env=dict(os.environ, BRIDGE_SETUP_PREVIEWS=str(out / 'setup-previews')), timeout=120)
    payload['Setup.exe'] = (runtime / 'Setup.exe').read_bytes()
    embedded = SRC / 'bridge_update_payload.zip'
    assert not embedded.exists()
    try:
        with zipfile.ZipFile(embedded, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
            for name, data in sorted(payload.items()):
                bundle.writestr(name, data)
        flags = '-s -w -H=windowsgui -X=main.bridgeUpdateOrigin=' + ORIGIN + ' -X=main.bridgeUpdateSequence=' + str(SEQUENCE)
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags, '-o', str(out / 'Atualizar-Bridge.exe'), *build_bridge_update.UPDATER_SOURCES],
            cwd=SRC, env=dict(env, GOARCH='amd64', GO111MODULE='off'))
    finally:
        embedded.unlink(missing_ok=True)
    installer = (out / 'Atualizar-Bridge.exe').read_bytes()
    assert embedded_payload(installer) == payload and len(installer) < 25 << 20
    assert all(sha(payload[n]) == old['components_sha256'][n] for n in payload if n != 'Setup.exe')
    run(['python', str(ROOT / 'scripts/test_2222_test2_install.py'), str(out), '--baseline', str(approved)], timeout=420)
    report = dict(old)
    report.update(version=VERSION, sequence=SEQUENCE, updater_sha256=sha(installer), installer_size=len(installer),
                  setup_sha256=sha(payload['Setup.exe']), components_sha256={n: sha(b) for n, b in payload.items()},
                  coop_archive_sha256=new_spec['sha256'], native_version=NATIVE, native_candidate_run=args.native_run,
                  native_candidate_commit=commit, native_build_commit=commit, native_source_sha256=meta['source_sha256'],
                  dynamic_p17_sha256=meta['native_sha256'], dynamic_p17_version=NATIVE, native_client_sha256={'openomsi.exe': meta['native_sha256']},
                  upstream_commit=UPSTREAM, passenger_upstream_tag='v0.3.52', passenger_upstream_commit=UPSTREAM,
                  p17_hosts_unchanged=False, p17_hosts_rebuilt_from_source=True, p17_host_hashes=host_hashes,
                  desktop_launcher_unchanged=False, desktop_launcher_sha256=meta['desktop_sha256'],
                  desktop_launcher_cli_sha256=meta['cli_sha256'], desktop_launcher_commit=commit,
                  desktop_launcher_run=args.native_run, actual_updater_first_attempt_windows_tested=True,
                  windows_setup_activation_tested=True, stale_legacy_sha256_activation_tested=True,
                  other_six_components_unchanged=True, signed=False, windows_gameplay_tested=False, paired_gameplay_tested=False)
    # Retire test totals from the predecessor rather than attributing them to 0.3.52.
    for key in ('simulation_tests_passed', 'simulation_tests_ignored',
                'wheelchair_nine_controller_cycles_passed', 'wheelchair_verified_door_bay_cycles'):
        report.pop(key, None)
    (out / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n')
    print('CUMULATIVE 0.3.52 CANDIDATE VERIFIED', report['updater_sha256'], report['installer_size'], len(parts), 'parts', flush=True)


if __name__ == '__main__':
    main()
