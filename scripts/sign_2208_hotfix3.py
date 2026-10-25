"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.8.3'
SEQUENCE=2026102514
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
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={
 'pt':'2.2.0.8 Hotfix 3: restaura o HostAgent/CompanyHost P15 completos anteriores à Hotfix2, com voz, rádio, alcance, lista pública, identificação da máquina e senha admin por mapa. Testes da janela real: controles, troca de mapa, salvar e reabrir. Mantém recuperação do relógio com 36 leituras consecutivas antes do encerramento (aproximadamente 3 minutos). Todo o restante do código desses executáveis foi preservado. HostAgent identifica a base 2.2.0.8; Setup identifica a Hotfix3 como2.2.0.8.3. Jogo e áudio iguais à Hotfix1. Feche os programas antes de atualizar.',
 'en':'2.2.0.8 Hotfix 3: restores complete pre-Hotfix2 P15 HostAgent/CompanyHost: voice, radio, range, public listing, machine ID and map admin password. Real Windows controls, map switching, Save and reopen tested. Clock recovery allows 36 consecutive failed readings (about3 minutes). All other executable code conserved. HostAgent base2.2.0.8, Setup hotfix2.2.0.8.3. Game/audio unchanged fromHotfix1. Close all programs before updating.',
 'de':'2.2.0.8 Hotfix 3: vollständige P15-Hosts mit Sprache, Funk, Reichweite, öffentlicher Liste, Rechnerkennung und Adminpasswort wiederhergestellt. Echte Windows-Steuerelemente, Kartenwechsel, Speichern und Neustart geprüft. Zeitabgleich mit36 Fehlmessungen vor Abbruch (etwa3Minuten). Übriger Programmcode unverändert. HostAgent-Basis2.2.0.8, Setup-Hotfix2.2.0.8.3. Spiel und Audio wieHotfix1.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2208-hotfix3';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
