import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const meta=JSON.parse(readFileSync('hotfix218/download.json','utf8'));
const response=await fetch(meta.url,{signal:AbortSignal.timeout(60000)});
if(!response.ok || !response.headers.get('cache-control')?.includes('immutable')) throw Error('Update installer unavailable');
const data=Buffer.from(await response.arrayBuffer());
if(data.length!==meta.size || createHash('sha256').update(data).digest('hex')!==meta.sha256) throw Error('Installer integrity failed');
console.log(`DOWNLOAD VERIFIED: ${meta.version}, ${meta.sha256}, ${data.length} bytes. Use publication.mjs verify-live to verify the signed automatic channel.`);
