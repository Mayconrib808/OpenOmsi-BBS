import assert from 'node:assert/strict';
import {setTimeout} from 'node:timers/promises';
const base='https://openomsi-bbs-directory.maycongamesmaximo.workers.dev/rooms/'+'0'.repeat(32);
for(let attempt=1;attempt<=8;attempt++){
 try{
  const current=await fetch(base+'/capabilities',{signal:AbortSignal.timeout(15000),cache:'no-store'});
  assert.equal(current.status,200);
  assert.deepEqual(await current.json(),{release:'directory-p16.1',supported_protocols:[6,7,14,15,16]});
  const legacy=await fetch(base+'/v3/capabilities',{signal:AbortSignal.timeout(15000),cache:'no-store'});
  assert.equal(legacy.status,200);
  const body=await legacy.json();assert.equal(body.version,3);assert.equal(body.lease_ms,60000);
  console.log('LIVE VERIFIED: directory-p16.1 accepts Bridge discovery protocols 6/7/14/15/16; existing v3 capabilities preserved.');
  process.exit(0);
 }catch(error){
  if(attempt===8)throw error;
  await setTimeout(3000);
 }
}
