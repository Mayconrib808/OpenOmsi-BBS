import {readFileSync,readdirSync} from 'node:fs';
import {createHash,verify,createPublicKey} from 'node:crypto';
import {join} from 'node:path';
const hash=b=>createHash('sha256').update(b).digest('hex');
const base='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const envelope=JSON.parse(readFileSync('assets/stable.json'));
const raw=Buffer.from(envelope.payload,'base64');
const pub=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
if(!verify(null,raw,pub,Buffer.from(envelope.signature,'base64'))) throw Error('Assinatura de publicação inválida');
const manifest=JSON.parse(raw);
if(manifest.version!=='2.2.0.1'||manifest.sequence!==2026102202) throw Error('Versão ou sequência incorreta');
if(manifest.installer.url!==`${base}/packages/${manifest.installer.sha256}.exe`) throw Error('Endereço do atualizador incorreto');
const installer=readFileSync(`assets/packages/${manifest.installer.sha256}.exe`);
if(hash(installer)!==manifest.installer.sha256||installer.length!==manifest.installer.size||installer.subarray(0,2).toString()!=='MZ') throw Error('Instalador inválido');
const checks=[['coop6.json','67741c25f265d829b310cdaedff1f625725eeddbdce9fa977181bcbcd88220db'],['coop7.json',manifest.installer.sha256]];
const expected=new Set([manifest.installer.sha256+'.exe']);
for(const [filename,exeSha] of checks){
 const spec=JSON.parse(readFileSync(filename));const blobs=[];
 if(!spec.parts||spec.parts.length!==4) throw Error('Quantidade inesperada de partes em '+filename);
 expected.add(exeSha+'.exe');
 for(const part of spec.parts){
  const p=`assets/packages/${part.sha256}.bin`,b=readFileSync(p);
  if(b.length!==part.size || hash(b)!==part.sha256) throw Error('Parte não confere: '+p);
  expected.add(part.sha256+'.bin'); blobs.push(b);
 }
 const archive=Buffer.concat(blobs);
 if(archive.length!==spec.size || hash(archive)!==spec.sha256) throw Error('Arquivo Coop não confere: '+filename);
}
for(const f of readdirSync('assets/packages'))if(!expected.has(f))throw Error('Arquivo adicional inesperado: '+f);
console.log('OK: assinatura Ed25519 original, versão, sequência, instaladores e Coop 6 + 7 íntegros.');
