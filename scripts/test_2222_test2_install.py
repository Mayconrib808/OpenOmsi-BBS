"""One real updater invocation over a running old Setup, in an isolated Windows fixture."""
import argparse
import ctypes
from ctypes import wintypes
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import uuid
from build_runtime_2203 import embedded_payload

def windows():
    user=ctypes.WinDLL('user32',use_last_error=True)
    callback=ctypes.WINFUNCTYPE(wintypes.BOOL,wintypes.HWND,wintypes.LPARAM)
    user.EnumWindows.argtypes=[callback,wintypes.LPARAM]
    user.GetWindowThreadProcessId.argtypes=[wintypes.HWND,ctypes.POINTER(wintypes.DWORD)]
    user.GetClassNameW.argtypes=[wintypes.HWND,wintypes.LPWSTR,ctypes.c_int]
    user.PostMessageW.argtypes=[wintypes.HWND,wintypes.UINT,wintypes.WPARAM,wintypes.LPARAM]
    found=[]
    @callback
    def visit(hwnd,_):
        cls=ctypes.create_unicode_buffer(128);pid=wintypes.DWORD()
        user.GetClassNameW(hwnd,cls,len(cls));user.GetWindowThreadProcessId(hwnd,ctypes.byref(pid))
        if cls.value=='OpenOmsiBBSSetup':found.append((hwnd,pid.value))
        return True
    user.EnumWindows(visit,0)
    return user,found

def wait_until(predicate,seconds,message):
    deadline=time.monotonic()+seconds
    while time.monotonic()<deadline:
        value=predicate()
        if value:return value
        time.sleep(0.05)
    raise AssertionError(message)

def main():
    p=argparse.ArgumentParser();p.add_argument('candidate',type=Path);p.add_argument('--baseline',type=Path,required=True);a=p.parse_args()
    out=a.candidate.resolve();spec=json.loads((out/'coop-spec.json').read_text())
    previous=embedded_payload((a.baseline/'Atualizar-Bridge.exe').read_bytes())
    expected_setup=hashlib.sha256((out/'runtime/Setup.exe').read_bytes()).hexdigest()
    old=None;updater=None;target=None
    with tempfile.TemporaryDirectory(prefix='bridge-one-click-',dir=out.parent) as temporary:
        folder=Path(temporary);target=folder/'Bridge with spaces'
        # Use independent files, including when a runner's system temp/cache
        # and checkout reside on different volumes. Hard links fail with
        # WinError 17 across C: and D: and can share fixture writes with inputs.
        shutil.copytree(out/'runtime',target)
        setup=target/'Setup.exe';setup.unlink();setup.write_bytes(previous['Setup.exe'])
        # Exercise the real failed case: every large existing game/server/launcher/
        # source/asset differs, at its full original size. The old test changed only
        # a tiny game marker, so it retained almost no rollback data in memory.
        changed_bytes=0
        for name in spec['files']:
            path=target/name
            if path.is_file() and path.stat().st_size:
                changed_bytes+=path.stat().st_size
                with path.open('r+b') as f:
                    f.seek(-1,2);last=f.read(1);f.seek(-1,2);f.write(bytes([last[0]^1]))
        assert changed_bytes>200<<20,'fixture must replace a full previous runtime'
        # Assert the shipped installer is actually x64, not just the unit tests.
        import struct
        pe=(out/'Atualizar-Bridge.exe').read_bytes()
        pe_offset=struct.unpack_from('<I',pe,0x3c)[0]
        assert struct.unpack_from('<H',pe,pe_offset+4)[0]==0x8664,'installer must be x64'
        del pe
        note=target/'company-user-setting.txt';note.write_text('preserve company preferences\n')
        cache=folder/'cache';roaming=folder/'roaming';cache.mkdir();roaming.mkdir()
        parts=cache/'OpenOmsi-BBS/Coop-0.3.41';parts.mkdir(parents=True)
        for part in (out/'coop-assets').glob('*.bin'):shutil.copy2(part,parts/part.name)
        ready=cache/'OpenOmsi-BBS/Updates'/('.bridge-handoff-'+uuid.uuid4().hex+'.signal')
        ready.parent.mkdir(parents=True)
        env=dict(os.environ,LOCALAPPDATA=str(cache),APPDATA=str(roaming))
        p17_migration = 'app/hosts-p17/HostAgent.exe' in spec['files']
        if p17_migration:
            private = cache/'OpenOmsi-BBS'; private.mkdir(exist_ok=True)
            p16 = private/'HostAgent-P16.local.json'
            original_p16 = json.dumps(dict(server=str(target/'app/server-dinamico-p16/openomsi.exe'),use_custom_server=True,company=dict(protocol=16,openomsi_version='0.3.47',future_company='keep'),maps={'ci-map':dict(voice=False,voice_radio=False,voice_range=135,public_server=False,require_hardware_id=False,admin_password='ci-private-admin')},host_key='ci-key',room_id='ci-room',control_key='ci-control',control_port=27199,future_private='keep')).encode()
            p16.write_bytes(original_p16)

        try:
            old=subprocess.Popen([str(setup)],cwd=target,env=env)
            wait_until(lambda:any(pid==old.pid for _,pid in windows()[1]),30,'old Setup window did not open')
            updater=subprocess.Popen([str(out/'Atualizar-Bridge.exe'),'--target-dir',str(target),'--wait-setup',str(old.pid),'--reopen','--handoff-ready',str(ready)],cwd=out,env=env)
            wait_until(ready.exists,30,'actual updater never acknowledged startup')
            assert ready.read_text().strip()=='READY',ready.read_text()
            assert old.poll() is None,'updater closed Setup before acknowledgement was accepted'
            # Exercise the Windows CRLF form explicitly rather than relying on
            # this Python interpreter's platform-specific newline conversion.
            Path(str(ready)+'.accepted').write_bytes(b'ACCEPTED\r\n')
            code=updater.wait(timeout=180)
            install_log=(target/'app/setup-v1.1.3.log').read_text(errors='replace')
            assert code==0,f'first updater invocation failed: exit={code}\n{install_log[-6000:]}'
            assert 'automatic-update-install: <nil>' in install_log,f'updater exited without installing:\n{install_log[-6000:]}'
            old.wait(timeout=20)
            assert hashlib.sha256(setup.read_bytes()).hexdigest()==expected_setup,'old Setup was not replaced'
            for name,digest in spec['files'].items():
                actual=hashlib.sha256((target/name).read_bytes()).hexdigest()
                assert actual==digest,f'one-click update did not install exact managed file: {name}'
            assert note.read_text()=='preserve company preferences\n','unmanaged preferences changed'
            if p17_migration:
                assert p16.read_bytes()==original_p16, 'P16 settings changed during the actual updater'
                migrated=json.loads((private/'HostAgent-P17.local.json').read_bytes())
                previous=json.loads(original_p16)
                for field in ('maps','host_key','room_id','control_key','control_port','future_private'):
                    assert migrated[field]==previous[field], 'P17 migration lost '+field
                assert migrated['company']==dict(protocol=17,openomsi_version='0.3.49',future_company='keep')
                assert os.path.samefile(migrated['server'],target/'app/server-dinamico-p17/openomsi.exe')

            wait_until(lambda:bool(windows()[1]),30,'updated Setup was not reopened visibly')
            log=(target/'app/setup-v1.1.3.log').read_text(errors='replace')
            assert 'automatic-update-install: <nil>' in log or 'automatic-update automatic-update-install: <nil>' in log,log[-6000:]
            (out/'one-click-install-test.json').write_text(json.dumps({
                'status':'passed','actual_executable':'Atualizar-Bridge.exe',
                'invocations':1,'running_old_setup_replaced':True,
                'startup_acceptance_required':True,'coop_extraction_exercised':True,
                'windows_crlf_acceptance_tested':True,
                'full_previous_runtime_replaced_bytes':changed_bytes,
                'rollback_storage':'disk','updater_architecture':'amd64',
                'managed_files_verified':len(spec['files']),'p17_actual_updater_migration_tested':p17_migration,
                'unmanaged_preferences_preserved':True,'setup_reopened':True,
                'setup_sha256':expected_setup,'windows_gameplay_tested':False,
            },indent=2)+'\n')
            print('ONE-CLICK UPDATE PASSED: actual old Setup replaced on first invocation')
        finally:
            if target:
                log=target/'app/setup-v1.1.3.log'
                if log.exists():shutil.copy2(log,out/'one-click-install-setup.log')
            user,opened=windows()
            for hwnd,pid in opened:user.PostMessageW(hwnd,0x10,0,0)
            time.sleep(0.3)
            for proc in (updater,old):
                if proc and proc.poll() is None:proc.terminate();proc.wait(timeout=10)
            # Close only fixture Setup windows left by --reopen on this runner.
            for _,pid in opened:
                subprocess.run(['taskkill','/PID',str(pid),'/F'],capture_output=True)

if __name__=='__main__':main()
