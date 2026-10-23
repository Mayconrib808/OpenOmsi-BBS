"""Assemble the exact Coop 9, managed streaming server and public host activation files."""
import argparse,hashlib,io,json,re,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
FOLDER='OpenOmsi Coop 0.3.38/'
PREFIX='Bridge-2.2.0.3/'
OLD_DYNAMIC='8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
def sha(data):return hashlib.sha256(data).hexdigest()
def dump(obj):return (json.dumps(obj,ensure_ascii=False,indent=2,sort_keys=True)+'\n').encode()
def go_manifest(spec):
    lines=['package main','','// Exact cumulative Coop 9 and dynamic host distribution; all paths and bytes pinned.',
      'const bundledCoopOrigin = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"','',
      'var bundledCoopDistribution = coopDistribution{','\tFullPaths: true,',
      '\tSHA256: '+json.dumps(spec['sha256'])+',','\tSize: '+str(spec['size'])+',',
      '\tPrefix: '+json.dumps(spec['prefix'])+',','\tParts: []coopDistributionPart{']
    for p in spec['parts']:lines.append('\t\t{SHA256: '+json.dumps(p['sha256'])+', Size: '+str(p['size'])+'},')
    lines+=['\t},','\tFiles: map[string]string{']
    for n,h in sorted(spec['files'].items()):lines.append('\t\t'+json.dumps(n,ensure_ascii=False)+': '+json.dumps(h)+',')
    return '\n'.join(lines+['\t},','}',''])
def main():
    p=argparse.ArgumentParser();p.add_argument('--previous-coop',type=Path,required=True);p.add_argument('--dynamic-baseline',type=Path,required=True)
    p.add_argument('--new-server',type=Path,required=True);p.add_argument('--server-run',type=int,required=True);p.add_argument('--server-commit',required=True);p.add_argument('--new-build',type=Path,required=True);p.add_argument('--game-run',type=int,required=True);p.add_argument('--game-commit',required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
    spec=json.loads((ROOT/'docs/releases/coop-0.3.38-coop.8.json').read_text())
    old=a.previous_coop.read_bytes()
    if sha(old)!=spec['sha256']:raise ValueError('Previous Coop 8 differs')
    with zipfile.ZipFile(io.BytesIO(old)) as z:
        if z.testzip():raise ValueError('Corrupt baseline')
        files={FOLDER+i.filename[len(spec['prefix']):]:z.read(i) for i in z.infolist() if not i.is_dir()}
    if {n:sha(d) for n,d in files.items()}!=spec['files']:raise ValueError('Previous file pins differ')
    read=lambda n:(a.new_build/n).read_bytes()
    js=lambda n:json.loads(read(n).decode('utf-8-sig'))
    exe=read('openomsi.exe');source=read('openomsi-coop9-source.zip')
    if exe[:2]!=b'MZ' or sha(exe)!=js('build-sha256.json')['openomsi.exe']:raise ValueError('Native executable differs')
    if '0.3.38-coop.9' not in read('windows-version.log').decode('utf-8-sig'):raise ValueError('Native identity differs')
    if not js('windows-startup.json')['passed'] or js('defender-report.json')['status']!='passed':raise ValueError('Native tests or Defender did not pass')
    greetings=read('greetings-tests.log').decode('utf-8-sig')
    if not re.search(r'test result: ok\. 9 passed; 0 failed',greetings):raise ValueError('Nine greeting regressions did not pass')
    for log in ['regressions.log','on-foot-network-test.log']:
        if not re.search(r'test result: ok\.\s+\d+ passed; 0 failed',read(log).decode('utf-8-sig')):raise ValueError('Coop regression missing: '+log)
    with zipfile.ZipFile(io.BytesIO(source)) as z:
        for n,v in js('greetings-source-sha256.json').items():
            if sha(z.read('openomsi-coop-source/'+n))!=v['after']:raise ValueError('Corresponding source differs: '+n)
    files[FOLDER+'programa/openomsi.exe']=exe
    files.pop(FOLDER+'source/openomsi-coop8-source.zip')
    files[FOLDER+'source/openomsi-coop9-source.zip']=source
    for n in list(files):
        if n.startswith(FOLDER+'validation/coop8-'):files.pop(n)
    for n in ['build.log','regressions.log','greetings-tests.log','on-foot-network-test.log','windows-self-test.log','windows-version.log','windows-help.log','windows-startup.json','defender-report.json','defender-scan.log','defender-update.log','greetings-source-sha256.json','driver-share-source-sha256.json','character-source-sha256.json','overlay-sha256.json','build-sha256.json']:
        files[FOLDER+'validation/coop9-'+n]=read(n)
    files[FOLDER+'source/BUILD-INFO.json']=dump(dict(product='OpenOMSI 0.3.38-coop.9',bridge_version='2.2.0.3',protocol=14,game_run_id=a.game_run,game_commit=a.game_commit,executable_sha256=sha(exe),source_sha256=sha(source),streaming_preserved=True,automated_greeting_tests=9,graphical_gameplay_tested=False,two_player_bcs_trip_tested=False))
    files[FOLDER+'source/BUILD.md']=('Fonte correspondente: openomsi-coop9-source.zip.\nPreparação: coop6, driver-share, character e greetings em build-support.\nO servidor dinâmico mantém a base 0.3.39; fonte correspondente em docs/source da Bridge.\n').encode()
    files[FOLDER+'LEIA-ME.txt']=('OpenOMSI 0.3.38-coop.9 — Bridge 2.2.0.3 — protocolo 14\n\n'
      'Ônibus da mesma linha que se aproximam em sentidos opostos se cumprimentam com buzina e dois lampejos.\n'
      'A IA também responde à buzina ou a um lampejo curto do jogador, mesmo em outra linha.\n'
      'Farol baixo e farol alto mantido aceso não contam como lampejo. Para sinalizar, ligue e desligue\n'
      'o farol alto em até 0,8 segundo. Há intervalo de 10 segundos por motorista e controle de repetição.\n'
      'Em multiplayer, o host ou servidor que simula a IA também precisa da Coop 9. Protocolo 14 mantido.\n'
      'Skins, troca de motorista, cliques do cobrador e catraca da Coop 8 foram mantidos.\n'
      'Testes automatizados e nativos do Windows incluídos. Viagem gráfica real com BCS e dois jogadores pendente.\n').encode()
    server_read=lambda n:(a.new_server/n).read_bytes()
    server_json=lambda n:json.loads(server_read(n).decode('utf-8-sig'))
    server_exe=server_read('openomsi.exe')
    if sha(server_exe)!=server_json('build-sha256.json')['openomsi.exe'] or server_exe[:2]!=b'MZ':raise ValueError('Server executable differs')
    if '0.3.39-bbs-stream.2' not in server_read('windows-version.log').decode('utf-8-sig'):raise ValueError('Server native base regressed')
    if server_json('defender-report.json')['status']!='passed' or not server_json('windows-startup.json')['passed']:raise ValueError('Native server validation did not pass')
    if not re.search(r'test result: ok\. 9 passed; 0 failed',server_read('greetings-tests.log').decode('utf-8-sig')):raise ValueError('Server greeting regressions did not pass')
    server_source=server_read('openomsi-dynamic-source.zip')
    with zipfile.ZipFile(io.BytesIO(server_source)) as z:
        for n,v in server_json('greetings-source-sha256.json').items():
            if sha(z.read('openomsi-dynamic-source/'+n))!=v['after']:raise ValueError('Server corresponding source differs')
    files['docs/source/OpenOMSI_0.3.39_Dinamico_Fonte.zip']=server_source
    for name in ['greetings-tests.log','greetings-source-sha256.json','windows-version.log','windows-startup.json','defender-report.json']:
        files['docs/validation-dynamic/'+name]=server_read(name)
    dynamic={}
    with zipfile.ZipFile(a.dynamic_baseline) as z:
        for n in z.namelist():
            if n.startswith('app/server/') and not n.endswith('/'):
                dynamic['app/server-dinamico-p14/'+Path(n).name]=z.read(n)
    if sha(dynamic['app/server-dinamico-p14/openomsi.exe'])!=OLD_DYNAMIC:raise ValueError('Dynamic source baseline differs')
    dynamic['app/server-dinamico-p14/openomsi.exe']=server_exe
    dynamic['app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json']=dump(dict(bridge_line='2.2.0.3',native_version='0.3.39-bbs-stream.2',protocol=14,streaming=True,oncoming_greetings=True,source='docs/source/OpenOMSI_0.3.39_Dinamico_Fonte.zip',source_commit='9eaf95d4caafabe041edcdfc37931b5259c30c8a',build_commit=a.server_commit,build_run=a.server_run,native_sha256=sha(server_exe)))
    files.update(dynamic)
    provenance=json.loads((ROOT/'build-support/bridge2203/host-provenance.json').read_text())
    previous={'HostAgent.exe':['a483e435a23b9a74276fb045945ce58357ab5ea99f8759ddc2dddae41a41dba4','9611698e6318da299c32841c08a770959693dbaa495c33caa6671ff3649cadd3','b02fd974513b2e5f94987f459af72fe736fb80e132a0af68d51f0df7d42e368a','10dd12ad4226b254ef262810c58d3ac089f108cfdc4933fb9eee361a4ca031b1'],
      'CompanyHost.exe':['3d57a4733cbfb8bec677c29d888de339a92ae43c161f9e48ccc9b7a97c7d6cb3','ebfd3836d70ea342e65494309496d85f22232af540faacbc62dbcf6df66beb5c','69c28833dab1ecec927acfbf61c26b75c7c4227d25b019baf0c3b68c29ed393f','bd4e777156515c60704ce7213d63478d71ad859279352c40d72f83e7bfa34f8a']}
    host_manifest=dict(schema=1,version='2.2.0.3',edition='publico.1',previous_hosts=previous,host_protocols=[6,7,14,15],included_dynamic_protocol=14,files=[{'path':n,'sha256':sha(d)} for n,d in sorted(dynamic.items())]+[{'path':n,'sha256':v['updated_sha256']} for n,v in provenance['components'].items()])
    files['docs/HOST-PESSOAL-2.2.0.3.json']=dump(host_manifest)
    script=(ROOT/'build-support/bridge2203/atualizar_host_pessoal.ps1').read_text(encoding='utf-8-sig').replace('2.2.0.2','2.2.0.3').replace('host-pessoal.3','publico.1').replace(OLD_DYNAMIC,sha(server_exe))
    script=script.replace("$knownDynamic = (Get-FileHash -LiteralPath $server -Algorithm SHA256).Hash -eq '"+sha(server_exe)+"'", "$knownDynamic = (Get-FileHash -LiteralPath $server -Algorithm SHA256).Hash.ToLowerInvariant() -in @('"+OLD_DYNAMIC+"','"+sha(server_exe)+"')")
    script=script.replace("@('Personal-Host-2.2.0.3/Backups'", "@('Personal-Host-2.2.0.2/Backups','Personal-Host-2.2.0.3/Backups'")
    script=script.replace("$recentReceipt = Join-Path $DataRoot 'Personal-Host-2.2.0.3/installed.json'", "$recentReceipt = Join-Path $DataRoot 'Personal-Host-2.2.0.3/installed.json'\n    if (-not [IO.File]::Exists($recentReceipt)) { $recentReceipt = Join-Path $DataRoot 'Personal-Host-2.2.0.2/installed.json' }")
    script=script.replace("(Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $file.sha256", "(Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -notin @('"+OLD_DYNAMIC+"','"+sha(server_exe)+"')")
    script=script.replace('Servidor dinamico P14 selecionado, com o mesmo executavel da sua edicao pessoal anterior.', 'Servidor dinamico P14 atualizado: base 0.3.39, carregamento por regioes e cumprimentos da IA.')
    files['scripts/atualizar_host_pessoal.ps1']=b'\xef\xbb\xbf'+script.encode()
    tests=(ROOT/'build-support/bridge2203/test_host_pessoal.ps1').read_text(encoding='utf-8-sig').replace('2.2.0.2','2.2.0.3').replace(OLD_DYNAMIC,sha(server_exe))
    (ROOT/'scripts/test_host_2203.ps1').write_text(tests,encoding='utf-8-sig',newline='\n')
    files['ATIVAR-SERVIDOR-DINAMICO.cmd']=('@echo off\r\n'
      'if exist "%~dp0app\\bridge.ini" (\r\n'
      '  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\\atualizar_host_pessoal.ps1" -PreferredDestination "%~dp0." -NoLaunch\r\n'
      ') else (\r\n'
      '  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\\atualizar_host_pessoal.ps1" -NoLaunch\r\n'
      ')\r\npause\r\n').encode()
    files['docs/HOST-P15-PROVENIENCIA.json']=dump(provenance)
    files[FOLDER+'SHA256.json']=dump({n[len(FOLDER):]:sha(d) for n,d in sorted(files.items()) if n.startswith(FOLDER) and n!=FOLDER+'SHA256.json'})
    out=io.BytesIO()
    with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
        for n,d in sorted(files.items()):
            i=zipfile.ZipInfo(PREFIX+n,(2026,10,23,0,0,0));i.compress_type=zipfile.ZIP_DEFLATED;i.create_system=3;i.external_attr=0o100644<<16;z.writestr(i,d,compresslevel=9)
    data=out.getvalue();parts=[data[i:i+(16<<20)] for i in range(0,len(data),16<<20)]
    if len(data)>128<<20 or len(parts)>8 or sum(len(d) for d in files.values())>384<<20:raise ValueError('Cumulative distribution exceeds verified limits')
    spec=dict(sha256=sha(data),size=len(data),prefix=PREFIX,full_paths=True,parts=[dict(sha256=sha(p),size=len(p)) for p in parts],files={n:sha(d) for n,d in sorted(files.items())})
    a.output.parent.mkdir(parents=True,exist_ok=True);a.output.write_bytes(data)
    (ROOT/'docs/releases/coop-0.3.38-coop.9.json').write_bytes(dump(spec))
    (ROOT/'source/coop_bundle_manifest.go').write_text(go_manifest(spec),encoding='utf-8',newline='\n')
    print(json.dumps(dict(size=len(data),files=len(files),parts=len(parts),sha256=sha(data))))
if __name__=='__main__':main()
