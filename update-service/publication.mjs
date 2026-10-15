// Validate a signed release before copying its approved Actions artifact.
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {createPublicKey,verify,createHash} from 'node:crypto';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const publicKey=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
const manifest=readFileSync('update-service/assets/stable.json');
const envelope=JSON.parse(manifest);
const payload=Buffer.from(envelope.payload,'base64');
if(!verify(null,payload,publicKey,Buffer.from(envelope.signature,'base64'))) throw Error('Invalid release signature');
const release=JSON.parse(payload), f=release.installer;
if(release.schema!==1 || !Number.isSafeInteger(release.sequence) || release.sequence<1 || !/^[a-f0-9]{64}$/.test(f.sha256) || f.url!==`${origin}/packages/${f.sha256}.exe` || f.size>25*1024*1024 || f.size<2) throw Error('Invalid release metadata');
if(process.argv[2]==='prepare') {
 const bytes=readFileSync(process.argv[3]);
 if(bytes.length!==f.size || bytes.subarray(0,2).toString()!=='MZ' || createHash('sha256').update(bytes).digest('hex')!==f.sha256) throw Error('Candidate installer does not match signed manifest');
 mkdirSync('update-service/assets/packages',{recursive:true});
 writeFileSync(`update-service/assets/packages/${f.sha256}.exe`,bytes);
 console.log(`Verified release ${release.version}, sequence ${release.sequence}, ${bytes.length} bytes`);
} else if(process.argv[2]==='verify-live') {
 const r=await fetch(`${origin}/stable.json`,{signal:AbortSignal.timeout(15000)});
 const live=Buffer.from(await r.arrayBuffer());
 if(!r.ok || r.headers.get('cache-control')!=='no-store' || !live.equals(manifest)) throw Error(`Live manifest differs from approved release: HTTP ${r.status}, Cache-Control ${r.headers.get('cache-control')}, SHA256 ${createHash('sha256').update(live).digest('hex')}`);
 const p=await fetch(f.url,{signal:AbortSignal.timeout(60000)});
 if(!p.ok || !p.headers.get('cache-control')?.includes('immutable')) throw Error('Live package unavailable');
 const bytes=Buffer.from(await p.arrayBuffer());
 if(bytes.length!==f.size || createHash('sha256').update(bytes).digest('hex')!==f.sha256) throw Error('Live installer integrity failed');
 console.log(`LIVE VERIFIED: ${release.version}, sequence ${release.sequence}, SHA256 ${f.sha256}, ${bytes.length} bytes`);
} else throw Error('Expected prepare or verify-live');
