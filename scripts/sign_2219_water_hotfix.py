"""Sign the verified camera release locally with the publisher already trusted by Setup."""
import argparse,base64,json,re
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.9-hotfix.1';SEQUENCE=2026102705
p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert a.candidate_run==40602950335 and a.candidate_commit=='9d5e204f69da673e3f1dc855634a863f1ba1c642'
for flag in ['umbrella_water_tested','camera_panel_tested','camera_live_adjustments','cross_player_connection_repatriation','complete_hosts_unchanged','complete_p15_ui_tested','admin_clock_opt_in_tested','admin_time_opt_in','p14_server_unchanged','desktop_launcher_unchanged','custom_closing_signal','setup_backup_restore_tested','setup_backup_controls_ui_tested','connection_simulation_tested','fuel_cli_tested','official_045_p15','app_server_native_045']:assert r[flag],flag
assert r['umbrella_water_test_run']==40602798735 and r['previous_camera_run']==40590327114
assert not r['complete_hosts_rebuilt'] and r['camera_test_run']==40587532083 and r['retained_hotfix_run']==40577793377
assert r['native_candidate_run']==40602798735 and r['native_candidate_commit']=='32b9a97016f10bba7cfed39fa9921b59a2a2f7c9'
assert r['native_version']=='0.3.45-coop.15-2.2.1.9-hotfix.1' and r['coop_protocol']==15
assert r['previous_installer_sha256']=='663be37b48d91b7297df1490f77e30ef7284f94cd22dad4f844343a209356f15'
assert r['previous_coop_sha256']=='c02b46219669a75b5b7836e941fb347df0c1fe4682913b30721ccafed7508ff8'
assert r['native_client_sha256']['openomsi.exe']==r['dynamic_p15_sha256']
assert 2<r['installer_size']<=25<<20 and re.fullmatch('[a-f0-9]{64}',r['updater_sha256'])
for name in ['HostAgent.exe','CompanyHost.exe']:assert r['retained_host_hashes'][name]==r['components_sha256'][name]==r['retained_runtime_sha256'][name]
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={
'pt':'2.2.1.9 hotfix 1: corrige as gotas do guarda-chuva sumindo no início do fechamento. A camada de água acompanha a dobra e a inclinação do tecido, inclusive ao abaixar, enquanto permanece molhado. Mantém Alt > Câmera, as correções da 2.2.1.8 hotfix, os hosts completos, configurações, sons e servidor P14.',
'en':'2.2.1.9 hotfix 1: fixes umbrella water vanishing at the start of closing. Wet fabric and rim droplets follow canopy folding and lowering. Retains Alt > Camera and the cumulative passenger-return, complete-host, settings, sound and P14 fixes.',
'de':'2.2.1.9 Hotfix 1: Regentropfen verschwinden beim Schließen nicht mehr sofort, sondern folgen dem Falten und Absenken des nassen Schirms. Alt > Kamera und bisherige Korrekturen, vollständige Hosts, Einstellungen, Töne und P14 bleiben erhalten.'}

m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge2219-hotfix1';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified 2.2.1.9-hotfix.1 candidate with the original trusted publisher.')
