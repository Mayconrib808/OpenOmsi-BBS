"""Run the shipped Lua companion against the public openOMSI API contract."""
from pathlib import Path
from lupa import LuaRuntime

SCRIPT=(Path(__file__).resolve().parents[1]/"source/bcs_trip_guard.lua").read_text(encoding="utf-8").replace("__BCS_SELECTION_JSON__",'"selection"')
BOOT=r'''
expected={Token="0123456789abcdef",ID="4974938_1234abcd",Language="pt-BR",
 Map="maps/Any layout/global.cfg",Bus="Vehicles/Private/model.bus",Line="130 & N30",Tour="Mo-Fr 3",
 Start=38,End=56,Origin="Nervenklinik",Terminus="U Rathaus Spandau",
 Route="Nervenklinik - U Rathaus Spandau",StartPoint="Spandau Nervenklinik"}
vehicle,lan=true,true
logs,sends,handlers={}, {}, {}
trips={
 {number=1,name="earlier",departure=-3600,arrival=-1800,terminus="Other"},
 {number=2,name="Pause",departure=2280,arrival=3360,terminus="Pause"},
 {number=3,name="opposite",departure=2280,arrival=3360,terminus="Nervenklinik"},
 {number=4,name="130_Ruhl-Nerv",departure=240,arrival=1800,terminus="Nervenklinik"},
 {number=5,name="130_Nerv-Rath",departure=2280,arrival=3360,terminus="U Rathaus Spandau"}}
stops={
 [1]={{number=1,name="Depot",stops=true},{number=2,name="Other",stops=true}},
 [2]={{number=1,name="Pause",stops=true}},
 [3]={{number=1,name="U Rathaus Spandau",stops=true},{number=2,name="Nervenklinik",stops=true}},
 [4]={{number=1,name="U Ruhleben",stops=true},{number=2,name="Nervenklinik",stops=true}},
 [5]={{number=1,name="Unserved approach point",stops=false},{number=2,name="Nervenklinik",stops=true},
       {number=9,name="U Rathaus Spandau",stops=true}}}
function apply(n)
 local t=trips[n]
 current={line=expected.Line,tour=expected.Tour,trip=n,trip_name=t.name,
  departure=t.departure,arrival=t.arrival,terminus=t.terminus}
end
apply(1)
function emit(from,text) handlers.lan_message(from,text) end
omsi={
 json={decode=function() return expected end,encode=function(t) status=t;return "json" end},
 every=function(_,fn) tick=fn end,on=function(e,fn) handlers[e]=fn end,
 has_vehicle=function() return vehicle end,
 log=function(s) logs[#logs+1]=s end,message=function() end,
 map={info=function() return {path="C:/OMSI/maps/Any layout/global.cfg"} end},
 bus={file=function() return "C:/OMSI/Vehicles/Private/model.bus" end},
 duty={get=function() return current end,trips=function() return trips end,
  trip_stops=function(n) return stops[n] end,
  start=function(_,_,n) assert(not lan,"LAN clients must use native host dispatch");apply(n);return true end},
 lan={active=function() return lan end,send=function(id,s) assert(id==1);sends[#sends+1]=s;return true end},
 files={write=function() return true end}}
'''

def runtime(extra=""):
    lua=LuaRuntime(unpack_returned_tuples=True)
    lua.execute(BOOT+extra)
    lua.execute(SCRIPT)
    return lua

def check(expr,lua):
    assert lua.eval(expr),expr

def handshake(lua):
    lua.execute('tick(); emit(1,"ready"); for i=1,22 do tick() end')
    check('sends[#sends]=="take 4974938_1234abcd 130%20&%20N30 Mo-Fr%203 4 1"',lua)

lua=runtime()
handshake(lua)
lua.execute('apply(5); tick()')
check('status.State~="ready"',lua)  # HTTP acceptance / duty mutation alone is insufficient.
lua.execute('emit(3,"done "..expected.ID);tick()')
check('status.State~="ready"',lua)  # Another player cannot acknowledge the host request.
lua.execute('emit(1,"done "..expected.ID);tick()')
check('status.State=="ready" and status.Trip==5 and status.Departure==2280',lua)
check('#logs==1 and logs[1]:find("[BBS trip confirmed]",1,true)~=nil',lua)
print("PASS: real BCS 00:38 vs old 00:04 regression; matching direction; native acknowledgment; readiness gate")

for shift in (-86400,86400,172800):
    lua=runtime(f'for _,t in ipairs(trips) do t.departure=t.departure+({shift});t.arrival=t.arrival+({shift}) end;lan=false')
    lua.execute('tick();tick()')
    check('status.State=="ready" and status.Trip==5',lua)
print("PASS: yesterday's night tour, 24h+ and 48h+ metadata")

lua=runtime('trips[6]={number=6,name="duplicate",departure=2280,arrival=3360,terminus="U Rathaus Spandau"};stops[6]=stops[5]')
lua.execute('tick()')
check('status.State=="error" and status.Error=="ambiguous" and #sends==0',lua)
lua=runtime('trips[5].arrival=3600')
lua.execute('tick()')
check('status.State=="error" and status.Error=="notfound" and #sends==0',lua)
print("PASS: duplicate departures and contradictory arrival rejected without choosing a different trip")

lua=runtime('vehicle=false')
lua.execute('for i=1,100 do tick() end')
check('status==nil and #sends==0 and #logs==0',lua)
lua=runtime()
lua.execute('for i=1,305 do tick() end')
check('status.State=="error" and status.Error=="host"',lua)
print("PASS: no vehicle, old host / missing handshake cannot emit readiness")

lua=runtime('expected.Route="Terminal A - Norte - Terminal B - Sul";stops[5][2].name="Terminal A - Norte";stops[5][3].name="Terminal B - Sul";lan=false')
lua.execute('tick();tick()')
check('status.State=="ready" and status.Trip==5',lua)
print("PASS: hyphens inside stop names and arbitrary trip names/layouts")

lua=runtime('expected.Map="maps/Another map/global.cfg"')
lua.execute('tick()')
check('status.State=="error" and status.Error=="vehicle" and #sends==0',lua)
print("PASS: loaded map/bus identity checked")
