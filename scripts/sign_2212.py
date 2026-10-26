"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.2'
SEQUENCE=2026102517
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
assert r['fuel_priority_tested']
assert r['ticket_chance_tested'] and r['rigid_umbrella']
assert r['complete_hosts_unchanged'] and r['p14_server_unchanged'] and r['p15_passenger_intent'] and r['gaze_tested'] and r['full_hailing_tested'] and r['passenger_migration_tested']
assert r['native_candidate_commit']=='76afed2d996c14b90cb806c8ef16e784e08b3b72'
assert r['native_version']=='0.3.41-coop.14-2.2.1.2'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt': '2.2.1.2: combustivel externo do BCS tem prioridade sobre tanque cheio automatico, inclusive zero; passageiros olham para o onibus enquanto dao sinal; alcance de 60 m, com linha/destino individual. Indicador correto, aceno um pouco mais rapido e palma em movimento. Onibus cheio recebe um unico pedido, sem insistencia ate aparecer vaga real. P15 atualizado para transmitir a preferencia individual; atualize e reinicie o servidor P15 para esta parte funcionar no multiplayer. Protocolo 15 mantido, descricoes antigas continuam aceitas. Preserva configuracoes de bilhetes, guarda-chuvas, hosts completos, P14, sons e correcoes Coop anteriores. Feche jogo e Setup antes de atualizar.', 'en': '2.2.1.2: passengers look toward the hailed bus, within 60 m and matching individual service preferences. Correct index finger and quicker palm wave. One request for a full bus until a place becomes free. Update and restart the P15 server for individual multiplayer service metadata. Protocol 15 and legacy descriptions remain compatible. Complete hosts, P14, ticket settings, umbrella features, sounds and prior Coop fixes retained.', 'de': '2.2.1.2: Blick zum Bus, Handzeichen bis 60 m fuer den passenden Fahrtwunsch. Zeigefinger und Handflaeche verbessert. Ein Zeichen bei vollem Bus, erneut erst bei freiem Platz. P15-Server fuer individuelle Fahrtwuensche aktualisieren und neu starten. Protokoll 15, vollstaendige Hosts, P14, Fahrkartenoptionen, Schirme und Ton erhalten.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2212';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
