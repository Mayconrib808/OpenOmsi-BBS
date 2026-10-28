import {readFileSync,writeFileSync,mkdirSync,readdirSync,copyFileSync,cpSync,existsSync} from 'node:fs';
import {createHash,createPublicKey,verify} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const repo=process.env.GITHUB_REPOSITORY;
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const sha=b=>createHash('sha256').update(b).digest('hex');
const json=p=>JSON.parse(readFileSync(p,'utf8').replace(/^\uFEFF/,''));
const pub=json('build-support/bridge2219-hotfix2/publication.json');
const signed=readFileSync('build-support/bridge2219-hotfix2/stable.json');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=','base64')]),format:'der',type:'spki'});
function decode(bytes){const e=JSON.parse(bytes),raw=Buffer.from(e.payload,'base64');if(!verify(null,raw,key,Buffer.from(e.signature,'base64')))throw Error('Wrong publisher signature');return JSON.parse(raw)}
const manifest=decode(signed),installer=manifest.installer;
if(manifest.schema!==1||manifest.version!=='2.2.1.9-hotfix.2'||manifest.sequence!==2026102801||pub.version!==manifest.version||pub.sequence!==manifest.sequence||installer.sha256!==pub.installer_sha256||installer.size!==pub.installer_size||installer.url!==origin+'/packages/'+installer.sha256+'.exe'||installer.size>25*1024*1024)throw Error('Release signature/identity invalid');
function run(id,commit,path){const r=JSON.parse(execFileSync('gh',['api','repos/'+repo+'/actions/runs/'+id],{encoding:'utf8'}));if(r.status!=='completed'||r.conclusion!=='success'||r.head_sha!==commit||r.path!==path)throw Error('Wrong approved build '+id);return r}
function collect(dir){
 const report=json(dir+'/build-report.json'),exe=readFileSync(dir+'/Atualizar-Bridge.exe');
 if(report.updater_sha256!==sha(exe))throw Error('Historical installer hash differs: '+dir);
 writeFileSync('distribution/assets/packages/'+sha(exe)+'.exe',exe);
 if(existsSync(dir+'/coop-assets'))for(const name of readdirSync(dir+'/coop-assets')){
  const bytes=readFileSync(dir+'/coop-assets/'+name);
  if(name!==sha(bytes)+'.bin'||bytes.length>20*1024*1024)throw Error('Historical chunk hash differs: '+dir+'/'+name);
  writeFileSync('distribution/assets/packages/'+name,bytes)
 }
}
if(process.argv[2]==='prepare'){
 const response=await fetch(origin+'/stable.json',{headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(30000)});
 if(!response.ok)throw Error('Cannot establish current production version');
 const live=Buffer.from(await response.arrayBuffer()),current=decode(live);
 if(current.sequence>manifest.sequence||(current.sequence===manifest.sequence&&!live.equals(signed)))throw Error('Production advanced: rebase before publishing');
 if(current.sequence<manifest.sequence&&current.installer.sha256!=='46407da31d7cbd586e14dfb50a86ef5b6912bb1465182836f318beffdcccf21c')throw Error('Production has a different base: rebase before publishing');
 const candidate=run(pub.candidate_run_id,pub.candidate_commit,'.github/workflows/release-2219-coop-hotfix.yml');
 if(candidate.head_branch!=='release/2.2.1.9-hotfix.2')throw Error('Wrong candidate branch');
 run(40577793377,'d7ad462b2e334d48356f4bf29e05c7f2b5a8cda4','.github/workflows/native-2218-return-hotfix.yml');
 run(40587532083,'7b0c2713849ea335e3c2598bd42cfe954106423a','.github/workflows/live-camera-panel.yml');
 run(40590327114,'f16ca511984084b13e97fbd911b22e3608e8daaa','.github/workflows/release-2219-camera.yml');
 run(40602798735,'942e03a65254580064898b472e0e11619261fb84','.github/workflows/umbrella-water-hotfix.yml');
 run(40602950335,'9d5e204f69da673e3f1dc855634a863f1ba1c642','.github/workflows/release-2219-water-hotfix.yml');
 run(40764717872,'55c11a9b4a3fdbf41de762824960431ae1862a2c','.github/workflows/bcs-incident-beta.yml');
 const report=json('approved-new/build-report.json'),exe=readFileSync('approved-new/Atualizar-Bridge.exe'),spec=json('approved-new/coop-spec.json');
 const prior=json('approved-hotfix2219/build-report.json'),priorSpec=json('approved-hotfix2219/coop-spec.json');
 if(prior.updater_sha256!=='46407da31d7cbd586e14dfb50a86ef5b6912bb1465182836f318beffdcccf21c'||priorSpec.sha256!=='58cc975604d084f40c29b0a04864058cbfd9d3b66a937a965ad50b04d766bc4a')throw Error('Wrong cumulative hotfix baseline');
 if(report.version!=='2.2.1.9-hotfix.2'||report.sequence!==2026102801||report.coop_protocol!==15||report.updater_sha256!==sha(exe)||sha(exe)!==installer.sha256||exe.length!==installer.size||report.coop_archive_sha256!==spec.sha256||spec.size>256*1024*1024||spec.parts.length>16)throw Error('Candidate binary differs from signed release');
 for(const flag of ['umbrella_water_tested','camera_panel_tested','camera_live_adjustments','cross_player_connection_repatriation','complete_hosts_unchanged','complete_p15_ui_tested','admin_clock_opt_in_tested','admin_time_opt_in','p14_server_unchanged','desktop_launcher_unchanged','custom_closing_signal','setup_backup_restore_tested','setup_backup_controls_ui_tested','connection_simulation_tested','fuel_cli_tested','official_045_p15','app_server_native_045'])if(!report[flag])throw Error('Missing candidate evidence: '+flag);
 if(report.umbrella_water_test_run!==40602798735||report.previous_camera_run!==40590327114)throw Error('Missing umbrella folding regression evidence');
 if(report.complete_hosts_rebuilt||report.camera_test_run!==40587532083||report.native_candidate_run!==40764717872||report.native_candidate_commit!=='55c11a9b4a3fdbf41de762824960431ae1862a2c'||report.native_version!=='0.3.45-coop.15-2.2.1.9-coop-fix.1'||report.previous_installer_sha256!==prior.updater_sha256||report.previous_coop_sha256!==priorSpec.sha256)throw Error('Wrong native/cumulative source identity');
 if(report.dynamic_p15_sha256!==report.native_client_sha256['openomsi.exe']||spec.files['app/server-dinamico-p15/openomsi.exe']!==report.dynamic_p15_sha256||spec.files['OpenOmsi Coop 0.3.41/programa/openomsi.exe']!==report.dynamic_p15_sha256)throw Error('Game and P15 server are not the matched native build');
 for(const [name,hash] of Object.entries(prior.components_sha256))if(!['Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'].includes(name)&&report.components_sha256[name]!==hash)throw Error('Retained runtime differs: '+name);
 for(const name of ['HostAgent.exe','CompanyHost.exe'])if(report.retained_host_hashes[name]!==prior.components_sha256[name])throw Error('Complete host bytes changed: '+name);
 for(const flag of ['coop_cash_collection_tested','coop_physical_cash_mirror','coop_current_position_return','coop_logical_passenger_cabin','coop_hud_owner_count','coop_network_backpressure','previous_hotfix_preserved'])if(!report[flag])throw Error('Missing cooperative evidence: '+flag);
 if(report.coop_focused_tests_passed!==34||report.previous_hotfix_run!==40602950335)throw Error('Missing cooperative regression/baseline identity');
 const permitted=new Set(['OpenOmsi Coop 0.3.41/programa/openomsi.exe','OpenOmsi Coop 0.3.41/source/BUILD-INFO.json','OpenOmsi Coop 0.3.41/SHA256.json','OpenOmsi Coop 0.3.41/validation/passengers-2218-version.txt','app/server-dinamico-p15/openomsi.exe','app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json','docs/HOST-PESSOAL-2.2.0.3.json']);
 for(const [name,hash] of Object.entries(priorSpec.files))if(!permitted.has(name)&&spec.files[name]!==hash)throw Error('Unrelated cumulative file differs: '+name);
 if(report.closing_sound_sha256!==prior.closing_sound_sha256)throw Error('Custom closing signal differs');
 const bytes=Buffer.concat(spec.parts.map(p=>{const b=readFileSync('approved-new/coop-assets/'+p.sha256+'.bin');if(sha(b)!==p.sha256||b.length!==p.size||!exe.includes(Buffer.from(p.sha256)))throw Error('Candidate chunk not authenticated');return b}));
 if(sha(bytes)!==spec.sha256||bytes.length!==spec.size||!exe.includes(Buffer.from(spec.sha256)))throw Error('Candidate archive differs');
 for(const h of Object.values(spec.files))if(!exe.includes(Buffer.from(h)))throw Error('Updater does not authenticate a bundled file');
 mkdirSync('distribution/assets/packages',{recursive:true});
 cpSync('approved-previous/assets','distribution/assets',{recursive:true,force:true});
 for(const dir of readdirSync('.').filter(n=>n.startsWith('approved-')&&existsSync(n+'/build-report.json')))collect(dir);
 writeFileSync('distribution/assets/stable.json',signed);
 for(const name of ['worker-assets.mjs','wrangler.jsonc'])copyFileSync('update-service/'+name,'distribution/'+name);
 for(const n of readdirSync('distribution/assets/packages'))if(!/^([a-f0-9]{64})\.(exe|bin)$/.test(n)||sha(readFileSync('distribution/assets/packages/'+n))!==n.slice(0,64))throw Error('Historical immutable asset mismatch: '+n);
 // Preserve every package still referenced by the live updater while deploying the next one.
 if(!existsSync('distribution/assets/packages/'+current.installer.sha256+'.exe'))throw Error('Current live updater missing from new deployment');
 console.log('STAGED verified cumulative 2.2.1.9-hotfix.2 cashier/handover correction with retained umbrella, camera, hosts and historical packages');
}else if(process.argv[2]==='verify-live'){
 const opts={headers:{'Cache-Control':'no-cache'},signal:AbortSignal.timeout(120000)};
 const response=await fetch(origin+'/stable.json',opts);
 if(!response.ok||!Buffer.from(await response.arrayBuffer()).equals(signed))throw Error('Live signed manifest mismatch');
 for(const file of [installer.sha256+'.exe',...json('approved-new/coop-spec.json').parts.map(p=>p.sha256+'.bin')]){
  const result=await fetch(origin+'/packages/'+file,opts);
  if(!result.ok||sha(Buffer.from(await result.arrayBuffer()))!==file.slice(0,64))throw Error('Live package/chunk mismatch: '+file);
 }
 console.log('LIVE VERIFIED: 2.2.1.9-hotfix.2 cashier, passengers, handover and retained cumulative fixes; Buscar atualizacoes points to the authenticated new installer');
}else throw Error('Expected prepare or verify-live');
