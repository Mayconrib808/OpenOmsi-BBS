"""Recover the exact already published runtime used in the 3.x test package."""
import hashlib,json,urllib.request,zipfile,io
from pathlib import Path
root=Path(__file__).resolve().parents[1]
url='https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v2.0.5-dev.4/OpenOmsi.%2B.BBS.2.0.5-dev.4.zip'
with urllib.request.urlopen(url,timeout=120) as response:data=response.read()
if hashlib.sha256(data).hexdigest()!='6c704e92037813aa16e8c3b14aa100a55bd1e2668d1491e65adcfda77a7190dc':raise SystemExit('Published native runtime archive hash mismatch')
expected=json.loads((root/'server/bbs-server.json').read_text())
with zipfile.ZipFile(io.BytesIO(data)) as archive:
 for name,sha in expected['files'].items():
  payload=archive.read('app/server/'+name)
  if hashlib.sha256(payload).hexdigest()!=sha:raise SystemExit('Native runtime hash mismatch: '+name)
  (root/'server'/name).write_bytes(payload)
print('Verified native runtime recovered; no 2.x player installation required.')
