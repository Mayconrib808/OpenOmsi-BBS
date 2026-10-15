#!/usr/bin/env python3
"""Exercise the actual Lua companion against the documented API through 0.3.0.

Requires lupa; no simulator or commercial map content is used.
"""
from pathlib import Path
from lupa import LuaRuntime

SCRIPT = (Path(__file__).resolve().parents[1] / "source/live_trip_plugin.lua").read_text(encoding="utf-8")
BOOT = r'''
local callback
command, status, messages, sends, starts, saves, handlers, logs = nil, nil, {}, {}, 0, 0, {}, {}
auto_sign, sign_writes, sign_accepts, sign_applies = true, 0, true, true
lan_active, bus_file, map_path = false, "Vehicles/Test/bus.bus", "/OMSI/maps/Test/global.cfg"
rows = {
  {trip=1, station=1, name="A", departure=14700},
  {trip=1, station=2, name="B", departure=16200},
  {trip=2, station=1, name="B", departure=16500},
  {trip=2, station=2, name="A", departure=18000}
}
duty, destination_index = {line="1", tour="1", trip=1, departure=14700, terminus="B"}, 2
function take(line, tour, trip)
  starts=starts+1
  duty={line=line, tour=tour, trip=trip, departure=16500, terminus="A", trip_name="return"}
  if auto_sign then destination_index=1 end
end
omsi = {
 on=function(event, fn)
   handlers[event]=handlers[event] or {}; table.insert(handlers[event],fn); return fn
 end,
 every=function(_, fn) callback=fn end,
 log=function(s) table.insert(logs,s) end,
 message=function(s) table.insert(messages,s) end,
 speed=function() return 0 end,
 command=function(s) assert(s=="save"); saves=saves+1; return true end,
 map={info=function() return {path=map_path} end},
 game={version=function() return "0.3.0" end},
 files={read=function() return command and "json" or nil end,write=function(_,_) return true end},
 json={decode=function() return command end,encode=function(t) status=t; return "json" end},
 duty={get=function() return duty end,start=function(l,t,n,stop) assert(stop==1);take(l,t,n);return true end},
 timetable={stops=function() return rows end},
 bus={file=function() return bus_file end,
      destination=function() return destination_index==1 and "A" or "B",destination_index end,
      set_destination=function(index)
        sign_writes=sign_writes+1
        if sign_accepts and sign_applies then destination_index=index end
        return sign_accepts
      end,
      destinations=function() return {{index=1,name="A"},{index=2,name="B"}} end},
 lan={active=function() return lan_active end,me=function() return 2,"Test",false end,
      send=function(id,text) assert(id==1);table.insert(sends,text);return true end}
}
-- Match the plugin runtime: registered handlers or on_<event> receive events.
-- A bare function named after the event is not called by the game.
function emit(event, ...)
  local global=_G["on_"..event]
  if type(global)=="function" then global(...) end
  for _,fn in ipairs(handlers[event] or {}) do fn(...) end
end
function step(n) for _=1,n do callback() end end
function prepare()
 command={token="0123456789abcdef",id="1002_1234abcd",phase="prepare",map=map_path,
   bus=bus_file,line="130 & N30",tour="Mo-Fr 9",minute=275,origin="B",terminus="A"}
end
'''

def runtime(language="en"):
    lua = LuaRuntime(unpack_returned_tuples=True)
    lua.execute(BOOT)
    lua.execute(SCRIPT.replace("__BRIDGE_TOKEN__", "0123456789abcdef").replace("__BRIDGE_LANGUAGE__", language))
    return lua

lua = runtime()
lua.execute('prepare(); step(6); assert(status.phase=="prepared" and saves==1 and starts==0)')
lua.execute('command.phase="apply";step(4);assert(status.phase=="applied" and status.sign_ok and starts==1 and duty.trip==2)')
lua.execute('step(20);assert(starts==1)')

lua = runtime()
lua.execute('lan_active=true;prepare();step(6);command.phase="apply";step(1);assert(sends[1]=="hello" and starts==0)')
lua.execute('emit("lan_message",3,"ready");step(8);assert(starts==0 and not status.host_ready)')
lua.execute('emit("lan_message",1,"ready");step(9);assert(sends[#sends]=="take 1002_1234abcd 130%20&%20N30 Mo-Fr%209 1 0", "host readiness event was not received")')
lua.execute('take("130 & N30","Mo-Fr 9",2);step(1);assert(status.phase=="applying" and status.issued and not status.host_ack);emit("lan_message",1,"done other_request");emit("lan_message",3,"done 1002_1234abcd");step(1);assert(status.phase=="applying" and not status.host_ack);emit("lan_message",1,"done 1002_1234abcd");step(1);assert(status.phase=="applied" and status.sign_ok and status.host_ack)')

# Carrão City regression: host.2 mistakes the native server's HTTP 202 for a
# rejection, then relays the actual duty-ok. Both arrive for the same request.
lua = runtime()
lua.execute('''
lan_active=true
rows={{trip=16,station=1,name="Terminal Dom Pedro 4315",departure=45900},
      {trip=16,station=2,name="ET Vila Carrao 4315",departure=47400}}
omsi.bus.destinations=function() return {{index=1,name="Terminal Dom Pedro 4315"},{index=34,name="ET Vila Carrao 4315"}} end
omsi.bus.destination=function() return "ET Vila Carrao 4315",destination_index end
prepare();command.line="4315";command.tour="SAB01";command.minute=765
command.origin="Terminal Dom Pedro 4315";command.terminus="ET Vila Carrao 4315"
step(6);command.phase="apply";step(1);emit("lan_message",1,"ready");step(9)
assert(sends[#sends]=="take 1002_1234abcd 4315 SAB01 15 0")
emit("lan_message",1,"no 1002_1234abcd Native dispatch rejected");step(1)
assert(status.phase=="applying", "HTTP submission failure aborted before the native acknowledgment")
duty={line="4315",tour="SAB01",trip=16,departure=45900,terminus="ET Vila Carrao 4315"}
step(1);assert(status.phase=="applying" and not status.host_ack)
emit("lan_message",1,"done 1002_1234abcd");step(1)
assert(status.phase=="applying" and status.host_ack and not status.sign_ok)
destination_index=34;step(1)
assert(status.phase=="applied" and status.sign_ok and #messages==1)
local submitted=#sends
emit("lan_message",1,"no 1002_1234abcd Native dispatch rejected");step(20)
assert(status.phase=="applied" and status.sign_ok and #sends==submitted and #messages==1)
''')

lua = runtime()
lua.execute('lan_active=true;prepare();step(6);command.phase="apply";step(85);assert(status.phase=="error" and status.error=="The native host did not answer the next-trip handshake" and starts==0)')

lua = runtime()
lua.execute('lan_active=true;prepare();step(6);command.phase="apply";step(1);emit("lan_message",1,"ready");step(9);emit("lan_message",1,"no 1002_1234abcd Native client rejected the duty");step(1);assert(status.phase=="error" and starts==0)')

# The actual duty-ok takes precedence over an HTTP submission error that
# arrives later, but only while this exact request is applying.
lua = runtime()
lua.execute('lan_active=true;prepare();step(6);command.phase="apply";step(1);emit("lan_message",1,"ready");step(9);take("130 & N30","Mo-Fr 9",2);emit("lan_message",1,"done 1002_1234abcd");emit("lan_message",1,"no 1002_1234abcd Native dispatch rejected");step(1);assert(status.phase=="applied" and status.sign_ok and #messages==1)')

# Transport errors are still failures if the native client never confirms.
# A cancel file and late LAN messages must not repeat or overwrite the error.
lua = runtime()
lua.execute('''
lan_active=true;prepare();step(6);command.phase="apply";step(1)
emit("lan_message",1,"ready");step(9)
emit("lan_message",1,"no 1002_1234abcd Native dispatch unavailable");step(1)
take("130 & N30","Mo-Fr 9",2);step(2)
assert(status.phase=="applying" and not status.host_ack)
step(80)
assert(status.phase=="error" and status.error=="The native client did not confirm the requested duty: Native dispatch unavailable")
local original=status.error
command.phase="cancel";step(60);emit("lan_message",1,"done 1002_1234abcd");step(1)
assert(status.phase=="error" and status.error==original and #messages==1)
''')

# An acknowledgment alone cannot confirm a different timetable or trigger a
# second dispatch while the native state catches up.
lua = runtime()
lua.execute('''
lan_active=true;prepare();step(6);command.phase="apply";step(1)
emit("lan_message",1,"ready");step(9);emit("lan_message",1,"done 1002_1234abcd")
local submitted=#sends
step(85)
assert(status.phase=="error" and status.error=="The loaded timetable does not match the selected BCS trip" and #sends==submitted)
''')

lua = runtime()
lua.execute('table.insert(rows,{trip=3,station=1,name="B",departure=16500});table.insert(rows,{trip=3,station=2,name="A",departure=18000});prepare();step(1);assert(status.phase=="error" and starts==0 and saves==0)')

lua = runtime()
lua.execute('prepare();command.bus="Vehicles/Other/bus.bus";step(1);assert(status.phase=="error" and starts==0)')
lua.execute('prepare();command.id="1003_5678abcd";command.map="/OMSI/maps/Other/global.cfg";step(1);assert(status.id=="1003_5678abcd" and status.phase=="error" and status.error=="The loaded map/bus differs from the BCS selection" and #messages==2)')

lua = runtime()
lua.execute('prepare();step(6);command.phase="cancel";step(1);command.phase="apply";step(8);assert(status.phase=="error" and starts==0)')
# Native programming has a grace period: do not interfere with an IBIS that
# finishes naturally. Already-correct signs must never be rewritten.
lua = runtime()
lua.execute('prepare();step(6);command.phase="apply";step(20);assert(status.phase=="applied" and sign_writes==0)')
lua = runtime()
lua.execute('auto_sign=false;prepare();step(6);command.phase="apply";step(8);assert(status.phase=="applying" and sign_writes==0);destination_index=1;step(1);assert(status.phase=="applied" and sign_writes==0)')

# A unique HOF match can be corrected once the native timetable is confirmed.
# Even a successful setter must be verified by a subsequent display reading.
lua = runtime()
lua.execute('''
auto_sign=false;prepare();step(6);command.phase="apply";step(13)
assert(sign_writes==1 and destination_index==1 and status.phase=="applying" and not status.sign_ok)
step(1);assert(status.phase=="applied" and status.sign_ok and status.sign_attempts==1)
step(100);assert(sign_writes==1 and #messages==1 and starts==1)
''')

# Acceptance is not display confirmation: bound retries and report a separate
# unverified state, keeping the confirmed timetable available to BCS.
lua = runtime()
lua.execute('''
auto_sign=false;sign_applies=false;prepare();step(6);command.phase="apply";step(65)
assert(status.phase=="applied_unverified" and not status.sign_ok and sign_writes==2)
assert(status.trip==2 and status.destination_index==2 and status.expected_destination_index==1)
step(80);assert(sign_writes==2 and #messages==1)
destination_index=1;step(1);assert(status.phase=="applied" and status.sign_ok and #messages==2)
step(80);assert(sign_writes==2 and #messages==2 and starts==1)
''')

for unavailable in (
    'omsi.bus.destinations=function() return {{index=2,name="B"}} end',
    'omsi.bus.destinations=function() return {{index=1,name="A"},{index=3,name="A"}} end',
    'omsi.bus.destinations=function() return {{index=0,name="A"}} end',
    'omsi.bus.set_destination=nil',
):
    lua = runtime()
    lua.execute('auto_sign=false;' + unavailable + ';prepare();step(6);command.phase="apply";step(65);assert(status.phase=="applied_unverified" and not status.sign_ok and sign_writes==0)')

# A rejection or Lua exception cannot turn into a false confirmation or crash
# the companion. Corrections remain bounded and the warning is visible.
for setter in ('sign_accepts=false', 'omsi.bus.set_destination=function() sign_writes=sign_writes+1;error("unsupported") end'):
    lua = runtime()
    lua.execute('auto_sign=false;' + setter + ';prepare();step(6);command.phase="apply";step(65);assert(status.phase=="applied_unverified" and not status.sign_ok and sign_writes==2 and #messages==1)')

lua = runtime()
lua.execute('auto_sign=false;prepare();step(6);command.phase="apply";step(2);omsi.speed=function() return 20 end;step(65);assert(status.phase=="applied_unverified" and sign_writes==0)')
lua = runtime()
lua.execute('auto_sign=false;lan_active=true;prepare();step(6);command.phase="apply";step(1);emit("lan_message",1,"ready");step(9);take("130 & N30","Mo-Fr 9",2);step(20);assert(sign_writes==0 and not status.host_ack);emit("lan_message",1,"done 1002_1234abcd");step(13);assert(sign_writes==1);step(1);assert(status.phase=="applied" and status.sign_ok)')

# Cancellation/vehicle changes stop corrections, even if the old bus later
# reports the requested destination. A new selection gets fresh retry state.
lua = runtime()
lua.execute('auto_sign=false;sign_applies=false;prepare();step(6);command.phase="apply";step(13);command.phase="cancel";step(1);destination_index=1;step(90);assert(status.phase=="error" and sign_writes==1)')
lua = runtime()
lua.execute('auto_sign=false;prepare();step(6);command.phase="apply";step(2);bus_file="Vehicles/Other/bus.bus";step(90);assert(status.phase=="error" and sign_writes==0)')
lua = runtime()
lua.execute('auto_sign=false;sign_applies=false;prepare();step(6);command.phase="apply";step(65);prepare();command.id="1003_5678abcd";step(6);assert(status.phase=="prepared" and status.sign_attempts==0 and not status.sign_ok)')

for language, phrase in (("pt", "destino não foi confirmado"), ("en", "destination is unconfirmed"), ("de", "Ziel aber nicht bestätigt")):
    lua = runtime(language)
    lua.execute('auto_sign=false;sign_applies=false;prepare();step(6);command.phase="apply";step(65)')
    assert phrase in lua.globals().messages[1], language

print("Lua companion: native acknowledgments, HOF confirmation/correction, bounded retries, ambiguous/missing HOF, cancellation, BCS recording continuity and PT/EN/DE warnings passed")
