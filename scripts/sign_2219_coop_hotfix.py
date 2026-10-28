"""Sign the verified cooperative release locally with the publisher already trusted by Setup."""
import argparse,base64,json,re
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.9-hotfix.2';SEQUENCE=2026102801
p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert a.candidate_run==40765546864 and a.candidate_commit=='c6a2361529b55c75154fc973ece14b612f3bd0fa'
for flag in ['umbrella_water_tested','camera_panel_tested','camera_live_adjustments','cross_player_connection_repatriation','complete_hosts_unchanged','complete_p15_ui_tested','admin_clock_opt_in_tested','admin_time_opt_in','p14_server_unchanged','desktop_launcher_unchanged','custom_closing_signal','setup_backup_restore_tested','setup_backup_controls_ui_tested','connection_simulation_tested','fuel_cli_tested','official_045_p15','app_server_native_045']:assert r[flag],flag
assert r['umbrella_water_test_run']==40602798735 and r['previous_camera_run']==40590327114
assert not r['complete_hosts_rebuilt'] and r['camera_test_run']==40587532083 and r['retained_hotfix_run']==40577793377
assert r['native_candidate_run']==40764717872 and r['native_candidate_commit']=='55c11a9b4a3fdbf41de762824960431ae1862a2c'
assert r['native_version']=='0.3.45-coop.15-2.2.1.9-coop-fix.1' and r['coop_protocol']==15
assert r['previous_installer_sha256']=='46407da31d7cbd586e14dfb50a86ef5b6912bb1465182836f318beffdcccf21c'
assert r['previous_coop_sha256']=='58cc975604d084f40c29b0a04864058cbfd9d3b66a937a965ad50b04d766bc4a'
assert r['native_client_sha256']['openomsi.exe']==r['dynamic_p15_sha256']
assert 2<r['installer_size']<=25<<20 and re.fullmatch('[a-f0-9]{64}',r['updater_sha256'])
for name in ['HostAgent.exe','CompanyHost.exe']:assert r['retained_host_hashes'][name]==r['components_sha256'][name]==r['retained_runtime_sha256'][name]
for flag in ['coop_cash_collection_tested','coop_physical_cash_mirror','coop_current_position_return','coop_logical_passenger_cabin','coop_hud_owner_count','coop_network_backpressure','previous_hotfix_preserved']:assert r[flag],flag
assert r['coop_focused_tests_passed']==34 and r['previous_hotfix_run']==40602950335
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={
'pt':'2.2.1.9 hotfix 2: sincroniza moedas e notas do cobrador e permite recolher o pagamento por Shift+T, botão e mouse, sem duplicação e preservando o troco. Corrige a cabine e o contador de passageiros após a troca de motorista, o retorno na posição atual do ônibus e causas de tremidas/fila de rede cheia. Mantém a viagem BCS, hosts completos, P14, sons, câmeras e correções anteriores.',
'en':'2.2.1.9 hotfix 2: synchronizes conductor cash and authorized collection, preserves passenger cabins and counts after driver handover, returns the player at the current bus position and fixes camera/network jitter causes. Retains the BCS trip, complete hosts, P14, sounds, cameras and previous fixes.',
'de':'2.2.1.9 Hotfix 2: synchronisiert Bargeld und berechtigten Geldeinzug, erhält Fahrgastkabine und Zähler beim Fahrerwechsel und gibt die Figur an der aktuellen Busposition zurück. Kamera- und Netzwerkruckeln korrigiert; BCS-Fahrt, vollständige Hosts, P14, Töne und bisherige Korrekturen bleiben erhalten.'}

m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2219-hotfix2';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified 2.2.1.9-hotfix.2 candidate with the original trusted publisher.')
