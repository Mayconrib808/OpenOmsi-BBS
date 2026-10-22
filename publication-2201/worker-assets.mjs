// Static-assets distribution: uses existing Workers token permissions.
export default {
 async fetch(request,env) {
  if(!['GET','HEAD'].includes(request.method)) return new Response('Method not allowed',{status:405,headers:{Allow:'GET, HEAD'}});
  const url=new URL(request.url);
  if(url.pathname==='/health' && !url.search) return new Response(request.method==='HEAD'?null:JSON.stringify({service:'openomsi-bbs-updates',schema:1}),{headers:{'Content-Type':'application/json','Cache-Control':'no-store'}});
  const stable=url.pathname==='/stable.json';
  if(url.search || (!stable && !/^\/packages\/[a-f0-9]{64}\.(exe|bin)$/.test(url.pathname))) return new Response('Not found',{status:404});
  const response=await env.ASSETS.fetch(request);
  if(response.status!==200) return new Response('Not found',{status:404,headers:{'Cache-Control':'no-store'}});
  const headers=new Headers(response.headers);
  headers.set('Cache-Control',stable?'no-store':'public, max-age=31536000, immutable');
  headers.set('Content-Type',stable?'application/json; charset=utf-8':'application/octet-stream');
  headers.set('X-Content-Type-Options','nosniff');
  if(!stable) headers.set('Content-Disposition',url.pathname.endsWith('.exe')?'attachment; filename="Atualizar-Bridge.exe"':'attachment; filename="Coop-parte.bin"');
  return new Response(response.body,{status:200,headers});
 }
};
