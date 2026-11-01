import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const json=path=>JSON.parse(readFileSync(path,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2222-test1/stable.json');
const oldSigned=readFileSync('build-support/release2222-hotfix1/stable.json');
const info=json('build-support/release2222-test1/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw) {
  const obj=JSON.parse(raw);
  const payload=Buffer.from(obj.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(obj.signature,'base64')))throw Error('Invalid publisher signature');
  return JSON.parse(payload);
}
const current=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));

if(current.version!=='2.2.2.2-hotfix.1'||current.sequence!==2026103106)throw Error('Expected previously deployed signed 2.2.2.2 baseline');
if(rollback.version!=='2.2.2.1-hotfix.2'||rollback.sequence!==2026103104)throw Error('Rollback Hotfix 2 manifest mismatch');
if(release.schema!==1||release.version!=='2.2.2.2-test.1'||release.sequence!==2026103107||release.sequence<=current.sequence||release.installer.sha256===current.installer.sha256)throw Error('Expected new signed TEST 1 with a distinct updater and higher sequence');
if(info.version!==release.version||info.sequence!==release.sequence)throw Error('Signed release disagrees with publication identity');
const installer=release.installer;
if(installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024)throw Error('Invalid signed updater identity');

function collect(directory) {
  const report=json(directory+'/build-report.json'), binary=readFileSync(directory+'/Atualizar-Bridge.exe'), hash=sha(binary);
  if(hash!==report.updater_sha256)throw Error('Corrupt previous installer: '+directory);
  writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
  const parts=directory+'/coop-assets';
  if(existsSync(parts))for(const name of readdirSync(parts)){
    const data=readFileSync(parts+'/'+name);
    if(name!==sha(data)+'.bin'||data.length>20*1024*1024)throw Error('Corrupt immutable Coop part: '+name);
    writeFileSync('distribution/assets/packages/'+name,data);
  }
}
async function loadLive(){
  const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw Error('Cannot fetch signed release feed');
  return Buffer.from(await response.arrayBuffer());
}
if(process.argv[2]==='validate-signatures'){
  console.log('SIGNED PREFLIGHT VERIFIED: repaired 2.2.2.2, new TEST 1, and 2.2.2.1 Hotfix 2 rollback');
}else if(process.argv[2]==='prepare'){
  const live=await loadLive(),now=decode(live);
  if(!live.equals(oldSigned)||now.version!==current.version||now.sequence!==current.sequence)throw Error('Production not exactly original signed 2.2.2.2; refusing repair overwrite');
  const report=json('approved-new/build-report.json'),spec=json('approved-new/coop-spec.json');
  const binary=readFileSync('approved-new/Atualizar-Bridge.exe');
  if(report.version!=='2.2.2.2-test.1'||report.sequence!==2026103107||report.update_origin!==origin
      ||report.updater_sha256!==installer.sha256||sha(binary)!==installer.sha256||binary.length!==installer.size
      ||report.coop_archive_sha256!==spec.sha256||report.native_version!=='0.3.46-coop.15-2.2.2.2-test.1'
      ||report.native_candidate_run!==41327386037||report.native_build_commit!=='9717969bd1890f5b03ba072a5526adfb61d919b9'||report.winter_test_preview_opt_in!==true||report.winter_visual_approved!==false
      ||report.complete_hosts_unchanged!==true||report.complete_hosts_rebuilt!==false
      ||report.p14_server_unchanged!==true||report.custom_sounds_unchanged!==true
      ||report.bridge_regressions_tested!==true||report.setup_changelog_ui_tested!==true
      ||report.setup_live_update_ui_tested!==true||report.coop_protocol!==15
      ||report.windows_gameplay_tested!==false||report.paired_gameplay_tested!==false
      ||report.native_build_commit!=='9717969bd1890f5b03ba072a5526adfb61d919b9'
      ||report.desktop_launcher_unchanged!==false||report.desktop_launcher_run!==41260965494
      ||report.desktop_launcher_commit!=='4dd5bb79d91f522ee452aba0854676a3d0fd716c')
    throw Error('Candidate incomplete, wrong version, or regression flags missing');
  if(!report.winter_real_artist_assets_included||!report.feature_event_audit_included||
      !report.per_release_changelog_including_hotfixes)throw Error('Winter/log/changelog not contained in candidate');
  for(const n of ['elvs_hooded_sweat_jacket1','mindfront_knitted_hat_01','toigo_gloves_short']){
    const f='OpenOmsi Coop 0.3.41/programa/bcs-assets/WinterClothes/'+n+'/garment.obj';
    const s='app/server-dinamico-p15/bcs-assets/WinterClothes/'+n+'/garment.obj';
    if(!spec.files[f]||spec.files[f]!==spec.files[s])throw Error('Real winter asset not matched client/server: '+n);
  }
  if(!spec.files['OpenOmsi Coop 0.3.41/programa/openomsi.exe'] || spec.files['OpenOmsi Coop 0.3.41/programa/openomsi.exe']===json('approved-prior2222-repaired/coop-spec.json').files['OpenOmsi Coop 0.3.41/programa/openomsi.exe']) throw Error('Winter native fix missing: identical binary to broken public 2.2.2.2');
  if(!report.per_release_changelog_including_hotfixes) throw Error('Hotfix is missing its independent cumulative changelog');
  const names=['HostAgent.exe','CompanyHost.exe'];
  for(const name of names)if(report.retained_host_hashes[name]!==report.retained_runtime_sha256[name]||report.retained_runtime_sha256[name]!==report.components_sha256[name])throw Error('Retained host changed: '+name);
  const game='OpenOmsi Coop 0.3.41/programa/openomsi.exe',server='app/server-dinamico-p15/openomsi.exe';
  if(spec.files[game]!==report.native_client_sha256['openomsi.exe']||spec.files[server]!==report.dynamic_p15_sha256||spec.files[game]!==spec.files[server])throw Error('Mismatched game / P15 server');
  for(const item of ['Police','Fire']){
    const client='OpenOmsi Coop 0.3.41/programa/bcs-assets/Vehicles/OpenOMSI_BCS/'+item+'/model/'+item+'.obj';
    const serverAsset='app/server-dinamico-p15/bcs-assets/Vehicles/OpenOMSI_BCS/'+item+'/model/'+item+'.obj';
    if(!spec.files[client]||spec.files[client]!==spec.files[serverAsset])throw Error('3D '+item+' client/server asset absent or mismatched');
  }
  // The previous 2.2.2.1 accidentally distributed a legacy Tauri lobby.
  // Reject a new candidate unless the complete updated launcher is included.
  const lobby='OpenOmsi Coop 0.3.41/programa/openomsi-launcher-app.exe';
  if(!report.desktop_launcher_sha256||spec.files[lobby]!==report.desktop_launcher_sha256)
    throw Error('New desktop launcher and optional settings missing from package');
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Historical assets absent');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Historical asset corrupt: '+name);
  }
  if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe'))throw Error('Previous repaired 2.2.2.2 installer missing');
  if(!existsSync('distribution/assets/packages/'+rollback.installer.sha256+'.exe'))throw Error('2.2.2.1 Hotfix 2 rollback installer missing');
  const chunks=[];
  for(const part of spec.parts){
    const bytes=readFileSync('distribution/assets/packages/'+part.sha256+'.bin');
    if(sha(bytes)!==part.sha256||bytes.length!==part.size||!binary.includes(Buffer.from(part.sha256)))throw Error('Cumulative ZIP chunk missing or changed');
    chunks.push(bytes);
  }
  if(sha(Buffer.concat(chunks))!==spec.sha256||!binary.includes(Buffer.from(spec.sha256)))throw Error('Cumulative ZIP was not authenticated by updater');
  console.log('VERIFIED TEST 1 staging: approved winter visual safety native, 14 settings, authenticated P15 and historical rollback intact');
}else if(process.argv[2]==='verify-live'){
  const live=await loadLive();
  if(!live.equals(signed))throw Error('Live update feed does not match signed 2.2.2.2 TEST 1');
  const spec=json('approved-new/coop-spec.json');
  const checks=[installer.sha256+'.exe',current.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(x=>x.sha256+'.bin')];
  for(const name of checks){
    const response=await fetch(origin+'/packages/'+name,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!response.ok||sha(Buffer.from(await response.arrayBuffer()))!==name.slice(0,64))throw Error('Live asset missing/corrupt: '+name);
  }
  console.log('LIVE VERIFIED: Signed 2.2.2.2 TEST 1 visual safety guard + repaired 2.2.2.2 rollback + exact P15 archive');
}else throw Error('Expected validate-signatures, prepare or verify-live');
