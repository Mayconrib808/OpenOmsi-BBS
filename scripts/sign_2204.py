"""Sign the exact Windows candidate with the existing trusted publisher."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
def main():
 p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
 r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
 assert r['version']=='2.2.0.4' and r['sequence']==2026102304 and r['world_interest_both_hosts'] and r['native_setup_smoke'] and r['native_server_lamp'] and r['native_manual_and_timer']
 material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
 key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
 assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
 assert material['last_sequence']<2026102304
 notes=dict(pt='2.2.0.4 + Coop 10: troca de motorista com autorizacao fora do onibus, personagem na direcao e motorista oculto na camera F1. IA e passageiros acompanham todos os jogadores a pe e os onibus em sessoes de jogador e no servidor dinamico P14 (base 0.3.39 preservada). Mantidos HostAgent completo P15 e status do servidor da Hotfix 2. Atualize tambem o host. Validado automaticamente e em Windows; sessao grafica real com dois jogadores ainda requer teste.',en='2.2.0.4 + Coop10: driver handover and on-foot authorization, driver character and first-person visibility fixes. AI and passengers follow walkers and buses on both player-hosted and dynamic servers. Complete P15 HostAgent, dedicated 0.3.39 base and Hotfix2 status lamp retained. Update the host too. Automated and native Windows checks passed; real graphical two-player gameplay remains untested.',de='2.2.0.4 + Coop10: Fahrerwechsel mit Freigabe auch ausserhalb des Busses, Fahrerfigur und F1-Sicht korrigiert. KI und Fahrgaeste folgen Fussgaengern und Bussen auf Spieler- und dynamischen Servern. P15-HostAgent, Serverbasis 0.3.39 und Hotfix2-Statusanzeige bleiben erhalten. Auch den Host aktualisieren. Automatische und native Windows-Pruefungen bestanden; grafischer Zwei-Spieler-Test steht aus.')
 manifest=dict(schema=1,version='2.2.0.4',sequence=2026102304,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
 raw=json.dumps(manifest,ensure_ascii=False,separators=(',',':')).encode()
 out=ROOT/'build-support/bridge2204'
 (out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
 publication=dict(version='2.2.0.4',sequence=2026102304,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],full_package_sha256=r['full_package_sha256'],coop_archive_sha256=r['coop_archive_sha256'])
 (out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
 print('Signed exact 2.2.0.4 candidate; private key remains outside the project.')
if __name__=='__main__':main()
