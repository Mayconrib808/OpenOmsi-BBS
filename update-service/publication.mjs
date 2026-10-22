// Validate a signed release before copying its approved Actions artifact.
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {createPublicKey,verify,createHash} from 'node:crypto';
import {join} from 'node:path';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const publicKey=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
const manifest=readFileSync('update-service/assets/stable.json');
const envelope=JSON.parse(manifest);
const payload=Buffer.from(envelope.payload,'base64');
if(!verify(null,payload,publicKey,Buffer.from(envelope.signature,'base64'))) throw Error('Invalid release signature');
const release=JSON.parse(payload), f=release.installer;
if(release.schema!==1 || !Number.isSafeInteger(release.sequence) || release.sequence<1 || !/^[a-f0-9]{64}$/.test(f.sha256) || f.url!==`${origin}/packages/${f.sha256}.exe` || f.size>25*1024*1024 || f.size<2) throw Error('Invalid release metadata');
const coop=release.version==='2.2.0'?JSON.parse(readFileSync('docs/releases/coop-0.3.38-coop.6.json')):null;
if(coop && (coop.size!==63884704 || coop.sha256!=='78da217b753f664f3c56c70875c5bd3ceb213a22d805140a69bbd3e42940c081' || coop.parts.length!==4 || Object.keys(coop.files).length!==45)) throw Error('Unexpected Coop distribution');
if(process.argv[2]==='prepare') {
 const bytes=readFileSync(process.argv[3]);
 if(bytes.length!==f.size || bytes.subarray(0,2).toString()!=='MZ' || createHash('sha256').update(bytes).digest('hex')!==f.sha256) throw Error('Candidate installer does not match signed manifest');
 mkdirSync('update-service/assets/packages',{recursive:true});
 writeFileSync(`update-service/assets/packages/${f.sha256}.exe`,bytes);
 if(coop) {
  if(!process.argv[4]) throw Error('Approved Coop parts are required');
  const parts=[];
  for(const part of coop.parts) {
   if(!/^[a-f0-9]{64}$/.test(part.sha256) || part.size<1 || part.size>20*1024*1024 || !bytes.includes(Buffer.from(part.sha256))) throw Error('Installer does not pin this Coop part');
   const content=readFileSync(join(process.argv[4],`${part.sha256}.bin`));
   if(content.length!==part.size || createHash('sha256').update(content).digest('hex')!==part.sha256) throw Error('Approved Coop part integrity failed');
   parts.push(content);
   writeFileSync(`update-service/assets/packages/${part.sha256}.bin`,content);
  }
  const archive=Buffer.concat(parts);
  if(archive.length!==coop.size || createHash('sha256').update(archive).digest('hex')!==coop.sha256 || !bytes.includes(Buffer.from(coop.sha256))) throw Error('Coop archive differs from the installer pin');
  for(const hash of Object.values(coop.files)) if(!bytes.includes(Buffer.from(hash))) throw Error('Installer does not pin every approved Coop file');
  console.log(`Verified ${coop.parts.length} Coop parts, exact archive ${coop.sha256}`);
 }
 console.log(`Verified release ${release.version}, sequence ${release.sequence}, ${bytes.length} bytes`);
} else if(process.argv[2]==='verify-live') {
 const r=await fetch(`${origin}/stable.json`,{headers:{'User-Agent':'OpenOmsi-BBS-Setup/2.1.9.8','Cache-Control':'no-cache'},signal:AbortSignal.timeout(15000)});
 const live=Buffer.from(await r.arrayBuffer());
 if(!r.ok || r.headers.get('cache-control')!=='no-store' || !live.equals(manifest)) throw Error(`Live manifest differs from approved release: HTTP ${r.status}, Cache-Control ${r.headers.get('cache-control')}, SHA256 ${createHash('sha256').update(live).digest('hex')}`);
 const p=await fetch(f.url,{headers:{'User-Agent':'Go-http-client/1.1'},signal:AbortSignal.timeout(60000)});
 if(!p.ok || !p.headers.get('cache-control')?.includes('immutable')) throw Error('Live package unavailable');
 const bytes=Buffer.from(await p.arrayBuffer());
 if(bytes.length!==f.size || createHash('sha256').update(bytes).digest('hex')!==f.sha256) throw Error('Live installer integrity failed');
 if(coop) {
  const archiveHash=createHash('sha256');let total=0;
  for(const part of coop.parts) {
   const result=await fetch(`${origin}/packages/${part.sha256}.bin`,{headers:{'User-Agent':'OpenOmsi-BBS-Updater/2.2.0'},signal:AbortSignal.timeout(60000)});
   if(!result.ok || !result.headers.get('cache-control')?.includes('immutable')) throw Error('Live Coop part unavailable');
   const content=Buffer.from(await result.arrayBuffer());
   if(content.length!==part.size || createHash('sha256').update(content).digest('hex')!==part.sha256) throw Error('Live Coop part integrity failed');
   archiveHash.update(content);total+=content.length;
  }
  if(total!==coop.size || archiveHash.digest('hex')!==coop.sha256) throw Error('Live complete Coop archive integrity failed');
  console.log(`LIVE COOP VERIFIED: ${coop.parts.length} parts, ${total} bytes, SHA256 ${coop.sha256}`);
 }
 console.log(`LIVE VERIFIED: ${release.version}, sequence ${release.sequence}, SHA256 ${f.sha256}, ${bytes.length} bytes`);
} else throw Error('Expected prepare or verify-live');
