import { test } from 'node:test';
import assert from 'node:assert/strict';
import { RoomProtocol } from '../src/room.js';
class Storage {
  data=new Map();
  async get(key){if(Array.isArray(key))return new Map(key.filter(k=>this.data.has(k)).map(k=>[k,structuredClone(this.data.get(k))]));return structuredClone(this.data.get(key));}
  async put(key,value){this.data.set(key,structuredClone(value));}
  async delete(key){this.data.delete(key);}
  async transaction(fn){return fn(this);}
}
const base='https://directory.example/rooms/0123456789abcdef0123456789abcdef';
const key='a'.repeat(64);
function profile(){return{schema_version:1,company_id:'transfort-br',company_name:'Transfort',protocol:6,directory_url:base,packages:[],sessions:[{id:'map-a',server_url:'http://127.0.0.1:27025'},{id:'map-b',server_url:'http://127.0.0.1:27026'}]};}
function fixture(){let now=100000;const room=new RoomProtocol(new Storage(),key,()=>now);return{room,setNow:n=>now=n,call:async(path,method='GET',body=null,auth=false)=>room.fetch(new Request(base+path,{method,headers:{...(auth?{Authorization:`Bearer ${key}`}:{})},...(body?{body:JSON.stringify(body)}:{})}))};}
test('host credentials stay private and player writes cannot replace profiles',async()=>{
 const{call}=fixture();assert.equal((await call('/publish','POST',profile())).status,401);
 assert.equal((await call('/publish','POST',{...profile(),host_key:key},true)).status,400);
 assert.equal((await call('/publish','POST',profile(),true)).status,200);
 const p=await(await call('/profile')).json();assert.ok(!JSON.stringify(p).includes(key));
 assert.equal((await call('/requests')).status,401);
 assert.equal((await call('/publish','POST',{...profile(),company_id:'other'},true)).status,409);
});
test('wake is deduplicated per registered map, expires, and never starts an unknown map',async()=>{
 const{call,setNow}=fixture();await call('/publish','POST',profile(),true);
 await call('/heartbeat','POST',{sessions:[{session_id:'map-a',state:'offline',players:0},{session_id:'map-b',state:'disabled',players:0}]},true);
 const responses=await Promise.all(Array.from({length:10},()=>call('/wake','POST',{session_id:'map-a'})));
 assert.ok(responses.every(r=>r.status===200));assert.equal((await(await call('/requests','GET',null,true)).json()).requests.length,1);
 assert.equal((await call('/wake','POST',{session_id:'../../command'})).status,404);
 await call('/wake','POST',{session_id:'map-b'});assert.equal((await(await call('/requests','GET',null,true)).json()).requests.length,1);
 setNow(221000);assert.deepEqual((await(await call('/requests','GET',null,true)).json()).requests,[]);
});
test('tunnel rotation uses the same profile URL; disconnected host is not announced online',async()=>{
 const{call,setNow}=fixture();let p=profile();await call('/publish','POST',p,true);
 for(const address of ['https://old-host.trycloudflare.com','https://new-host.trycloudflare.com']){
  p.sessions[0].server_url=address;await call('/publish','POST',p,true);
  await call('/heartbeat','POST',{sessions:[{session_id:'map-a',state:'online',address,players:1},{session_id:'map-b',state:'offline',players:0}]},true);
  assert.equal((await(await call('/profile')).json()).sessions[0].server_url,address);
  assert.equal((await(await call('/sessions/map-a')).json()).session.address,address);
 }
 setNow(146000);const reply=await(await call('/sessions/map-a')).json();assert.equal(reply.host_online,false);assert.equal(reply.session.state,'offline');assert.equal(reply.session.address,undefined);
 const wake=await(await call('/wake','POST',{session_id:'map-a'})).json();assert.equal(wake.host_online,false);
});
test('legacy manifest is chunked beneath Durable Object per-value limits and replaces atomically',async()=>{
 const{call,room}=fixture();const p=profile();p.packages=[{name:'á'.repeat(150000)}];await call('/publish','POST',p,true);
 assert.deepEqual(await(await call('/profile')).json(),p);
 for(const[k,v]of room.storage.data){if(k.startsWith('profile_')&&k!=='profile_count')assert.ok(v.length<=65536);}
 await call('/publish','POST',profile(),true);assert.equal(await room.storage.get('profile_count'),1);assert.equal(await room.storage.get('profile_1'),undefined);
});
test('only verified HTTPS tunnel addresses can be advertised online',async()=>{
 const{call}=fixture();await call('/publish','POST',profile(),true);
 for(const address of ['http://bad.trycloudflare.com','https://evil.example','https://good.trycloudflare.com/?redirect=1'])assert.equal((await call('/heartbeat','POST',{sessions:[{session_id:'map-a',state:'online',address,players:0},{session_id:'map-b',state:'offline',players:0}]},true)).status,400);
});
