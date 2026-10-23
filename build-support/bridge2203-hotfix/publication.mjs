import {readFileSync,writeFileSync,readdirSync,mkdirSync,cpSync} from 'node:fs';
import {join} from 'node:path';
import {createHash,createPublicKey,verify} from 'node:crypto';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const directory='build-support/bridge2203-hotfix';
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const publication=json(join(directory,'publication.json'));
const signed=readFileSync(join(directory,'stable.json'));
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(bytes){const envelope=JSON.parse(bytes),raw=Buffer.from(envelope.payload,'base64');if(!verify(null,raw,key,Buffer.from(envelope.signature,'base64')))throw Error('Publisher signature differs');return JSON.parse(raw);}
const manifest=decode(signed),installer=manifest.installer;
const sha=b=>createHash('sha256').update(b).digest('hex');
const packageName='OpenOmsi_BBS_2.2.0.3_hotfix1_Setup_e_Coop9.zip';
if(manifest.version!=='2.2.0.3-hotfix.1'||manifest.sequence!==2026102302||publication.version!==manifest.version||publication.sequence!==manifest.sequence||installer.sha256!==publication.installer_sha256||installer.url!==`${origin}/packages/${installer.sha256}.exe`||!Number.isSafeInteger(installer.size)||installer.size<2||installer.size>25*1024*1024)throw Error('Hotfix release identity differs');
async function request(url){const r=await fetch(url,{headers:{'User-Agent':'OpenOmsi-BBS/2.2.0.3','Cache-Control':'no-cache'},signal:AbortSignal.timeout(90000)});if(!r.ok)throw Error('HTTP '+r.status+' '+url);return r;}
function addInstaller(path,report){const b=readFileSync(path);if(sha(b)!==report.updater_sha256)throw Error('Immutable updater differs');writeFileSync(`distribution/assets/packages/${report.updater_sha256}.exe`,b);}
function addParts(path){for(const name of readdirSync(path)){const b=readFileSync(join(path,name));if(sha(b)+'.bin'!==name)throw Error('Immutable Coop part differs');writeFileSync('distribution/assets/packages/'+name,b);}}
if(process.argv[2]==='prepare'){
 const report=json('approved-hotfix/build-report.json'),exe=readFileSync('approved-hotfix/Atualizar-Bridge.exe'),full=readFileSync('approved-full/'+packageName);
 if(report.version!==manifest.version||report.sequence!==manifest.sequence||report.updater_sha256!==installer.sha256||report.installer_size!==installer.size||exe.length!==installer.size||sha(exe)!==installer.sha256||report.full_package_sha256!==publication.full_package_sha256||sha(full)!==publication.full_package_sha256||!report.native_setup_smoke||!report.native_server_lamp||!report.native_manual_and_timer||report.poll_interval_seconds!==5||!report.read_only_status)throw Error('Approved native hotfix candidate differs');
 const native=json('approved-evidence/server-status-hotfix.json');
 if(native.status!=='passed'||!native.native_setup_gui||!native.lamp_ready_and_offline_rendered||!native.manual_button||native.automatic_interval_seconds!==5||native.local_requests<3||!native.authenticated||!native.read_only||!native.host_preferences_unchanged)throw Error('Native readiness validation did not pass');
 const scan=json('approved-evidence/defender-2203-hotfix.json');
 if(scan.status!=='passed'||!scan.archive_scanning_enabled||scan.files.length!==2||!scan.files.every(f=>f.passed&&f.exit_code===0))throw Error('Defender exact-file checks did not pass');
 for(const [name,hash] of [['Atualizar-Bridge.exe',installer.sha256],[packageName,publication.full_package_sha256]])if(scan.files.find(f=>f.name===name)?.sha256!==hash)throw Error('Defender checked different bytes');
 const base=json('approved-base/build-report.json');
 if(base.version!=='2.2.0.3'||base.updater_sha256!=='53a6fe30e6f074cbfc6d82a41f7af65337022b1691f06270e59ce03bb964f9d1'||report.base_updater_sha256!==base.updater_sha256||base.full_package_sha256!==report.base_package_sha256||base.coop_archive_sha256!==report.coop_archive_sha256)throw Error('Cumulative baseline differs');
 const retained=Object.entries(report.retained_components_sha256);
 if(retained.length!==6||retained.some(([name,hash])=>name==='Setup.exe'||base.components_sha256[name]!==hash))throw Error('Six complete release binaries were not preserved');
 const current=decode(await(await request(origin+'/stable.json')).text());
 if(current.sequence>manifest.sequence||(current.sequence===manifest.sequence&&current.installer.sha256!==installer.sha256))throw Error('Newer or different release is already live');
 cpSync('approved-previous/assets','distribution/assets',{recursive:true});mkdirSync('distribution/assets/packages',{recursive:true});
 addInstaller('approved-previous2/Atualizar-Bridge.exe',json('approved-previous2/build-report.json'));addParts('approved-previous2/coop-assets');
 addInstaller('approved-base/Atualizar-Bridge.exe',base);addParts('approved-base/coop-assets');
 for(const name of ['worker-assets.mjs','wrangler.jsonc'])cpSync('update-service/'+name,'distribution/'+name);
 writeFileSync('distribution/assets/stable.json',signed);writeFileSync(`distribution/assets/packages/${installer.sha256}.exe`,exe);
 const spec=json('docs/releases/coop-0.3.38-coop.9.json');
 if(spec.sha256!==report.coop_archive_sha256||spec.sha256!==publication.coop_archive_sha256||!exe.includes(Buffer.from(spec.sha256)))throw Error('Coop 9 pin changed');
 const chunks=spec.parts.map(part=>{const b=readFileSync(`distribution/assets/packages/${part.sha256}.bin`);if(b.length!==part.size||sha(b)!==part.sha256||!exe.includes(Buffer.from(part.sha256)))throw Error('Pinned Coop part differs');return b;});
 const coop=Buffer.concat(chunks);if(coop.length!==spec.size||sha(coop)!==spec.sha256)throw Error('Pinned cumulative Coop archive differs');
 for(const hash of Object.values(spec.files))if(!exe.includes(Buffer.from(hash)))throw Error('Coop file pin absent');
 for(const name of readdirSync('distribution/assets/packages')){const match=/^([a-f0-9]{64})\.(exe|bin)$/.exec(name);if(!match||sha(readFileSync('distribution/assets/packages/'+name))!==match[1])throw Error('Invalid immutable asset');}
 console.log('PREPARED: signed 2.2.0.3-hotfix.1; exact native status, timer, manual check, preserved P15 binaries and Defender evidence; every prior immutable asset retained.');
}else if(process.argv[2]==='verify-live'){
 const r=await request(origin+'/stable.json');if(!Buffer.from(await r.arrayBuffer()).equals(signed)||!r.headers.get('cache-control')?.includes('no-store'))throw Error('Live signed manifest differs');
 let count=0;for(const name of readdirSync('distribution/assets/packages')){const expected=readFileSync('distribution/assets/packages/'+name),r=await request(origin+'/packages/'+name),b=Buffer.from(await r.arrayBuffer());if(b.length!==expected.length||sha(b)!==sha(expected)||!r.headers.get('cache-control')?.includes('immutable'))throw Error('Live immutable asset differs: '+name);count++;}
 console.log(`LIVE VERIFIED: Bridge ${manifest.version}, sequence ${manifest.sequence}, updater ${installer.sha256}, ${installer.size} bytes, ${count} immutable files verified.`);
}else throw Error('Expected prepare or verify-live');
