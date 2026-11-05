import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

// Preserve the existing release trust key. The private key is NEVER used on CI.
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const json=path=>JSON.parse(readFileSync(path,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release223-hotfix1/stable.json');
const oldSigned=readFileSync('build-support/release223/stable.json');
const info=json('build-support/release223-hotfix1/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw) {
  const obj=JSON.parse(raw);
  const payload=Buffer.from(obj.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(obj.signature,'base64')))throw Error('Invalid publisher signature');
  return JSON.parse(payload);
}
const current=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));
if(current.version!=='2.2.3'||current.sequence!==2026110403)throw Error('Signed prior 2.2.3 baseline mismatch');
if(rollback.version!=='2.2.2.1-hotfix.2'||rollback.sequence!==2026103104)throw Error('Historical rollback manifest mismatch');
if(release.schema!==1||release.version!=='2.2.3-hotfix.1'||release.sequence!==2026110404||release.sequence<=current.sequence||release.installer.sha256===current.installer.sha256)throw Error('Invalid new signed 2.2.3 identity');
if(info.version!==release.version||info.sequence!==release.sequence||info.previous_version!==current.version||info.previous_sequence!==current.sequence)throw Error('Publication pin disagrees with signed identities');
if(!['pt','en','de'].every(k=>typeof release.notes?.[k]==='string'&&release.notes[k].includes('2.2.3-hotfix.1')))throw Error('Missing signed localized release notes');
const installer=release.installer;
if(installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024)throw Error('Signed updater address/hash/size invalid');
if(info.candidate_run_id!==41950261402||info.candidate_commit!=='fa703f432f7ff0663535f8101cd5ac890de7ab87'||!Number.isSafeInteger(info.candidate_artifact_id)||!/^[a-f0-9]{64}$/.test(info.candidate_artifact_sha256))throw Error('Unapproved Windows build provenance');

function collect(directory) {
  const report=json(directory+'/build-report.json'), binary=readFileSync(directory+'/Atualizar-Bridge.exe'), hash=sha(binary);
  if(hash!==report.updater_sha256)throw Error('Corrupt historical installer: '+directory);
  writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
  const parts=directory+'/coop-assets';
  if(existsSync(parts))for(const name of readdirSync(parts)){
    const data=readFileSync(parts+'/'+name);
    if(name!==sha(data)+'.bin'||data.length>20*1024*1024)throw Error('Corrupt historical Coop part: '+name);
    writeFileSync('distribution/assets/packages/'+name,data);
  }
}
async function loadLive(){
  const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw Error('Cannot fetch current signed live release');
  return Buffer.from(await response.arrayBuffer());
}
function inspectApprovedNew(){
  const report=json('approved-new/build-report.json'),spec=json('approved-new/coop-spec.json');
  const binary=readFileSync('approved-new/Atualizar-Bridge.exe');
  const installed=json('approved-new/one-click-install-test.json');
  const hostUI=json('approved-new/native-p16-controls.json');
  if(report.version!=='2.2.3-hotfix.1'||report.sequence!==2026110404||report.update_origin!==origin||sha(binary)!==installer.sha256||report.updater_sha256!==installer.sha256||binary.length!==installer.size||report.installer_size!==installer.size||spec.sha256!==info.coop_archive_sha256||report.coop_archive_sha256!==spec.sha256)throw Error('2.2.3 updater/Coop artifact identity mismatch');
  if(report.native_candidate_run!==41946684898||report.native_candidate_commit!=='7b34c59d77890804909ba7698d7e5ede1e50f345'||report.native_version!=='0.3.47-coop.16-2.2.3-hotfix.1'||report.desktop_launcher_run!==41946684898||report.desktop_launcher_commit!=='7b34c59d77890804909ba7698d7e5ede1e50f345')throw Error('2.2.3 native/lobby source identity mismatch');
  for(const key of ['actual_updater_first_attempt_windows_tested','setup_changelog_ui_tested','setup_live_update_ui_tested','p16_host_ui_tested','complete_hosts_unchanged','p16_hosts_unchanged','p14_server_unchanged','p15_server_unchanged','bridge_regressions_tested','per_release_changelog_including_hotfixes','wheelchair_passengers','accessibility_menu_alt','wheelchair_alt_request_persistent','wheelchair_occupied_bay_priority','setup_hotfix_cumulative_changelog_tested']){
    if(report[key]!==true)throw Error('Missing verified release gate: '+key);
  }
  if(report.wheelchair_nine_controller_cycles_passed!==9||report.simulation_tests_passed!==1013||report.network_tests_passed!==208)throw Error('Missing verified nine-vehicle and regression evidence');
  if(report.wheelchair_spawn_percent_default!==3||report.accessibility_ui_languages!==30||report.setup_changelog_languages.join(',')!=='pt,en,de'||report.windows_gameplay_tested!==false||report.paired_gameplay_tested!==false)throw Error('Wrong wheelchair defaults, translations or gameplay caveat');
  if(report.accessibility_vehicle_validation?.controller_cycle_passed!==9||report.accessibility_vehicle_validation?.access_blocked!==0)throw Error('Unexpected bus-compatibility report');
  if(installed.status!=='passed'||installed.invocations!==1||installed.setup_reopened!==true||installed.running_old_setup_replaced!==true||installed.unmanaged_preferences_preserved!==true||installed.windows_crlf_acceptance_tested!==true||installed.managed_files_verified!==Object.keys(spec.files).length||installed.setup_sha256!==report.components_sha256['Setup.exe'])throw Error('Actual first-attempt update verification is incomplete');
  if(hostUI.status!=='passed'||hostUI.save_reopen!==true||hostUI.large_inventory_tested!==true)throw Error('Real P16 host UI verification is incomplete');
  if(report.complete_hosts_rebuilt!==false||report.custom_sounds_unchanged!==true||report.winter_clothing_removed!==true||report.coop_protocol!==16||report.updater_architecture!=='amd64')throw Error('Historical host/audio/P16 preservation gate failed');
  if(!Array.isArray(spec.parts)||spec.parts.length===0||!spec.files||Object.keys(spec.files).length<200||Object.keys(spec.files).some(n=>n.includes('/WinterClothes/')))throw Error('Missing or altered cumulative Coop assets');
  for(const name of ['app/server-dinamico-p14/openomsi.exe','app/server-dinamico-p15/openomsi.exe','app/hosts-p16/HostAgent.exe','app/hosts-p16/CompanyHost.exe'])if(!spec.files[name])throw Error('Missing preserved server/host: '+name);
  const prefix='OpenOmsi Coop 0.3.41/';
  const game=prefix+'programa/openomsi.exe',lobby=prefix+'programa/openomsi-launcher-app.exe',server='app/server-dinamico-p16/openomsi.exe';
  if(spec.files[game]!==report.native_client_sha256['openomsi.exe']||spec.files[server]!==report.dynamic_p16_sha256||spec.files[game]!==spec.files[server]||spec.files[lobby]!==report.desktop_launcher_sha256||spec.files['app/hosts-p16/HostAgent.exe']!==report.p16_host_hashes['HostAgent.exe']||spec.files['app/hosts-p16/CompanyHost.exe']!==report.p16_host_hashes['CompanyHost.exe'])throw Error('Mismatched actual game/lobby/P16 hosts');
  if(!spec.files[prefix+'CHANGELOG-2.2.3-hotfix.1.md']||!spec.files['docs/TESTE-2.2.3-hotfix.1-pt-BR.md']||!spec.files['docs/accessibility-223-vehicle-validation.json'])throw Error('Missing 2.2.3 dedicated changelog or accessibility report');
  return {report,spec,binary};
}
if(process.argv[2]==='validate-signatures'){
  console.log('SIGNED PREFLIGHT VERIFIED: 2.2.3 Hotfix 1, 2.2.3 baseline and rollback; PT/EN/DE notes');
}else if(process.argv[2]==='prepare'){
  const live=await loadLive(),now=decode(live);
  if(!live.equals(oldSigned)||now.version!==current.version||now.sequence!==current.sequence)throw Error('Live feed is not EXACTLY signed 2.2.3; refusing replacement');
  const {report,spec,binary}=inspectApprovedNew();
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Missing immutable historical distribution baseline');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Historical immutable asset corrupt: '+name);
  }
  if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe')||!existsSync('distribution/assets/packages/'+rollback.installer.sha256+'.exe'))throw Error('Missing previous/rollback installer');
  const chunks=[];
  for(const part of spec.parts){
    const bytes=readFileSync('distribution/assets/packages/'+part.sha256+'.bin');
    if(sha(bytes)!==part.sha256||bytes.length!==part.size||!binary.includes(Buffer.from(part.sha256)))throw Error('Cumulative ZIP chunk missing or not referenced by updater');
    chunks.push(bytes);
  }
  if(sha(Buffer.concat(chunks))!==spec.sha256||!binary.includes(Buffer.from(spec.sha256)))throw Error('Full package ZIP not authenticated by updater');
  console.log('VERIFIED HOTFIX 1 READY:',Object.keys(spec.files).length,'managed files,',spec.parts.length,'authenticated chunks; nine bus cycles passed');
}else if(process.argv[2]==='verify-live'){
  const live=await loadLive();
  if(!live.equals(signed))throw Error('Signed live feed is not 2.2.3');
  const spec=json('approved-new/coop-spec.json');
  const checks=[installer.sha256+'.exe',current.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(x=>x.sha256+'.bin')];
  for(const name of checks){
    const response=await fetch(origin+'/packages/'+name,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!response.ok||sha(Buffer.from(await response.arrayBuffer()))!==name.slice(0,64))throw Error('Live asset unavailable or corrupt: '+name);
  }
  console.log('LIVE VERIFIED: Signed 2.2.3 Hotfix 1, first-attempt updater, 2.2.3 baseline, rollback and cumulative chunks');
}else throw Error('Expected validate-signatures, prepare or verify-live');
