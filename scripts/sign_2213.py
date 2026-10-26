"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.3'
SEQUENCE=2026102518
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
assert r['fuel_priority_tested'] and r['neck_body_turn_tested'] and r['servers_unchanged'] and r['p15_server_unchanged']
assert r['ticket_chance_tested'] and r['rigid_umbrella']
assert r['complete_hosts_unchanged'] and r['p14_server_unchanged'] and r['p15_passenger_intent'] and r['gaze_tested'] and r['full_hailing_tested'] and r['passenger_migration_tested']
assert r['native_candidate_commit']=='8693cbaec8337a5945c0df5215f07d6769df581c'
assert r['native_version']=='0.3.41-coop.14-2.2.1.2-neck-fix'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt': '2.2.1.3: corrige o pescoco torcido ao dar sinal para onibus vindo de lado ou por tras. O corpo acompanha a virada, com ate 18 graus no tronco e 27 graus na cabeca em relacao ao tronco, preservando o encaixe, a escala e a inclinacao lateral da cabeca. Mantem todas as mudancas da 2.2.1.2, inclusive combustivel BCS, linha/destino individual, alcance, indicador e onibus cheio. Hosts completos, servidores P14/P15, configuracoes, sons e demais recursos cumulativos preservados. Feche o jogo e o Setup antes de atualizar.', 'en': '2.2.1.3: fixes the twisted neck when hailing a bus approaching from the side or behind. The whole body follows large turns; torso and relative head rotations are limited to 18 and 27 degrees. Head attachment, scale and roll are retained. All 2.2.1.2 features, settings, complete hosts, both dedicated servers and sounds remain unchanged. Close the game and Setup before updating.', 'de': '2.2.1.3: behebt den verdrehten Hals beim Handzeichen fuer seitlich oder von hinten kommende Busse. Der Koerper folgt grossen Drehungen; Oberkoerper und Kopf relativ zum Oberkoerper sind auf 18 bzw. 27 Grad begrenzt. Kopfbefestigung, Skalierung und Neigung bleiben erhalten. Alle Funktionen der 2.2.1.2, Einstellungen, vollstaendige Hosts, beide Server und Toene bleiben erhalten.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2213';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
