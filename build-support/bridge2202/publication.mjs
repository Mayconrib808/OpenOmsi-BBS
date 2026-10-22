// Publish only the exact native-tested, originally signed Bridge 2.2.0.2 files.
import {readFileSync,writeFileSync,readdirSync,mkdirSync,cpSync} from 'node:fs';
import {join} from 'node:path';
import {createHash,createPublicKey,verify} from 'node:crypto';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const directory='build-support/bridge2202';
const publication=JSON.parse(readFileSync(join(directory,'publication.json')));
const signed=readFileSync(join(directory,'stable.json'));
const envelope=JSON.parse(signed);
const raw=Buffer.from(envelope.payload,'base64');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
if(!verify(null,raw,key,Buffer.from(envelope.signature,'base64')))throw Error('Publisher signature differs');
const manifest=JSON.parse(raw),installer=manifest.installer;
if(manifest.version!=='2.2.0.2'||manifest.sequence!==2026102203||publication.version!==manifest.version||publication.sequence!==manifest.sequence||installer.sha256!==publication.installer_sha256||installer.url!==`${origin}/packages/${installer.sha256}.exe`||!Number.isSafeInteger(installer.size)||installer.size<2||installer.size>25*1024*1024)throw Error('Release identity differs');
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const request=async url=>{
 const response=await fetch(url,{headers:{'User-Agent':'OpenOmsi-BBS-Setup/2.2.0.1','Cache-Control':'no-cache'},signal:AbortSignal.timeout(90000)});
 if(!response.ok)throw Error('HTTP '+response.status+' '+url);
 return response;
};

if(process.argv[2]==='prepare'){
 const report=json('approved-candidate/build-report.json');
 const exe=readFileSync('approved-candidate/Atualizar-Bridge.exe');
 if(report.version!=='2.2.0.2'||report.sequence!==2026102203||report.updater_sha256!==installer.sha256||report.full_package_sha256!==publication.full_package_sha256||report.coop_archive_sha256!==publication.coop_archive_sha256||exe.length!==installer.size||sha(exe)!==installer.sha256||exe.subarray(0,2).toString()!=='MZ')throw Error('Candidate bytes differ from the signed release');
 const full=readFileSync('approved-full/OpenOmsi_BBS_2.2.0.2_Setup_e_Coop8.zip');
 if(sha(full)!==publication.full_package_sha256)throw Error('Full package differs');
 const native=json('approved-evidence/windows-native.json');
 if(native.setup_gui!=='passed'||native.bundled_coop_button!=='passed'||native.updater_progress!=='passed')throw Error('Native Windows interface checks did not pass');
 const scan=json('approved-evidence/defender-2202.json');
 if(scan.status!=='passed'||!scan.archive_scanning_enabled||scan.files.length!==2||!scan.files.every(f=>f.passed&&f.exit_code===0))throw Error('Exact-file Defender checks did not pass');
 for(const [name,wanted] of [['Atualizar-Bridge.exe',installer.sha256],['OpenOmsi_BBS_2.2.0.2_Setup_e_Coop8.zip',publication.full_package_sha256]])if(scan.files.find(f=>f.name===name)?.sha256!==wanted)throw Error('Defender checked different bytes');
 const live=await request(origin+'/stable.json');
 const current=JSON.parse(Buffer.from(JSON.parse(await live.text()).payload,'base64'));
 if(current.sequence>manifest.sequence||(current.sequence===manifest.sequence&&current.installer.sha256!==installer.sha256))throw Error('A newer or different release is already live');
 cpSync('approved-previous/assets','distribution/assets',{recursive:true});
 for(const name of ['worker-assets.mjs','wrangler.jsonc'])cpSync('update-service/'+name,'distribution/'+name);
 mkdirSync('distribution/assets/packages',{recursive:true});
 writeFileSync('distribution/assets/stable.json',signed);
 writeFileSync(`distribution/assets/packages/${installer.sha256}.exe`,exe);
 const spec=json('approved-candidate/coop-0.3.38-coop.8.json');
 if(spec.sha256!==publication.coop_archive_sha256||spec.parts.length!==4)throw Error('Coop identity differs');
 const chunks=[];
 for(const part of spec.parts){
  const data=readFileSync(`approved-candidate/coop-assets/${part.sha256}.bin`);
  if(data.length!==part.size||sha(data)!==part.sha256||!exe.includes(Buffer.from(part.sha256)))throw Error('Coop part differs or is not pinned in the installer');
  writeFileSync(`distribution/assets/packages/${part.sha256}.bin`,data);chunks.push(data);
 }
 const coop=Buffer.concat(chunks);
 if(coop.length!==spec.size||sha(coop)!==spec.sha256||!exe.includes(Buffer.from(spec.sha256)))throw Error('Complete Coop archive differs');
 for(const hash of Object.values(spec.files))if(!exe.includes(Buffer.from(hash)))throw Error('A Coop file is not pinned in the updater');
 for(const name of readdirSync('distribution/assets/packages')){
  const match=/^([a-f0-9]{64})\.(exe|bin)$/.exec(name);
  if(!match||sha(readFileSync('distribution/assets/packages/'+name))!==match[1])throw Error('Invalid immutable asset: '+name);
 }
 console.log('PREPARED: exact signed 2.2.0.2, native Windows evidence, full package, Coop 8; previous 2.2.0 and 2.2.0.1 files retained.');
}else if(process.argv[2]==='verify-live'){
 const response=await request(origin+'/stable.json');
 const actual=Buffer.from(await response.arrayBuffer());
 if(!actual.equals(signed)||!response.headers.get('cache-control')?.includes('no-store'))throw Error('Live signed manifest differs');
 let count=0;
 for(const name of readdirSync('distribution/assets/packages')){
  const expected=readFileSync('distribution/assets/packages/'+name);
  const response=await request(origin+'/packages/'+name);
  const bytes=Buffer.from(await response.arrayBuffer());
  if(bytes.length!==expected.length||sha(bytes)!==sha(expected)||!response.headers.get('cache-control')?.includes('immutable'))throw Error('Live asset differs: '+name);
  count++;
 }
 console.log(`LIVE VERIFIED: Bridge 2.2.0.2 + Coop 8, sequence 2026102203, updater ${installer.sha256}, ${installer.size} bytes, ${count} immutable files verified.`);
}else throw Error('Expected prepare or verify-live');
