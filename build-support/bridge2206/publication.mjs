import {readFileSync,writeFileSync,readdirSync,mkdirSync,cpSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const dir='build-support/bridge2206', json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const publication=json(dir+'/publication.json'),signed=readFileSync(dir+'/stable.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
const sha=b=>createHash('sha256').update(b).digest('hex');
function decode(bytes){const e=JSON.parse(bytes),raw=Buffer.from(e.payload,'base64');if(!verify(null,raw,key,Buffer.from(e.signature,'base64')))throw Error('Invalid publisher signature');return JSON.parse(raw);}
const manifest=decode(signed),installer=manifest.installer;
if(manifest.version!=='2.2.0.6'||manifest.sequence!==2026102406||publication.version!==manifest.version||publication.sequence!==manifest.sequence||installer.sha256!==publication.installer_sha256||installer.url!==`${origin}/packages/${installer.sha256}.exe`||installer.size>25*1024*1024)throw Error('Release identity differs');
async function request(url){const r=await fetch(url,{headers:{'User-Agent':'OpenOmsi-BBS/2.2.0.6','Cache-Control':'no-cache'},signal:AbortSignal.timeout(90000)});if(!r.ok)throw Error('HTTP '+r.status);return r;}
function addInstaller(path,report){const b=readFileSync(path);if(sha(b)!==report.updater_sha256)throw Error('Previous updater differs');writeFileSync(`distribution/assets/packages/${sha(b)}.exe`,b);}
function addParts(path){for(const n of readdirSync(path)){const b=readFileSync(path+'/'+n);if(sha(b)+'.bin'!==n)throw Error('Previous part differs');writeFileSync('distribution/assets/packages/'+n,b);}}
if(process.argv[2]==='prepare'){
 const report=json('approved-new/build-report.json'),exe=readFileSync('approved-new/Atualizar-Bridge.exe'),spec=json('approved-new/coop-spec.json');
 if(report.coop_protocol!==15||!report.native_clock_readiness_fix||!report.double_horn||!report.active_map_copy_buttons||report.version!==manifest.version||report.sequence!==manifest.sequence||!report.world_interest_both_hosts||!report.dynamic_p15_sha256||spec.files['app/server-dinamico-p15/openomsi.exe']!==report.dynamic_p15_sha256||!report.native_setup_smoke||!report.native_server_lamp||!report.native_manual_and_timer||sha(exe)!==installer.sha256||exe.length!==installer.size||report.updater_sha256!==installer.sha256||report.full_package_sha256!==publication.full_package_sha256||spec.sha256!==report.coop_archive_sha256)throw Error('Candidate differs');
 const scan=json('approved-evidence/defender-2206.json'),native=json('approved-evidence/server-status-hotfix.json');
 if(scan.status!=='passed'||!scan.archive_scanning_enabled||scan.files.length!==2||!scan.files.every(f=>f.passed&&f.exit_code===0)||!native.active_map_copy_buttons||!native.clipboard_and_pagination||!native.native_setup_gui||!native.manual_button||native.automatic_interval_seconds!==5||!native.host_preferences_unchanged)throw Error('Native or Defender validation incomplete');
 for(const [n,h] of [['Atualizar-Bridge.exe',installer.sha256],['OpenOmsi_BBS_2.2.0.6_Setup_e_Coop12.zip',publication.full_package_sha256]])if(scan.files.find(f=>f.name===n)?.sha256!==h)throw Error('Scan covers different bytes');
 const clock=json('approved-evidence/native-clock-regression.json');
 if(!clock.passed||clock.protocol!==15||clock.clock_updates.length!==3)throw Error('Actual P15 clock roundtrip evidence missing');
 const prior=json('approved-prior2205/build-report.json');
 if(report.base_package_sha256!==prior.full_package_sha256)throw Error('2.2.0.5 baseline mismatch');
 const retained=Object.entries(report.retained_components_sha256);
 if(retained.length!==4||retained.some(([n,h])=>prior.components_sha256[n]!==h))throw Error('P15 host or compatibility helpers changed');
 const current=decode(await(await request(origin+'/stable.json')).text());
 if(current.sequence>manifest.sequence||(current.sequence===manifest.sequence&&current.installer.sha256!==installer.sha256))throw Error('Newer release already live');
 cpSync('approved-previous/assets','distribution/assets',{recursive:true});mkdirSync('distribution/assets/packages',{recursive:true});
 for(const d of ['approved-previous2','approved-base','approved-hotfix','approved-prior2204','approved-prior2205'])addInstaller(d+'/Atualizar-Bridge.exe',json(d+'/build-report.json'));
 for(const d of ['approved-previous2','approved-base','approved-prior2204','approved-prior2205','approved-new'])addParts(d+'/coop-assets');
 const chunks=spec.parts.map(p=>{const b=readFileSync('distribution/assets/packages/'+p.sha256+'.bin');if(b.length!==p.size||sha(b)!==p.sha256||!exe.includes(Buffer.from(p.sha256)))throw Error('New part differs');return b;});
 const all=Buffer.concat(chunks);if(all.length!==spec.size||sha(all)!==spec.sha256||!exe.includes(Buffer.from(spec.sha256)))throw Error('New Coop bundle differs');
 for(const h of Object.values(spec.files))if(!exe.includes(Buffer.from(h)))throw Error('New file pin missing');
 for(const n of ['worker-assets.mjs','wrangler.jsonc'])cpSync('update-service/'+n,'distribution/'+n);
 writeFileSync('distribution/assets/stable.json',signed);writeFileSync('distribution/assets/packages/'+installer.sha256+'.exe',exe);
 for(const n of readdirSync('distribution/assets/packages'))if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(n)||sha(readFileSync('distribution/assets/packages/'+n))!==n.slice(0,64))throw Error('Immutable asset mismatch');
 console.log('Verified signed 2.2.0.6, native lamp, exact Defender checks, preserved complete P15 host and every previous immutable download.');
}else if(process.argv[2]==='verify-live'){
 const response=await request(origin+'/stable.json');
 if(!Buffer.from(await response.arrayBuffer()).equals(signed)||!response.headers.get('cache-control')?.includes('no-store'))throw Error('Live manifest differs');
 for(const n of readdirSync('distribution/assets/packages')){const expected=readFileSync('distribution/assets/packages/'+n),r=await request(origin+'/packages/'+n),b=Buffer.from(await r.arrayBuffer());if(b.length!==expected.length||sha(b)!==sha(expected)||!r.headers.get('cache-control')?.includes('immutable'))throw Error('Live download differs: '+n);}
 console.log('LIVE VERIFIED: Bridge 2.2.0.6, Coop12 0.3.40 protocol15, native clock confirmations, two horn taps and active map copy buttons; every new and previous immutable download verified.');
}else throw Error('Expected prepare or verify-live');
