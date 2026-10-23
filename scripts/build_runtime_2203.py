#!/usr/bin/env python3
"""Build player components and retain complete P15 host code with current version data.

Apply Bridge-Coop.patch to 865de091f47afffd0f87d45fc178d662c46c1a90,
then supply the delivered Atualizar-Bridge.exe (or its ZIP) as --baseline-update.
No simulator, dedicated server, online update feed or user configuration is built
or modified by this script. Go 1.23.2, Python 3.11+; no C compiler required.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import tempfile
import zipfile

import build
import build_bridge_update
import setup_resources

EXPECTED = {
    'HostAgent.exe': 'a483e435a23b9a74276fb045945ce58357ab5ea99f8759ddc2dddae41a41dba4',
    'CompanyHost.exe': '3d57a4733cbfb8bec677c29d888de339a92ae43c161f9e48ccc9b7a97c7d6cb3',
    'Setup.exe': 'bb8603190bef8aa3f41dc0158149aea5bf647abbbd534e24d42994628e9045d5',
    'PeDePeAdapter/openomsi.exe': 'a4b341f7af27e51ebef8731214ae341e17a8a58739a08ff9c52ddef984ab2ff7',
    'app/OpenOMSI_BCS_Bridge.exe': '242a0de85571728529807b997f6c022212521177ec60aa7dc9433f9f816abe83',
    'app/compat/Omsi.exe': 'e028405534e6545e5d4385c35d1a5e348174c8efbe030dbe6199e54393531320',
    'app/compat/omsi-plugin-host32.exe': '8c39f41ef970c1ce198b8a97b504f29e9a766ae93b48488295b270d7652a25c0',
}
REBUILD = ('Setup.exe', 'PeDePeAdapter/openomsi.exe', 'app/OpenOMSI_BCS_Bridge.exe')
BASE_UPDATER_SHA256 = '7b026ea86b88acef6e18c2be6c8c4f198e954610ee5e23834f86ecdbfb7035e7'


def digest(data):
    return hashlib.sha256(data).hexdigest()


def embedded_payload(data):
    """Locate the existing Go-embedded ZIP using its central-directory offsets."""
    pos = 0
    while (pos := data.find(b'PK\x05\x06', pos)) >= 0:
        try:
            _, _, _, _, _, size, offset, comment = struct.unpack_from('<4s4H2IH', data, pos)
            start = pos - size - offset
            if start >= 0 and data[start:start + 4] == b'PK\x03\x04':
                with zipfile.ZipFile(io.BytesIO(data[start:pos + 22 + comment])) as z:
                    if set(z.namelist()) == set(EXPECTED):
                        return {name: z.read(name) for name in EXPECTED}
        except (ValueError, struct.error, zipfile.BadZipFile):
            pass
        pos += 4
    raise ValueError('Exact seven-component update payload was not found')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--baseline-update', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--go', default='go')
    p.add_argument('--update-sequence', default='2026102102')
    a = p.parse_args()
    output = a.output.resolve()
    if output.exists():
        p.error('Choose a new output directory')
    version = subprocess.check_output([a.go, 'version'], text=True).split()[2]
    if version != 'go1.23.2':
        p.error('Go 1.23.2 is required')
    if zipfile.is_zipfile(a.baseline_update):
        with zipfile.ZipFile(a.baseline_update) as z:
            baseline = z.read('Atualizar-Bridge.exe')
    else:
        baseline = a.baseline_update.read_bytes()
    payload = embedded_payload(baseline)
    # The three other files are always replaced by this source build, never
    # executed. Checking the four retained hashes also permits reproduction
    # directly from this delivered updater without needing a historical ZIP.
    for name in EXPECTED.keys() - set(REBUILD):
        expected = EXPECTED[name]
        if digest(payload[name]) != expected:
            raise ValueError('Baseline component differs: ' + name)
    output.mkdir(parents=True)
    retained = {name: EXPECTED[name] for name in EXPECTED.keys() - set(REBUILD)}
    runtime = output / 'runtime'
    for name, data in payload.items():
        path = runtime / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    provenance = json.loads((ROOT / 'build-support/bridge2203/host-provenance.json').read_text())
    for name, item in provenance['components'].items():
        path = runtime / name
        original = path.read_bytes()
        if digest(original) != item['original_sha256']:
            raise ValueError('Complete P15 baseline differs: ' + name)
        updated = original.replace(b'2.1.9.8', b'2.2.0.3')
        if digest(updated) != item['updated_sha256']:
            raise ValueError('Unexpected host identification change: ' + name)
        for begin in item['version_literal_offsets']:
            if original[begin:begin+7] != b'2.1.9.8': raise ValueError('Host version data layout differs')
        # Every changed byte must be in the four documented read-only version literals.
        allowed = {at+k for at in item['version_literal_offsets'] for k in range(7)}
        if any(a != b and i not in allowed for i,(a,b) in enumerate(zip(original,updated))):
            raise ValueError('Executable host code changed')
        path.write_bytes(updated); retained[name] = digest(updated)
    env = dict(os.environ, GOOS='windows' , GOARCH='386', GO386='sse2', CGO_ENABLED='0',
               GOTOOLCHAIN='local', GO111MODULE='off', GOEXPERIMENT='')
    host_hash = EXPECTED['app/compat/omsi-plugin-host32.exe']
    with tempfile.TemporaryDirectory(prefix='bridge-coop-build-') as temporary:
        work = Path(temporary)
        for name in REBUILD:
            print('Building', name, flush=True)
            target = runtime / name
            flags = '-s -w -H=windowsgui -X=main.bundledPluginHostSHA256=' + host_hash
            if name == 'Setup.exe':
                # Retain the same publisher and move the local sequence forward
                # so the previous published update cannot undo this local patch.
                if not a.update_sequence.isdigit():
                    raise ValueError('Invalid update sequence')
                flags += ' -X=main.bridgeUpdateOrigin=https://openomsi-bbs-updates.maycongamesmaximo.workers.dev -X=main.bridgeUpdateSequence=' + a.update_sequence
                source = work / 'setup'
                source.mkdir()
                for file in build.PROGRAMS[name]:
                    shutil.copy2(build.SOURCE / file, source / file)
                (source / 'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
                resources = setup_resources.prepare(source, build.SOURCE / 'resources')
                args, build_env = ['.'], dict(env, GO111MODULE='on')
            else:
                source, args, build_env = build.SOURCE, build.PROGRAMS[name], env
            subprocess.run([a.go, 'vet', '-unsafeptr=false', *args], cwd=source, env=build_env, check=True)
            subprocess.run([a.go, 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags,
                            '-o', str(target), *args], cwd=source, env=build_env, check=True)
            if name == 'Setup.exe':
                setup_resources.verify_executable(target, resources)
        for name in EXPECTED.keys() - set(REBUILD):
            if digest((runtime / name).read_bytes()) != retained[name]:
                raise ValueError('A preserved component was unexpectedly changed: ' + name)
        generated_payload = build.SOURCE / 'bridge_update_payload.zip'
        if generated_payload.exists():
            raise ValueError('Do not overwrite an existing build payload')
        try:
            with zipfile.ZipFile(generated_payload, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as z:
                for name in EXPECTED:
                    # File mtimes are not part of the program. Fixed metadata
                    # makes the complete installer reproducible as well.
                    entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
                    entry.compress_type = zipfile.ZIP_DEFLATED
                    entry.create_system = 3
                    entry.external_attr = 0o100644 << 16
                    z.writestr(entry, (runtime / name).read_bytes(), compresslevel=9)
            subprocess.run([a.go, 'vet', '-unsafeptr=false', *build_bridge_update.UPDATER_SOURCES],
                           cwd=build.SOURCE, env=env, check=True)
            subprocess.run([a.go, 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w -H=windowsgui',
                            '-o', str(output / 'Atualizar-Bridge.exe'), *build_bridge_update.UPDATER_SOURCES],
                           cwd=build.SOURCE, env=env, check=True)
        finally:
            generated_payload.unlink(missing_ok=True)
    actual = {name: digest((runtime / name).read_bytes()) for name in EXPECTED}
    embedded = embedded_payload((output / 'Atualizar-Bridge.exe').read_bytes())
    if {name: digest(data) for name, data in embedded.items()} != actual:
        raise ValueError('Embedded final payload differs from the built runtime')
    report = {'version': build.VERSION, 'go': version, 'target': 'windows/386',
              'source_base': '865de091f47afffd0f87d45fc178d662c46c1a90',
              'original_2_1_9_8_updater_sha256': BASE_UPDATER_SHA256,
              'input_updater_sha256': digest(baseline), 'rebuilt': list(REBUILD),
              'preserved_unchanged': sorted(set(retained) - set(provenance['components'])),
              'complete_p15_host_code_preserved': provenance,
              'retained_components_sha256': retained,
              'components_sha256': actual, 'updater_sha256': digest((output / 'Atualizar-Bridge.exe').read_bytes()),
              'windows_gameplay_tested': False}
    (output / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n')
    print('Built and verified', output, flush=True)


if __name__ == '__main__':
    main()
