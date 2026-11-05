import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=data=>createHash('sha256').update(data).digest('hex');
const json=path=>JSON.parse(readFileSync(path,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2231/stable.json');
const oldSigned=readFileSync('build-support/release223-hotfix2/stable.json');
const info=json('build-support/release2231/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});

function decode(raw){
  const env=JSON.parse(raw),payload=Buffer.from(env.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(env.signature,'base64')))throw Error('Invalid signed publisher envelope');
  return JSON.parse(payload);
}
const previous=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));
if(previous.version!=='2.2.3-hotfix.2'||previous.sequence!==2026110405)throw Error('Previous signed Hotfix 2 identity changed');
if(rollback.version!=='2.2.2.1-hotfix.2'||rollback.sequence!==2026103104)throw Error('Historical rollback signature changed');
if(release.schema!==1||release.version!=='2.2.3.1'||release.sequence!==2026110501||release.sequence<=previous.sequence||release.installer.sha256===previous.installer.sha256)throw Error('Invalid signed 2.2.3.1 identity');
if(info.version!==release.version||info.sequence!==release.sequence||info.previous_version!==previous.version||info.previous_sequence!==previous.sequence)throw Error('Signed version/sequence disagrees with publication');
if(!['pt','en','de'].every(k=>typeof release.notes?.[k]==='string'&&release.notes[k].includes('2.2.3.1')))throw Error('Localized signed changelog absent');
const installer=release.installer;
if(installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024||installer.size<1000000)throw Error('Signed updater identity invalid');
if(info.candidate_run_id!==42116052029||info.candidate_commit!=='4a9f9c5bde053fcce92e100e65c544fb4163c9e7'||info.candidate_artifact_id!==13457334107||info.candidate_artifact_sha256!=='cbd0f2221a780b09433c4621b8f8158d235ef8226f0493cfbe798775ecec51ba')throw Error('Unapproved artifact provenance');
if(info.coop_archive_sha256!=='34c80650a32b26348b9da3e64f127429b20d184fad673ed9e47d38545f4fccea')throw Error('Wrong cumulative archive pin');

function collect(dir){
  const report=json(dir+'/build-report.json');
  const binary=readFileSync(dir+'/Atualizar-Bridge.exe');
  const hash=sha(binary);
  if(hash!==report.updater_sha256)throw Error('Corrupt historical installer '+dir);
  writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
  const parts=dir+'/coop-assets';
  if(existsSync(parts))for(const name of readdirSync(parts)){
    const buf=readFileSync(parts+'/'+name);
    if(name!==sha(buf)+'.bin'||buf.length>20*1024*1024)throw Error('Corrupt historical Coop part '+name);
    writeFileSync('distribution/assets/packages/'+name,buf);
  }
}
async function loadLive(){
  const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw Error('Cannot retrieve existing live signed manifest: '+response.status);
  return Buffer.from(await response.arrayBuffer());
}
function inspectApproved(){
  const report=json('approved-new/build-report.json');
  const spec=json('approved-new/coop-spec.json');
  const install=json('approved-new/one-click-install-test.json');
  const host=json('approved-new/native-p17-controls.json');
  const binary=readFileSync('approved-new/Atualizar-Bridge.exe');
  if(report.version!=='2.2.3.1'||report.sequence!==2026110501||report.update_origin!==origin||report.updater_sha256!==installer.sha256||sha(binary)!==installer.sha256||binary.length!==installer.size||report.installer_size!==installer.size||spec.sha256!==info.coop_archive_sha256||report.coop_archive_sha256!==spec.sha256)throw Error('Windows updater/Coop identity mismatch');
  if(report.native_candidate_run!==42116052029||report.native_candidate_commit!=='4aa69dbc1e05e467cb519b19f4d656cec53dafc8'||report.native_version!=='0.3.49-coop.17-2.2.3.1'||report.coop_protocol!==17)throw Error('Wrong native P17 source');
  if(report.desktop_launcher_run!==42116052029||report.desktop_launcher_commit!=='4aa69dbc1e05e467cb519b19f4d656cec53dafc8'||report.desktop_launcher_unchanged!==false)throw Error('Wrong desktop Launcher build');
  for(const k of ['complete_hosts_unchanged','p14_server_unchanged','p15_server_unchanged','p16_hosts_unchanged','p16_server_unchanged','p17_host_ui_tested','new_p17_host_ui_tested','setup_live_update_ui_tested','actual_updater_first_attempt_windows_tested','setup_changelog_ui_tested','custom_sounds_unchanged','bridge_regressions_tested','wheelchair_alt_request_persistent','wheelchair_occupied_bay_priority']){
    if(report[k]!==true)throw Error('Failed P17/retention test '+k);
  }
  if(report.wheelchair_nine_controller_cycles_passed!==9||report.wheelchair_spawn_percent_default!==3||report.accessibility_ui_languages!==30||report.windows_gameplay_tested!==false||report.paired_gameplay_tested!==false)throw Error('Accessibility, default, or gameplay evidence mismatch');
  if(report.p17_host_architecture!=='amd64'||report.p17_hosts_rebuilt_from_source!==true||host.status!=='passed'||host.large_inventory_tested!==true)throw Error('Real P17 HostAgent validation missing');
  if(install.status!=='passed'||install.invocations!==1||install.setup_reopened!==true||install.p17_actual_updater_migration_tested!==true)throw Error('Windows first-click migration missing');
  const prefix='OpenOmsi Coop 0.3.41/';
  const game=prefix+'programa/openomsi.exe',p17='app/server-dinamico-p17/openomsi.exe',lobby=prefix+'programa/openomsi-launcher-app.exe';
  if(spec.files[game]!==report.native_client_sha256['openomsi.exe']||spec.files[p17]!==spec.files[game]||spec.files[lobby]!==report.desktop_launcher_sha256||!spec.files['app/hosts-p17/HostAgent.exe']||!spec.files['app/hosts-p17/CompanyHost.exe'])throw Error('Missing exact matching Windows game, launcher, server or P17 hosts');
  if(!Array.isArray(spec.parts)||spec.parts.length<1||Object.keys(spec.files).length<200)throw Error('Incomplete cumulative package');
  if(spec.files['app/hosts-p16/HostAgent.exe']!==report.retained_runtime_sha256['app/hosts-p16/HostAgent.exe'] && !spec.files['app/hosts-p16/HostAgent.exe'])throw Error('P16 retained host missing');
  if(!spec.files[prefix+'CHANGELOG-2.2.3.1.md']||!spec.files['docs/TESTE-2.2.3.1-pt-BR.md'])throw Error('Missing cumulative changelog or tester notes');
  for(const part of spec.parts){
    const buf=readFileSync('approved-new/coop-assets/'+part.sha256+'.bin');
    if(buf.length!==part.size||sha(buf)!==part.sha256||!binary.includes(Buffer.from(part.sha256)))throw Error('Malformed authenticated P17 Coop archive part');
  }
  const buffers=spec.parts.map(p=>readFileSync('approved-new/coop-assets/'+p.sha256+'.bin'));
  if(sha(Buffer.concat(buffers))!==spec.sha256||!binary.includes(Buffer.from(spec.sha256)))throw Error('Cumulative ZIP hash is not authenticated by updater');
  return {report,spec,binary};
}
if(process.argv[2]==='validate-signatures'){
  console.log('SIGNED PREFLIGHT VERIFIED: 2.2.3.1, Hotfix 2, old rollback and pt/en/de changelog');
}else if(process.argv[2]==='prepare'){
  const live=await loadLive();
  const now=decode(live);
  if(!live.equals(oldSigned)||now.version!==previous.version||now.sequence!==previous.sequence)throw Error('Live update feed is not exactly signed Hotfix 2; refusing overwrite');
  const {report,spec,binary}=inspectApproved();
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Missing historical immutable assets baseline');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
  for(const name of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Historical immutable file corrupt: '+name);
  }
  for(const hash of [installer.sha256,previous.installer.sha256,rollback.installer.sha256]){
    if(!existsSync('distribution/assets/packages/'+hash+'.exe'))throw Error('Missing new or rollback updater '+hash);
  }
  console.log('P17 CUMULATIVE PUBLISH READY:',report.version,spec.parts.length,'authenticated archive parts',Object.keys(spec.files).length,'managed runtime files');
}else if(process.argv[2]==='verify-live'){
  const live=await loadLive();
  if(!live.equals(signed))throw Error('Published signed feed is not 2.2.3.1');
  const spec=json('approved-new/coop-spec.json');
  for(const name of [installer.sha256+'.exe',previous.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(x=>x.sha256+'.bin')]){
    const r=await fetch(origin+'/packages/'+name,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!r.ok||sha(Buffer.from(await r.arrayBuffer()))!==name.slice(0,64))throw Error('Live update part missing or corrupt: '+name);
  }
  console.log('LIVE VERIFIED: signed Bridge 2.2.3.1 P17, old Hotfix 2, rollback and every cumulative archive part');
}else throw Error('Expected validate-signatures, prepare or verify-live');
