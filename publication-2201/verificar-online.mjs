import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const hash=b=>createHash('sha256').update(b).digest('hex');
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const expected=readFileSync('assets/stable.json');const envelope=JSON.parse(expected);
const payload=JSON.parse(Buffer.from(envelope.payload,'base64').toString());
const req=async url=>{const r=await fetch(url,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(90000)});if(!r.ok)throw Error('HTTP '+r.status+' '+url);return r;};
const stable=await req(origin+'/stable.json');
if(!stable.headers.get('cache-control')?.includes('no-store'))throw Error('Manifesto não é no-store');
const got=Buffer.from(await stable.arrayBuffer());
if(!got.equals(expected))throw Error('Manifesto publicado não confere com o assinado');
const installer=Buffer.from(await(await req(payload.installer.url)).arrayBuffer());
if(installer.length!==payload.installer.size||hash(installer)!==payload.installer.sha256)throw Error('Instalador público divergente');
for(const filename of ['coop6.json','coop7.json']){
 const spec=JSON.parse(readFileSync(filename));const blobs=[];
 for(const part of spec.parts){
  const b=Buffer.from(await(await req(origin+'/packages/'+part.sha256+'.bin')).arrayBuffer());
  if(b.length!==part.size || hash(b)!==part.sha256)throw Error('Parte pública divergente: '+part.sha256);
  blobs.push(b);
 }
 if(hash(Buffer.concat(blobs))!==spec.sha256)throw Error('Coop pública divergente: '+filename);
}
console.log('PUBLICAÇÃO CONFIRMADA: v2.2.0.1 e todos os binários/Coop 6+7 verificados no endereço público.');
