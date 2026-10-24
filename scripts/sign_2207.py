"""Sign the exact Windows candidate with the existing trusted publisher."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
def main():
 p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
 r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
 assert r['version']=='2.2.0.7' and r['sequence']==2026102409 and r['world_interest_both_hosts'] and r['native_setup_smoke'] and r['native_server_lamp'] and r['native_manual_and_timer']
 material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
 key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
 assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
 assert material['last_sequence']<2026102409
 assert r['coop_protocol']==15 and r['native_clock_readiness_fix'] and r['double_horn'] and r['active_map_copy_buttons']
 assert all(r.get(k) for k in ('character_skin_with_vehicle','coop13_regressions','passive_cabin_alias','avatar_camera_return','parked_audio_capture_fixed','repaint_by_name','official_launcher_040','driver_handover_path_fixed'))
 notes=dict(pt='2.2.0.7 + Coop13: corrige skin do personagem que desaparecia para os outros ao spawnar o onibus, retorno F4/F1 ao personagem, vinculo da cabine e dos personagens ao onibus transferido, bloqueio de G para o antigo motorista, fila e ciclo de vida de sons, bilhetes fisicos e pintura por nome e apos respawn. O motorista visitante simula o veiculo; o proprietario mantem o vinculo BCS e uma replica passiva. Mantem as correcoes cumulativas da 2.2.0.6.2 e protocolo 15. Atualize todos os jogadores e o host. Testes automaticos e nativos Windows; teste grafico com dois jogadores ainda pendente.',en='2.2.0.7 + Coop13 fixes F4/F1 avatar return, transferred cabin and avatar bindings, former-driver G protection, sound queue and lifecycle, physical tickets and livery identity across respawn. The visiting driver simulates the bus; its owner retains BCS integration and a passive replica. Cumulative 2.2.0.6.2 fixes and protocol 15 retained. Update every player and host. Automated and native Windows checks; graphical two-player testing remains pending.',de='2.2.0.7 + Coop13 korrigiert F4/F1, Kabinen- und Figurenbindung nach Fahrerwechsel, G beim bisherigen Fahrer, Tonwarteschlange, Fahrkarten und Lackierung nach erneutem Spawn. Der neue Fahrer simuliert den Bus; der Besitzer behaelt BCS und eine passive Replik. Bisherige Korrekturen und Protokoll 15 bleiben erhalten. Alle Spieler und den Host aktualisieren. Automatische und native Windows-Pruefungen; grafischer Zwei-Spieler-Test steht aus.')
 manifest=dict(schema=1,version='2.2.0.7',sequence=2026102409,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
 raw=json.dumps(manifest,ensure_ascii=False,separators=(',',':')).encode()
 out=ROOT/'build-support/bridge2207'
 (out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
 publication=dict(version='2.2.0.7',sequence=2026102409,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],full_package_sha256=r['full_package_sha256'],coop_archive_sha256=r['coop_archive_sha256'])
 (out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
 print('Signed exact 2.2.0.7 candidate; private key remains outside the project.')
if __name__=='__main__':main()
