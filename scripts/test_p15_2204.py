"""Exercise the shipped P15 Win32 controls, Save and reopen; no host is started."""
import argparse
import ctypes as C
from ctypes import wintypes as W
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unicodedata

FIELDS = ('voice', 'voice_radio', 'voice_range', 'public_server',
          'require_hardware_id', 'admin_password')


def normalized(value):
    return ''.join(c for c in unicodedata.normalize('NFD', value.lower())
                   if unicodedata.category(c) != 'Mn')


class NativeUI:
    def __init__(self, pid):
        self.pid = pid
        self.u = C.WinDLL('user32', use_last_error=True)
        self.callback = C.WINFUNCTYPE(W.BOOL, W.HWND, W.LPARAM)
        self.u.EnumWindows.argtypes = [self.callback, W.LPARAM]
        self.u.EnumChildWindows.argtypes = [W.HWND, self.callback, W.LPARAM]
        self.u.GetWindowThreadProcessId.argtypes = [W.HWND, C.POINTER(W.DWORD)]
        self.u.SendMessageTimeoutW.argtypes = [W.HWND, W.UINT, W.WPARAM,
                                              W.LPARAM, W.UINT, W.UINT,
                                              C.POINTER(C.c_size_t)]
        self.u.SendMessageTimeoutW.restype = W.LPARAM
        self.u.PostMessageW.argtypes = [W.HWND, W.UINT, W.WPARAM, W.LPARAM]
        self.u.GetClassNameW.argtypes = [W.HWND, W.LPWSTR, C.c_int]
        self.u.GetWindowLongW.argtypes = [W.HWND, C.c_int]
        self.u.GetWindowRect.argtypes = [W.HWND, C.POINTER(W.RECT)]
        self.u.GetDlgCtrlID.argtypes = [W.HWND]
        self.u.GetParent.argtypes = [W.HWND]
        self.u.GetParent.restype = W.HWND

    def send(self, hwnd, message, wparam=0, lparam=0):
        result = C.c_size_t()
        ok = self.u.SendMessageTimeoutW(hwnd, message, wparam, lparam,
                                       2, 3000, C.byref(result))
        if not ok:
            raise RuntimeError('Native control did not respond')
        return result.value

    def text(self, hwnd):
        length = min(self.send(hwnd, 0xE), 4096)
        cls = C.create_unicode_buffer(128)
        self.u.GetClassNameW(hwnd, cls, 128)
        if cls.value.lower() == 'edit' and self.u.GetWindowLongW(hwnd, -16) & 0x20:
            # Windows protects cross-process WM_GETTEXT for password controls.
            # Preserve their HWND/layout; verify the real saved value in JSON.
            return '*' * length
        buffer = C.create_unicode_buffer(length + 1)
        self.send(hwnd, 0xD, length + 1, C.cast(buffer, C.c_void_p).value)
        return buffer.value

    def windows(self):
        found = []
        @self.callback
        def collect(hwnd, _):
            pid = W.DWORD()
            self.u.GetWindowThreadProcessId(hwnd, C.byref(pid))
            if pid.value == self.pid:
                found.append(hwnd)
            return True
        self.u.EnumWindows(collect, 0)
        return found

    def main_window(self):
        for hwnd in self.windows():
            if 'Host Agent 2.2.0.3' in self.text(hwnd):
                return hwnd
        return None

    def children(self, window):
        found = []
        @self.callback
        def collect(hwnd, _):
            name = C.create_unicode_buffer(128)
            self.u.GetClassNameW(hwnd, name, 128)
            rect = W.RECT()
            self.u.GetWindowRect(hwnd, C.byref(rect))
            found.append(dict(hwnd=hwnd, text=self.text(hwnd), cls=name.value,
                              id=self.u.GetDlgCtrlID(hwnd),
                              rect=(rect.left, rect.top, rect.right, rect.bottom)))
            return True
        self.u.EnumChildWindows(window, collect, 0)
        return found

    def controls(self, window):
        children = self.children(window)
        def button(*words):
            matches = [c for c in children if c['cls'].lower() == 'button'
                       and all(w in normalized(c['text']) for w in words)]
            if len(matches) != 1:
                raise RuntimeError('Expected one P15 control: ' + repr(words))
            return matches[0]
        def edit_near(*words):
            labels = [c for c in children if c['cls'].lower() == 'static'
                      and all(w in normalized(c['text']) for w in words)]
            if len(labels) != 1:
                raise RuntimeError('Expected one P15 edit label: ' + repr(words))
            label = labels[0]['rect']
            edits = [c for c in children if c['cls'].lower() == 'edit']
            # Prefer the box immediately below a label, or on its right.
            def distance(c):
                r = c['rect']
                below = abs(r[0]-label[0]) + abs(r[1]-label[3])*3
                beside = abs(r[0]-label[2])*2 + abs(r[1]-label[1])*5
                return min(below, beside)
            return min(edits, key=distance)
        result = {
            'voice': button('voz', 'proximidade'),
            'voice_radio': button('radio'),
            'voice_range': edit_near('alcance'),
            'public_server': button('lista', 'publica'),
            'require_hardware_id': button('identificador'),
            'admin_password': edit_near('senha', 'admin'),
        }
        combos = [c for c in children if c['cls'].lower() == 'combobox'
                  and c['text'] in ('CI primary', 'CI secondary')]
        if len(combos) != 1:
            raise RuntimeError('Configured map combo not found')
        result['map'] = combos[0]
        result['save'] = next(c for c in children
                              if c['cls'].lower() == 'button' and c['text'] == 'Salvar')
        return result

    def set_values(self, controls, values):
        for name in FIELDS:
            hwnd = controls[name]['hwnd']
            if isinstance(values[name], bool):
                self.send(hwnd, 0xF1, int(values[name]))  # BM_SETCHECK
            else:
                text = C.create_unicode_buffer(str(values[name]))
                self.send(hwnd, 0xC, 0, C.cast(text, C.c_void_p).value)

    def assert_values(self, controls, values):
        for name in FIELDS:
            hwnd = controls[name]['hwnd']
            actual = bool(self.send(hwnd, 0xF0)) if isinstance(values[name], bool) else self.text(hwnd)
            expected = values[name] if isinstance(values[name], bool) else str(values[name])
            if name == 'admin_password':
                expected = '*' * len(expected)
            if actual != expected:
                raise AssertionError('Native control did not retain ' + name)

    def select_map(self, controls, index):
        combo = controls['map']
        self.send(combo['hwnd'], 0x14E, index)  # CB_SETCURSEL
        self.u.PostMessageW(self.u.GetParent(combo['hwnd']), 0x111,
                            combo['id'] | (1 << 16), combo['hwnd'])
        time.sleep(.25)

    def save(self, controls):
        save = controls['save']
        # Post the native button command; an error dialog cannot deadlock this test.
        self.u.PostMessageW(self.u.GetParent(save['hwnd']), 0x111, save['id'], save['hwnd'])


def fixture(folder, server):
    root = folder / 'OMSI 2'
    root.mkdir()
    (root / 'Omsi.exe').write_bytes(b'CI fixture; never executed')
    for name in ('Vehicles', 'Drivers'):
        (root / name).mkdir()
    files = []
    sessions = []
    maps = {}
    for index, suffix in enumerate(('A', 'B')):
        name = 'CI-' + suffix
        path = 'maps/' + name + '/global.cfg'
        f = root / path
        f.parent.mkdir(parents=True)
        f.write_text('[name]\nCI Map ' + suffix + '\n', encoding='utf-8')
        files.append(dict(path=path, sha256=hashlib.sha256(f.read_bytes()).hexdigest()))
        sessions.append(dict(id='ci-'+suffix.lower(), name=('CI primary','CI secondary')[index],
                             map_name='CI Map '+suffix, map_file=path, date='company',
                             server_url='http://127.0.0.1:'+str(27025+index),
                             required_packages=['ci-data'], clock_tolerance_seconds=90))
        maps['ci-'+suffix.lower()] = dict(enabled=True, port=27015+index,
            web_port=27025+index, max_players=16, traffic=50, passengers=True,
            max_people=1000, timetable=True, idle_minutes=30, metar_station='EPSC',
            voice=False, voice_radio=False, voice_range=(70,210)[index],
            public_server=False, require_hardware_id=False, admin_password='')
    company = dict(schema_version=1, company_id='ci-protocol', company_name='CI native P15 test',
                   openomsi_version='0.3.39', protocol=14,
                   clock=dict(timezone='Europe/Berlin', shift_minutes=-480),
                   packages=[dict(id='ci-data',name='CI assets',version='1',
                                  folders=['maps/CI-A','maps/CI-B'],files=files)],
                   sessions=sessions)
    return dict(schema=1, root=str(root), server=str(server), use_custom_server=True,
                language='pt', host_key='a'*64, room_id='b'*32, control_key='c'*32,
                control_port=27199, start_with_windows=False, company=company, maps=maps)


def wait_for(predicate, timeout=15):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        try:
            last = predicate()
            if last:
                return last
        except (OSError, ValueError, RuntimeError, AssertionError) as exc:
            last = exc
        time.sleep(.2)
    raise RuntimeError('Native UI condition timed out: ' + str(last))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--package', type=Path, required=True)
    args = parser.parse_args()
    package = args.package.resolve()
    report = package / 'validation/native-p15-controls.json'
    local_temp = Path(os.environ['LOCALAPPDATA'])/'Temp'
    local_temp.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='host2202-real-ui-',dir=local_temp) as temp, tempfile.TemporaryDirectory(prefix='ui-moved-install-', dir=package.parent) as install_temp:
        folder = Path(temp)
        env = os.environ.copy()
        for key in ('LOCALAPPDATA','APPDATA'):
            env[key] = str(folder / key)
            Path(env[key]).mkdir()
        config = Path(env['LOCALAPPDATA']) / 'OpenOmsi-BBS/HostAgent.local.json'
        config.parent.mkdir()
        old_server = folder/'Downloads/Bridge_2.2.0_Maycon_Servidor_Dinamico_P14 (1)/app/server-dinamico-p14/openomsi.exe'
        before = fixture(folder, old_server)
        config.write_text(json.dumps(before), encoding='utf-8')
        installed = Path(install_temp)/'OpenOmsi_BBS_2.1.1 Host'
        installed.mkdir()
        for name in ('Setup.exe','PeDePeAdapter/openomsi.exe','app/bridge.ini'):
            path = installed/name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b'Existing player file, preserved by the host installer')
        for name in ('HostAgent.exe','CompanyHost.exe'):
            shutil.copy2(package/name,installed/name)
        assert not old_server.exists()
        assert old_server.drive.lower() != installed.drive.lower(), 'Cross-drive fixture did not use C: and D:'
        installer_env = env.copy()
        # A Python child of pwsh inherits PowerShell 7 module paths. Let the
        # Windows PowerShell 5.1 process initialize its own standard module paths.
        for key in list(installer_env):
            if key.lower() == 'psmodulepath':
                del installer_env[key]
        installer_env['PSModulePath'] = str(Path(os.environ['SystemRoot'])/'System32/WindowsPowerShell/v1.0/Modules')
        subprocess.run(['powershell.exe','-NoProfile','-ExecutionPolicy','Bypass','-File',
            str(package/'scripts/atualizar_host_pessoal.ps1'),'-Destination',str(installed),
            '-NonInteractive','-NoLaunch'],env=installer_env,cwd=package,check=True,timeout=60)
        saved = json.loads(config.read_text(encoding='utf-8-sig'))
        actual_server = installed/'app/server-dinamico-p14/openomsi.exe'
        assert saved['server'] == str(actual_server) and actual_server.is_file()
        assert hashlib.sha256(actual_server.read_bytes()).hexdigest() == hashlib.sha256((package/'app/server-dinamico-p14/openomsi.exe').read_bytes()).hexdigest()
        for key in ('company','maps','host_key','room_id','control_key'):
            assert saved[key] == before[key], 'Moved install changed '+key
        before['server'] = str(actual_server)
        version = subprocess.run([str(actual_server),'--version'],cwd=installed,
            capture_output=True,text=True,check=True,timeout=30)
        assert '0.3.39-bbs-stream.3' in version.stdout + version.stderr
        values = dict(voice=True,voice_radio=True,voice_range=135,public_server=True,
                      require_hardware_id=True,admin_password='ci-admin-password-2202')
        second = dict(voice=False,voice_radio=False,voice_range=210,public_server=False,
                      require_hardware_id=False,admin_password='')
        title = None
        for reopen in (False, True):
            command = [str(installed/'HostAgent.exe')]
            if reopen:
                command += ['--config',str(config)]
            process = subprocess.Popen(command,cwd=installed,env=env)
            ui = NativeUI(process.pid)
            window = None
            try:
                window = wait_for(ui.main_window)
                controls = wait_for(lambda: ui.controls(window))
                title = ui.text(window)
                if reopen:
                    ui.assert_values(controls, values)
                    # Exercise explicit false values too: Go may omit them from JSON.
                    all_off = dict(values, voice=False, voice_radio=False,
                                   public_server=False, require_hardware_id=False)
                    ui.set_values(controls, all_off)
                    ui.save(controls)
                    wait_for(lambda: all(json.loads(config.read_text())['maps']['ci-a'].get(k,False) == v
                                         for k,v in all_off.items()))
                else:
                    ui.set_values(controls, values)
                    ui.select_map(controls, 1)
                    ui.assert_values(controls, second)
                    ui.select_map(controls, 0)
                    ui.assert_values(controls, values)
                    ui.save(controls)
                    wait_for(lambda: all(json.loads(config.read_text())['maps']['ci-a'].get(k) == v
                                         for k,v in values.items()))
                    saved = json.loads(config.read_text())
                    for key in ('host_key','room_id','control_key','server','company'):
                        if saved[key] != before[key]:
                            raise AssertionError('Native Save changed unrelated ' + key)
            except Exception:
                diagnostics = []
                for top in ui.windows():
                    diagnostics.append({'window':ui.text(top),'controls':[
                        dict(cls=c['cls'],id=c['id'],rect=c['rect'],
                             text=c['text'] if c['cls'].lower() != 'edit' else '<edit>')
                        for c in ui.children(top)]})
                (package/'validation/native-p15-ui-diagnostics.json').write_text(
                    json.dumps(diagnostics,ensure_ascii=False,indent=2),encoding='utf-8')
                print(json.dumps(diagnostics,ensure_ascii=True))
                raise
            finally:
                if window:
                    ui.u.PostMessageW(window, 0x10, 0, 0)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.terminate()
                    process.wait(timeout=5)
        report.write_text(json.dumps(dict(status='passed',persisted_fields=FIELDS,
            actual_window_title=title,map_switch_preserves_values=True,
            moved_host_cross_drive_save_passed=True,old_managed_server_missing=True,
            installed_server_exists_and_sha256_verified=True,
            installed_native_version_checked=True,default_launch_uses_repaired_config=True,
            saved_values_retained_on_reopen=True,false_values_saved=True,
            real_two_player_bcs_trip_tested=False),indent=2)+'\n',encoding='utf-8')
        print('ACTUAL P15 WIN32 CONTROLS PASSED: missing C: server repaired to D: installation; default launch, six fields, map switching, Save, reopen and false values')


if __name__ == '__main__':
    main()
