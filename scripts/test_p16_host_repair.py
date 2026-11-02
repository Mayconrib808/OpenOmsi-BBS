"""Run the actual PS5 migration, including old/P16 preferences and unrelated paths."""
import copy, hashlib, json, os, shutil, subprocess, sys, tempfile
from pathlib import Path

package = Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    installed = root/'Bridge com espacos'
    shutil.copytree(package, installed)
    server = installed/'app/server-dinamico-p16/openomsi.exe'
    server.parent.mkdir(parents=True)
    server.write_bytes(b'fixture server, never executed')
    for name in ('steam_api64.dll','msvcp140.dll','vcruntime140.dll','vcruntime140_1.dll'):
        (server.parent/name).write_bytes(b'fixture DLL, never executed')
    old_exe = installed/'HostAgent.exe';old_exe.write_bytes(b'complete released host retained')
    appdata = root/'Local AppData'
    private = appdata/'OpenOmsi-BBS';private.mkdir(parents=True)
    old_path = private/'HostAgent.local.json'
    target = private/'HostAgent-P16.local.json'
    config = dict(root=str(root/'OMSI 2'),server=str(server),control_key='private-ci-only',control_port=27199,
                  company=dict(protocol=15,openomsi_version='0.3.46',clock=dict(timezone='Europe/Berlin',shift_minutes=-480),future_company='keep'),
                  maps={'brigadeiro':dict(voice=False,voice_radio=False,voice_range=135,public_server=False,
                        require_hardware_id=False,admin_password='private-admin-ci',port=27015,web_port=27025,future_map='keep')},
                  host_key='private-host-ci',room_id='same-company-code',start_with_windows=True,future_root='keep')
    old_path.write_text(json.dumps(config),encoding='utf-8')
    original = old_path.read_bytes()
    env=dict(os.environ,LOCALAPPDATA=str(appdata))
    def prepare(ok=True):
        result=subprocess.run(['powershell.exe','-NoLogo','-NoProfile','-ExecutionPolicy','Bypass','-File',str(installed/'Abrir-Servidor-P16.ps1'),'-PrepareOnly'],env=env,capture_output=True,text=True)
        assert (result.returncode==0)==ok,result.stdout+result.stderr
    prepare()
    got=json.loads(target.read_bytes())
    expected=copy.deepcopy(config);expected.update(server=str(server),use_custom_server=True)
    expected['company'].update(protocol=16,openomsi_version='0.3.47')
    assert got==expected
    assert old_path.read_bytes()==original and old_exe.read_bytes()==b'complete released host retained'
    got['maps']['brigadeiro']['voice_range']=210
    target.write_text(json.dumps(got),encoding='utf-8');previous=target.read_bytes()
    prepare()
    assert json.loads(target.read_bytes())==got
    assert any(p.read_bytes()==previous for p in private.glob('HostAgent-P16.local.json.before-p16-*.bak'))
    got['server']=str(root/'independent-custom-server/openomsi.exe')
    target.write_text(json.dumps(got),encoding='utf-8');previous=target.read_bytes()
    prepare(ok=False)
    assert target.read_bytes()==previous and old_path.read_bytes()==original
    print('PASS: Windows PS5 migration, spaces, exact original preserved, keys/maps/options/false/extra fields, existing P16 choices/backups, independent custom server rejected without changes')
(package/'validacao/repair-migration-windows.json').write_text(json.dumps(dict(status='passed',original_unchanged=True,unknown_fields_preserved=True,explicit_false_preserved=True,p16_preferences_preserved=True,backup_verified=True,custom_server_preserved=True),indent=2)+'\n')
