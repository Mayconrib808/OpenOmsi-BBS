#!/usr/bin/env python3
"""Package the native-tested Coop 8 game with the exact existing portable support."""
import argparse
import hashlib
import io
import json
import re
import zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
ORIGINAL_SHA='d662199e85c31d5934cb7e1261f68e446df7ae025f0bb30caac1f0e7b1e50ce8'
OLD_PREFIX='OpenOMSI_Coop_0.3.38_coop.7_Portatil/'
PREFIX='OpenOMSI_Coop_0.3.38_coop.8_Portatil/'
FOLDER='OpenOmsi Coop 0.3.38/'
VALIDATION=['build.log','regressions.log','on-foot-network-test.log','windows-self-test.log','windows-startup.json','windows-version.log','defender-report.json','defender-scan.log','defender-update.log','build-sha256.json','character-source-sha256.json','driver-share-source-sha256.json','overlay-sha256.json']

def sha(data):return hashlib.sha256(data).hexdigest()
def dump(value):return (json.dumps(value,ensure_ascii=False,sort_keys=True,indent=2)+'\n').encode()
def read_json(path):return json.loads(path.read_text(encoding='utf-8-sig'))

def go_manifest(spec):
    lines=['package main','','// Exact Coop 8 distribution from the verified native Windows artifact.',
           'const bundledCoopOrigin = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"','',
           'var bundledCoopDistribution = coopDistribution{',
           '\tSHA256: '+json.dumps(spec['sha256'])+',','\tSize: '+str(spec['size'])+',',
           '\tPrefix: '+json.dumps(spec['prefix'])+',','\tParts: []coopDistributionPart{']
    for part in spec['parts']:lines.append('\t\t{SHA256: '+json.dumps(part['sha256'])+', Size: '+str(part['size'])+'},')
    lines+=['\t},','\tFiles: map[string]string{']
    for name,digest in sorted(spec['files'].items()):lines.append('\t\t'+json.dumps(name,ensure_ascii=False)+': '+json.dumps(digest)+',')
    lines+=['\t},','}','']
    return '\n'.join(lines)

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--previous-coop',type=Path,required=True)
    ap.add_argument('--new-build',type=Path,required=True)
    ap.add_argument('--game-run',type=int,required=True)
    ap.add_argument('--game-commit',required=True)
    ap.add_argument('--output',type=Path,required=True)
    args=ap.parse_args()
    if not re.fullmatch('[a-f0-9]{40}',args.game_commit):raise ValueError('Invalid source identity')
    old=args.previous_coop.read_bytes()
    if sha(old)!=ORIGINAL_SHA:raise ValueError('The published Coop 7 archive differs')
    with zipfile.ZipFile(io.BytesIO(old)) as z:
        if z.testzip():raise ValueError('Corrupt previous package')
        files={i.filename[len(OLD_PREFIX):]:z.read(i) for i in z.infolist() if i.filename.startswith(OLD_PREFIX) and not i.is_dir()}
    if len(files)!=57:raise ValueError('Unexpected original Coop 7 package')
    read=lambda name:(args.new_build/name).read_bytes()
    build=read_json(args.new_build/'build-sha256.json')
    exe=read('openomsi.exe')
    if sha(exe)!=build['openomsi.exe'] or exe[:2]!=b'MZ':raise ValueError('Game executable differs')
    regressions=read('regressions.log').decode('utf-8-sig')
    for name in ['only_installed_relative_human_names_are_accepted','catalog_has_passengers_and_drivers_in_nested_folders_without_duplicates','choosing_and_resetting_a_character_survive_a_restart']:
        if not re.search(re.escape(name)+r'\s+\.\.\.\s+ok',regressions):raise ValueError('Missing character regression: '+name)
    if not re.search(r'test result: ok\.\s+\d+ passed; 0 failed',regressions):raise ValueError('Coop regressions did not pass')
    network=read('on-foot-network-test.log').decode('utf-8-sig')
    if not re.search(r'walking_without_a_bus_keeps_the_players_chosen_character\s+\.\.\.\s+ok',network):raise ValueError('No-bus network test did not pass')
    if '0.3.38-coop.8' not in read('windows-version.log').decode('utf-8-sig'):raise ValueError('Unexpected native game identity')
    if read_json(args.new_build/'defender-report.json')['status']!='passed':raise ValueError('Defender evidence is missing')
    native=read_json(args.new_build/'windows-startup.json')
    if native['passed'] is not True or native['protocol']!=14:raise ValueError('Native startup did not pass')
    source=read('openomsi-coop8-source.zip')
    source_files=read_json(args.new_build/'character-source-sha256.json')
    with zipfile.ZipFile(io.BytesIO(source)) as z:
        for name,digest in source_files.items():
            if sha(z.read('openomsi-coop-source/'+name))!=digest:raise ValueError('Corresponding source differs: '+name)
    if sha(read('coop_turnstile_coin.wav'))!=sha(files['programa/assets/sounds/coop_turnstile_coin.wav']):raise ValueError('Original audio changed')
    files['programa/openomsi.exe']=exe
    files.pop('source/openomsi-coop7-source.zip')
    files['source/openomsi-coop8-source.zip']=source
    files['source/BUILD-INFO-coop7-original.json']=files['source/BUILD-INFO.json']
    for name in list(files):
        if name.startswith('validation/coop7-'):files.pop(name)
    for name in VALIDATION:files['validation/coop8-'+name]=read(name)
    files['source/BUILD-INFO.json']=dump({'product':'OpenOMSI 0.3.38-coop.8','bridge_version':'2.2.0.2','game_run_id':args.game_run,'game_commit':args.game_commit,'game_executable_sha256':sha(exe),'source_zip_sha256':sha(source),'protocol':14,'native_startup_tested':True,'character_catalog_and_persistence_tested':True,'no_bus_figure_network_tested':True,'defender_checked':True,'graphical_gameplay_tested':False,'two_player_bcs_trip_tested':False})
    files['LEIA-ME.txt']=('OpenOMSI 0.3.38-coop.8 — Bridge 2.2.0.2 — protocolo 14\n\n'
        'Personagem / skin: dentro do jogo, toque Alt, abra Cooperativo e Escolher personagem / skin.\n'
        'Motorista, trocador e passageiro usam sua própria escolha. Busque os .hum instalados\n'
        'na pasta Humans do seu OMSI 2. A escolha fica salva para a próxima sessão.\n'
        'O amigo precisa ter o mesmo personagem instalado para enxergar a mesma aparência.\n\n'
        'Para entrar sem ônibus: no lobby, prepare a entrada no servidor e ative Start on foot\n'
        '/ A pé na tela de preparação da viagem. Essa opção já existe no Open.\n'
        'Nesta versão, a pessoa sem ônibus também aparece para os outros jogadores com a skin escolhida.\n\n'
        'Fonte correspondente e testes em source/ e validation/.\n'
        'Testes automatizados e de Windows passaram. Uma viagem real com dois jogadores e BCS\n'
        'não foi executada no ambiente de compilação.\n').encode()
    files['source/BUILD.md']=('Código-fonte correspondente: openomsi-coop8-source.zip\n'
        'Build nativo do Windows: '+str(args.game_run)+'\nCommit de suporte: '+args.game_commit+'\n'
        'Preparação: build-support/coop6/prepare.py, driver-share/prepare.py, character/prepare.py.\n'
        'As DLLs e o abridor portátil são os mesmos da distribuição anterior verificada.\n').encode()
    files['SHA256.json']=dump({name:sha(data) for name,data in sorted(files.items()) if name!='SHA256.json'})
    output=io.BytesIO()
    with zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
        for name,data in sorted(files.items()):
            info=zipfile.ZipInfo(PREFIX+name,(2026,10,22,0,0,0));info.compress_type=zipfile.ZIP_DEFLATED
            info.create_system=3;info.external_attr=0o100644<<16
            z.writestr(info,data,compresslevel=9)
    archive=output.getvalue()
    parts=[archive[at:at+(16<<20)] for at in range(0,len(archive),16<<20)]
    spec={'sha256':sha(archive),'size':len(archive),'prefix':PREFIX,'parts':[{'sha256':sha(part),'size':len(part)} for part in parts],'files':{FOLDER+name:sha(data) for name,data in sorted(files.items())}}
    if len(parts)!=4:raise ValueError('Unexpected distribution size')
    args.output.parent.mkdir(parents=True,exist_ok=True);args.output.write_bytes(archive)
    (ROOT/'docs/releases/coop-0.3.38-coop.8.json').write_bytes(dump(spec))
    (ROOT/'source/coop_bundle_manifest.go').write_text(go_manifest(spec),encoding='utf-8')
    print(json.dumps({'archive_sha256':sha(archive),'size':len(archive),'files':len(files),'parts':len(parts)}))

if __name__=='__main__':main()
