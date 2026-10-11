import test from 'node:test';
import assert from 'node:assert/strict';
import worker from './worker.mjs';
const hash='a'.repeat(64);
let keys=[];
const object={size:2,httpEtag:'"test"',body:'MZ'};
const env={UPDATES:{async get(key){keys.push(key);return key==='stable.json'||key===`packages/${hash}.exe`?object:null;},async head(key){return this.get(key);}}};
test('serves manifest fresh and installer immutable',async()=>{
 const manifest=await worker.fetch(new Request('https://updates.example/stable.json'),env);
 assert.equal(manifest.status,200);assert.equal(manifest.headers.get('cache-control'),'no-store');
 const installer=await worker.fetch(new Request(`https://updates.example/packages/${hash}.exe`),env);
 assert.match(installer.headers.get('cache-control'),/immutable/);assert.equal(await installer.text(),'MZ');
 const head=await worker.fetch(new Request(`https://updates.example/packages/${hash}.exe`,{method:'HEAD'}),env);
 assert.equal(await head.text(),'');assert.equal(head.headers.get('content-length'),'2');
});
test('no uploads, listing, arbitrary paths or queried assets',async()=>{
 keys=[];
 for(const [path,method,status] of [['/stable.json','PUT',405],['/','GET',404],['/packages/','GET',404],['/private-key.json','GET',404],['/stable.json?key=x','GET',404]]) {
  const r=await worker.fetch(new Request(`https://updates.example${path}`,{method}),env);assert.equal(r.status,status);
 }
 assert.deepEqual(keys,[]);
});
test('missing object and storage failure are bounded errors',async()=>{
 assert.equal((await worker.fetch(new Request(`https://updates.example/packages/${'b'.repeat(64)}.exe`),env)).status,404);
 assert.equal((await worker.fetch(new Request('https://updates.example/stable.json'),{UPDATES:{get(){throw Error('private details');}}})).status,503);
});
