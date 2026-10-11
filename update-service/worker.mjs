// Public distribution only. Publishing uses the owner's Cloudflare dashboard.
export default {
  async fetch(request, env) {
    if (!['GET', 'HEAD'].includes(request.method)) {
      return new Response('Method not allowed', {status:405, headers:{Allow:'GET, HEAD'}});
    }
    const url = new URL(request.url);
    const stable = url.pathname === '/stable.json';
    if (url.search || (!stable && !/^\/packages\/[a-f0-9]{64}\.exe$/.test(url.pathname))) {
      return new Response('Not found', {status:404});
    }
    const key = url.pathname.slice(1);
    try {
      const object = request.method === 'HEAD' ? await env.UPDATES.head(key) : await env.UPDATES.get(key);
      if (!object) return new Response('Not found', {status:404, headers:{'Cache-Control':'no-store'}});
      const headers = new Headers({
        'Content-Type':stable ? 'application/json; charset=utf-8' : 'application/octet-stream',
        'Cache-Control':stable ? 'no-store' : 'public, max-age=31536000, immutable',
        'Content-Length':String(object.size),
        'X-Content-Type-Options':'nosniff',
        'ETag':object.httpEtag,
      });
      if (!stable) headers.set('Content-Disposition','attachment; filename="Atualizar-Bridge.exe"');
      return new Response(request.method === 'HEAD' ? null : object.body, {headers});
    } catch {
      return new Response('Distribution temporarily unavailable', {status:503,headers:{'Cache-Control':'no-store'}});
    }
  }
};
