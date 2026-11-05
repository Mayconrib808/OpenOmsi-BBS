#!/usr/bin/env python3
"""Diagnose exact published 2.2.3.1 package integrity without skipping SHA checks."""
from pathlib import Path
import hashlib,json,zipfile,sys

z=Path('approved-new/OpenOMSI-BCS-Bridge-2.2.3.1-FULL.zip')
if not z.is_file():
    raise SystemExit('Approved portable zip is missing')
core=['Setup.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe','app/compat/omsi-plugin-host32.exe','PeDePeAdapter/openomsi.exe']
def digest(b):return hashlib.sha256(b).hexdigest()
with zipfile.ZipFile(z) as f:
    names=set(f.namelist())
    base='docs/SHA256.txt'
    entries={}
    if base in names:
        for l in f.read(base).decode('utf-8-sig').splitlines():
            if len(l)>66 and l[64:66]=='  ': entries[l[66:]]=l[:64]
    patches={}
    for fname in ['docs/PATCH-PLAYER-SHA256.txt','docs/PATCH-HOST-SHA256.txt']:
        if fname in names:
            for l in f.read(fname).decode('utf-8-sig').splitlines():
                if len(l)>66 and l[64:66]=='  ': patches[l[66:]]=l[:64]
    checks={}
    for name in core:
        actual=digest(f.read(name)) if name in names else None
        base_expected=entries.get(name)
        patch=patches.get(name)
        effective=patch or base_expected
        checks[name]={'present':name in names,'base_sha256':base_expected,'patch_sha256':patch,'actual_sha256':actual,'match':actual==effective,'listed_in_base':bool(base_expected)}
    game='OpenOmsi Coop 0.3.41/programa/openomsi.exe'
    binary=f.read(game)[:512] if game in names else b''
    machine=None
    if binary[:2]==b'MZ':
        pos=int.from_bytes(binary[0x3c:0x40],'little')
        machine=int.from_bytes(binary[pos+4:pos+6],'little') if pos+6<=len(binary) else None
    output={'archive':str(z),'number_of_files':len(names),'base_sha256_present':base in names,'core_checks':checks,'game_present':game in names,'game_pe_machine':hex(machine) if machine else None,'manifest_base_entries':len(entries),'patch_entries':len(patches),'all_core_match':all(x['match'] for x in checks.values())}
Path('activation-diagnostics.json').write_text(json.dumps(output,indent=2)+'\n',encoding='utf-8')
print(json.dumps(output,indent=2))
