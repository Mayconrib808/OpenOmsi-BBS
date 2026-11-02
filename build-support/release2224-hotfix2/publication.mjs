import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const json=path=>JSON.parse(readFileSync(path,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2224-hotfix2/stable.json');
const oldSigned=readFileSync('build-support/release2224-hotfix1/stable.json');
const info=json('build-support/release2224-hotfix2/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw) {
  const obj=JSON.parse(raw);
  const payload=Buffer.from(obj.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(obj.signature,'base64')))throw Error('Invalid publisher signature');
  return JSON.parse(payload);
}
const current=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));

if(current.version!=='2.2.2.4-hotfix.1'||current.sequence!==2026110204)throw Error('Expected previously deployed signed 2.2.2.4-hotfix.1 baseline');
if(rollback.version!=='2.2.2.1-hotfix.2'||rollback.sequence!==2026103104)throw Error('Rollback Hotfix 2 manifest mismatch');
if(release.schema!==1||release.version!=='2.2.2.4-hotfix.2'||release.sequence!==2026110205||release.sequence<=current.sequence||release.installer.sha256===current.installer.sha256)throw Error('Expected signed HOTFIX 2.2.2.4-hotfix.2 with distinct updater and higher sequence');
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
  console.log('SIGNED PREFLIGHT VERIFIED: 2.2.2.4 baseline, 2.2.2.4-hotfix.1 and older rollback');
}else if(process.argv[2]==='prepare'){
  const live=await loadLive(),now=decode(live);
  if(!live.equals(oldSigned)||now.version!==current.version||now.sequence!==current.sequence)throw Error('Production is not exactly the signed 2.2.2.4-hotfix.1 baseline; refusing overwrite');
  const report=json('approved-new/build-report.json'),spec=json('approved-new/coop-spec.json');
  const binary=readFileSync('approved-new/Atualizar-Bridge.exe');
  const installed=json('approved-new/one-click-install-test.json');
  if(report.actual_updater_first_attempt_windows_tested!==true||installed.status!=='passed'||installed.invocations!==1||
      installed.windows_crlf_acceptance_tested!==true||installed.running_old_setup_replaced!==true||
      installed.setup_reopened!==true||installed.unmanaged_preferences_preserved!==true||
      installed.managed_files_verified!==Object.keys(spec.files).length||
      installed.setup_sha256!==report.components_sha256['Setup.exe'])
    throw Error('Actual first-attempt Windows installation did not pass');
  if(report.version!=='2.2.2.4-hotfix.2'||report.sequence!==2026110205||report.update_origin!==origin
      ||report.updater_sha256!==installer.sha256||sha(binary)!==installer.sha256||binary.length!==installer.size
      ||report.coop_archive_sha256!==spec.sha256||report.native_version!=='0.3.47-coop.16-2.2.2.4'
      ||report.native_candidate_run!==41528869591
      ||report.native_candidate_commit!=='b15541549872aaee2dcd2d1be645bec978324495'
      ||report.native_build_commit!=='b15541549872aaee2dcd2d1be645bec978324495'
      ||report.desktop_launcher_commit!=='b15541549872aaee2dcd2d1be645bec978324495'
      ||report.desktop_launcher_run!==41528869591
      ||report.complete_hosts_unchanged!==true||report.complete_hosts_rebuilt!==false
      ||report.p14_server_unchanged!==true||report.p15_server_unchanged!==true||report.new_p16_host_ui_tested!==true||report.custom_sounds_unchanged!==true
      ||report.bridge_regressions_tested!==true||report.setup_changelog_ui_tested!==true
      ||report.setup_live_update_ui_tested!==true||report.coop_protocol!==16
      ||report.windows_gameplay_tested!==false||report.paired_gameplay_tested!==false
      ||report.desktop_launcher_unchanged!==false||report.winter_clothing_removed!==true
      ||report.passenger_defaults_enabled!==true||report.passenger_coop_handback!==true
      ||report.passenger_upstream_tag!=='v0.3.47'||report.brazilian_passenger_recordings!==8||report.updater_architecture!=='amd64'
      ||report.rollback_storage!=='disk'||report.feature_event_audit_included!==true
      ||report.per_release_changelog_including_hotfixes!==true)
    throw Error('FINAL candidate incomplete, wrong version, or regression flags missing');
  if(!Array.isArray(spec.parts)||!spec.parts.length||!spec.files||Object.keys(spec.files).length<200
     ||Object.keys(spec.files).some(n=>n.includes('/WinterClothes/')))
    throw Error('Invalid complete runtime or obsolete winter clothing remains');

  if(report.native_candidate_run!==41528869591 || report.desktop_launcher_run!==41528869591
     ||report.updater_sha256!==info.installer_sha256 ||report.installer_size!==info.installer_size
     ||report.coop_archive_sha256!==info.coop_archive_sha256
     ||report.dynamic_p16_version!=='0.3.47-coop.16-2.2.2.4'
     ||report.upstream_commit!=='8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6')
    throw Error('P16 candidate provenance, source or signed archive mismatch');
  for(const name of ['app/server-dinamico-p14/openomsi.exe','app/server-dinamico-p15/openomsi.exe']) {
    if(!spec.files[name])throw Error('Prior protocol server removed: '+name);
  }
  if(report.setup_changelog_languages.join(',')!=='pt,en,de')throw Error('Cumulative changelog languages missing');
  if(!spec.files['OpenOmsi Coop 0.3.41/programa/openomsi.exe'])
    throw Error('New native executable missing from runtime');
  const names=['HostAgent.exe','CompanyHost.exe'];
  for(const name of names)if(report.retained_host_hashes[name]!==report.retained_runtime_sha256[name]||report.retained_runtime_sha256[name]!==report.components_sha256[name])throw Error('Retained host changed: '+name);
  const game='OpenOmsi Coop 0.3.41/programa/openomsi.exe',server='app/server-dinamico-p16/openomsi.exe';
  if(spec.files[game]!==report.native_client_sha256['openomsi.exe']||spec.files[server]!==report.dynamic_p16_sha256||spec.files[game]!==spec.files[server])throw Error('Mismatched game / P16 server');
  for(const item of ['Police','Fire']){
    const client='OpenOmsi Coop 0.3.41/programa/bcs-assets/Vehicles/OpenOMSI_BCS/'+item+'/model/'+item+'.obj';
    const serverAsset='app/server-dinamico-p16/bcs-assets/Vehicles/OpenOMSI_BCS/'+item+'/model/'+item+'.obj';
    if(!spec.files[client]||spec.files[client]!==spec.files[serverAsset])throw Error('3D '+item+' client/server asset absent or mismatched');
  }
  // The previous 2.2.2.1 accidentally distributed a legacy Tauri lobby.
  // Reject a new candidate unless the complete updated launcher is included.
  const lobby='OpenOmsi Coop 0.3.41/programa/openomsi-launcher-app.exe';
  if(!report.desktop_launcher_sha256||spec.files[lobby]!==report.desktop_launcher_sha256||spec.files['app/hosts-p16/HostAgent.exe']!==report.p16_host_hashes?.['HostAgent.exe']||spec.files['app/hosts-p16/CompanyHost.exe']!==report.p16_host_hashes?.['CompanyHost.exe'])
    throw Error('New P16 hosts or desktop launcher/optional settings missing from package');
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Historical assets absent');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Historical asset corrupt: '+name);
  }
  if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe'))throw Error('Previous 2.2.2.4-hotfix.1 installer missing');
  if(!existsSync('distribution/assets/packages/'+rollback.installer.sha256+'.exe'))throw Error('2.2.2.1 Hotfix 2 rollback installer missing');
  const chunks=[];
  for(const part of spec.parts){
    const bytes=readFileSync('distribution/assets/packages/'+part.sha256+'.bin');
    if(sha(bytes)!==part.sha256||bytes.length!==part.size||!binary.includes(Buffer.from(part.sha256)))throw Error('Cumulative ZIP chunk missing or changed');
    chunks.push(bytes);
  }
  if(sha(Buffer.concat(chunks))!==spec.sha256||!binary.includes(Buffer.from(spec.sha256)))throw Error('Cumulative ZIP was not authenticated by updater');
  console.log('VERIFIED HOTFIX 2.2.2.4-hotfix.2 staging: full 0.3.47/P16, 8 Brazilian recordings, no winter clothes, preserved P14/P15 and rollback');
}else if(process.argv[2]==='verify-live'){
  const live=await loadLive();
  if(!live.equals(signed))throw Error('Live update feed does not match signed 2.2.2.4-hotfix.2');
  const spec=json('approved-new/coop-spec.json');
  const checks=[installer.sha256+'.exe',current.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(x=>x.sha256+'.bin')];
  for(const name of checks){
    const response=await fetch(origin+'/packages/'+name,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!response.ok||sha(Buffer.from(await response.arrayBuffer()))!==name.slice(0,64))throw Error('Live asset missing/corrupt: '+name);
  }
  console.log('LIVE VERIFIED: Signed 2.2.2.4-hotfix.2 + 2.2.2.4-hotfix.1 baseline + rollback + exact P16 archive');
}else throw Error('Expected validate-signatures, prepare or verify-live');
