"""Sign the verified camera release locally with the publisher already trusted by Setup."""
import argparse,base64,json,re
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.9';SEQUENCE=2026102704
p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert a.candidate_run==40590327114 and a.candidate_commit=='f16ca511984084b13e97fbd911b22e3608e8daaa'
for flag in ['camera_panel_tested','camera_live_adjustments','cross_player_connection_repatriation','complete_hosts_unchanged','complete_p15_ui_tested','admin_clock_opt_in_tested','admin_time_opt_in','p14_server_unchanged','desktop_launcher_unchanged','custom_closing_signal','setup_backup_restore_tested','setup_backup_controls_ui_tested','connection_simulation_tested','fuel_cli_tested','official_045_p15','app_server_native_045']:assert r[flag],flag
assert not r['complete_hosts_rebuilt'] and r['camera_test_run']==40587532083 and r['retained_hotfix_run']==40577793377
assert r['native_candidate_run']==40590075476 and r['native_candidate_commit']=='7b0c2713849ea335e3c2598bd42cfe954106423a'
assert r['native_version']=='0.3.45-coop.15-2.2.1.9' and r['coop_protocol']==15
assert r['previous_installer_sha256']=='5c599ee43138b43706045548ff87f0f5750cac7fe4ef2cab825f0de1f4000095'
assert r['previous_coop_sha256']=='a9894eb80f3fd4fd1765b4a88557aaa7dcdc606eab77b8e884d0f5f2b3b8aaf8'
assert r['native_client_sha256']['openomsi.exe']==r['dynamic_p15_sha256']
assert 2<r['installer_size']<=25<<20 and re.fullmatch('[a-f0-9]{64}',r['updater_sha256'])
for name in ['HostAgent.exe','CompanyHost.exe']:assert r['retained_host_hashes'][name]==r['components_sha256'][name]==r['retained_runtime_sha256'][name]
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={
'pt':'2.2.1.9: Alt > Câmera abre uma janela flutuante sobre a cabine para ajustar posição, altura, direção do olhar, FOV e espelhos em tempo real. Permite salvar por ônibus, restaurar e fechar por Alt/Esc. Inclui as correções da 2.2.1.8 hotfix, com retorno dos passageiros de conexão. Mantém hosts completos, relógio, configurações, sons e servidor P14.',
'en':'2.2.1.9: Alt > Camera opens the floating live cab editor for position, height, look direction, FOV and mirrors. Save per bus, reset, or close with Alt/Esc. Includes the 2.2.1.8 passenger-return hotfix and retains complete hosts, clock controls, settings, sounds and P14 server.',
'de':'2.2.1.9: Alt > Kamera öffnet den schwebenden Live-Editor für Position, Höhe, Blickrichtung, Sichtfeld und Spiegel. Pro Bus speichern, zurücksetzen oder mit Alt/Esc schließen. Enthält den Umsteiger-Hotfix 2.2.1.8 und behält vollständige Hosts, Uhrsteuerung, Einstellungen, Töne und P14-Server.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2219';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified 2.2.1.9 candidate with the original trusted publisher.')
