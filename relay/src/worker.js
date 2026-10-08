import { DurableObject } from 'cloudflare:workers';
import { RoomProtocol } from './room.js';
export class CompanyRoom extends DurableObject {
  constructor(ctx,env) {super(ctx,env);this.room=new RoomProtocol(ctx.storage,env.HOST_KEY);}
  fetch(request) {return this.room.fetch(request);}
}
export default {
  fetch(request,env) {
    const match=new URL(request.url).pathname.match(/^\/rooms\/([a-f0-9]{32})\//);
    if(!match) return new Response('OpenOmsi + BBS directory',{status:404});
    return env.ROOMS.get(env.ROOMS.idFromName(match[1])).fetch(request);
  }
};
