#!/usr/bin/env node
// Offline release preparation. NEVER copy the Ed25519 publisher private key
// to GitHub, an artifact, a log or ChatGPT. The installer and host assets are
// verified before signing. The original key is required for existing clients.
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {createHash,createPrivateKey,createPublicKey,sign} from 'node:crypto';
import {resolve,join} from 'node:path';
const [candidateDir,privateKeyPath,run,commit]=process.argv.slice(2);
if(!candidateDir||!privateKeyPath||!/^[0-9]{8,16}$/.test(run||'')||!/^[a-f0-9]{40}$/.test(commit||'')){
 console.error('Usage: node scripts/sign_2216.mjs <approved-candidate-folder> <offline-publisher-private-key.pem> <approved-actions-run-id> <run-head-commit-sha>');
 process.exit(2);
}
const path=resolve(candidateDir), pkey=resolve(privateKeyPath), sha=b=>createHash('sha256').update(b).digest('hex');
const report=JSON.parse(readFileSync(join(path,'build-report.json'),'utf8'));
const installer=readFileSync(join(path,'Atualizar-Bridge.exe'));
if(report.version!=='2.2.1.6'||report.sequence!==2026102603||report.native_version!=='0.3.44-coop.15-2.2.1.6'||
!report.official_044_p15||!report.app_server_native_044||!report.official_launcher_044||
!report.p14_server_unchanged||report.complete_hosts_unchanged||!report.complete_hosts_rebuilt||!report.admin_time_opt_in||!report.delegated_passenger_role||
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
 schema:1,version:'2.2.1.6',sequence:2026102603,
 notes:{
 pt:'0.3.44 Coop15 e correções acumuladas de passageiros e troca de motorista. Mantém hosts P15 completos, servidor P14 e comandos de combustível BCS. O modo de horário livre do HostAgent não está incluído.',
 en:'Authentic 0.3.44 Coop15, retained passenger and driver-handover changes, P14/P15 hosts and BCS fuel commands. HostAgent administrator clock override is not included.',
 de:'Echtes 0.3.44 Coop15, Fahrgast- und Fahrerwechsel-Anpassungen, P14/P15-Hosts und BCS-Kraftstoff. Admin-Uhr-Freigabe im HostAgent ist noch nicht enthalten.'
 },
 installer:{url:origin+'/packages/'+report.updater_sha256+'.exe',sha256:report.updater_sha256,size:installer.length}
};
const payload=Buffer.from(JSON.stringify(release));
const envelope=JSON.stringify({payload:payload.toString('base64'),signature:sign(null,payload,key).toString('base64')})+'\n';
mkdirSync('build-support/bridge2216',{recursive:true});
writeFileSync('build-support/bridge2216/stable.json',envelope);
writeFileSync('build-support/bridge2216/publication.json',JSON.stringify({
 version:'2.2.1.6',sequence:2026102603,
 candidate_run_id:Number(run),candidate_commit:commit,
 installer_sha256:report.updater_sha256,installer_size:installer.length
},null,2)+'\n');
console.log('Created only signed public manifest and approval metadata; original private key was NOT copied.');
console.log('A signed manifest is NOT a deployment. Run the authorized publish-2216 workflow, then verify the production Worker.');
