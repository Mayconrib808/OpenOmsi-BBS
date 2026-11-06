import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';

// Only the signed manifest and Setup-containing Windows updater change.
// All P17 Coop archive parts, HostAgents, game binaries and historical releases
// are re-deployed from previously approved immutable artifacts.
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2231-hotfix1/stable.json');
const oldSigned=readFileSync('build-support/release2231/stable.json');
const info=json('build-support/release2231-hotfix1/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw){
  const env=JSON.parse(raw),payload=Buffer.from(env.payload,'base64');
  if(!verify(null,payload,key,Buffer.from(env.signature,'base64')))throw Error('Invalid publisher signature');
  return JSON.parse(payload);
}
const before=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));
if(before.schema!==1||before.version!=='2.2.3.1'||before.sequence!==2026110501||before.installer.sha256!=='c0cf59361d96dbfe97e414d134b339db62b429ea805cd34387505e53eb664275')throw Error('Current 2.2.3.1 baseline changed');
if(rollback.version!=='2.2.2.1-hotfix.2'||rollback.sequence!==2026103104)throw Error('Historical rollback signature changed');
if(release.schema!==1||release.version!=='2.2.3.1-hotfix.1'||release.sequence!==2026110502||release.sequence<=before.sequence||release.installer.sha256===before.installer.sha256)throw Error('Invalid new signed version/sequence');
// The ORIGINAL 2.2.3.1 Setup refuses any release-notes key other than pt/en/de.
// Six-language Setup UI remains bundled in the Hotfix installer, but the feed
// MUST remain backwards-compatible so installed clients can download it.
if(Object.keys(release.notes||{}).sort().join(',')!=='de,en,pt')throw Error('Legacy Setup will reject unsupported manifest notes languages');
for(const lang of ['pt','en','de']){
  if(typeof release.notes[lang]!=='string'||release.notes[lang].length<70||!release.notes[lang].includes('2.2.3.1'))throw Error('Missing compatible changelog '+lang);
}
if(info.version!==release.version||info.sequence!==release.sequence||info.previous_version!==before.version||info.previous_sequence!==before.sequence)throw Error('Publication metadata/manifest mismatch');
const installer=release.installer;
if(installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024||installer.size<1000000)throw Error('Signed installer URL/hash/size mismatch');
if(info.base_run_id!==42116052029||info.base_artifact_id!==13457334107||info.base_coop_sha256!=='34c80650a32b26348b9da3e64f127429b20d184fad673ed9e47d38545f4fccea')throw Error('Baseline P17 artifact pin invalid');
function collect(dir){
  const report=json(dir+'/build-report.json'),binary=readFileSync(dir+'/Atualizar-Bridge.exe');
  const hash=sha(binary);
  if(report.updater_sha256!==hash)throw Error('Corrupt immutable installer in '+dir);
  writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
  const parts=dir+'/coop-assets';
  if(existsSync(parts))for(const name of readdirSync(parts)){
    const buf=readFileSync(parts+'/'+name);
    if(name!==sha(buf)+'.bin'||buf.length>20*1024*1024)throw Error('Corrupt immutable Coop part '+name);
    writeFileSync('distribution/assets/packages/'+name,buf);
  }
}
async function getLive(){
  const r=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
  if(!r.ok)throw Error('Cannot retrieve current update feed '+r.status);
  return Buffer.from(await r.arrayBuffer());
}
function inspectCandidate(){
  const report=json('approved-2231hf1/build-report.json');
  const install=json('approved-2231hf1/one-click-install-test.json');
  const original=json('approved-new/build-report.json');
  const spec=json('approved-new/coop-spec.json');
  const bin=readFileSync('approved-2231hf1/Atualizar-Bridge.exe');
  if(report.version!==release.version||report.sequence!==release.sequence||report.updater_sha256!==installer.sha256||report.installer_size!==installer.size||sha(bin)!==installer.sha256||bin.length!==installer.size)throw Error('Approved Hotfix Windows updater/hash incorrect');
  if(report.candidate_base_run!==42116052029||report.candidate_base_artifact!==13457334107||report.candidate_base_updater_sha256!==before.installer.sha256||report.candidate_base_coop_archive_sha256!==spec.sha256)throw Error('Hotfix was not built over original approved P17');
  if(report.coop_protocol!==17||report.native_version!=='0.3.49-coop.17-2.2.3.1'||report.windows_ifoe_denied_registry_tested!==true||report.windows_bcs_native_fallback_tested!==true||report.windows_setup_activation_tested!==true||report.installer_first_click_tested!==true||report.other_six_components_unchanged!==true)throw Error('IFEO/fallback/Windows setup test missing');
  if(['pt','en','de','es','fr','ru'].some(x=>!report.languages.includes(x)))throw Error('Six Setup locales missing');
  if(install.status!=='passed'||install.invocations!==1||install.setup_reopened!==true||install.p17_actual_updater_migration_tested!==true||install.unmanaged_preferences_preserved!==true)throw Error('Actual Windows first-click/P17 migration test failed');
  if(original.version!=='2.2.3.1'||original.sequence!==2026110501||original.updater_sha256!==before.installer.sha256||original.coop_protocol!==17||original.native_version!=='0.3.49-coop.17-2.2.3.1')throw Error('Original P17 game changed');
  if(spec.sha256!=='34c80650a32b26348b9da3e64f127429b20d184fad673ed9e47d38545f4fccea'||spec.parts.length!==21)throw Error('P17 Coop archive changed');
  if(report.parts_unchanged!==spec.parts.length||report.managed_files_unchanged!==Object.keys(spec.files).length)throw Error('Hotfix native assets changed');
  if(info.candidate_run_id!==42136557845||info.candidate_commit!=='2f7cfa39ad8a80310abf7546daff17dd65b0e80c'||info.candidate_artifact_id!==13465592792||info.candidate_artifact_sha256!=='6af1fa504e00a4419c1fe1b1069bc97fc51c962dbdfd62e41b0d9dbb27276183'||info.setup_sha256!==report.setup_sha256)throw Error('Approved Hotfix source/artifact or Setup hash changed');
  for(const p of spec.parts){
    const buffer=readFileSync('approved-new/coop-assets/'+p.sha256+'.bin');
    if(buffer.length!==p.size||sha(buffer)!==p.sha256)throw Error('P17 historical part corrupted');
  }
  return {report,spec};
}
if(process.argv[2]==='validate-signatures'){
  console.log('SIGNED PREFLIGHT VERIFIED: 2.2.3.1 Hotfix 1, original P17, immutable rollback and all six languages');
}else if(process.argv[2]==='prepare'){
  const live=await getLive();
  const current=decode(live);
  const originalFeed=live.equals(oldSigned)&&current.sequence===before.sequence;
  const brokenSixLanguageFeed=current.version===release.version&&current.sequence===release.sequence&&current.installer.sha256===installer.sha256&&current.installer.size===installer.size&&Object.keys(current.notes||{}).sort().join(',')==='de,en,es,fr,pt,ru';
  if(!originalFeed&&!brokenSixLanguageFeed)throw Error('Live update feed changed unexpectedly; refusing overwrite');
  console.log('FEED REPAIR: replacing incompatible 6-language manifest with 3-language signed manifest, keeping the same approved Hotfix binary');
  const {report,spec}=inspectCandidate();
  mkdirSync('distribution/assets/packages',{recursive:true});
  if(!existsSync('approved-previous/assets'))throw Error('Missing historical immutable assets baseline');
  cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
  for(const dir of readdirSync('.').filter(x=>x.startsWith('approved-')&&existsSync(x+'/build-report.json')))collect(dir);
  writeFileSync('distribution/assets/stable.json',signed);
  for(const n of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+n,'distribution/'+n);
  for(const n of readdirSync('distribution/assets/packages')){
    if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(n)||sha(readFileSync('distribution/assets/packages/'+n))!==n.slice(0,64))throw Error('Deployed immutable file corrupt '+n);
  }
  for(const hash of [installer.sha256,before.installer.sha256,rollback.installer.sha256]){
    if(!existsSync('distribution/assets/packages/'+hash+'.exe'))throw Error('New installer or rollback asset missing '+hash);
  }
  for(const part of spec.parts){
    if(!existsSync('distribution/assets/packages/'+part.sha256+'.bin'))throw Error('Retained P17 part missing '+part.sha256);
  }
  console.log('HOTFIX PUBLISH READY',report.version,report.sequence,'P17 Coop parts kept',spec.parts.length);
}else if(process.argv[2]==='verify-live'){
  const live=await getLive();
  if(!live.equals(signed))throw Error('Live signed Hotfix 1 feed was not published');
  const spec=json('approved-new/coop-spec.json');
  for(const n of [installer.sha256+'.exe',before.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(p=>p.sha256+'.bin')]){
    const r=await fetch(origin+'/packages/'+n,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});
    if(!r.ok||sha(Buffer.from(await r.arrayBuffer()))!==n.slice(0,64))throw Error('Missing or corrupt live signed asset '+n);
  }
  console.log('LIVE VERIFIED: Bridge 2.2.3.1 Hotfix 1; unchanged P17, original 2.2.3.1 and rollback');
}else throw Error('Usage: validate-signatures | prepare | verify-live');
