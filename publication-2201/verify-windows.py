"""Exercise the exact new Setup and updater on native Windows before publication."""
import hashlib
import io
import json
import os
import re
import struct
import subprocess
import zipfile
from pathlib import Path

ROOT = Path('prepared-publication').resolve()
EVIDENCE = ROOT/'evidence'
EVIDENCE.mkdir(exist_ok=True)
UPDATER = ROOT/'assets/packages/c734faca3808127012892b6045a1995c03ecda8abf3cf7242b13a3034449662d.exe'
data = UPDATER.read_bytes()
if hashlib.sha256(data).hexdigest() != UPDATER.stem:
    raise ValueError('Updater integrity differs')
test = ROOT/'native-test'
test.mkdir(exist_ok=True)
extracted = False
for found in re.finditer(b'PK\x05\x06',data):
    end = found.start()+22+struct.unpack_from('<H',data,found.start()+20)[0]
    try:
        archive = zipfile.ZipFile(io.BytesIO(data[:end]))
    except zipfile.BadZipFile:
        continue
    with archive:
        if 'Setup.exe' not in archive.namelist():
            continue
        if len(archive.namelist()) != 7:
            raise ValueError('Unexpected embedded Bridge payload')
        for entry in archive.infolist():
            if entry.filename.startswith('/') or '..' in Path(entry.filename).parts:
                raise ValueError('Unsafe embedded entry')
        archive.extractall(test)
        extracted = True
        break
if not extracted:
    raise ValueError('Embedded Setup not found')
with zipfile.ZipFile(ROOT/'Coop7.zip') as archive:
    prefix = json.loads((ROOT/'coop7.json').read_text())['prefix']
    for entry in archive.infolist():
        relative = entry.filename[len(prefix):]
        dest = test/'OpenOmsi Coop 0.3.38'/relative
        dest.parent.mkdir(parents=True,exist_ok=True)
        dest.write_bytes(archive.read(entry))
env = dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(EVIDENCE),OMSI_NO_UPDATE='1',OMSI_NO_GUARD='1')
subprocess.run([str(test/'Setup.exe'),'--gui-smoke'],env=env,check=True,timeout=60)
subprocess.run([str(UPDATER),'--coop-progress-smoke'],env=env,check=True,timeout=30)
game = test/'OpenOmsi Coop 0.3.38/programa/openomsi.exe'
results = {}
for flag in ['--version','--self-test']:
    run = subprocess.run([str(game),flag],cwd=game.parent,env=env,capture_output=True,text=True,encoding='utf-8',check=True,timeout=90)
    results[flag] = {'exit_code':run.returncode,'stdout':run.stdout,'stderr':run.stderr}
    if flag == '--version' and '0.3.38-coop.7' not in run.stdout+run.stderr:
        raise ValueError('Unexpected Coop game identity')
results.update(version='2.2.0.1',setup_gui='passed',updater_progress='passed',two_player_bcs_gameplay_tested=False)
(EVIDENCE/'windows-native-2201.json').write_text(json.dumps(results,indent=2)+'\n')
print('Native Windows Setup, updater progress and Coop 7 startup checks passed.')
