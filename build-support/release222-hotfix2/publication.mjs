import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const json=path=>JSON.parse(readFileSync(path,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release222-hotfix2/stable.json');
const oldSigned=readFileSync('build-support/release222-patrol/stable.json');
const info=json('build-support/release222-hotfix2/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw) {
  const obj=JSON.parse(raw);
  const payload=Buffer.from(obj.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(obj.signature,'base64')))throw Error('Invalid publisher signature');
  return JSON.parse(payload);
}
const current=decode(oldSigned), release=decode(signed);
if(current.version!=='2.2.2-hotfix.1'||current.sequence!==2026103002)throw Error('Unexpected previous signed version');
if(release.schema!==1||release.version!=='2.2.2-hotfix.2'||release.sequence!==2026103003||release.sequence<=current.sequence)throw Error('Wrong signed new release identity');
if(info.version!==release.version||info.sequence!==release.sequence)throw Error('Publication record disagrees with signature');
const installer=release.installer;
if(installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024)throw Error('Invalid signed installer');
function collect(directory){
  const report=json(directory+'/build-report.json');
  const binary=readFileSync(directory+'/Atualizar-Bridge.exe'),hash=sha(binary);
  if(hash!==report.updater_sha256)throw Error('Corrupt historical installer: '+directory);
  writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
  const chunks=directory+'/coop-assets';
  if(existsSync(chunks))for(const file of readdirSync(chunks)){
    const data=readFileSync(chunks+'/'+file);
    if(file!==sha(data)+'.bin'||data.length>20*1024*1024)throw Error('Corrupt historical chunk: '+file);
    writeFileSync('distribution/assets/packages/'+file,data);
  }
}
async function loadLive() {
  const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw Error('Current production manifest unavailable');
  return Buffer.from(await response.arrayBuffer());
}
if(process.argv[2]==='prepare'){
  const live=await loadLive(),now=decode(live);
  // Never overwrite a newer or different update with a stale test version.
  if(!live.equals(oldSigned)||now.sequence!==current.sequence)throw Error('Live release is different; refuse to publish');
  const report=json('approved-new/build-report.json'),spec=json('approved-new/coop-spec.json');
  const binary=readFileSync('approved-new/Atualizar-Bridge.exe');
  if(report.version!=='2.2.2'||report.sequence!==2026103003||report.update_origin!==origin
      ||report.updater_sha256!==installer.sha256||sha(binary)!==installer.sha256||binary.length!==installer.size
      ||report.coop_archive_sha256!==spec.sha256||report.previous_installer_sha256!=='cd055e9a00c2b0459128c088f93b05b7cfd8668bdd42616e582673cf338f4f6b'
      ||report.previous_coop_sha256!=='bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574'
      ||report.native_version!=='0.3.46-coop.15-2.2.2'
      ||report.native_candidate_run!==41157142308||report.native_build_commit!=='64db8910d135d123162ed90d772b06831909c9e2'
      ||report.complete_hosts_unchanged!==true||report.complete_hosts_rebuilt!==false||report.p14_server_unchanged!==true
      ||report.custom_sounds_unchanged!==true||report.bcs_single_and_company_event_fixture_tested!==true
      ||report.bcs_capture_normal_launch_tested!==true||report.setup_changelog_ui_tested!==true
      ||report.bridge_regressions_tested!==true||report.setup_live_update_ui_tested!==true
      ||report.windows_gameplay_tested!==false||report.paired_gameplay_tested!==false)
    throw Error('2.2.2 candidate is incomplete, misidentified or regressed');
  const names=['HostAgent.exe','CompanyHost.exe'];
  for(const name of names)if(report.retained_host_hashes[name]!==report.retained_runtime_sha256[name]||report.retained_runtime_sha256[name]!==report.components_sha256[name])throw Error('Complete host changed unexpectedly: '+name);
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Historical asset baseline unavailable');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name))throw Error('Unexpected immutable asset: '+name);
    if(sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Historical asset hash mismatch: '+name);
  }
  if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe'))throw Error('Missing Hotfix 1 updater fallback');
  const chunks=[];
  for(const part of spec.parts){
    const bytes=readFileSync('distribution/assets/packages/'+part.sha256+'.bin');
    if(sha(bytes)!==part.sha256||bytes.length!==part.size||!binary.includes(Buffer.from(part.sha256)))throw Error('Coop manifest chunk error');
    chunks.push(bytes);
  }
  if(sha(Buffer.concat(chunks))!==spec.sha256||!binary.includes(Buffer.from(spec.sha256)))throw Error('Coop archive mismatch');
  console.log('STAGED HOTFIX 2: verified signed 2.2.2 Hotfix 2; unchanged complete hosts, P14 and previous immutable versions retained');
}else if(process.argv[2]==='verify-live'){
  const live=await loadLive();
  if(!live.equals(signed))throw Error('Live manifest not 2.2.2 Hotfix 2 signed payload');
  const spec=json('approved-new/coop-spec.json');
  const checks=[installer.sha256+'.exe',current.installer.sha256+'.exe',...spec.parts.map(x=>x.sha256+'.bin')];
  for(const name of checks){
    const resp=await fetch(origin+'/packages/'+name,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!resp.ok||sha(Buffer.from(await resp.arrayBuffer()))!==name.slice(0,64))throw Error('Live package missing or corrupt: '+name);
  }
  console.log('LIVE VERIFIED: Bridge 2.2.2 Hotfix 2 available through Buscar atualizacoes with signed installer and exact P15 chunks');
}else throw Error('Expected prepare or verify-live');
