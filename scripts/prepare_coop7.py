#!/usr/bin/env python3
"""Recreate the pinned Coop 7 portable ZIP from approved Windows artifacts.

This script has no publisher credentials, updates no user files, and verifies
that every downloaded executable is the exact reviewed artifact.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import zipfile

PREFIX = 'OpenOmsi Coop 0.3.38/'
ZIP_PREFIX = 'OpenOMSI_Coop_0.3.38_coop.7_Portatil/'
BASE_SHA = 'd85aa6a5f6234714d025f90914c1300ae175d89e79ea7875b727760fbb43d5a5'
ARTIFACT_SHA = 'f32c5995e2dfa95125634aa90e7e32f9bbf2b0d3b74dea3f03ef77926860183c'
GAME_COMMIT = 'b6949d797997f453457bd9cc0601032c698c2cf5'
GAME_RUN = 39775741405
VALIDATION = ('build.log','regressions.log','windows-self-test.log','windows-startup.json','windows-version.log',
              'defender-report.json','defender-scan.log','defender-update.log','build-sha256.json',
              'driver-share-source-sha256.json','overlay-sha256.json')

def sha(data): return hashlib.sha256(data).hexdigest()
def jsonbytes(data): return (json.dumps(data,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode('utf-8')

def prepare(base_package, coop7_artifact):
    base_bytes=Path(base_package).read_bytes()
    if sha(base_bytes)!=BASE_SHA: raise ValueError('Original Bridge 2.2.0 artifact differs')
    source = Path(coop7_artifact)
    if source.is_dir():
        game_files={f.name: f.read_bytes() for f in source.iterdir() if f.is_file()}
        game = type('ExtractedArtifact', (), {'read': lambda _, name: game_files[name], 'testzip': lambda _: None})()
    else:
        game_bytes=source.read_bytes()
        if sha(game_bytes)!=ARTIFACT_SHA: raise ValueError('Original Coop7 artifact ZIP differs')
        game=zipfile.ZipFile(io.BytesIO(game_bytes))
    with zipfile.ZipFile(io.BytesIO(base_bytes)) as base:
        if base.testzip() or game.testzip(): raise ValueError('Corrupted input ZIP')
        files={p[len(PREFIX):]:base.read(p) for p in base.namelist() if p.startswith(PREFIX) and not p.endswith('/')}
        if len(files)!=45: raise ValueError('Unexpected original Coop 6 file count')
        exe=game.read('openomsi.exe')
        if sha(exe)!=json.loads(game.read('build-sha256.json'))['openomsi.exe']:
            raise ValueError('Game executable checksum mismatch')
        if b'41 passed; 0 failed' not in game.read('regressions.log'):
            raise ValueError('Coop 7 regression summary missing')
        if '0.3.38-coop.7' not in game.read('windows-version.log').decode('utf-8'):
            raise ValueError('Unexpected native game version')
        if json.loads(game.read('defender-report.json'))['status'] != 'passed':
            raise ValueError('Native game Defender evidence unavailable')
        if json.loads(game.read('windows-startup.json'))['two_player_gameplay_tested'] is not False:
            raise ValueError('Unexpected native test limitation')
        if sha(game.read('coop_turnstile_coin.wav'))!=sha(files['programa/assets/sounds/coop_turnstile_coin.wav']):
            raise ValueError('Catraca audio differs from validated Coop')
        files['programa/openomsi.exe']=exe
        files.pop('source/openomsi-coop6-source.zip')
        files['source/openomsi-coop7-source.zip']=game.read('openomsi-coop7-source.zip')
        files['source/BUILD-INFO-coop6-original.json']=files['source/BUILD-INFO.json']
        for name in VALIDATION: files['validation/coop7-'+name]=game.read(name)
        files['source/BUILD-INFO.json']=jsonbytes({
          'product':'OpenOMSI Coop 0.3.38-coop.7', 'game_commit':GAME_COMMIT,
          'game_run_id':GAME_RUN,'game_executable_sha256':sha(exe),
          'source_zip_sha256':sha(files['source/openomsi-coop7-source.zip']),
          'coop_regressions_passed':41, 'coop_regressions_ignored':1,
          'windows_native_startup_tested':True, 'defender_checked':True,
          'graphical_gameplay_tested':False,'live_two_player_tested':False,'bcs_trip_tested':False,
        })
        files['source/BUILD.md']=(
          'Coop 7 (0.3.38) — fonte correspondente\n\n'
          'O executável desta pasta foi compilado no GitHub Actions a partir do commit '+GAME_COMMIT+'\n'
          'Código-fonte: source/openomsi-coop7-source.zip; testes: validation/coop7-*\n'
          'Executáveis de suporte foram mantidos da Coop 6 oficial verificada.\n'
          'Não houve verificação ao vivo de dois jogadores ou encerramento do BCS.\n').encode()
        files['LEIA-ME.txt']=(
          'OpenOMSI 0.3.38-coop.7 — protocolo 14, Windows 64 bits\n'
          'Distribuição integrada com Bridge 2.2.0.1.\n\n'
          'Abra Setup.exe na pasta principal da Bridge. Selecione sua pasta real do OMSI 2,\n'
          'clique em Usar Coop incluído e depois em Salvar e ativar.\n'
          'Ambos os jogadores precisam estar na mesma versão e protocolo.\n'
          'O executável passou no build Windows, regressões e Defender. A compatibilidade\n'
          'com BCS e transferência de motorista em uma viagem real ainda não foi testada.\n'
          'Fonte e avisos de licença incluídos neste diretório.\n').encode('utf-8')
        files['SHA256.json']=jsonbytes({name:sha(data) for name,data in files.items() if name!='SHA256.json'})
    out=io.BytesIO()
    with zipfile.ZipFile(out,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9,allowZip64=True) as archive:
        for name in sorted(files):
            entry=zipfile.ZipInfo(ZIP_PREFIX+name,(2026,10,22,0,0,0))
            entry.compress_type=zipfile.ZIP_DEFLATED
            entry.create_system=3
            entry.external_attr=0o100644<<16
            archive.writestr(entry,files[name],compress_type=zipfile.ZIP_DEFLATED,compresslevel=9)
    contents=out.getvalue()
    parts=[contents[x:x+(16<<20)] for x in range(0,len(contents),16<<20)]
    spec={'sha256':sha(contents),'size':len(contents),'prefix':ZIP_PREFIX,
          'parts':[{'sha256':sha(x),'size':len(x)} for x in parts],
          'files':{PREFIX+k:sha(v) for k,v in sorted(files.items())}}
    return contents,parts,spec

def main():
    a=argparse.ArgumentParser()
    a.add_argument('--base-package',required=True)
    a.add_argument('--coop7-artifact',required=True)
    a.add_argument('--output',required=True)
    a.add_argument('--spec',required=True)
    args=a.parse_args()
    data,parts,spec=prepare(args.base_package,args.coop7_artifact)
    out=Path(args.output);out.parent.mkdir(parents=True,exist_ok=True)
    out.write_bytes(data)
    if Path(args.spec).exists() and json.loads(Path(args.spec).read_text()) != spec:
        raise ValueError('Coop7 differs from the source-pinned distribution manifest')
    print(json.dumps({'archive_sha256':spec['sha256'],'size':len(data),'parts':len(parts),'files':len(spec['files'])}))

if __name__=='__main__': main()
