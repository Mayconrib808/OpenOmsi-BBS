#!/usr/bin/env node
// Offline release preparation. NEVER copy the Ed25519 publisher private key
// to GitHub, an artifact or a log. The installer and host assets are
// verified before signing. The original key is required for existing clients.
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {createHash,createPrivateKey,createPublicKey,sign} from 'node:crypto';
import {resolve,join} from 'node:path';
const [candidateDir,privateKeyPath,run,commit]=process.argv.slice(2);
if(!candidateDir||!privateKeyPath||!/^[0-9]{8,16}$/.test(run||'')||!/^[a-f0-9]{40}$/.test(commit||'')){
 console.error('Usage: node scripts/sign_2218.mjs <approved-candidate-folder> <offline-publisher-private-key.pem> <approved-actions-run-id> <run-head-commit-sha>');
 process.exit(2);
}
const path=resolve(candidateDir), pkey=resolve(privateKeyPath), sha=b=>createHash('sha256').update(b).digest('hex');
const report=JSON.parse(readFileSync(join(path,'build-report.json'),'utf8'));
const installer=readFileSync(join(path,'Atualizar-Bridge.exe'));
if(report.version!=='2.2.1.8'||report.sequence!==2026102702||report.native_version!=='0.3.45-coop.15-2.2.1.8'||
!report.official_045_p15||!report.app_server_native_045||!report.official_launcher_045||
!report.p14_server_unchanged||report.complete_hosts_unchanged||!report.complete_hosts_rebuilt||!report.admin_time_opt_in||!report.admin_clock_opt_in_tested||!report.delegated_passenger_role||
!report.setup_backup_controls_ui_tested||!report.setup_backup_restore_tested||!report.connection_simulation_tested||report.default_passenger_transfer_percent!==20||
!report.connection_journey_tested||!report.same_stop_connection_tested||!report.conditional_connection_probability_tested||!report.wrist_motion_tested||!report.horn_mixer_tested||!report.desktop_launcher_unchanged||
!report.roof_cover_tested||!report.execution_ack_tested||!report.researched_hailing||
report.native_client_sha256?.['openomsi.exe']!==report.dynamic_p15_sha256||
sha(installer)!==report.updater_sha256||installer.length!==report.installer_size){
 throw Error('Approved version, candidate, game and retained P15 host evidence do not match');
}
const expected='5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=';
const key=createPrivateKey(readFileSync(pkey));
const pub=createPublicKey(key).export({format:'der',type:'spki'});
if(pub.subarray(-32).toString('base64')!==expected)throw Error('The key is NOT the original publisher key trusted by installed Setup');
const origin='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev';
const release={
 schema:1,version:'2.2.1.8',sequence:2026102702,
 notes:{
 "pt": "OpenOMSI 0.3.45 Coop15: corrige conexões em pontos com outras linhas e o segundo sorteio que descartava passageiros; adiciona movimento do pulso ao pedir parada e preserva duas buzinadas da saudação da IA. Mantém backup/restauração, HostAgent completo e as correções anteriores.",
 "en": "OpenOMSI 0.3.45 Coop15: fixes connections at shared stops and the second destination draw that discarded passengers; adds wrist movement while hailing and preserves two distinct AI greeting horn presses. Retains backup/restore, complete HostAgent and previous fixes.",
 "de": "OpenOMSI 0.3.45 Coop15: Umstiege an gemeinsam genutzten Haltestellen ohne zweite Verwerfungswahrscheinlichkeit, Handgelenkbewegung beim Haltewunsch und zwei getrennte KI-Grußhupen. Sicherung/Wiederherstellung, vollständiger HostAgent und bisherige Korrekturen bleiben erhalten."
},
 installer:{url:origin+'/packages/'+report.updater_sha256+'.exe',sha256:report.updater_sha256,size:installer.length}
};
const payload=Buffer.from(JSON.stringify(release));
const envelope=JSON.stringify({payload:payload.toString('base64'),signature:sign(null,payload,key).toString('base64')})+'\n';
mkdirSync('build-support/bridge2218',{recursive:true});
writeFileSync('build-support/bridge2218/stable.json',envelope);
writeFileSync('build-support/bridge2218/publication.json',JSON.stringify({
 version:'2.2.1.8',sequence:2026102702,
 candidate_run_id:Number(run),candidate_commit:commit,
 installer_sha256:report.updater_sha256,installer_size:installer.length
},null,2)+'\n');
console.log('Created only signed public manifest and approval metadata; original private key was NOT copied.');
console.log('A signed manifest is NOT a deployment. Run the authorized publish-2218 workflow, then verify the production Worker.');
