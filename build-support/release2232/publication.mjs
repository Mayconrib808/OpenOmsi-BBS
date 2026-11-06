import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const signed=readFileSync('build-support/release2232/stable.json');
const oldSigned=readFileSync('build-support/release2231-hotfix2/stable.json');
const info=json('build-support/release2232/publication.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(raw){const env=JSON.parse(raw),payload=Buffer.from(env.payload,'base64');if(!verify(null,payload,key,Buffer.from(env.signature,'base64')))throw Error('Invalid publisher signature');return JSON.parse(payload);}
const before=decode(oldSigned),release=decode(signed),rollback=decode(readFileSync('build-support/release2221-hotfix2/stable.json'));
if(before.schema!==1||before.version!=='2.2.3.1-hotfix.2'||before.sequence!==2026110503||before.installer.sha256!=='f3a545cc3a4ec65483043b208e380fdb41369ec60ab9771d6ef337c94ac26edb')throw Error('Signed predecessor changed');
if(release.schema!==1||release.version!=='2.2.3.2'||release.sequence!==2026110601||release.sequence<=before.sequence)throw Error('Invalid new release/sequence');
if(Object.keys(release.notes||{}).sort().join(',')!=='de,en,pt')throw Error('Legacy Setup rejects unsupported feed locales');
for(const lang of ['pt','en','de'])if(typeof release.notes[lang]!=='string'||release.notes[lang].length<70||!release.notes[lang].includes('0.3.50'))throw Error('Missing release notes '+lang);
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
 const dir='approved-2232',report=json(dir+'/build-report.json'),spec=json(dir+'/coop-spec.json'),install=json(dir+'/one-click-install-test.json'),host=json(dir+'/native-p17-controls.json'),bin=readFileSync(dir+'/Atualizar-Bridge.exe');
 if(report.version!==release.version||report.sequence!==release.sequence||report.updater_sha256!==installer.sha256||report.installer_size!==installer.size||sha(bin)!==installer.sha256||bin.length!==installer.size||bin.subarray(0,2).toString()!=='MZ')throw Error('New installer/report mismatch');
 if(report.native_candidate_run!==42202071912||report.native_candidate_commit!=='9b1d236c84b8af2f4c5a13d7c110b0db5e0c1c68'||report.native_version!=='0.3.50-coop.17-2.2.3.2-candidate'||report.coop_protocol!==17||report.passenger_upstream_tag!=='v0.3.50'||report.upstream_commit!=='11ff78bab1998963162be97c49e909a2996710e1')throw Error('Full official 0.3.50 provenance invalid');
 for(const field of ['complete_hosts_unchanged','p14_server_unchanged','p15_server_unchanged','p16_hosts_unchanged','p16_server_unchanged','p17_host_ui_tested','wheelchair_universal_alight_hud','wheelchair_signal_reset','windows_setup_activation_tested','stale_legacy_sha256_activation_tested','installer_first_click_tested','other_six_components_unchanged'])if(report[field]!==true)throw Error('Missing release regression '+field);
 if(install.status!=='passed'||install.invocations!==1||install.setup_reopened!==true||install.p17_actual_updater_migration_tested!==true||install.unmanaged_preferences_preserved!==true||host.status!=='passed'||host.large_inventory_tested!==true||host.save_reopen!==true||host.map_switch!==true)throw Error('Actual Windows update/host controls not verified');
 if(spec.sha256!==info.coop_archive_sha256||spec.sha256!==report.coop_archive_sha256||spec.prefix!=='Bridge-2.2.3.2/')throw Error('New Coop manifest mismatch');
 const files=spec.files,game='OpenOmsi Coop 0.3.41/programa/';
 if(files[game+'openomsi.exe']!=='0d73ff860bb473363f7c4feb83bd68426e7eaf1cedd97d7afbc50a1eaf3a607e'||files['app/server-dinamico-p17/openomsi.exe']!==files[game+'openomsi.exe']||files[game+'openomsi-launcher-app.exe']!=='e38f45f3fdfebea416960c92648e370162d5ac22734f05e6409b3c8d3feca0cc'||files[game+'openomsi-launcher.exe']!=='48a1cc57d32e13cc260c49fdca4669c9239dffc9dfbce65871fc9ecbd2d2359f')throw Error('Approved game/launcher bytes changed');
 const hash=createHash('sha256');let size=0;
 for(const part of spec.parts){const b=readFileSync(dir+'/coop-assets/'+part.sha256+'.bin');if(sha(b)!==part.sha256||b.length!==part.size||b.length>20*1024*1024)throw Error('New Coop part corrupt');hash.update(b);size+=b.length;}
 if(hash.digest('hex')!==spec.sha256||size!==spec.size)throw Error('New complete Coop archive corrupt');
 return {report,spec};
}
if(process.argv[2]==='validate-signatures'){
 console.log('SIGNED PREFLIGHT VERIFIED: 2.2.3.2, 0.3.50/P17, original trust key and compatible feed locales');
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
 const live=await getLive();if(!live.equals(signed))throw Error('Live signed 2.2.3.2 feed not published');
 const spec=json('approved-2232/coop-spec.json'),previous=json('approved-new/coop-spec.json');
 const names=[installer.sha256+'.exe',before.installer.sha256+'.exe',rollback.installer.sha256+'.exe',...spec.parts.map(p=>p.sha256+'.bin'),...previous.parts.map(p=>p.sha256+'.bin')];
 for(let i=0;i<names.length;i+=4)await Promise.all(names.slice(i,i+4).map(async n=>{const r=await fetch(origin+'/packages/'+n,{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)});if(!r.ok||!r.headers.get('cache-control')?.includes('immutable')||sha(Buffer.from(await r.arrayBuffer()))!==n.slice(0,64))throw Error('Missing/corrupt live signed asset '+n);}));
 console.log('LIVE VERIFIED: Bridge 2.2.3.2, official 0.3.50/P17 game and Launcher, complete new archive, previous P17 and rollback retained');
}else throw Error('Usage: validate-signatures | prepare | verify-live');
