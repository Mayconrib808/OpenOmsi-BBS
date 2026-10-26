"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.5'
SEQUENCE=2026102602
p=argparse.ArgumentParser()
p.add_argument('--publisher-key',type=Path,required=True)
p.add_argument('--candidate-report',type=Path,required=True)
p.add_argument('--candidate-run',type=int,required=True)
p.add_argument('--candidate-commit',required=True)
a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert r['complete_p15_ui_tested'] and all(x['all_other_bytes_preserved'] for x in r['complete_host_patch'].values())
assert r['official_043_p15'] and r['app_server_native_043'] and r['custom_closing_signal']
assert r['coop_protocol']==15 and r['base_package_sha256']=='74e1c44bfe88ab10bb16c31daaa36a72794d6a91893994badd03db785604fe45'
assert r['installer_size']<=25<<20 and len(a.candidate_commit)==40
assert r['fuel_cli_tested'] and r['fuel_cli_cases']==3 and r['hail_arm_direction_tested'] and r['all_native_regressions_tested']
assert r['fuel_priority_tested'] and r['neck_body_turn_tested'] and not r['servers_unchanged'] and not r['p15_server_unchanged']
assert r['ticket_chance_tested'] and r['rigid_umbrella']
assert r['complete_hosts_unchanged'] and r['p14_server_unchanged'] and r['p15_passenger_intent'] and r['gaze_tested'] and r['full_hailing_tested'] and r['passenger_migration_tested']
inputs=json.loads((ROOT/'build-support/bridge2215/native.json').read_text())
assert r['native_candidate_commit']==inputs['commit'] and r['native_candidate_run']==inputs['run_id']
assert r['native_version']=='0.3.43-coop.15-2.2.1.5'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt':'2.2.1.5: atualiza jogo, lobby e servidor P15 para openOMSI 0.3.43 real com as modificacoes Coop. Corrige a direcao do braco e indicador ao dar sinal: acompanham o onibus, com limite na dobra do punho. Preserva combustivel do BCS inclusive zero, correcao do pescoco, guarda-chuvas, regras dos passageiros, troca de motorista, cameras, skins, sons e hosts completos. Servidor P14 e configuracoes preservados. Feche o BCS, jogo e HostAgent antes de atualizar. Todos os jogadores e o host devem atualizar para usar a mesma base Coop.', 'en':'2.2.1.5 upgrades the game, lobby and P15 server to genuine openOMSI 0.3.43 with cumulative Coop changes. Hailing hands follow the bus instead of a fixed backwards model direction; wrist bend is limited. Retains BCS fuel including zero, neck correction, umbrellas, passenger rules, driver handover, cameras, skins, custom sounds, complete hosts and P14 server. Close BCS, game and HostAgent before updating.', 'de':'2.2.1.5 aktualisiert Spiel, Launcher und P15-Server auf echtes openOMSI 0.3.43 mit allen bisherigen Coop-Aenderungen. Handzeichen folgen dem Bus; die Handgelenksbeugung ist begrenzt. BCS-Kraftstoff einschliesslich null, Halskorrektur, Schirme, Fahrgastregeln, Fahrerwechsel, Kameras, Skins, Toene, vollstaendige Hosts und P14 bleiben erhalten.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2215';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
