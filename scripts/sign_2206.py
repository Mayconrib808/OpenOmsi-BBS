"""Sign the exact Windows candidate with the existing trusted publisher."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
def main():
 p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
 r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
 assert r['version']=='2.2.0.6' and r['sequence']==2026102406 and r['world_interest_both_hosts'] and r['native_setup_smoke'] and r['native_server_lamp'] and r['native_manual_and_timer']
 material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
 key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
 assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
 assert material['last_sequence']<2026102406
 assert r['coop_protocol']==15 and r['native_clock_readiness_fix'] and r['double_horn'] and r['active_map_copy_buttons']
 notes=dict(pt='2.2.0.6 + Coop12: jogo e servidor atualizados para openOMSI 0.3.40, protocolo 15, mantendo todas as correcoes cooperativas anteriores. Corrige prontidao inicial e confirmacao do relogio do dedicado. IA cumprimenta com duas buzinadas curtas. Setup lista os mapas online com Copiar link para passageiro/cobrador. Mantidos HostAgent completo, voz, administracao, limites de passageiros, BCS no proprietario e transferencia integral de motorista. Atualize todos os jogadores e o host; P14 e P15 nao se misturam na mesma sessao. Validacoes automaticas e nativas Windows; viagem grafica real com dois jogadores ainda requer teste.',en='2.2.0.6 + Coop12: cumulative game and dedicated server upgrade to openOMSI 0.3.40 protocol 15. Fixes native startup readiness and clock confirmation; AI greets with two short horn taps. Setup lists online maps with Copy link for conductors/passengers. Full P15 HostAgent, voice/admin preferences, population settings, BCS ownership and native driver handover retained. Update every participant and the host. Automated and native Windows checks passed; real graphical two-player gameplay remains untested.',de='2.2.0.6 + Coop12: kumulatives Spiel- und Server-Update auf openOMSI 0.3.40 Protokoll 15. Startbereitschaft und Uhrbestaetigung korrigiert, KI-Gruss mit zwei kurzen Huptoenen. Setup zeigt aktive Karten mit Link kopieren fuer Schaffner und Fahrgaeste. Vollstaendiger HostAgent, Sprach-/Adminoptionen und bisherige Koop-Funktionen bleiben erhalten. Alle Spieler und den Host aktualisieren. Native Windows-Pruefungen bestanden; grafischer Zwei-Spieler-Test steht aus.')
 manifest=dict(schema=1,version='2.2.0.6',sequence=2026102406,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
 raw=json.dumps(manifest,ensure_ascii=False,separators=(',',':')).encode()
 out=ROOT/'build-support/bridge2206'
 (out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
 publication=dict(version='2.2.0.6',sequence=2026102406,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],full_package_sha256=r['full_package_sha256'],coop_archive_sha256=r['coop_archive_sha256'])
 (out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
 print('Signed exact 2.2.0.6 candidate; private key remains outside the project.')
if __name__=='__main__':main()
