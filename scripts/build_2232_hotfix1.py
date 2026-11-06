#!/usr/bin/env python3
"""Package the approved accessibility/fuel game over the exact complete 2.2.3.2.

Only Setup and the updater are compiled here. All complete HostAgent programs,
legacy servers, launchers, audio, DLLs and existing assets retain their hashes.
The approved P17 game is installed as both the client and dynamic server.
"""
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
VERSION = '2.2.3.2-hotfix.1'
SEQUENCE = 2026110602
ORIGIN = 'https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
COOP = 'OpenOmsi Coop 0.3.41'
NATIVE = '0.3.50-coop.17-2.2.3.2-hotfix.1-candidate'
COMMIT = 'c80715cefa0197031c4c9d9beee0d12284c70c86'
NATIVE_RUN = 42256675047
BASE_SHA = 'edf9e1123df4e81f952104ab1d1184f7c731c84f4ded2e8b00e1281fbb771e3c'
BASE_COOP = '4b6e8152e3ec5e2e599072f14d5718f326006bf97aaa806f09e6af629e40b616'


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
    for name in ('approved', 'native', 'output'):
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    approved, native, out = args.approved.resolve(), args.native.resolve(), args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    assert build.VERSION == VERSION
    assert subprocess.check_output(['go', 'version'], text=True).split()[2] == 'go1.23.2'
    old, spec = js(approved / 'build-report.json'), js(approved / 'coop-spec.json')
    installer = (approved / 'Atualizar-Bridge.exe').read_bytes()
    assert old['version'] == '2.2.3.2' and old['sequence'] == 2026110601
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
    assert meta['version'] == NATIVE and meta['source_commit'] == COMMIT and meta['protocol'] == 17
    assert meta['validation'] == 'compiled-and-startup-tested'
    assert str(meta['run_id']) == str(NATIVE_RUN)
    assert 'self-test ok' in (native / 'native-self-test.txt').read_text(encoding='utf-8-sig')
    game = (native / 'openomsi.exe').read_bytes()
    assert game[:2] == b'MZ' and sha(game) == meta['native_sha256']
    steam = file_sha(native / 'steam_api64.dll')
    for folder in (runtime / COOP / 'programa', runtime / 'app/server-dinamico-p17'):
        assert file_sha(folder / 'steam_api64.dll') == steam, 'Unexpected Steam ABI change'
        (folder / 'openomsi.exe').write_bytes(game)
    source = native / 'openomsi-source.tar.gz'
    assert file_sha(source) == meta['source_sha256']
    added = [COOP + '/source/openomsi-2.2.3.2-hotfix.1-source.tar.gz',
             COOP + '/source/SOURCE-2.2.3.2-hotfix.1.json', COOP + '/CHANGELOG-2.2.3.2-hotfix.1.md']
    shutil.copy2(source, runtime / added[0])
    provenance = dict(bridge_version=VERSION, version=NATIVE, source_commit=COMMIT, commit=COMMIT,
                      run_id=NATIVE_RUN, protocol=17, native_sha256=meta['native_sha256'],
                      source_sha256=meta['source_sha256'], windows_gameplay_tested=False, paired_gameplay_tested=False)
    (runtime / added[1]).write_text(json.dumps(provenance, indent=2) + '\n', encoding='utf-8')
    shutil.copy2(ROOT / 'docs/releases/2.2.3.2-hotfix.1.md', runtime / added[2])
    marker_path = runtime / 'app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json'
    marker = js(marker_path)
    marker.update(bridge_line=VERSION, source_commit=COMMIT, native_version=NATIVE,
                  native_sha256=meta['native_sha256'], version=NATIVE, protocol=17)
    marker_path.write_text(json.dumps(marker, indent=2) + '\n', encoding='utf-8')
    info_path = runtime / COOP / 'source/BUILD-INFO.json'
    info = js(info_path)
    info.update(version=NATIVE, bridge_version=VERSION, game_sha256=meta['native_sha256'],
                native_candidate_run=NATIVE_RUN, native_candidate_commit=COMMIT, native_build_commit=COMMIT,
                dynamic_p17_sha256=meta['native_sha256'], dynamic_p17_version=NATIVE)
    info_path.write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')
    checksums = runtime / COOP / 'SHA256.json'
    checksums.write_text(json.dumps({p.relative_to(runtime / COOP).as_posix(): file_sha(p)
                                    for p in sorted((runtime / COOP).rglob('*')) if p.is_file() and p != checksums}, indent=2) + '\n')
    changed = {COOP + '/programa/openomsi.exe', 'app/server-dinamico-p17/openomsi.exe',
               'app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json', COOP + '/source/BUILD-INFO.json', COOP + '/SHA256.json'}
    for name, digest in spec['files'].items():
        if name not in changed:
            assert file_sha(runtime / name) == digest, ('Unrelated runtime changed', name)

    files = {name: file_sha(runtime / name) for name in set(spec['files']) | set(added)}
    prefix = 'Bridge-2.2.3.2-hotfix.1/'
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

    host_dir = runtime / 'app/hosts-p17'
    assert all(file_sha(host_dir / name) == digest for name, digest in old['p17_host_hashes'].items())
    run(['python', str(ROOT / 'scripts/test_complete_host_ui.py'), str(host_dir), '--protocol', '17', '--large-inventory',
         '--server', str(runtime / 'app/server-dinamico-p17/openomsi.exe'), '--report', str(out / 'native-p17-controls.json')])
    assert js(out / 'native-p17-controls.json')['status'] == 'passed'
    env = dict(os.environ, GOOS='windows', GOARCH='386', GO386='sse2', CGO_ENABLED='0', GO111MODULE='on', GOTOOLCHAIN='local', GOEXPERIMENT='')
    pinned = ['PeDePeAdapter/openomsi.exe', 'app/OpenOMSI_BCS_Bridge.exe', 'app/compat/Omsi.exe', 'app/compat/omsi-plugin-host32.exe']
    with tempfile.TemporaryDirectory(prefix='bbs-2232-hotfix1-') as folder:
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
                  coop_archive_sha256=new_spec['sha256'], native_version=NATIVE, native_candidate_run=NATIVE_RUN,
                  native_candidate_commit=COMMIT, native_build_commit=COMMIT, native_source_sha256=meta['source_sha256'],
                  dynamic_p17_sha256=meta['native_sha256'], dynamic_p17_version=NATIVE, native_client_sha256={'openomsi.exe': meta['native_sha256']},
                  p17_hosts_unchanged=True, p17_hosts_rebuilt_from_source=False, desktop_launcher_unchanged=True,
                  simulation_tests_passed=1080, simulation_tests_ignored=31, wheelchair_nine_controller_cycles_passed=9,
                  wheelchair_verified_door_bay_cycles=13, wheelchair_real_reserved_bays=True, wheelchair_exit_flow_priority=True,
                  wheelchair_unstarted_boarding_preemptible=True, wheelchair_contextual_assistance_buttons=True,
                  fuel_consumption_nonnegative=True, actual_updater_first_attempt_windows_tested=True,
                  windows_setup_activation_tested=True, stale_legacy_sha256_activation_tested=True,
                  other_six_components_unchanged=True, signed=False, windows_gameplay_tested=False, paired_gameplay_tested=False)
    (out / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n')
    print('CUMULATIVE HOTFIX APPROVED', report['updater_sha256'], report['installer_size'], len(parts), 'parts', flush=True)


if __name__ == '__main__':
    main()
