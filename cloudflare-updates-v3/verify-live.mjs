import {createPublicKey,verify,createHash} from 'node:crypto';
const origin='https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev';
const headers={'User-Agent':'OpenOmsi-BBS-Setup/3.0.2-dev.8','Cache-Control':'no-cache'};
let response,envelope,bytes,release;
for(let n=0;n<12;n++){
 response=await fetch(origin+'/preview.json?sequence=2026101208&attempt='+n,{headers});
 if(response.ok){
  envelope=await response.json();
  bytes=Buffer.from(envelope.payload,'base64');
  release=JSON.parse(bytes);
  if(release.version==='3.0.2-dev.8')break;
  console.log('Waiting for Cloudflare publication to propagate; current version:',release.version);
 }else{console.log('Cloudflare manifest status:',response.status)}
 if(n<11)await new Promise(r=>setTimeout(r,5000));
}
if(!response.ok||release?.version!=='3.0.2-dev.8')throw Error('New manifest unavailable after propagation window');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('GKwwvTrPr3W3ctc9dfPrGtsjztkET1XAVPnAIlbwKLU=','base64')]),format:'der',type:'spki'});
if(!verify(null,bytes,key,Buffer.from(envelope.signature,'base64')))throw Error('Signature mismatch');

if(release.product!=='bbs-openomsi-multiplayer'||release.version!=='3.0.2-dev.8')throw Error('Wrong update channel');
const binary=await fetch(release.installer.url,{headers});
if(!binary.ok)throw Error('Updater download failed: '+binary.status);
const data=Buffer.from(await binary.arrayBuffer());
if(data.length!==release.installer.size||createHash('sha256').update(data).digest('hex')!==release.installer.sha256)throw Error('Updater checksum mismatch');
const zip=await fetch(origin+'/download/full.zip',{headers});
console.log('Full ZIP',zip.status,Object.fromEntries(zip.headers));
if(!zip.ok)throw Error('Full package unavailable');
const fullData=Buffer.from(await zip.arrayBuffer());
if(fullData.length!==28381188||createHash('sha256').update(fullData).digest('hex')!=="87932dd3b7e8dcbdb00c8738ccb0f84c8e3bf5b31160a70dffcf6374d74fd3c3")throw Error('Full ZIP checksum mismatch');
console.log('CLOUDFLARE_3X_VERIFIED: signed manifest and updater match; complete ZIP accessible.');
