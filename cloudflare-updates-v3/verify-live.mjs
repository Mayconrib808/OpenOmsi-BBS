import {createPublicKey,verify,createHash} from 'node:crypto';
const origin='https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev';
const headers={'User-Agent':'OpenOmsi-BBS-Setup/3.0.2-dev.7','Cache-Control':'no-cache'};
let response;
for(let n=0;n<6;n++){
  response=await fetch(origin+'/preview.json',{headers});
  if(response.ok)break;
  console.log('Cloudflare status',response.status,'headers',Object.fromEntries(response.headers),'body',(await response.text()).slice(0,900));
  if(n<5)await new Promise(r=>setTimeout(r,10000));
}
if(!response.ok)throw Error('Manifest unavailable');
const envelope=await response.json();
const bytes=Buffer.from(envelope.payload,'base64');
const key=createPublicKey({key:Buffer.concat([Buffer.from('302a300506032b6570032100','hex'),Buffer.from('GKwwvTrPr3W3ctc9dfPrGtsjztkET1XAVPnAIlbwKLU=','base64')]),format:'der',type:'spki'});
if(!verify(null,bytes,key,Buffer.from(envelope.signature,'base64')))throw Error('Signature mismatch');
const release=JSON.parse(bytes);
if(release.product!=='bbs-openomsi-multiplayer'||release.version!=='3.0.2-dev.7')throw Error('Wrong update channel');
const binary=await fetch(release.installer.url,{headers});
if(!binary.ok)throw Error('Updater download failed: '+binary.status);
const data=Buffer.from(await binary.arrayBuffer());
if(data.length!==release.installer.size||createHash('sha256').update(data).digest('hex')!==release.installer.sha256)throw Error('Updater checksum mismatch');
const zip=await fetch(origin+'/download/full.zip',{headers});
console.log('Full ZIP',zip.status,Object.fromEntries(zip.headers));
if(!zip.ok||Number(zip.headers.get('content-length'))<20000000)throw Error('Full package unavailable');
const fullData=Buffer.from(await zip.arrayBuffer());
if(fullData.length!==Number(zip.headers.get('content-length'))||createHash('sha256').update(fullData).digest('hex')!==zip.headers.get('x-content-sha256'))throw Error('Full ZIP checksum mismatch');
console.log('CLOUDFLARE_3X_VERIFIED: signed manifest and updater match; complete ZIP accessible.');
