"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.1'
SEQUENCE=2026102516
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
assert r['ticket_chance_tested'] and r['rigid_umbrella']
assert r['complete_hosts_unchanged'] and r['servers_unchanged'] and r['passenger_migration_tested']
assert r['native_candidate_commit']=='080650a1e2de0ee3f0e2b57224ae102e3a29d8a9'
assert r['native_version']=='0.3.41-coop.14-2.2.1.1'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt': '2.2.1.1: guarda-chuva fecha antes de baixar a mao; haste e tecido acompanham um conjunto rigido. Sinal com braco e dedo suavizados. Nova opcao, desligada por padrao, para escolher a chance de compra de bilhete: Padrao do mapa ou 0 a 100%, no lobby e no jogo. Vale para novos embarques em onibus com caixa de venda. Hosts completos, servidores P14/P15, sons e correcoes anteriores preservados. Feche jogo e Setup antes de atualizar.', 'en': '2.2.1.1: rigid umbrella closing before lowering the hand, smoother hailing. Optional ticket purchase probability, off by default: native map chance or 0-100%, in lobby and game. Applies to new boarding passengers on buses with a ticket desk. Complete hosts, P14/P15 servers and cumulative Coop fixes preserved.', 'de': '2.2.1.1: Schirm schliesst vor dem Absenken; sanftere Handzeichen. Optionale Fahrkarten-Kaufwahrscheinlichkeit: Kartenstandard oder 0-100%, in Lobby und Spiel. Hosts und P14/P15-Server erhalten.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2211';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')

