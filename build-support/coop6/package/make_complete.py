from pathlib import Path
import base64
import hashlib
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.request
import zipfile

ROOT = Path.cwd()
SUPPORT = ROOT / 'build-support/coop6/package'
BUILT = ROOT / 'build-output'
FINAL = ROOT / 'final'
PACKAGE = FINAL / 'OpenOMSI_Coop_0.3.38_coop.6_Portatil'
RUN_ID = 39705231489
REPO = 'Mayconrib808/OpenOmsi-BBS'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def write_json(path, obj):
    path.write_text(json.dumps(obj, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')


def download(url, expected):
    with urllib.request.urlopen(url, timeout=120) as response:
        data = response.read()
    if sha(data) != expected:
        raise RuntimeError(f'Download hash mismatch: {url}')
    return data


mode = sys.argv[1]
if mode == 'wait':
    request = urllib.request.Request(
        f'https://api.github.com/repos/{REPO}/actions/runs/{RUN_ID}',
        headers={'Authorization': 'Bearer ' + os.environ['GH_TOKEN'], 'Accept': 'application/vnd.github+json'},
    )
    for _ in range(70):
        with urllib.request.urlopen(request, timeout=45) as response:
            result = json.load(response)
        print('Reviewed executable build:', result['status'], flush=True)
        if result['status'] == 'completed':
            if result['conclusion'] != 'success':
                raise RuntimeError('Executable build did not pass.')
            break
        time.sleep(30)
    else:
        raise RuntimeError('Timed out waiting for the executable build.')
elif mode == 'assemble':
    FINAL.mkdir(exist_ok=True)
    PACKAGE.mkdir(exist_ok=True)
    for name, expected in json.loads((BUILT / 'build-sha256.json').read_text(encoding='utf-8-sig')).items():
        if sha((BUILT / name).read_bytes()) != expected:
            raise RuntimeError(f'Build artifact hash mismatch: {name}')
    shutil.copytree(SUPPORT / 'files', PACKAGE, dirs_exist_ok=True)
    support_info = json.loads((SUPPORT / 'support-sha256.json').read_text())
    opener = base64.b64decode((SUPPORT / 'ABRIR-COOPERATIVO.exe.b64').read_text())
    (PACKAGE / 'ABRIR-COOPERATIVO.exe').write_bytes(opener)
    ico = base64.b64decode((SUPPORT / 'openomsi.ico.b64').read_text())
    (PACKAGE / 'source/portable-launcher/openomsi.ico').write_bytes(ico)
    official = FINAL / 'official.zip'
    official.write_bytes(download(
        'https://github.com/openOMSI-org/openOMSI/releases/download/v0.3.38/openOMSI-0.3.38-windows-x64.zip',
        'b97ec9457a81d55ed01f716dc2e020195abf9a64f7d464dab0dc6478e13df216',
    ))
    with zipfile.ZipFile(official) as archive:
        for relative, expected in support_info.items():
            if relative == 'ABRIR-COOPERATIVO.exe' or relative.endswith('steam_api64.dll'):
                continue
            candidates = [p for p in archive.namelist() if Path(p).name == Path(relative).name]
            if len(candidates) != 1:
                raise RuntimeError(f'Ambiguous official runtime: {relative}')
            data = archive.read(candidates[0])
            if sha(data) != expected:
                raise RuntimeError(f'Official runtime mismatch: {relative}')
            target = PACKAGE / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
    official.unlink()
    steam = download(
        'https://raw.githubusercontent.com/Noxime/steamworks-rs/bfb3326d13c7c6d773adbb1d7c731cc131c433c7/steamworks-sys/lib/steam/redistributable_bin/win64/steam_api64.dll',
        support_info['programa/steam_api64.dll'],
    )
    (PACKAGE / 'programa/steam_api64.dll').write_bytes(steam)
    for relative, expected in support_info.items():
        if sha((PACKAGE / relative).read_bytes()) != expected:
            raise RuntimeError(f'Package runtime mismatch: {relative}')
    shutil.copyfile(BUILT / 'openomsi.exe', PACKAGE / 'programa/openomsi.exe')
    sound = PACKAGE / 'programa/assets/sounds/coop_turnstile_coin.wav'
    sound.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(BUILT / 'coop_turnstile_coin.wav', sound)
    shutil.copyfile(BUILT / 'openomsi-coop6-source.zip', PACKAGE / 'source/openomsi-coop6-source.zip')
    for name in ['build.log', 'regressions.log', 'defender-report.json', 'defender-scan.log', 'overlay-sha256.json']:
        if (BUILT / name).exists():
            shutil.copyfile(BUILT / name, PACKAGE / 'validation' / ('build-' + name))
    output = subprocess.run([sys.executable, str(PACKAGE / 'source/audit_pe.py'), str(PACKAGE)], capture_output=True, text=True)
    (PACKAGE / 'validation/pe-import-audit.json').write_text(output.stdout, encoding='utf-8')
    if output.returncode:
        print(output.stdout, output.stderr)
        raise RuntimeError('Runtime dependency validation failed.')
    game_data = (PACKAGE / 'programa/openomsi.exe').read_bytes()
    if b'0.3.38-coop.6' not in game_data:
        raise RuntimeError('The game does not contain the expected Coop 6 version marker.')
    info = json.loads((SUPPORT / 'build-metadata.json').read_text())
    info['executable_build_run'] = RUN_ID
    info['runtime_sha256'] = {p.relative_to(PACKAGE).as_posix(): sha(p.read_bytes()) for p in PACKAGE.rglob('*') if p.is_file() and p.suffix.lower() in {'.exe', '.dll'}}
    info['source_zip_sha256'] = sha((PACKAGE / 'source/openomsi-coop6-source.zip').read_bytes())
    summaries = re.findall(r'test result: ok\.\s+(\d+) passed; (\d+) failed; (\d+) ignored', (BUILT / 'regressions.log').read_text(errors='replace'))
    if not summaries:
        raise RuntimeError('No successful cooperative regression summary found.')
    passed, failed, ignored = max((tuple(map(int, values)) for values in summaries), key=lambda v: v[0])
    info['cooperative_regressions'] = {'passed': passed, 'failed': failed, 'ignored': ignored}
    info['static_pe_audit_passed'] = True
    info['omitted_unused_legacy_dlls'] = ['libgcc_s_seh-1.dll', 'libstdc++-6.dll', 'libwinpthread-1.dll']
    write_json(PACKAGE / 'source/BUILD-INFO.json', info)
    print('Assembled complete Coop 6 with verified official runtime files.')
elif mode == 'zip':
    info_path = PACKAGE / 'source/BUILD-INFO.json'
    info = json.loads(info_path.read_text())
    info['windows_startup'] = json.loads((PACKAGE / 'validation/windows-startup.json').read_text(encoding='utf-8-sig'))
    info['defender_file_scan'] = json.loads((PACKAGE / 'validation/defender-files.json').read_text(encoding='utf-8-sig'))
    write_json(info_path, info)
    manifest = {p.relative_to(PACKAGE).as_posix(): sha(p.read_bytes()) for p in sorted(PACKAGE.rglob('*')) if p.is_file() and p.name != 'SHA256.json'}
    write_json(PACKAGE / 'SHA256.json', manifest)
    target = FINAL / 'Coop_6_Windows_x64.zip'
    with zipfile.ZipFile(target, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(PACKAGE.rglob('*')):
            if path.is_file():
                archive.write(path, path.relative_to(FINAL))
    with zipfile.ZipFile(target) as archive:
        if archive.testzip() is not None:
            raise RuntimeError('Final ZIP integrity check failed.')
    (FINAL / 'Coop_6_Windows_x64.sha256').write_text(sha(target.read_bytes()) + '  ' + target.name + '\n')
    print('Complete portable ZIP created and integrity checked:', target.stat().st_size)
else:
    raise SystemExit('Unknown packaging step')
