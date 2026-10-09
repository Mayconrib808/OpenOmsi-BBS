// A room URL is the permanent invitation. Each map has one renewable lease.
const json=(v,s=200)=>new Response(JSON.stringify(v),{status:s,headers:{'Content-Type':'application/json','Cache-Control':'no-store'}});
const token=()=>crypto.randomUUID().replaceAll('-','');
const validToken=v=>typeof v==='string'&&/^[a-f0-9]{32}$/.test(v);
export const LEASE_MS=60000;
export function canonicalMap(v){
 if(typeof v!=='string'||v.length>512||/[\x00-\x1f]/.test(v))return null;
 const p=v.replaceAll('\\','/').toLowerCase();
 return p.startsWith('maps/')&&p.endsWith('/global.cfg')&&!p.split('/').some(s=>!s||s==='.'||s==='..')?p:null;
}
function validClock(v){try{return v&&typeof v.timezone==='string'&&v.timezone!=='Local'&&v.timezone.length<=100&&Number.isInteger(v.shift_minutes)&&Math.abs(v.shift_minutes)<=1440&&!!new Intl.DateTimeFormat('en',{timeZone:v.timezone});}catch{return false;}}
function tunnel(v){try{const u=new URL(v);return u.protocol==='https:'&&u.hostname.endsWith('.trycloudflare.com')&&u.pathname==='/'&&!u.search&&!u.hash&&!u.port&&!u.username&&!u.password;}catch{return false;}}
export async function multiplayer3(room,request,path){
 const parts=path.split('/');
 if(parts[0]!=='v3')return null;
 if(request.method==='GET'&&path==='v3/capabilities')return json({version:3,lease_ms:LEASE_MS});
 let b={};if(request.method==='POST'){try{b=await room.body(request);}catch{return json({error:'invalid JSON'},400);}}
 const now=room.now();
 if(path==='v3/settings'){
  if(request.method==='GET'){const s=await room.storage.get('v3_settings');return s?json(s):json({error:'company not created'},404);}
  if(request.method!=='POST')return json({error:'method'},405);
  if(typeof b.name!=='string'||!b.name.trim()||b.name.length>120||/[\x00-\x1f]/.test(b.name)||!validClock(b.clock))return json({error:'invalid company or clock'},400);
  return room.storage.transaction(async tx=>{
   const old=await tx.get('v3_settings');const admin=await tx.get('v3_admin');
   if(old){
    if(request.headers.get('Authorization')!==`Bearer ${admin}`)return json({error:'company already exists'},409);
    const maps=await tx.list({prefix:'v3_map_'});
    if([...maps.values()].some(lease=>lease.expires_at>room.now()))return json({error:'stop active maps before editing the company clock'},409);
    const settings={...old,name:b.name.trim(),clock:b.clock};await tx.put('v3_settings',settings);return json(settings);
   }
   // Existing 2.x rooms cannot be taken over by an invitation holder.
   if(await tx.get('profile_count'))return json({error:'use a new 3.0 invitation'},409);
   const secret=token();const settings={version:3,name:b.name.trim(),clock:b.clock,protocol:6};
   await tx.put('v3_settings',settings);await tx.put('v3_admin',secret);return json({...settings,admin_token:secret},201);
  });
 }
 if(parts.length!==4||parts[1]!=='maps'||!validToken(parts[2])||!['claim','state','renew','release'].includes(parts[3]))return json({error:'not found'},404);
 const key=`v3_map_${parts[2]}`;const action=parts[3];
 if((action==='state'&&request.method!=='GET')||(action!=='state'&&request.method!=='POST'))return json({error:'method'},405);
 const settings=await room.storage.get('v3_settings');if(!settings)return json({error:'company not created'},404);
 const publicState=lease=>({role:'guest',state:lease?.state||'offline',address:lease?.address||'',expires_at:lease?.expires_at||0,clock:settings.clock,map:lease?.map||'',generation:lease?.generation||''});
 return room.storage.transaction(async tx=>{
  let lease=await tx.get(key);if(lease&&lease.expires_at<=now)lease=null;
  if(action==='state')return json(publicState(lease));
  if(action==='claim'){
   const map=canonicalMap(b.map);if(!map||!validToken(b.owner))return json({error:'invalid map or owner'},400);
   const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(map));const id=Array.from(new Uint8Array(digest).slice(0,16),x=>x.toString(16).padStart(2,'0')).join('');
   if(id!==parts[2])return json({error:'map identity mismatch'},400);
   if(lease&&lease.owner!==b.owner)return json(publicState(lease));
   if(!lease){lease={owner:b.owner,token:token(),generation:token(),map,state:'starting',address:'',expires_at:now+LEASE_MS};await tx.put(key,lease);}
   return json({...publicState(lease),role:'host',lease_token:lease.token});
  }
  if(!lease||request.headers.get('Authorization')!==`Bearer ${lease.token}`)return json({error:'lease lost'},409);
  if(action==='release'){await tx.delete(key);return json({ok:true});}
  if(!['starting','online'].includes(b.state)||(b.state==='online'&&!tunnel(b.address)))return json({error:'invalid state or tunnel'},400);
  lease={...lease,state:b.state,address:b.state==='online'?b.address:'',expires_at:now+LEASE_MS};await tx.put(key,lease);return json({...publicState(lease),role:'host',lease_token:lease.token});
 });
}
