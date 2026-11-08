#!/usr/bin/env python3
"""Package the BCS input fix over the exact released 2.2.3.3 runtime."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

import build_bridge_update
import setup_resources
from build_2233 import build, ROOT, SRC, ORIGIN, COOP, UPSTREAM, file_sha, js, run, sha, write_distribution
from build_runtime_2203 import embedded_payload, EXPECTED

VERSION = '2.2.3.3-hotfix.1'
SEQUENCE = 2026110801
NATIVE = '0.3.52-coop.17-2.2.3.3-hotfix.1'
COMMIT = '4d4ada3423370b9287415e9584d0b596d8d65994'
BASE_INSTALLER = 'b963278bb5a651c483c691aadf11c9d2bce3e08cbc58b7a710aeeef89d0beaae'
BASE_COOP = '894b857be73dce64eff9fb957de1444c60ccd0fa16b866e789306e8d60190159'
ADAPTERS = ('PeDePeAdapter/openomsi.exe', 'app/OpenOMSI_BCS_Bridge.exe')


def main():
    parser = argparse.ArgumentParser()
    for name in ('approved', 'native', 'output'):
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    approved, native, out = args.approved.resolve(), args.native.resolve(), args.output.resolve()
    out.mkdir(parents=True)
    assert build.VERSION == VERSION
    assert subprocess.check_output(['go', 'version'], text=True).split()[2] == 'go1.23.2'
    assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=native, text=True).strip() == COMMIT
    previous, spec = js(approved / 'build-report.json'), js(approved / 'coop-spec.json')
    installer = (approved / 'Atualizar-Bridge.exe').read_bytes()
    assert previous['version'] == '2.2.3.3' and previous['sequence'] == 2026110701
    assert sha(installer) == previous['updater_sha256'] == BASE_INSTALLER
    assert spec['sha256'] == previous['coop_archive_sha256'] == BASE_COOP
    payload = embedded_payload(installer)
    assert set(payload) == set(EXPECTED)
    assert all(sha(data) == previous['components_sha256'][name] for name, data in payload.items())

    archive = out / 'coop.zip'
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
            target = (runtime / name).resolve()
            assert target.is_relative_to(runtime) and not Path(name).is_absolute()
            target.parent.mkdir(parents=True, exist_ok=True)
            with bundle.open(spec['prefix'] + name) as source, target.open('wb') as dest:
                shutil.copyfileobj(source, dest, 1 << 20)
            assert file_sha(target) == digest
    archive.unlink()
    for name, data in payload.items():
        (runtime / name).parent.mkdir(parents=True, exist_ok=True)
        (runtime / name).write_bytes(data)

    game = runtime / COOP / 'programa/openomsi.exe'
    steam = game.with_name('steam_api64.dll')
    assert file_sha(steam) == file_sha(native / 'assets/steam_redist/steam_api64.dll')
    shutil.copy2(native / 'target/release/openomsi.exe', game)
    run([str(game), '--self-test'], cwd=game.parent, timeout=90)
    source = runtime / COOP / 'source'
    source_name = 'openomsi-' + VERSION + '-source.tar.gz'
    shutil.copy2(native / 'openomsi-source.tar.gz', source / source_name)
    source_sha, game_sha = file_sha(source / source_name), file_sha(game)
    run_id = int(os.environ['GITHUB_RUN_ID'])
    info = js(source / 'BUILD-INFO.json')
    info.update(bridge_version=VERSION, version=NATIVE, native_candidate_commit=COMMIT, native_build_commit=COMMIT,
                native_candidate_run=run_id, game_sha256=game_sha, upstream_commit=UPSTREAM)
    (source / 'BUILD-INFO.json').write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')
    source_marker = 'SOURCE-' + VERSION + '.json'
    provenance = dict(bridge_version=VERSION, version=NATIVE, source_commit=COMMIT, upstream_commit=UPSTREAM,
                      source_sha256=source_sha, native_sha256=game_sha, protocol=17, run_id=run_id)
    (source / source_marker).write_text(json.dumps(provenance, indent=2) + '\n', encoding='utf-8')
    checksums = runtime / COOP / 'SHA256.json'
    checksums.write_text(json.dumps({p.relative_to(runtime / COOP).as_posix(): file_sha(p)
                                    for p in sorted((runtime / COOP).rglob('*')) if p.is_file() and p != checksums}, indent=2) + '\n', encoding='utf-8')
    changes = {game, source / 'BUILD-INFO.json', source / source_marker, source / source_name, checksums}
    for name, digest in spec['files'].items():
        if runtime / name not in changes:
            assert file_sha(runtime / name) == digest, name

    files = dict(spec['files'])
    files.update({p.relative_to(runtime).as_posix(): file_sha(p) for p in changes})
    prefix = 'Bridge-' + VERSION + '/'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
        for name in sorted(files):
            bundle.write(runtime / name, prefix + name)
    assert archive.stat().st_size <= 512 << 20
    assert sum((runtime / name).stat().st_size for name in files) <= 1536 << 20
    partdir = out / 'coop-assets'
    partdir.mkdir()
    parts = []
    with archive.open('rb') as inp:
        while data := inp.read(16 << 20):
            digest = sha(data)
            (partdir / (digest + '.bin')).write_bytes(data)
            parts.append(dict(sha256=digest, size=len(data)))
    new_spec = dict(files=files, parts=parts, prefix=prefix, sha256=file_sha(archive), size=archive.stat().st_size)
    archive.unlink()
    (out / 'coop-spec.json').write_text(json.dumps(new_spec, indent=2) + '\n', encoding='utf-8')
    write_distribution(new_spec)

    env = dict(os.environ, GOOS='windows', GOARCH='386', GO386='sse2', CGO_ENABLED='0',
               GO111MODULE='off', GOTOOLCHAIN='local', GOEXPERIMENT='')
    for name in ADAPTERS:
        flags = '-s -w' + (' -H=windowsgui' if name == 'PeDePeAdapter/openomsi.exe' else '')
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags,
             '-o', str(runtime / name), *build.PROGRAMS[name]], cwd=SRC, env=env)
        payload[name] = (runtime / name).read_bytes()
    pinned = (*ADAPTERS, 'app/compat/Omsi.exe', 'app/compat/omsi-plugin-host32.exe')
    update_flags = f' -X=main.bridgeUpdateOrigin={ORIGIN} -X=main.bridgeUpdateSequence={SEQUENCE}'
    with tempfile.TemporaryDirectory(prefix='bbs-2233-input-') as folder:
        work = Path(folder)
        for name in build.PROGRAMS['Setup.exe']:
            shutil.copy2(SRC / name, work / name)
        (work / 'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n', encoding='utf-8')
        resources = setup_resources.prepare(work, SRC / 'resources')
        flags = '-s -w -H=windowsgui' + update_flags
        flags += ' -X=main.bundledPluginHostSHA256=' + sha(payload[pinned[-1]])
        flags += ' -X=main.activationPinnedRuntimeSHA256=' + ':'.join(sha(payload[name]) for name in pinned)
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags, '-o', str(runtime / 'Setup.exe'), '.'],
            cwd=work, env=dict(env, GO111MODULE='on'))
        setup_resources.verify_executable(runtime / 'Setup.exe', resources)
    run([str(runtime / 'Setup.exe'), '--verify-activation'], cwd=runtime, timeout=90)
    payload['Setup.exe'] = (runtime / 'Setup.exe').read_bytes()
    embedded = SRC / 'bridge_update_payload.zip'
    assert not embedded.exists()
    try:
        with zipfile.ZipFile(embedded, 'x', zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
            for name, data in sorted(payload.items()):
                bundle.writestr(name, data)
        run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w -H=windowsgui' + update_flags,
             '-o', str(out / 'Atualizar-Bridge.exe'), *build_bridge_update.UPDATER_SOURCES],
            cwd=SRC, env=dict(env, GOARCH='amd64'))
    finally:
        embedded.unlink(missing_ok=True)
    installer = (out / 'Atualizar-Bridge.exe').read_bytes()
    assert embedded_payload(installer) == payload and len(installer) < 25 << 20
    run(['python', str(ROOT / 'scripts/test_2222_test2_install.py'), str(out), '--baseline', str(approved)], timeout=420)
    report = dict(version=VERSION, sequence=SEQUENCE, native_version=NATIVE, native_build_commit=COMMIT,
                  native_source_sha256=source_sha, native_game_sha256=game_sha, native_candidate_run=run_id,
                  upstream_commit=UPSTREAM, protocol=17, setup_sha256=sha(payload['Setup.exe']),
                  updater_sha256=sha(installer), installer_size=len(installer), coop_archive_sha256=new_spec['sha256'],
                  components_sha256={name: sha(data) for name, data in payload.items()},
                  p17_host_hashes=previous['p17_host_hashes'], control_profile_regression_tested_on_windows=True,
                  actual_updater_first_attempt_windows_tested=True, signed=False, windows_gameplay_tested=False)
    (out / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')


if __name__ == '__main__':
    main()
