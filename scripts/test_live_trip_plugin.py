#!/usr/bin/env python3
"""Exercise the actual Lua companion against the documented 0.2.29 API shape.

Requires lupa; no simulator or commercial map content is used.
"""
from pathlib import Path
from lupa import LuaRuntime

SCRIPT = (Path(__file__).resolve().parents[1] / "source/live_trip_plugin.lua").read_text()
BOOT = r'''
local callback
command, status, messages, sends, starts, saves = nil, nil, {}, {}, 0, 0
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
  destination_index=1
end
omsi = {
 every=function(_, fn) callback=fn end,
 message=function(s) table.insert(messages,s) end,
 speed=function() return 0 end,
 command=function(s) assert(s=="save"); saves=saves+1; return true end,
 map={info=function() return {path=map_path} end},
 game={version=function() return "0.2.29" end},
 files={read=function() return command and "json" or nil end,write=function(_,_) return true end},
 json={decode=function() return command end,encode=function(t) status=t; return "json" end},
 duty={get=function() return duty end,start=function(l,t,n,stop) assert(stop==1);take(l,t,n);return true end},
 timetable={stops=function() return rows end},
 bus={file=function() return bus_file end,
      destination=function() return destination_index==1 and "A" or "B",destination_index end,
      destinations=function() return {{index=1,name="A"},{index=2,name="B"}} end},
 lan={active=function() return lan_active end,me=function() return 2,"Test",false end,
      send=function(id,text) assert(id==1);table.insert(sends,text);return true end}
}
function step(n) for _=1,n do callback() end end
function prepare()
 command={token="0123456789abcdef",id="1002_1234abcd",phase="prepare",map=map_path,
   bus=bus_file,line="130 & N30",tour="Mo-Fr 9",minute=275,origin="B",terminus="A"}
end
'''

def runtime():
    lua = LuaRuntime(unpack_returned_tuples=True)
    lua.execute(BOOT)
    lua.execute(SCRIPT.replace("__BRIDGE_TOKEN__", "0123456789abcdef"))
    return lua

lua = runtime()
lua.execute('prepare(); step(6); assert(status.phase=="prepared" and saves==1 and starts==0)')
lua.execute('command.phase="apply";step(4);assert(status.phase=="applied" and status.sign_ok and starts==1 and duty.trip==2)')
lua.execute('step(20);assert(starts==1)')

lua = runtime()
lua.execute('lan_active=true;prepare();step(6);command.phase="apply";step(1);assert(sends[1]=="hello" and starts==0)')
lua.execute('lan_message(3,"ready");step(8);assert(starts==0)')
lua.execute('lan_message(1,"ready");step(9);assert(sends[#sends]=="take 1002_1234abcd 130%20&%20N30 Mo-Fr%209 1 0")')
lua.execute('take("130 & N30","Mo-Fr 9",2);step(1);assert(status.phase=="applying");lan_message(1,"done 1002_1234abcd");step(1);assert(status.phase=="applied" and status.sign_ok)')

lua = runtime()
lua.execute('table.insert(rows,{trip=3,station=1,name="B",departure=16500});table.insert(rows,{trip=3,station=2,name="A",departure=18000});prepare();step(1);assert(status.phase=="error" and starts==0 and saves==0)')

lua = runtime()
lua.execute('prepare();command.bus="Vehicles/Other/bus.bus";step(1);assert(status.phase=="error" and starts==0)')

lua = runtime()
lua.execute('prepare();step(6);command.phase="cancel";step(1);command.phase="apply";step(8);assert(status.phase=="error" and starts==0)')
print("Lua companion: local and multiplayer handoffs, host identity, duplicate, ambiguity, vehicle and cancellation checks passed")
