import {createPublicKey,verify,createHash} from 'node:crypto';
const origin='https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev';
const headers={'User-Agent':'OpenOmsi-BBS-Setup/3.0.2-dev.11','Cache-Control':'no-cache'};
const hash=data=>createHash('sha256').update(data).digest('hex');

async function fetchBytes(url){
 const response=await fetch(url,{headers});
 if(!response.ok)throw Error('HTTP '+response.status+' for '+url);
 return {response,data:Buffer.from(await response.arrayBuffer())};
}

// Existing dev.8 installations must keep receiving the exact signed legacy feed.
const legacyResponse=await fetch(origin+'/preview.json?legacy=dev8',{headers});
if(!legacyResponse.ok)throw Error('Legacy manifest unavailable: '+legacyResponse.status);
const legacyEnvelope=await legacyResponse.json();
const legacyPayload=Buffer.from(legacyEnvelope.payload,'base64');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('GKwwvTrPr3W3ctc9dfPrGtsjztkET1XAVPnAIlbwKLU=','base64')]),format:'der',type:'spki'});
if(!verify(null,legacyPayload,key,Buffer.from(legacyEnvelope.signature,'base64')))throw Error('Legacy publisher signature mismatch');
const legacyRelease=JSON.parse(legacyPayload);
if(legacyRelease.product!=='bbs-openomsi-multiplayer'||legacyRelease.version!=='3.0.2-dev.8')throw Error('Legacy dev.8 feed changed');
const legacyBinary=await fetchBytes(legacyRelease.installer.url);
if(legacyBinary.data.length!==legacyRelease.installer.size||hash(legacyBinary.data)!==legacyRelease.installer.sha256)throw Error('Legacy updater checksum mismatch');

// dev.9+ metadata is anchored to an immutable file in the publisher repository.
let response,anchor,payload,release;
for(let n=0;n<12;n++){
 response=await fetch(origin+'/preview-v2.json?sequence=2026101211&attempt='+n,{headers});
 if(response.ok){
  anchor=await response.json();
  payload=Buffer.from(anchor.payload,'base64');
  release=JSON.parse(payload);
  if(release.version==='3.0.2-dev.11')break;
  console.log('Waiting for Cloudflare publication to propagate; current version:',release.version);
 }else console.log('Cloudflare v2 manifest status:',response.status);
 if(n<11)await new Promise(r=>setTimeout(r,5000));
}
if(!response?.ok||release?.version!=='3.0.2-dev.11')throw Error('New v2 manifest unavailable after propagation window');
if(!/^[a-f0-9]{40}$/.test(anchor.source_commit))throw Error('Invalid publisher commit');
const trusted=await fetchBytes('https://raw.githubusercontent.com/Mayconrib808/OpenOmsi-BBS/'+anchor.source_commit+'/cloudflare-updates-v3/release-v2.json');
if(!trusted.data.equals(payload))throw Error('Cloudflare metadata differs from immutable GitHub publisher metadata');
if(release.product!=='bbs-openomsi-multiplayer'||release.sequence!==2026101211)throw Error('Wrong dev.11 update channel');
const binary=await fetchBytes(release.installer.url);
if(binary.data.length!==release.installer.size||hash(binary.data)!==release.installer.sha256)throw Error('Dev.11 updater checksum mismatch');

const zip=await fetch(origin+'/download/full.zip',{headers});
console.log('Full ZIP',zip.status,Object.fromEntries(zip.headers));
if(!zip.ok)throw Error('Full package unavailable');
const fullData=Buffer.from(await zip.arrayBuffer());
if(fullData.length!==28401329||hash(fullData)!=='34c2ad58737e36a9b8b9fdcb17fc2cf0bf0bbae57cde1c303d4c8c6e1e6fdb3d')throw Error('Full ZIP checksum mismatch');
console.log('CLOUDFLARE_3X_VERIFIED: legacy dev.8 signature preserved; dev.11 metadata anchored to GitHub; updater and complete ZIP match.');
