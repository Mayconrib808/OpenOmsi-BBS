from pathlib import Path
import base64,hashlib,io,json,zipfile

support=Path(__file__).resolve().parent
root=support.parents[2]
game=root/'game'
spec=json.loads((support/'overlay.json').read_text())
data=base64.b64decode(spec['zip_base64'],validate=True)
if hashlib.sha256(data).hexdigest()!=spec['zip_sha256']:raise ValueError('Character source archive differs')
with zipfile.ZipFile(io.BytesIO(data)) as z:
 if set(z.namelist())!=set(spec['files']):raise ValueError('Unexpected source entry set')
 for name in z.namelist():
  relative=Path(name)
  if relative.is_absolute() or '..' in relative.parts:raise ValueError('Invalid source entry')
  data=z.read(name)
  if hashlib.sha256(data).hexdigest()!=spec['files'][name]:raise ValueError('Character source file differs')
  target=game/relative;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
out=root/'out';out.mkdir(exist_ok=True)
(out/'character-source-sha256.json').write_text(json.dumps(spec['files'],indent=2)+'\n')
print('Applied verified character selection and on-foot network fixes.')
