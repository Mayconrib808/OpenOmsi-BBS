#!/usr/bin/env python3
"""Reproduce Bridge 2.2.0.2 with exact reviewed Coop 8 portable distribution.

The four previously delivered host/helper binaries stay byte-for-byte intact.
Setup, the player adapter and player Bridge are rebuilt from the included Go
source. The signed updater contains seven core binaries and pins all Coop bytes.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import zipfile

import build
import build_coop_compat

ROOT = Path(__file__).resolve().parents[1]
SEQUENCE = '2026102203'
FOLDER = 'OpenOmsi Coop 0.3.38'
PACKAGE = 'OpenOmsi_BBS_2.2.0.2_Setup_e_Coop8'
SPEC = json.loads((ROOT / 'docs/releases/coop-0.3.38-coop.8.json').read_text())


def sha(data):
    return hashlib.sha256(data).hexdigest()


def check_coop(path):
    data = path.read_bytes()
    if len(data) != SPEC['size'] or sha(data) != SPEC['sha256']:
        raise ValueError('Coop ZIP differs from the exact approved archive')
    with zipfile.ZipFile(path) as archive:
        files = {}
        for entry in archive.infolist():
            if not entry.filename.startswith(SPEC['prefix']) or entry.is_dir():
                raise ValueError('Unexpected Coop archive entry')
            relative = FOLDER + '/' + entry.filename[len(SPEC['prefix']):]
            if relative not in SPEC['files'] or relative in files:
                raise ValueError('Unexpected or duplicate Coop file: ' + relative)
            content = archive.read(entry)
            if sha(content) != SPEC['files'][relative]:
                raise ValueError('Coop content differs: ' + relative)
            files[relative] = content
        if set(files) != set(SPEC['files']):
            raise ValueError('Incomplete Coop distribution')
    parts = {}
    offset = 0
    for part in SPEC['parts']:
        content = data[offset:offset + part['size']]
        offset += part['size']
        if sha(content) != part['sha256']:
            raise ValueError('Coop part hash differs')
        parts[part['sha256'] + '.bin'] = content
    if offset != len(data):
        raise ValueError('Incomplete Coop parts')
    return files, parts


def tests(go):
    env = dict(os.environ, GO111MODULE='off', CGO_ENABLED='0', GOTOOLCHAIN='local', GOEXPERIMENT='')
    for name in ('GOOS', 'GOARCH'):
        env.pop(name, None)
    platform = ['company_host_process_windows.go', 'openomsi_probe_windows.go'] if os.name == 'nt' else ['company_host_process_other.go', 'openomsi_probe_other.go']
    sources = [*build.TEST_COMMON, 'process_teststub_test.go', *platform, *build.TEST_FILES]
    subprocess.run([go, 'test', '-count=1', *sources], cwd=build.SOURCE, env=env, check=True)
    subprocess.run([go, 'vet', '-unsafeptr=false', *sources], cwd=build.SOURCE, env=env, check=True)


def archive(directory, target):
    with zipfile.ZipFile(target, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as out:
        for path in sorted(directory.rglob('*')):
            if not path.is_file():
                continue
            info = zipfile.ZipInfo(path.relative_to(directory).as_posix(), (2026, 10, 22, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = 0o100644 << 16
            out.writestr(info, path.read_bytes(), compresslevel=9)
    target.with_suffix('.zip.sha256').write_text(sha(target.read_bytes()) + '  ' + target.name + '\n', encoding='ascii')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--coop', type=Path)
    p.add_argument('--baseline-update', type=Path, required=True)
    p.add_argument('--output', type=Path, default=ROOT / 'build/release-2202')
    p.add_argument('--go', default='go')
    p.add_argument('--tests-only', action='store_true')
    a = p.parse_args()
    tests(a.go)
    if a.tests_only:
        return
    if build.VERSION != '2.2.0.2' or not a.coop:
        p.error('Bridge 2.2.0.2 source and the approved --coop archive are required')
    files, parts = check_coop(a.coop)
    output = a.output.resolve()
    if output.exists():
        p.error('Choose a new output directory')
    baseline = a.baseline_update.resolve()
    if not baseline.is_file():
        raise ValueError('The exact published baseline updater is required')
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_coop_compat.py'), '--baseline-update', str(baseline),
                    '--output', str(output), '--go', a.go, '--update-sequence', SEQUENCE], check=True)
    updater = output / 'Atualizar-Bridge.exe'
    if updater.stat().st_size > 25 << 20:
        raise ValueError('Updater must fit the existing distribution and old Setup clients')
    assets = output / 'coop-assets'
    assets.mkdir()
    for name, content in parts.items():
        (assets / name).write_bytes(content)
    package = output / PACKAGE
    shutil.copytree(output / 'runtime', package)
    for name, content in files.items():
        path = package / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content)
    ignore = shutil.ignore_patterns('__pycache__', '*.pyc', '*.exe', '*.dll', '*.syso', '*.zip', 'assets', 'node_modules')
    for name in ('source', 'scripts', 'docs', 'native', 'examples', 'relay', 'update-service'):
        shutil.copytree(ROOT / name, package / name, ignore=ignore)
    for name in ('LICENSE', 'THIRD_PARTY_NOTICES.md', 'NOTICE_FOR_PEDEPE.md'):
        shutil.copy2(ROOT / name, package / name)
    shutil.copy2(ROOT / 'app/bridge.ini.example', package / 'app/bridge.ini.example')
    instructions = '''OpenOmsi BBS 2.2.0.2 + OpenOmsi Coop 0.3.38 (Coop 8)

1. Extraia o ZIP inteiro em uma pasta. Não execute os programas dentro do ZIP.
2. Abra Setup.exe, selecione a pasta do OMSI 2 e clique em Usar Coop incluído.
3. Confira suas opções e clique em Salvar e ativar. Depois abra o BCS normalmente.

O jogo está em OpenOmsi Coop 0.3.38/programa/openomsi.exe. A pasta inclui as
DLLs, o áudio da catraca, as licenças e o código-fonte correspondente.
O ABRIR-COOPERATIVO.exe dentro dessa pasta também abre o jogo diretamente.

Já usa a Bridge? Prefira Buscar atualizações no seu Setup atual: a versão 2.2.0.2
instala os componentes e essa mesma pasta da Coop. As configurações existentes
são mantidas. Para trocar o jogo selecionado, use Usar Coop incluído e Salvar e ativar.
A primeira atualização baixa aproximadamente 61 MB adicionais da Coop e mostra
o progresso. Feche o jogo, BCS, Setup antigo e os programas do host quando solicitado.

Jogadores e servidor de uma sessão precisam usar o mesmo protocolo. A Coop 8
usa protocolo 14. O servidor já configurado na empresa continua sendo usado.
O pacote não contém o OMSI 2, o BCS, mapas, ônibus ou outros produtos pagos.

Personagem / skin: dentro do jogo, toque Alt, Cooperativo, Escolher personagem / skin.
Motorista, trocador e passageiro podem escolher qualquer personagem .hum instalado em
Humans. A escolha é salva e enviada aos amigos, que precisam ter o mesmo personagem.
Para entrar sem ônibus, ative Start on foot / A pé na tela de preparação da viagem
do lobby. Quem entra a pé também aparece para os amigos e pode atuar como trocador.

Validação: testes automatizados de integração, interface nativa do Setup e
inicialização do executável no Windows. Uma sessão completa com dois jogadores
e avaliação de viagem no BCS ainda precisa de validação dentro do jogo.
'''
    (package / 'LEIA-ME.txt').write_text(instructions, encoding='utf-8')
    report = json.loads((output / 'build-report.json').read_text())
    report.update(sequence=int(SEQUENCE), coop_archive_sha256=SPEC['sha256'], coop_files=len(files),
                  coop_folder=FOLDER, coop_protocol=14, existing_server_replaced=False,
                  setup_bundled_selection=True, installer_size=updater.stat().st_size)
    (output / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    shutil.copy2(output / 'build-report.json', package / 'docs/BUILD-2.2.0.2.json')
    paths = sorted(path.relative_to(package).as_posix() for path in package.rglob('*') if path.is_file() and path.relative_to(package).as_posix() not in ('docs/SHA256.txt', 'docs/PACKAGE_FILES.txt'))
    paths.append('docs/PACKAGE_FILES.txt')
    (package / 'docs/PACKAGE_FILES.txt').write_text('\n'.join(sorted(paths)) + '\n', encoding='utf-8')
    (package / 'docs/SHA256.txt').write_text(''.join(sha((package / name).read_bytes()) + '  ' + name + '\n' for name in sorted(paths)), encoding='utf-8')
    archive(package, output / (PACKAGE + '.zip'))
    report['full_package_sha256'] = sha((output / (PACKAGE + '.zip')).read_bytes())
    (output / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
