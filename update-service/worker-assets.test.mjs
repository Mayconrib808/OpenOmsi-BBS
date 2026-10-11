import test from 'node:test';
import assert from 'node:assert/strict';
import worker from './worker-assets.mjs';
const hash='a'.repeat(64);
let reads=0;
const env={ASSETS:{async fetch(r){reads++;return new Response(r.method==='HEAD'?null:'MZ',{headers:{'Content-Length':'2'}});}}};
test('health, manifest and binary work with existing assets permissions',async()=>{
 const health=await worker.fetch(new Request('https://updates.example/health'),env);assert.equal((await health.json()).schema,1);
 const stable=await worker.fetch(new Request('https://updates.example/stable.json'),env);assert.equal(stable.headers.get('cache-control'),'no-store');
 const exe=await worker.fetch(new Request(`https://updates.example/packages/${hash}.exe`),env);assert.equal(await exe.text(),'MZ');assert.match(exe.headers.get('cache-control'),/immutable/);
 const head=await worker.fetch(new Request(`https://updates.example/packages/${hash}.exe`,{method:'HEAD'}),env);assert.equal(await head.text(),'');
});
test('no arbitrary files or publishing endpoint is exposed',async()=>{
 reads=0;
 for(const [path,method,status] of [['/private-key.json','GET',404],['/stable.json?token=x','GET',404],['/stable.json','PUT',405]]) { assert.equal((await worker.fetch(new Request(`https://updates.example${path}`,{method}),env)).status,status); }
 assert.equal(reads,0);
});
