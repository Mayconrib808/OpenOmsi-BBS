import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
import {execFileSync} from 'node:child_process';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const base='98ec07db801f028deb0874718aac3251fb6f44893dd6c96a38d4e792289ee338';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const config=json('build-support/bridge2219-hotfix3/publication.json');
const signed=readFileSync('build-support/bridge2219-hotfix3/stable.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(bytes){
  const e=JSON.parse(bytes),raw=Buffer.from(e.payload,'base64');
  if(!verify(null,raw,key,Buffer.from(e.signature,'base64')))throw Error('Publisher signature invalid');
  return JSON.parse(raw);
}
const release=decode(signed),installer=release.installer;
if(release.schema!==1||release.version!=='2.2.1.9-hotfix.3'||release.sequence!==2026102901||config.sequence!==release.sequence||config.version!==release.version||installer.sha256!==config.installer_sha256||installer.size!==config.installer_size||installer.size>25*1024*1024||installer.url!==origin+'/packages/'+installer.sha256+'.exe')throw Error('Invalid signed repair identity');
function collect(dir){
  const report=json(dir+'/build-report.json'),exe=readFileSync(dir+'/Atualizar-Bridge.exe');
  if(sha(exe)!==report.updater_sha256)throw Error('Historical installer hash differs: '+dir);
  writeFileSync('distribution/assets/packages/'+sha(exe)+'.exe',exe);
  if(existsSync(dir+'/coop-assets'))for(const name of readdirSync(dir+'/coop-assets')){
    const bytes=readFileSync(dir+'/coop-assets/'+name);
    if(name!==sha(bytes)+'.bin'||bytes.length>20*1024*1024)throw Error('Historical chunk differs: '+dir+'/'+name);
    writeFileSync('distribution/assets/packages/'+name,bytes);
  }
}
if(process.argv[2]==='prepare'){
  const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw Error('Cannot read current production release');
  const live=Buffer.from(await response.arrayBuffer()),current=decode(live);
  if(current.sequence>release.sequence||(current.sequence===release.sequence&&!live.equals(signed)))throw Error('Production advanced; refusing overwrite');
  if(current.sequence<release.sequence&&(current.sequence!==2026102801||current.installer.sha256!==base))throw Error('Different production baseline');
  const run=JSON.parse(execFileSync('gh',['api','repos/'+process.env.GITHUB_REPOSITORY+'/actions/runs/'+config.candidate_run_id],{encoding:'utf8'}));
  if(run.status!=='completed'||run.conclusion!=='success'||run.head_sha!==config.candidate_commit||run.head_branch!=='fix/distribution-configuration'||run.path!=='.github/workflows/build-2219-update-repair.yml')throw Error('Wrong tested candidate');
  const report=json('approved-new/build-report.json'),exe=readFileSync('approved-new/Atualizar-Bridge.exe'),spec=json('approved-new/coop-spec.json');
  if(report.version!==release.version||report.sequence!==release.sequence||report.update_origin!==origin||report.updater_sha256!==installer.sha256||sha(exe)!==installer.sha256||exe.length!==installer.size)throw Error('Tested installer differs from signed manifest');
  if(!report.setup_live_update_ui_tested||!report.complete_host_ui_tested||!report.localized_bcs_driver||!report.setup_update_unsaved_root||!report.coop_archive_unchanged||!report.native_game_and_servers_unchanged||report.complete_hosts_rebuilt||report.base_installer_sha256!==base||spec.sha256!=='bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574')throw Error('Missing repair/preservation evidence');
  execFileSync('python3',['scripts/verify_2219_repair_candidate.py','approved-new','approved-hotfix2'],{stdio:'inherit'});
  mkdirSync('distribution/assets/packages',{recursive:true});
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages'))if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Immutable package integrity mismatch');
  if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe'))throw Error('Current updater was not retained');
  const chunks=[];
  for(const part of spec.parts){
    const bytes=readFileSync('distribution/assets/packages/'+part.sha256+'.bin');
    if(sha(bytes)!==part.sha256||bytes.length!==part.size||!exe.includes(Buffer.from(part.sha256)))throw Error('Retained Coop chunk invalid');
    chunks.push(bytes);
  }
  if(sha(Buffer.concat(chunks))!==spec.sha256||!exe.includes(Buffer.from(spec.sha256)))throw Error('Retained Coop archive invalid');
  console.log('STAGED: Windows-tested localized update repair with every historical immutable package retained');
}else if(process.argv[2]==='verify-live'){
  const options={headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)};
  const response=await fetch(origin+'/stable.json',options);
  if(!response.ok||!Buffer.from(await response.arrayBuffer()).equals(signed))throw Error('Live repair manifest mismatch');
  const spec=json('approved-new/coop-spec.json');
  for(const name of [installer.sha256+'.exe',base+'.exe',...spec.parts.map(p=>p.sha256+'.bin')]){
    const r=await fetch(origin+'/packages/'+name,options);
    if(!r.ok||sha(Buffer.from(await r.arrayBuffer()))!==name.slice(0,64))throw Error('Live package integrity mismatch: '+name);
  }
  console.log('LIVE VERIFIED: signed Hotfix 3, repaired localized installer, retained current updater and all native game/server chunks');
}else throw Error('Expected prepare or verify-live');
