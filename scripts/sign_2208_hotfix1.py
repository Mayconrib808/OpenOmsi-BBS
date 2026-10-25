"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.8.1'
SEQUENCE=2026102412
p=argparse.ArgumentParser()
p.add_argument('--publisher-key',type=Path,required=True)
p.add_argument('--candidate-report',type=Path,required=True)
p.add_argument('--candidate-run',type=int,required=True)
p.add_argument('--candidate-commit',required=True)
a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert r['official_041_p15'] and r['app_server_native_041'] and r['custom_closing_signal']
assert r['coop_protocol']==15 and r['base_package_sha256']=='74e1c44bfe88ab10bb16c31daaa36a72794d6a91893994badd03db785604fe45'
assert r['installer_size']<=25<<20 and len(a.candidate_commit)==40
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={
 'pt':'2.2.0.8 Hotfix 1: separa os limites de rede dos passageiros, distribui descrições e preparação visual para reduzir rajadas nos pontos. Mantém Coop14, protocolo 15, troca de motorista, câmeras, skins e som Fechou. Carregador de mapas, horários e prazo de abertura de mapas grandes preservados. Atualize todos os jogadores e o servidor. Testes automáticos de rede e Bridge; viagem real no Carrão City ainda depende de validação no jogo.',
 'en':'2.2.0.8 Hotfix 1: separate passenger network budgets, paced descriptions and bounded visual preparation at crowded stops. Retains Coop14, P15, driver transfer, cameras, skins and Fechou audio. Map loader, timetable and large-map startup deadline unchanged. Update every player and server. Automated network and Bridge regressions; a live Carrao City trip still needs gameplay validation.',
 'de':'2.2.0.8 Hotfix 1: getrennte Netzwerklimits für Fahrgäste, verteilte Beschreibungen und begrenzte visuelle Vorbereitung an vollen Haltestellen. Coop14, P15, Fahrerwechsel, Kameras, Skins und Türsignal bleiben erhalten. Kartenlader, Fahrpläne und Startfrist großer Karten unverändert. Alle Spieler und den Server aktualisieren. Automatische Netzwerk- und Bridge-Tests; eine reale Carrao-City-Fahrt ist noch im Spiel zu prüfen.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2208-hotfix1';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')
