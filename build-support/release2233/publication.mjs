import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2233/stable.json');
const oldSigned=readFileSync('build-support/release2232-hotfix1/stable.json');
const info=json('build-support/release2233/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw){const env=JSON.parse(raw),payload=Buffer.from(env.payload,'base64');if(!verify(null,payload,key,Buffer.from(env.signature,'base64')))throw Error('Invalid publisher signature');return JSON.parse(payload);}
const before=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));
if(before.schema!==1||before.version!=='2.2.3.2-hotfix.1'||before.sequence!==2026110602||before.installer.sha256!=='bd6b6b3ce5dd14a0e13f41031b02feb66ac7d763fbf87492e92e1fdfb2f249bf')throw Error('Signed predecessor changed');
if(release.schema!==1||release.version!=='2.2.3.3'||release.sequence!==2026110701||release.sequence<=before.sequence)throw Error('Invalid new release/sequence');
if(Object.keys(release.notes||{}).sort().join(',')!=='de,en,pt')throw Error('Legacy Setup rejects unsupported feed locales');
for(const lang of ['pt','en','de'])if(typeof release.notes[lang]!=='string'||release.notes[lang].length<70||!release.notes[lang].includes('0.3.52'))throw Error('Missing release notes '+lang);
const installer=release.installer;
if(info.version!==release.version||info.sequence!==release.sequence||info.previous_version!==before.version||info.previous_sequence!==before.sequence||installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.sha256!==info.installer_sha256||installer.size!==info.installer_size||installer.size>25*1024*1024||installer.size<1000000)throw Error('Publication/signature/installer mismatch');
function collect(dir){
 const report=json(dir+'/build-report.json'),binary=readFileSync(dir+'/Atualizar-Bridge.exe');const hash=sha(binary);
 if(report.updater_sha256!==hash)throw Error('Corrupt immutable installer in '+dir);
 writeFileSync('distribution/assets/packages/'+hash+'.exe',binary);
 const parts=dir+'/coop-assets';
 if(existsSync(parts))for(const name of readdirSync(parts)){const buf=readFileSync(parts+'/'+name);if(name!==sha(buf)+'.bin'||buf.length>20*1024*1024)throw Error('Corrupt immutable Coop part '+name);writeFileSync('distribution/assets/packages/'+name,buf);}
}
async function getLive(){const r=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});if(!r.ok)throw Error('Cannot retrieve live feed '+r.status);return Buffer.from(await r.arrayBuffer());}
function inspectCandidate(){
 const dir='approved-2233',report=json(dir+'/build-report.json'),spec=json(dir+'/coop-spec.json'),install=json(dir+'/one-click-install-test.json'),host=json(dir+'/native-p17-controls.json'),bin=readFileSync(dir+'/Atualizar-Bridge.exe');
 if(report.version!==release.version||report.sequence!==release.sequence||report.updater_sha256!==installer.sha256||report.installer_size!==installer.size||sha(bin)!==installer.sha256||bin.length!==installer.size||bin.subarray(0,2).toString()!=='MZ')throw Error('New installer/report mismatch');
 if(report.native_candidate_run!==info.native_run_id||report.native_candidate_commit!==info.native_source_commit||report.native_build_commit!==info.native_source_commit||report.desktop_launcher_commit!==info.native_source_commit||report.native_version!=='0.3.52-coop.17-candidate'||report.coop_protocol!==17||report.passenger_upstream_tag!=='v0.3.52'||report.upstream_commit!=='41d69ab19043656b49d6cad0c34f85f3178503b8')throw Error('Full official 0.3.52 provenance invalid');
 for(const field of ['complete_hosts_unchanged','p14_server_unchanged','p15_server_unchanged','p16_hosts_unchanged','p16_server_unchanged','p17_host_ui_tested','windows_setup_activation_tested','stale_legacy_sha256_activation_tested','actual_updater_first_attempt_windows_tested','other_six_components_unchanged','p17_hosts_rebuilt_from_source'])if(report[field]!==true)throw Error('Missing release regression '+field);
 if(install.status!=='passed'||install.invocations!==1||install.setup_reopened!==true||install.p17_actual_updater_migration_tested!==true||install.unmanaged_preferences_preserved!==true||host.status!=='passed'||host.large_inventory_tested!==true||host.save_reopen!==true||host.map_switch!==true)throw Error('Actual Windows update/host controls not verified');
 if(spec.sha256!==info.coop_archive_sha256||spec.sha256!==report.coop_archive_sha256||spec.prefix!=='Bridge-2.2.3.3/')throw Error('New Coop manifest mismatch');
 const files=spec.files,game='OpenOmsi Coop 0.3.41/programa/';
 if(files[game+'openomsi.exe']!==info.native_game_sha256||files['app/server-dinamico-p17/openomsi.exe']!==files[game+'openomsi.exe']||files[game+'openomsi-launcher-app.exe']!==info.desktop_launcher_sha256||files[game+'openomsi-launcher.exe']!==info.desktop_launcher_cli_sha256)throw Error('Approved game/launcher bytes changed');
 for(const name of ['HostAgent.exe','CompanyHost.exe'])if(files['app/hosts-p17/'+name]!==report.p17_host_hashes[name])throw Error('Tested P17 host bytes changed '+name);
 const previous=json('approved-2232hotfix1/coop-spec.json').files;
 const replaced=new Set([
  game+'openomsi.exe',game+'openomsi-launcher-app.exe',game+'openomsi-launcher.exe',
  'app/server-dinamico-p17/openomsi.exe','app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json',
  'app/hosts-p17/HostAgent.exe','app/hosts-p17/CompanyHost.exe','docs/HOST-PESSOAL-2.2.0.3.json',
  'OpenOmsi Coop 0.3.41/SHA256.json','OpenOmsi Coop 0.3.41/source/BUILD-INFO.json',
 ]);
 for(const [name,hash] of Object.entries(previous))if(!replaced.has(name)&&files[name]!==hash)throw Error('Previous runtime asset changed '+name);
 const hash=createHash('sha256');let size=0;
 for(const part of spec.parts){const b=readFileSync(dir+'/coop-assets/'+part.sha256+'.bin');if(sha(b)!==part.sha256||b.length!==part.size||b.length>20*1024*1024)throw Error('New Coop part corrupt');hash.update(b);size+=b.length;}
 if(hash.digest('hex')!==spec.sha256||size!==spec.size)throw Error('New complete Coop archive corrupt');
 return {report,spec};
}
if(process.argv[2]==='validate-signatures'){
 console.log('SIGNED PREFLIGHT VERIFIED: 2.2.3.3, 0.3.52/P17, original trust key and compatible feed locales');
}else if(process.argv[2]==='inspect-candidate'){
 inspectCandidate();
 console.log('WINDOWS PACKAGE VERIFIED');
}else if(process.argv[2]==='prepare'){
 const live=await getLive();if(!live.equals(oldSigned))throw Error('Live feed changed from approved predecessor; refusing overwrite');
 const {report,spec}=inspectCandidate();
 mkdirSync('distribution/assets/packages',{recursive:true});
 if(!existsSync('approved-previous/assets'))throw Error('Historical immutable baseline missing');
 cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
 for(const dir of readdirSync('.').filter(x=>x.startsWith('approved-')&&existsSync(x+'/build-report.json')))collect(dir);
 writeFileSync('distribution/assets/stable.json',signed);
 for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
 for(const name of readdirSync('distribution/assets/packages'))if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(name)||sha(readFileSync('distribution/assets/packages/'+name))!==name.slice(0,64))throw Error('Deployed immutable asset corrupt '+name);
 for(const h of [installer.sha256,before.installer.sha256,rollback.installer.sha256])if(!existsSync('distribution/assets/packages/'+h+'.exe'))throw Error('New or rollback installer missing');
 console.log('PUBLICATION READY',report.version,report.sequence,spec.parts.length,'new parts; all historical immutable assets retained');
}else if(process.argv[2]==='verify-live'){
 const live=await getLive();if(!live.equals(signed))throw Error('Live signed 2.2.3.3 feed not published');
 const spec=json('approved-2233/coop-spec.json'),previous=json('approved-2232hotfix1/coop-spec.json');
 const names=[installer.sha256+'.exe',before.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(p=>p.sha256+'.bin'),...previous.parts.map(p=>p.sha256+'.bin')];
 for(let i=0;i<names.length;i+=4)await Promise.all(names.slice(i,i+4).map(async n=>{const r=await fetch(origin+'/packages/'+n,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});if(!r.ok||!r.headers.get('cache-control')?.includes('immutable')||sha(Buffer.from(await r.arrayBuffer()))!==n.slice(0,64))throw Error('Missing/corrupt live signed asset '+n);}));
 console.log('LIVE VERIFIED: Bridge 2.2.3.3, official 0.3.52/P17 game and Launcher, complete new archive, previous P17 and rollback retained');
}else throw Error('Usage: validate-signatures | prepare | verify-live');
