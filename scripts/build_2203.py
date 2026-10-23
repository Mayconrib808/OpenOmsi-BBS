#!/usr/bin/env python3
"""Reproduce Bridge 2.2.0.3 with exact reviewed Coop 9 portable distribution.

The complete P15 host code and helper binaries are retained; host version data is updated.
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
SEQUENCE = '2026102301'
FOLDER = 'OpenOmsi Coop 0.3.38'
PACKAGE = 'OpenOmsi_BBS_2.2.0.3_Setup_e_Coop9'
SPEC = json.loads((ROOT / 'docs/releases/coop-0.3.38-coop.9.json').read_text())


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
            relative = entry.filename[len(SPEC['prefix']):]
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
            info = zipfile.ZipInfo(path.relative_to(directory).as_posix(), (2026, 10, 23, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = 0o100644 << 16
            out.writestr(info, path.read_bytes(), compresslevel=9)
    target.with_suffix('.zip.sha256').write_text(sha(target.read_bytes()) + '  ' + target.name + '\n', encoding='ascii')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--coop', type=Path)
    p.add_argument('--baseline-update', type=Path)
    p.add_argument('--output', type=Path, default=ROOT / 'build/release-2203')
    p.add_argument('--go', default='go')
    p.add_argument('--tests-only', action='store_true')
    a = p.parse_args()
    tests(a.go)
    if a.tests_only:
        return
    if a.baseline_update is None:
        p.error('--baseline-update is required for a release build')
    if build.VERSION != '2.2.0.3' or not a.coop:
        p.error('Bridge 2.2.0.3 source and the approved --coop archive are required')
    files, parts = check_coop(a.coop)
    output = a.output.resolve()
    if output.exists():
        p.error('Choose a new output directory')
    baseline = a.baseline_update.resolve()
    if not baseline.is_file():
        raise ValueError('The exact published baseline updater is required')
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_runtime_2203.py'), '--baseline-update', str(baseline),
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
    for name in ('source', 'scripts', 'docs', 'native', 'examples', 'relay', 'update-service', 'build-support'):
        shutil.copytree(ROOT / name, package / name, ignore=ignore, dirs_exist_ok=True)
    for name in ('LICENSE', 'THIRD_PARTY_NOTICES.md', 'NOTICE_FOR_PEDEPE.md'):
        shutil.copy2(ROOT / name, package / name)
    shutil.copy2(ROOT / 'app/bridge.ini.example', package / 'app/bridge.ini.example')
    instructions = '''OpenOmsi BBS 2.2.0.3 + OpenOmsi Coop 0.3.38 (Coop 9)

1. Extraia o ZIP inteiro em uma pasta. Não execute os programas dentro do ZIP.
2. Abra Setup.exe, selecione a pasta do OMSI 2 e clique em Usar Coop incluído.
3. Confira suas opções e clique em Salvar e ativar. Depois abra o BCS normalmente.

O jogo está em OpenOmsi Coop 0.3.38/programa/openomsi.exe. A pasta inclui as
DLLs, o áudio da catraca, as licenças e o código-fonte correspondente.
O ABRIR-COOPERATIVO.exe dentro dessa pasta também abre o jogo diretamente.

Já usa a Bridge? Prefira Buscar atualizações no seu Setup atual: a versão 2.2.0.3
instala os componentes e essa mesma pasta da Coop. As configurações existentes
são mantidas. Para trocar o jogo selecionado, use Usar Coop incluído e Salvar e ativar.
A primeira atualização baixa o pacote adicional da Coop e mostra
o progresso. Feche o jogo, BCS, Setup antigo e os programas do host quando solicitado.

Jogadores e servidor de uma sessão precisam usar o mesmo protocolo. A Coop 9
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
    instructions += """
HostAgent completo: inclui voz, rádio, alcance, lista pública, identificação para
banimentos e senha admin por mapa. O código executável P15 foi preservado; a
identificação da versão passa a 2.2.0.3. Configurações e preferências são mantidas.
O fonte Go arquivado do host é anterior ao binário P15 e não deve ser usado para
recompilá-lo; scripts/build_runtime_2203.py retém os binários completos e verifica
cada alteração nos dados de identificação. Provas em docs/HOST-P15-PROVENIENCIA.json.

Servidor dinâmico: incluído em app/server-dinamico-p14 dentro desta instalação.
Quem já usa esse caminho recebe a atualização da IA junto com a Coop. Para ativar
ou corrigir o caminho gerenciado, feche o host e abra ATIVAR-SERVIDOR-DINAMICO.cmd.
Servidores P15 ou personalizados selecionados são preservados. O servidor dinâmico
incluído usa P14 e faz o carregamento das regiões ao redor de cada jogador.
Os recursos de rede P15 exigem também jogo e servidor P15.

IA: ônibus da mesma linha em sentidos opostos se cumprimentam. A IA responde à buzina
ou ao farol alto ligado e desligado em até 0,8 segundo. Farol fixo não dispara a resposta.
Há dois lampejos curtos e uma buzina por cumprimento, com intervalo e controle de repetição.
No multiplayer, atualize também o servidor que simula a IA. A viagem gráfica real com
BCS e dois jogadores ainda precisa de validação no jogo.
"""
    (package / 'LEIA-ME.txt').write_text(instructions, encoding='utf-8')
    report = json.loads((output / 'build-report.json').read_text())
    report.update(complete_p15_host_code_preserved=True, dynamic_server_bundled=True, sequence=int(SEQUENCE), coop_archive_sha256=SPEC['sha256'], coop_files=len(files),
                  coop_folder=FOLDER, coop_protocol=14, existing_server_replaced=False, managed_dynamic_server_updated=True,
                  setup_bundled_selection=True, installer_size=updater.stat().st_size)
    (output / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    shutil.copy2(output / 'build-report.json', package / 'docs/BUILD-2.2.0.3.json')
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
