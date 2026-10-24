import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const repo=process.env.GITHUB_REPOSITORY;
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const pub=json('build-support/bridge2208/publication.json');
const signed=readFileSync('build-support/bridge2208/stable.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(b){const e=JSON.parse(b);const raw=Buffer.from(e.payload,'base64');if(!verify(null,raw,key,Buffer.from(e.signature,'base64')))throw Error('Publisher private-key signature differs from original installed key');return JSON.parse(raw);}
const manifest=decode(signed),installer=manifest.installer;
if(manifest.schema!==1||manifest.version!=='2.2.0.8'||manifest.sequence!==2026102411||pub.version!==manifest.version||pub.sequence!==manifest.sequence||installer.sha256!==pub.installer_sha256||installer.size!==pub.installer_size||installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.size>25*1024*1024)throw Error('Release identity/signature invalid');
function collect(dir,allowMissing=false){
 if(!allowMissing&&!readdirSync(dir).length)throw Error('Empty previous artifact: '+dir);
 const report=json(dir+'/build-report.json'),exe=readFileSync(dir+'/Atualizar-Bridge.exe');
 if(report.updater_sha256!==sha(exe))throw Error('Historical installer SHA mismatch: '+dir);
 const name=sha(exe)+'.exe';
 writeFileSync('distribution/assets/packages/'+name,exe);
 const parts=dir+'/coop-assets';
 if(existsSync(parts))for(const f of readdirSync(parts)){const bytes=readFileSync(parts+'/'+f);if(f!==sha(bytes)+'.bin'||bytes.length>20*1024*1024)throw Error('Immutable chunk differs: '+dir+'/'+f);writeFileSync('distribution/assets/packages/'+f,bytes)}
 return report;
}
if(process.argv[2]==='prepare'){
 if(!Number.isSafeInteger(pub.candidate_run_id)||!/^[a-f0-9]{40}$/.test(pub.candidate_commit))throw Error('No approved candidate');
 const r=JSON.parse(execFileSync('gh',['api','repos/'+repo+'/actions/runs/'+pub.candidate_run_id],{encoding:'utf8'}));
 if(r.conclusion!=='success'||r.status!=='completed'||r.head_sha!==pub.candidate_commit||r.path!=='.github/workflows/release-2208-updater.yml'||r.head_branch!=='release/2.2.0.8-candidate')throw Error('Wrong candidate build run');
 const report=json('approved-new/build-report.json'),exe=readFileSync('approved-new/Atualizar-Bridge.exe'),spec=json('approved-new/coop-spec.json');
 if(report.version!=='2.2.0.8'||report.sequence!==2026102411||report.coop_protocol!==15||!report.official_041_p15||!report.app_server_native_041||!report.custom_closing_signal||report.dynamic_p15_sha256!==spec.files['app/server-dinamico-p15/openomsi.exe']||report.updater_sha256!==sha(exe)||report.updater_sha256!==installer.sha256||exe.length!==installer.size||report.coop_archive_sha256!==spec.sha256||spec.size>128*1024*1024||spec.parts.length>8||report.base_package_sha256!=='74e1c44bfe88ab10bb16c31daaa36a72794d6a91893994badd03db785604fe45')throw Error('Candidate payload not authentic to new Coop14 beta');
 const all=Buffer.concat(spec.parts.map(p=>{const bytes=readFileSync('approved-new/coop-assets/'+p.sha256+'.bin');if(sha(bytes)!==p.sha256||bytes.length!==p.size||!exe.includes(Buffer.from(p.sha256)))throw Error('New chunk integrity mismatch');return bytes}));
 if(sha(all)!==spec.sha256||all.length!==spec.size||!exe.includes(Buffer.from(spec.sha256)))throw Error('Coop archive not authenticated by signed updater');
 const prev=json('approved-prior2207/build-report.json');
 if(prev.full_package_sha256!==report.base_package_sha256)throw Error('Unexpected previous release');
 const needed=[
 'approved-previous2','approved-base','approved-hotfix','approved-prior2204','approved-prior2205',
 'approved-prior2206','approved-prior22061','approved-prior22062','approved-prior2207','approved-new'
 ];
 mkdirSync('distribution/assets/packages',{recursive:true});
 cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
 for(const dir of needed)collect(dir);
 writeFileSync('distribution/assets/stable.json',signed);
 for(const n of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+n,'distribution/'+n);
 for(const n of readdirSync('distribution/assets/packages'))if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(n)||sha(readFileSync('distribution/assets/packages/'+n))!==n.slice(0,64))throw Error('Immutable historical asset mismatch '+n);
 console.log('STAGED valid signed 2.2.0.8, real Coop14 v0.3.41 P15 binaries, Fechou and '+readdirSync('distribution/assets/packages').length+' retained assets');
}else if(process.argv[2]==='verify-live'){
 const opts={headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)};
 const response=await fetch(origin+'/stable.json',opts);
 if(!response.ok||!Buffer.from(await response.arrayBuffer()).equals(signed))throw Error('LIVE SIGNED MANIFEST MISMATCH');
 for(const file of [installer.sha256+'.exe',...json('approved-new/coop-spec.json').parts.map(p=>p.sha256+'.bin')]){
  const remote=await fetch(origin+'/packages/'+file,opts);
  if(!remote.ok)throw Error('LIVE CHUNK HTTP '+remote.status+' '+file);
  const bytes=Buffer.from(await remote.arrayBuffer());
  if(sha(bytes)!==file.slice(0,64))throw Error('LIVE CHUNK HASH MISMATCH '+file);
 }
 console.log('LIVE VERIFIED: Update 2.2.0.8 + genuine Coop14 0.3.41, managed P15 server and Fechou; Setup Buscar atualizacoes will show this signed sequence');
}else throw Error('Expected prepare or verify-live');