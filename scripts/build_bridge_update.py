#!/usr/bin/env python3
"""Build a self-contained update for an already configured Bridge installation.

The cumulative package replaces the seven Bridge executables. The vendor JAR stays local:
the source contains a checksum-guarded delta for our observational log method.
"""
import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

import build
import setup_resources

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / 'source'

UPDATER_SOURCES = (
    'bridge_update_main.go update_process_windows.go bridge_update_options.go bridge_update_handoff_windows.go bridge_update.go bridge_update_native.go bridge_update_discovery_windows.go bridge_update_windows.go '
    'bcs_trip_log_patch.go bcs_trip_auto.go bcs_trip_auto_payload.go '
    'package_integrity.go setup_windows.go config.go plugin_host.go registry.go '
    'paths.go version.go company.go company_directory.go weather.go log_text.go '
    'company_content.go company_clock.go profile_store.go openomsi_compatibility.go openomsi_probe_windows.go'
).split()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--components', type=Path, help='Optional source-built components for package integration tests.')
    parser.add_argument('--package', type=Path, help='Complete source-built package to embed.')
    args = parser.parse_args()
    args.output = args.output.resolve()
    if args.package:
        args.package = args.package.resolve()
    if args.components:
        args.components = args.components.resolve()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(GOOS='windows', GOARCH='386', GO386='sse2', CGO_ENABLED='0', GOTOOLCHAIN='local')
    env.pop('GO111MODULE', None)
    host = ROOT/'app/compat/omsi-plugin-host32.exe'
    digest = ''
    if host.is_file():
        digest = ' -X=main.bundledPluginHostSHA256='+hashlib.sha256(host.read_bytes()).hexdigest()
    with tempfile.TemporaryDirectory(prefix='bridge-automatic-build-') as temporary:
        work = Path(temporary)
        if not args.package:
            setup = work / 'setup'; setup.mkdir()
            for name in build.PROGRAMS['Setup.exe']:
                shutil.copy2(SOURCE / name, setup / name)
            (setup/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
            resources = setup_resources.prepare(setup, SOURCE/'resources')
            binaries = work/'binaries'; binaries.mkdir()
            subprocess.run([args.go,'vet','-unsafeptr=false','.'],cwd=setup,env=env,check=True)
            subprocess.run([args.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui'+digest,'-o',str(binaries/'Setup.exe'),'.'],cwd=setup,env=env,check=True)
            setup_resources.verify_executable(binaries/'Setup.exe',resources)
            adapter = binaries/'openomsi.exe'
            subprocess.run([args.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui'+digest,'-o',str(adapter),*build.PROGRAMS['PeDePeAdapter/openomsi.exe']],cwd=SOURCE,env=env,check=True)
        payload = SOURCE/'bridge_update_payload.zip'
        try:
            with zipfile.ZipFile(payload,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
                if args.package:
                    for name in [*build.PROGRAMS,build.HOST]:
                        z.write(args.package/name,name)
                else:
                    z.write(binaries/'Setup.exe','Setup.exe')
                    z.write(adapter,'PeDePeAdapter/openomsi.exe')
            if args.components:
                args.components.mkdir(parents=True,exist_ok=True)
                shutil.copy2(payload,args.components/'payload.zip')
            subprocess.run([args.go,'vet','-unsafeptr=false',*UPDATER_SOURCES],cwd=SOURCE,env=env,check=True)
            updater = args.output.parent/'Atualizar-Bridge.exe'
            subprocess.run([args.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(updater),*UPDATER_SOURCES],cwd=SOURCE,env=env,check=True)
        finally:
            payload.unlink(missing_ok=True)
        instructions = f'''OpenOMSI BCS Bridge {build.VERSION}

Português
1. Feche BCS, openOMSI, Setup, HostAgent e CompanyHost.
2. Execute Atualizar-Bridge.exe. Você pode executá-lo da pasta Downloads.
3. Abra o BCS normalmente após a confirmação.
Atualização cumulativa: não exige instalar a 2.1.8 ou 2.1.9 primeiro. Inclui o hotfix de recuperação e a confirmação/correção do letreiro na Próxima viagem. Preserva a empresa, convite, caminhos e preferências; não substitui o jogo nem o servidor.

English
Close BCS, openOMSI, Setup, HostAgent and CompanyHost. Run Atualizar-Bridge.exe, including from Downloads, then reopen BCS. This cumulative update does not require 2.1.8 or 2.1.9 first. Includes recovery and next-trip destination verification/correction. Your company, invitation, paths, preferences and game/server files are preserved.

Deutsch
BBS, openOMSI, Setup, HostAgent und CompanyHost schließen. Atualizar-Bridge.exe starten, auch aus Downloads, danach BBS wieder öffnen. Kumulatives Update: 2.1.8 oder 2.1.9 sind vorher nicht nötig. Enthält Wiederherstellung und Zielanzeige-Prüfung/Korrektur bei Nächste Fahrt. Firma, Einladung, Pfade, Einstellungen sowie Spiel/Server bleiben erhalten.
'''
        with zipfile.ZipFile(args.output,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
            z.write(updater,'Atualizar-Bridge.exe')
            z.writestr('LEIA-ME.txt',instructions.replace('atualização 2.1.2 da Bridge','atualização '+build.VERSION+' da Bridge').encode('utf-8'))
            z.writestr('SHA256.txt',hashlib.sha256(updater.read_bytes()).hexdigest()+'  Atualizar-Bridge.exe\n')
        print('BUILT',args.output,args.output.stat().st_size,hashlib.sha256(args.output.read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
