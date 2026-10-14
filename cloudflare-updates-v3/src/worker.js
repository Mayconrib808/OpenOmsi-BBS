export default {async fetch(request,env){
 const path=new URL(request.url).pathname;
 if(request.method!=='GET'&&request.method!=='HEAD')return new Response('Method not allowed',{status:405});
 if(path==='/download/full.zip'){
  const m=await (await env.ASSETS.fetch(new URL('/downloads.json',request.url))).json();
  const headers={'Content-Type':'application/zip','Content-Disposition':'attachment; filename="'+m.name+'"','Content-Length':String(m.size),'X-Content-SHA256':m.sha256,'Cache-Control':'no-store'};
  if(request.method==='HEAD')return new Response(null,{headers});
  const body=new ReadableStream({async start(controller){try{for(const p of m.parts){const r=await env.ASSETS.fetch(new URL(p,request.url));if(!r.ok)throw new Error('Package chunk missing');const reader=r.body.getReader();while(true){const {done,value}=await reader.read();if(done)break;controller.enqueue(value)}}controller.close()}catch(e){controller.error(e)}}});
  return new Response(body,{headers});
 }
 if(path!=='/preview.json'&&path!=='/preview-v2.json'&&!/^\/packages\/[a-f0-9]{64}\.exe$/.test(path))return new Response('Not found',{status:404});
 const asset=await env.ASSETS.fetch(request);const response=new Response(asset.body,asset);
 response.headers.set('X-Update-Product','bbs-openomsi-multiplayer');response.headers.set('X-Update-Channel','3.x');response.headers.set('X-Content-Type-Options','nosniff');
 response.headers.set('Cache-Control',(path==='/preview.json'||path==='/preview-v2.json')?'no-store':'public, max-age=31536000, immutable');return response;
}};