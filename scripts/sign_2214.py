"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.4'
SEQUENCE=2026102601
p=argparse.ArgumentParser()
p.add_argument('--publisher-key',type=Path,required=True)
p.add_argument('--candidate-report',type=Path,required=True)
p.add_argument('--candidate-run',type=int,required=True)
p.add_argument('--candidate-commit',required=True)
a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert r['complete_p15_ui_tested'] and all(x['all_other_bytes_preserved'] for x in r['complete_host_patch'].values())
assert r['official_041_p15'] and r['app_server_native_041'] and r['custom_closing_signal']
assert r['coop_protocol']==15 and r['base_package_sha256']=='74e1c44bfe88ab10bb16c31daaa36a72794d6a91893994badd03db785604fe45'
assert r['installer_size']<=25<<20 and len(a.candidate_commit)==40
assert r['fuel_cli_tested'] and r['fuel_cli_cases']==3 and r['coop_archive_unchanged'] and r['gameplay_files_unchanged']
assert r['fuel_priority_tested'] and r['neck_body_turn_tested'] and r['servers_unchanged'] and r['p15_server_unchanged']
assert r['ticket_chance_tested'] and r['rigid_umbrella']
assert r['complete_hosts_unchanged'] and r['p14_server_unchanged'] and r['p15_passenger_intent'] and r['gaze_tested'] and r['full_hailing_tested'] and r['passenger_migration_tested']
assert r['native_candidate_commit']=='8693cbaec8337a5945c0df5215f07d6769df581c'
assert r['native_version']=='0.3.41-coop.14-2.2.1.2-neck-fix'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt': '2.2.1.4: corrige o jogo fechando na abertura pelo BCS com unexpected argument --keep-fuel. Remove o argumento nao suportado e mantem o combustivel informado pelo BCS via --setvar, inclusive zero. Testado contra o executavel real da 2.2.1.3. Jogo, correcoes dos passageiros/pescoco, servidores P14/P15, hosts completos, sons e configuracoes preservados. Feche o BCS e o jogo antes de atualizar.', 'en': '2.2.1.4 fixes BCS startup failing with unsupported --keep-fuel. BCS fuel is retained through supported --setvar assignments, including zero. Verified against the actual released Windows game. The full native game distribution, neck correction, servers, complete hosts, sounds and settings are unchanged.', 'de': '2.2.1.4 behebt den BCS-Startabbruch durch das unbekannte Argument --keep-fuel. Kraftstoff wird weiterhin ueber --setvar uebergeben, auch bei null. Mit dem echten Windows-Spiel getestet. Spiel, Halskorrektur, Server, vollstaendige Hosts, Toene und Einstellungen bleiben erhalten.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2214';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
