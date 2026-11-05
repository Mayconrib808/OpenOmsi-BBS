import {test} from 'node:test';
import assert from 'node:assert/strict';
import {RoomProtocol} from '../src/room.js';
import {LEASE_MS,canonicalMap} from '../src/multiplayer3.js';
class Storage {
 data=new Map(); queue=Promise.resolve();
 async get(k){return structuredClone(this.data.get(k));}
 async put(k,v){this.data.set(k,structuredClone(v));}
 async delete(k){this.data.delete(k);}
 async list({prefix}){return new Map([...this.data].filter(([k])=>k.startsWith(prefix)).map(([k,v])=>[k,structuredClone(v)]));}
 transaction(fn){const result=this.queue.then(()=>fn(this));this.queue=result.catch(()=>{});return result;}
}
const base='https://directory.example/rooms/'+ 'a'.repeat(32);
async function fixture(){let now=1000;const storage=new Storage();const room=new RoomProtocol(storage,'host'.repeat(16),()=>now);
 const call=async(p,method='POST',body={},token='')=>room.fetch(new Request(base+'/v3/'+p,{method,headers:{Authorization:'Bearer '+token},...(method==='GET'?{}:{body:JSON.stringify(body)})}));
 await call('settings','POST',{name:'Transfort',clock:{timezone:'Europe/Berlin',shift_minutes:-480}});
 const map=canonicalMap('Maps/Berlin-Spandau/global.cfg');const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(map));const id=Buffer.from(digest).subarray(0,16).toString('hex');
 return {call,id,map,setNow:n=>now=n,storage};
}
test('simultaneous entries choose exactly one host; peers receive no ownership secret',async()=>{
 const{call,id,map}=await fixture();const replies=await Promise.all(Array.from({length:20},(_,i)=>call(`maps/${id}/claim`,'POST',{map,owner:i.toString(16).padStart(32,'0')})));
 const states=await Promise.all(replies.map(r=>r.json()));assert.equal(states.filter(s=>s.role==='host').length,1);
 for(const s of states.filter(s=>s.role==='guest'))assert.equal(s.lease_token,undefined);
});
test('renewal fences old hosts, stale releases cannot erase the replacement, clocks stay fixed',async()=>{
 const{call,id,map,setNow}=await fixture();const a=await(await call(`maps/${id}/claim`,'POST',{map,owner:'b'.repeat(32)})).json();
 assert.equal((await call(`maps/${id}/renew`,'POST',{state:'online',address:'https://evil.example'},a.lease_token)).status,400);
 assert.equal((await call(`maps/${id}/renew`,'POST',{state:'online',address:'https://a.trycloudflare.com'},a.lease_token)).status,200);
 setNow(1001+LEASE_MS);const b=await(await call(`maps/${id}/claim`,'POST',{map,owner:'c'.repeat(32)})).json();assert.equal(b.role,'host');assert.notEqual(b.lease_token,a.lease_token);assert.equal(b.clock.shift_minutes,-480);
 assert.equal((await call(`maps/${id}/release`,'POST',{},a.lease_token)).status,409);
 assert.equal((await call(`maps/${id}/renew`,'POST',{state:'starting'},a.lease_token)).status,409);
 assert.equal((await call(`maps/${id}/renew`,'POST',{state:'starting'},b.lease_token)).status,200);
});
test('different maps get independent hosts and restart never needs a new invitation',async()=>{
 const{call,id,map}=await fixture();const owner='d'.repeat(32);let first=await(await call(`maps/${id}/claim`,'POST',{map,owner})).json();
 const map2='maps/grundorf/global.cfg';const id2=Buffer.from(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(map2))).subarray(0,16).toString('hex');
 const second=await(await call(`maps/${id2}/claim`,'POST',{map:map2,owner})).json();assert.equal(second.role,'host');assert.notEqual(second.lease_token,first.lease_token);
 await call(`maps/${id}/release`,'POST',{},first.lease_token);first=await(await call(`maps/${id}/claim`,'POST',{map,owner})).json();assert.equal(first.role,'host');
 const settings=await(await call('settings','GET')).json();assert.equal(settings.name,'Transfort');assert.equal(settings.admin_token,undefined);
});
test('company cannot be overwritten and claim map hash cannot alias another map',async()=>{
 const{call,id}=await fixture();assert.equal((await call('settings','POST',{name:'Other',clock:{timezone:'UTC',shift_minutes:0}})).status,409);
 assert.equal((await call(`maps/${id}/claim`,'POST',{map:'maps/other/global.cfg',owner:'e'.repeat(32)})).status,400);
 for(const p of ['maps/../x/global.cfg','maps/x\n/global.cfg','maps//x/global.cfg'])assert.equal(canonicalMap(p),null);
});

test('creator can edit an idle company clock using the same invitation; active maps block it',async()=>{
 const{call,id,map,storage}=await fixture();const admin=await storage.get('v3_admin');const update={name:'Transfort',clock:{timezone:'UTC',shift_minutes:-180}};
 assert.equal((await call('settings','POST',update,admin)).status,200);
 const l=await(await call(`maps/${id}/claim`,'POST',{map,owner:'f'.repeat(32)})).json();assert.equal(l.clock.shift_minutes,-180);
 assert.equal((await call('settings','POST',update,admin)).status,409);
 await call(`maps/${id}/release`,'POST',{},l.lease_token);assert.equal((await call('settings','POST',update,admin)).status,200);
});
