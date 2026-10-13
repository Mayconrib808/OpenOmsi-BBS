import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const meta=JSON.parse(readFileSync('hotfix218/download.json','utf8'));
const response=await fetch(meta.url,{signal:AbortSignal.timeout(60000)});
if(!response.ok || !response.headers.get('cache-control')?.includes('immutable')) throw Error('Manual update unavailable');
const data=Buffer.from(await response.arrayBuffer());
if(data.length!==meta.size || createHash('sha256').update(data).digest('hex')!==meta.sha256) throw Error('Manual installer integrity failed');
console.log(`MANUAL DOWNLOAD VERIFIED: ${meta.version}, ${meta.sha256}, ${data.length} bytes. Stable automatic channel remains unchanged.`);
