"""Recover the unmodified official server pinned by the package provenance."""
import hashlib,json,urllib.request,zipfile,io
from pathlib import Path
root=Path(__file__).resolve().parents[1]
receipt=json.loads((root/"server/bbs-server.json").read_text())
assert receipt["protocol"]==7 and receipt["unmodified"]
with urllib.request.urlopen(receipt["upstream_url"],timeout=120) as response:data=response.read()
assert hashlib.sha256(data).hexdigest()==receipt["archive_sha256"],"Official runtime archive hash mismatch"
with zipfile.ZipFile(io.BytesIO(data)) as archive:
 for name,sha in receipt["files"].items():
  assert Path(name).name==name
  candidates=[p for p in archive.namelist() if Path(p).name==name]
  assert len(candidates)==1
  payload=archive.read(candidates[0])
  assert hashlib.sha256(payload).hexdigest()==sha,"Official runtime file hash mismatch: "+name
  (root/"server"/name).write_bytes(payload)
print("Verified unmodified official server",receipt["version"],"protocol",receipt["protocol"])
