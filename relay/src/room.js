// Shared protocol logic, used by the Worker and the local regression harness.
// Public room addresses are unguessable invitations, never host credentials.
const idPattern = /^[a-z0-9][a-z0-9_-]{0,63}$/;
const states = new Set(['offline','starting','online','stopping','error','disabled']);
const limit = 8 * 1024 * 1024;
const json = (data, status=200) => new Response(JSON.stringify(data), {status,headers:{'Content-Type':'application/json','Cache-Control':'no-store'}});
const fail = (message,status) => json({error:message},status);
export class RoomProtocol {
  constructor(storage,hostKey,now=()=>Date.now()) { this.storage=storage;this.hostKey=hostKey;this.now=now; }
  async readProfile(tx) {
    if(!tx) return this.storage.transaction(t=>this.readProfile(t));
    const count=await tx.get('profile_count');
    if (!count) return null;
    const keys=Array.from({length:count},(_,i)=>`profile_${i}`);
    const parts=await tx.get(keys);
    const bytes=new Uint8Array([...parts.values()].reduce((n,b)=>n+b.length,0));
    let offset=0;
    for (const key of keys) {const b=parts.get(key);bytes.set(b,offset);offset+=b.length;}
    return JSON.parse(new TextDecoder().decode(bytes));
  }
  async body(request) {
    const reader=request.body?.getReader();if(!reader) throw new Error('missing body');
    const chunks=[];let size=0;
    try {for(;;){const{value,done}=await reader.read();if(done)break;size+=value.length;if(size>limit){await reader.cancel();throw new Error('body too large');}chunks.push(value);}}
    finally{reader.releaseLock();}
    const bytes=new Uint8Array(size);let offset=0;for(const b of chunks){bytes.set(b,offset);offset+=b.length;}
    return JSON.parse(new TextDecoder().decode(bytes));
  }
  async publicState(id) {
    const heartbeat=await this.storage.get('heartbeat');
    const host_online=!!heartbeat && !heartbeat.offline && this.now()-heartbeat.time<=45000;
    const state=heartbeat?.sessions?.find(s=>s.session_id===id) ?? {session_id:id,state:'offline',players:0};
    return {host_online,session:host_online?state:{session_id:id,state:'offline',players:0}};
  }
  async fetch(request) {
    try {
      const url=new URL(request.url);
      const match=url.pathname.match(/^\/rooms\/([a-f0-9]{32})\/(profile|publish|heartbeat|requests|wake|sessions\/[a-z0-9_-]{1,64})$/);
      if(!match || url.search) return fail('not found',404);
      const action=match[2];
      const privateAction=['publish','heartbeat','requests'].includes(action);
      if(privateAction && (!this.hostKey || this.hostKey.length<32 || request.headers.get('Authorization')!==`Bearer ${this.hostKey}`)) return fail('unauthorized',401);
      if(action==='publish' && request.method==='POST') {
        const p=await this.body(request);
        const allowed=new Set(['schema_version','company_id','company_name','openomsi_version','protocol','clock','directory_url','packages','sessions']);
        if(!p || Object.keys(p).some(k=>!allowed.has(k)) || p.schema_version!==1 || !idPattern.test(p.company_id) || p.protocol!==6 || !Array.isArray(p.sessions) || !p.sessions.length || p.sessions.length>16 || p.directory_url!==`${url.origin}/rooms/${match[1]}`) return fail('invalid player profile',400);
        const ids=new Set();for(const s of p.sessions){if(!idPattern.test(s.id)||ids.has(s.id))return fail('invalid sessions',400);ids.add(s.id);}
        const existing=await this.readProfile();if(existing && existing.company_id!==p.company_id)return fail('company identity cannot change',409);
        const bytes=new TextEncoder().encode(JSON.stringify(p));
        if(bytes.length>limit) return fail('profile too large',413);
        // Durable Object values are bounded. Split large legacy asset manifests
        // into byte chunks and replace the whole snapshot in one transaction.
        await this.storage.transaction(async tx=>{
          const count=Math.ceil(bytes.length/65536);const old=await tx.get('profile_count')||0;
          for(let i=0;i<count;i++)await tx.put(`profile_${i}`,bytes.slice(i*65536,(i+1)*65536));
          for(let i=count;i<old;i++)await tx.delete(`profile_${i}`);
          await tx.put('profile_count',count);
        });
        return json({ok:true});
      }
      if(action==='profile' && request.method==='GET') {
        const profile=await this.storage.transaction(tx=>this.readProfile(tx));
        return profile?json(profile):fail('host has not published this company',404);
      }
      const profile=await this.readProfile();if(!profile)return fail('unknown company',404);
      const ids=new Set(profile.sessions.map(s=>s.id));
      if(action==='heartbeat' && request.method==='POST') {
        const b=await this.body(request);
        if(!Array.isArray(b.sessions) || b.sessions.length!==ids.size || new Set(b.sessions.map(s=>s.session_id)).size!==ids.size) return fail('invalid heartbeat',400);
        for(const s of b.sessions){if(!ids.has(s.session_id)||!states.has(s.state)||!Number.isInteger(s.players)||s.players<0||s.players>128||s.error?.length>2000)return fail('invalid session state',400);if(s.state==='online'){let address;try{address=new URL(s.address);}catch{return fail('invalid tunnel',400);}if(address.protocol!=='https:'||!address.hostname.endsWith('.trycloudflare.com')||address.username||address.password||address.search||address.hash||address.port||address.pathname!=='/')return fail('invalid tunnel',400);}}
        await this.storage.put('heartbeat',{time:this.now(),offline:b.offline===true,sessions:b.sessions.map(s=>({session_id:s.session_id,state:s.state,address:s.address||'',players:s.players,error:s.error||''}))});
        return json({ok:true});
      }
      if(action==='wake' && request.method==='POST') {
        const b=await this.body(request);if(!idPattern.test(b.session_id)||!ids.has(b.session_id))return fail('map is not registered',404);
        const reply=await this.publicState(b.session_id);
        if(reply.host_online && reply.session.state!=='disabled'){
          // Per-session entries collapse simultaneous clients into one demand.
          await this.storage.put(`demand_${b.session_id}`,this.now());
        }
        return json(reply);
      }
      if(action==='requests' && request.method==='GET') {
        const requests=[];const now=this.now();
        for(const id of ids){const at=await this.storage.get(`demand_${id}`);if(at && now-at<120000)requests.push({session_id:id,requested_at:at});}
        return json({requests});
      }
      if(action.startsWith('sessions/') && request.method==='GET') {
        const id=action.slice(9);if(!ids.has(id))return fail('unknown map',404);return json(await this.publicState(id));
      }
      return fail('method not allowed',405);
    } catch {return fail('invalid request',400);}
  }
}
