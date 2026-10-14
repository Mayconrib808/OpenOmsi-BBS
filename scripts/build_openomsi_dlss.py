#!/usr/bin/env python3
"""Build an isolated openOMSI 0.2.50 DLSS client; never publish an automatic update."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / 'experimental/openomsi-dlss'
PIN = json.loads((DATA / 'upstream.json').read_text(encoding='utf-8'))
TARGET = 'x86_64-pc-windows-msvc'

def run(args, cwd, env=None, capture=False):
    print('+ ' + ' '.join(map(str, args)), flush=True)
    return subprocess.run(list(map(str, args)), cwd=cwd, env=env, check=True,
                          text=True, stdout=subprocess.PIPE if capture else None).stdout

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def prepare(source):
    if source.exists():
        raise ValueError('Use a fresh source directory: ' + str(source))
    source.parent.mkdir(parents=True, exist_ok=True)
    run(['git', '-c', 'core.autocrlf=false', 'clone', '--depth', '1', '--branch',
         PIN['upstream_tag'], PIN['repository'], source], ROOT)
    commit = run(['git', 'rev-parse', 'HEAD'], source, capture=True).strip()
    if commit != PIN['upstream_commit']:
        raise ValueError('Upstream tag identity changed')
    # Persist LF policy for git apply; Windows runners default to autocrlf=true.
    run(['git', 'config', 'core.autocrlf', 'false'], source)
    patch = DATA / 'openomsi-0.2.50-dlss.patch'
    if sha(patch) != PIN['patch_sha256']:
        raise ValueError('DLSS patch identity differs from the manifest')
    run(['git', 'apply', '--check', patch], source)
    run(['git', 'apply', patch], source)
    for name, digest in PIN['patched_files'].items():
        if sha(source / name) != digest:
            raise ValueError('Patched source identity differs: ' + name)
    net = (source / 'crates/omsi-net/src/lib.rs').read_text(encoding='utf-8')
    if not re.search(r'pub const PROTOCOL: u32 = 7;', net):
        raise ValueError('The protocol-7 client must not be replaced by the older fork')
    print('Pinned 0.2.50 source, DLSS patch and protocol 7 verified', flush=True)

def patch_wgpu(source):
    # A local copy keeps Cargo's shared git cache immutable. Every wgpu crate stays
    # from the same upstream revision, including the platform dependency crates.
    metadata = json.loads(run(['cargo', 'metadata', '--format-version', '1',
                              '--filter-platform', TARGET], source, capture=True))
    package = next(p for p in metadata['packages'] if p['name'] == 'wgpu-hal'
                   and 'wgpu-openomsi' in (p.get('source') or ''))
    if not package['source'].endswith('#' + PIN['wgpu_revision']):
        raise ValueError('Unexpected wgpu-hal source revision')
    original = Path(package['manifest_path']).parent.parent
    vendor = source / 'vendor/wgpu-openomsi'
    shutil.copytree(original, vendor, ignore=shutil.ignore_patterns('.git', 'target'))
    file = vendor / 'wgpu-hal/src/dx12/instance.rs'
    text = file.read_text(encoding='utf-8')
    before = '        let factory_media = lib_dxgi.create_factory_media().ok();'
    if text.count(before) != 1:
        raise ValueError('Review the ReShade workaround against this wgpu revision')
    text = text.replace(before,
        '        // ReShade workaround: openOMSI does not use composition swapchains.\n'
        '        let factory_media: Option<Dxgi::IDXGIFactoryMedia> = None;\n'
        '        let _ = &lib_dxgi;')
    file.write_text(text, encoding='utf-8', newline='\n')
    paths = {}
    for manifest in vendor.rglob('Cargo.toml'):
        match = re.search(r'^name\s*=\s*"([^"]+)"', manifest.read_text(encoding='utf-8'), re.M)
        if match:
            paths[match[1]] = manifest.parent.relative_to(source).as_posix()
    cargo = source / 'Cargo.toml'
    text = cargo.read_text(encoding='utf-8')
    pattern = r'(?m)^([\w-]+) = \{ git = "[^"\n]*wgpu-openomsi", rev = "' + PIN['wgpu_revision'] + r'" \}'
    count = 0
    def replace(match):
        nonlocal count
        name = match[1]
        if name not in paths:
            raise ValueError('Missing vendored wgpu crate: ' + name)
        count += 1
        return name + ' = { path = "' + paths[name] + '" }'
    text = re.sub(pattern, replace, text)
    if count != 9:
        raise ValueError('Expected all nine patched wgpu/Naga crates, got ' + str(count))
    text = text.replace('exclude = ["launcher/src-tauri"]',
                        'exclude = ["launcher/src-tauri", "vendor/wgpu-openomsi"]')
    cargo.write_text(text, encoding='utf-8', newline='\n')
    print('ReShade workaround applied to a pinned, isolated wgpu copy', flush=True)

def download(url, path, digest):
    if not path.exists():
        path.parent.mkdir(parents=True, exist_ok=True)
        request = urllib.request.Request(url, headers={'User-Agent': 'OpenOMSI-DLSS-Test-Builder'})
        with urllib.request.urlopen(request, timeout=60) as response, path.open('wb') as out:
            shutil.copyfileobj(response, out)
    if sha(path) != digest:
        raise ValueError('Download checksum differs: ' + str(path))

def package(source, output, env):
    archive = ROOT / 'build/openomsi-0.2.50-official-windows-x64.zip'
    download(PIN['runtime_url'], archive, PIN['runtime_sha256'])
    output.mkdir(parents=True, exist_ok=False)
    with zipfile.ZipFile(archive) as zip_file:
        entries = zip_file.namelist()
        exe = next(n for n in entries if n.replace('\\', '/').endswith('/openomsi.exe')) if 'openomsi.exe' not in entries else 'openomsi.exe'
        prefix = exe[:-len('openomsi.exe')]
        for entry in entries:
            if not entry.startswith(prefix) or entry.endswith('/'):
                continue
            relative = Path(entry[len(prefix):])
            if relative.is_absolute() or '..' in relative.parts:
                raise ValueError('Invalid official archive path')
            dest = output / relative
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(zip_file.read(entry))
    binary = source / ('target/' + TARGET + '/release')
    for name in ['openomsi.exe', 'openomsi-launcher.exe']:
        shutil.copy2(binary / name, output / name)
    shutil.copy2(source / ('launcher/src-tauri/target/' + TARGET + '/release/openomsi-launcher-app.exe'), output / 'openomsi-launcher-app.exe')
    factory_guid = bytes.fromhex('f2d1e74191a57b4fa2e5fa9c843e1c12')
    if factory_guid in (output / 'openomsi.exe').read_bytes():
        raise ValueError('The binary still asks for IDXGIFactoryMedia: review the ReShade workaround')
    for option in ['--version', '--help']:
        result = subprocess.run([str((output / 'openomsi.exe').resolve()), option],
                                cwd=output, capture_output=True, text=True, check=True, timeout=20)
        text = result.stdout + result.stderr
        if option == '--version' and PIN['version'] not in text:
            raise ValueError('Client build identity missing')
        if option == '--help':
            for flag in ['--root', '--map', '--bus', '--date', '--time', '--weather',
                         '--schedule', '--line', '--tour', '--trip', '--auto-entry',
                         '--no-menu', '--content-zip', '--driver', '--hof', '--paint',
                         '--traffic', '--passengers', '--all', '--autostart',
                         '--lan-join', '--lan-name', '--server']:
                if flag not in text:
                    raise ValueError('Bridge startup capability missing: ' + flag)
        print(text.strip(), flush=True)
    if b'OMSI_PLUGIN_HOST32' not in (output / 'openomsi.exe').read_bytes():
        raise ValueError('BBS plugin-host override missing from the client')
    for name in ['Preparar-DLSS.ps1', 'Preparar-DLSS.cmd', 'LEIA-ME.txt', 'README-EN.txt', 'LIES-MICH-DE.txt', 'CREDITS-DLSS.txt']:
        shutil.copy2(DATA / name, output / name)
    info = {'version': PIN['version'], 'protocol': 7, 'upstream_commit': PIN['upstream_commit'],
            'reference_commit': PIN['reference_commit'], 'patch_sha256': PIN['patch_sha256'],
            'wgpu_revision': PIN['wgpu_revision'], 'rustc': run(['rustc', '--version'], source, capture=True).strip(),
            'hardware_validation': 'pending: DLSS/NR and a BBS multiplayer trip need a Windows RTX test',
            'files': {p.relative_to(output).as_posix(): sha(p) for p in output.rglob('*') if p.is_file()}}
    (output / 'dlss-build.json').write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8', newline='\n')
    archive_path = output.parent / (PIN['version'] + '-windows-x64.zip')
    with zipfile.ZipFile(archive_path, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=6) as z:
        for file in sorted(output.rglob('*')):
            if file.is_file():
                z.write(file, (output.name + '/' + file.relative_to(output).as_posix()))
    archive_path.with_suffix('.zip.sha256').write_text(sha(archive_path) + '  ' + archive_path.name + '\n', encoding='ascii')
    print('Windows client compiled, CLI checked and test ZIP packaged: ' + str(archive_path), flush=True)

def build(source, output):
    if os.name != 'nt':
        raise ValueError('Build the Windows client on Windows/MSVC')
    env = os.environ.copy()
    env['OPENOMSI_VERSION'] = PIN['version']
    env['TAURI_CONFIG'] = json.dumps({'version': '0.2.50'})
    env['CARGO_PROFILE_RELEASE_DEBUG'] = '0'
    patch_wgpu(source)
    tests = [
        ['test', '--release', '--target', TARGET, '-p', 'omsi-render', '--lib', 'dlss'],
        ['test', '--locked', '--release', '--target', TARGET, '-p', 'omsi-render', '--lib', 'shaders_validate_and_match_the_uniforms'],
        ['test', '--locked', '--release', '--target', TARGET, '-p', 'omsi-render', '--lib', 'the_scene_shader_translates_to_glsl'],
        ['test', '--locked', '--release', '--target', TARGET, '-p', 'omsi-launcher-core', '--lib', 'dlss_settings_round_trip'],
        ['test', '--locked', '--release', '--target', TARGET, '-p', 'omsi-platform', '--lib', 'updater::tests::'],
        ['build', '--locked', '--release', '--target', TARGET, '-p', 'omsi-app', '-p', 'omsi-launcher-core'],
        ['test', '--locked', '--release', '--target', TARGET, '-p', 'omsi-app', '--lib', 'settings::tests::dlss_is_read_and_written'],
        ['build', '--locked', '--release', '--target', TARGET, '--manifest-path', 'launcher/src-tauri/Cargo.toml'],
    ]
    for command in tests:
        run(['cargo', *command], source, env)
    package(source, output, env)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, default=ROOT / 'build/openomsi-dlss-source')
    parser.add_argument('--output', type=Path, default=ROOT / ('build/openOMSI-' + PIN['version'] + '-windows-x64'))
    parser.add_argument('--prepare-only', action='store_true')
    parser.add_argument('--build-only', action='store_true')
    args = parser.parse_args()
    if not args.build_only:
        prepare(args.source.resolve())
    if not args.prepare_only:
        build(args.source.resolve(), args.output.resolve())

if __name__ == '__main__':
    main()
