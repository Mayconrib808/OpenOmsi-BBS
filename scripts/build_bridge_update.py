#!/usr/bin/env python3
"""Build a self-contained update for an already configured Bridge installation.

The cumulative package replaces the seven Bridge executables. The vendor JAR stays local:
the source carries a locally reconstructed observer and discovers its state interface.
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
    'coop_update.go coop_bundle_manifest.go bridge_update_progress_windows.go setup_updates.go bridge_update_main.go update_process_windows.go bridge_update_options.go bridge_update_setup.go bridge_update_handoff_windows.go dynamic_host_update.go bridge_update.go bridge_update_native.go bridge_update_discovery_windows.go bridge_update_windows.go '
    'bcs_trip_log_patch.go bcs_trip_auto.go bcs_trip_auto_payload.go bcs_capture.go bcs_capture_payload.go '
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
Hotfix 1: corrige a detecção da próxima viagem no BCS atual e atualiza nossa extensão já instalada. Preserva a opção de iniciar com o ônibus desligado, combustível, letreiro e demais preferências.
1. Feche BCS, openOMSI, Setup, HostAgent e CompanyHost.
2. Execute Atualizar-Bridge.exe. Você pode executá-lo da pasta Downloads.
3. Abra o BCS normalmente após a confirmação.
Atualização cumulativa: não exige instalar a 2.1.8 ou 2.1.9 primeiro. Inclui as correções anteriores e ativa o envio e o download nativos de ônibus e pinturas com servidor openOMSI 0.3.38 ou superior. O mapa continua instalado localmente. Para esse recurso, atualize o openOMSI dos jogadores e o servidor para 0.3.38 ou superior; a Bridge não substitui esses executáveis. Servidores anteriores mantêm os substitutos e a correção do erro 10054. Instale também no computador do host e reabra o HostAgent após atualizar. Remove o bloqueio por versão do BCS e aceita os protocolos 6, 7 e 14. Jogadores e servidor precisam usar a mesma versão do openOMSI. Preserva a empresa, convite, caminhos e preferências; não substitui o jogo nem o servidor.

English
Hotfix 1 fixes next-trip detection with the current BCS and upgrades our installed logging extension. Manual engine startup, fuel, destination handling and other preferences are preserved.
Close BCS, openOMSI, Setup, HostAgent and CompanyHost. Run Atualizar-Bridge.exe, including from Downloads, then reopen BCS. An older Setup already stuck without a window may need to be closed once in Task Manager to install this fix. This cumulative update does not require 2.1.8 or 2.1.9 first. Includes previous fixes and enables native bus and livery uploads/downloads with an openOMSI 0.3.38 or later server. Maps remain locally installed. Update the players' openOMSI and the server to 0.3.38 or later for this feature; the Bridge does not replace those executables. Older servers retain stand-ins and the error 10054 fix. Update the company host too and restart HostAgent afterward. Removes the BCS version gate and supports protocols 6, 7 and 14. Players and server must use the same openOMSI version. Your company, invitation, paths, preferences and game/server files are preserved.

Deutsch
Hotfix 1 korrigiert die Erkennung der nächsten Fahrt im aktuellen BBS und aktualisiert unsere installierte Protokollerweiterung. Manueller Motorstart, Kraftstoff, Zielanzeige und weitere Einstellungen bleiben erhalten.
BBS, openOMSI, Setup, HostAgent und CompanyHost schließen. Atualizar-Bridge.exe starten, auch aus Downloads, danach BBS wieder öffnen. Ein altes, bereits unsichtbar hängendes Setup muss für diese erste Installation eventuell einmal im Task-Manager beendet werden. Kumulatives Update: 2.1.8 oder 2.1.9 sind vorher nicht nötig. Enthält bisherige Korrekturen und aktiviert native Bus- und Lackierungsübertragungen mit einem openOMSI-Server ab 0.3.38. Karten bleiben lokal installiert. Dafür openOMSI bei den Spielern und den Server auf mindestens 0.3.38 aktualisieren; die Bridge ersetzt diese Programme nicht. Ältere Server behalten Ersatzmodelle und die Korrektur für Fehler 10054. Auch den Firmen-Host aktualisieren und danach HostAgent neu starten. Entfernt die BBS-Versionssperre und unterstützt die Protokolle 6, 7 und 14. Spieler und Server benötigen dieselbe openOMSI-Version. Firma, Einladung, Pfade, Einstellungen sowie Spiel/Server bleiben erhalten.
'''
        with zipfile.ZipFile(args.output,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
            z.write(updater,'Atualizar-Bridge.exe')
            z.writestr('LEIA-ME.txt',instructions.replace('atualização 2.1.2 da Bridge','atualização '+build.VERSION+' da Bridge').encode('utf-8'))
            z.writestr('SHA256.txt',hashlib.sha256(updater.read_bytes()).hexdigest()+'  Atualizar-Bridge.exe\n')
        print('BUILT',args.output,args.output.stat().st_size,hashlib.sha256(args.output.read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
