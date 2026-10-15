-- Per-launch Bridge companion. PeDePe's bbs.lua remains unchanged.
local expected = omsi.json.decode(__BCS_SELECTION_JSON__)
local phase, problem, selected = "loading", nil, nil
local elapsed, resend, issued, host_ready, host_ack = 0, 0, false, false, false
local function key(s)
  return tostring(s or ""):match("^%s*(.-)%s*$"):lower():gsub("\\", "/"):gsub("%s+", " ")
end
local function name(s) return key(s) end
local function minute(t)
  if type(t) ~= "number" or t ~= t or math.abs(t) == math.huge then return nil end
  return math.floor(t / 60) % 1440
end
local function translated(pt, en, de)
  local lang = key(expected.Language):sub(1,2)
  if lang == "pt" then return pt elseif lang == "de" then return de else return en end
end
local function fail(reason)
  if phase == "error" or phase == "ready" then return end
  phase, problem = "error", reason
  omsi.log("BCS trip verification failed: " .. reason)
  omsi.message(translated("BCS: não foi possível confirmar a viagem escolhida. Consulte o diagnóstico no Setup.",
    "BCS: the selected trip could not be confirmed. Check Setup diagnostics.",
    "BBS: Die gewählte Fahrt konnte nicht bestätigt werden. Diagnose im Setup prüfen."), 8)
end
local function vehicle_matches()
  local function asset(actual,wanted)
    actual,wanted=key(actual),key(wanted)
    return wanted~="" and (actual==wanted or actual:sub(-#wanted-1)=="/"..wanted)
  end
  return asset((omsi.map.info() or {}).path,expected.Map) and asset(omsi.bus.file(),expected.Bus)
end
local function endpoints(number)
  local first,last
  for _,row in ipairs(omsi.duty.trip_stops(number) or {}) do
    if row.stops~=false then
      if not first or row.number<first.number then first=row end
      if not last or row.number>last.number then last=row end
    end
  end
  return first,last
end
local function route_matches(trip,first,last)
  -- Compare the complete BCS route so hyphens inside stop names do not
  -- require splitting or guessing either endpoint. Startpunkt is a map entry
  -- label and may differ from the actual first public stop.
  local wanted=key(expected.Route)
  return key(first.name).." - "..key(last.name)==wanted or
    key(first.name).." - "..key(trip.terminus)==wanted
end
local function selection_diagnostics(d)
  -- Preserve the actual API evidence in the exported logs before cleanup.
  -- Limit the snapshot to twelve trips, preferring the selected departure.
  omsi.log("BCS expected selection: "..omsi.json.encode({Line=expected.Line,Tour=expected.Tour,
    DepartureMinute=expected.Start,ArrivalMinute=expected.End,Route=expected.Route,StartPoint=expected.StartPoint}))
  omsi.log("BCS loaded duty: "..omsi.json.encode(d or {}))
  local trips=omsi.duty.trips() or {}
  local shown=0
  for priority=1,2 do
    for _,trip in ipairs(trips) do
      local same=minute(trip.departure)==expected.Start%1440
      if shown<12 and ((priority==1 and same) or (priority==2 and not same)) then
        local first,last=endpoints(trip.number)
        omsi.log("BCS resolved trip: "..omsi.json.encode({Trip=trip.number,Name=trip.name,
          Departure=trip.departure,Arrival=trip.arrival,Terminus=trip.terminus,
          FirstStop=first and first.name,LastStop=last and last.name}))
        shown=shown+1
      end
    end
  end
end
local function choose()
  local d=omsi.duty.get()
  if not d or key(d.line)~=key(expected.Line) or key(d.tour)~=key(expected.Tour) then
    selection_diagnostics(d);return nil,"notfound"
  end
  local count,result=0,nil
  for _,trip in ipairs(omsi.duty.trips() or {}) do
    if minute(trip.departure)==expected.Start%1440 then
      local first,last=endpoints(trip.number)
      -- Resolved simulator metadata covers base TTData and active Chrono.
      if first and last and route_matches(trip,first,last) and
         minute(trip.arrival)==expected.End%1440 then
        count,result=count+1,{trip=trip.number,name=trip.name,departure=trip.departure,
          first=first.number,origin=first.name,terminus=last.name}
      end
    end
  end
  if count==0 then selection_diagnostics(d);return nil,"notfound" end
  if count>1 then selection_diagnostics(d);return nil,"ambiguous" end
  return result
end
local function matches()
  local d=omsi.duty.get()
  if not d or not selected or key(d.line)~=key(expected.Line) or key(d.tour)~=key(expected.Tour) or
     d.trip~=selected.trip or key(d.trip_name)~=key(selected.name) or
     minute(d.departure)~=expected.Start%1440 or minute(d.arrival)~=expected.End%1440 then return false end
  local first,last=endpoints(d.trip)
  return first and last and route_matches(d,first,last)
end
local function publish()
  local d=omsi.duty.get() or {}
  omsi.files.write("status.json",omsi.json.encode({Token=expected.Token,ID=expected.ID,
    State=phase,Error=problem,Line=d.line,Tour=d.tour,Trip=d.trip,
    TripName=d.trip_name,Departure=d.departure}))
end
local function dispatch()
  if omsi.lan.active() then
    if not host_ready then omsi.lan.send(1,"hello");return end
    local function esc(s) return s:gsub("%%","%%25"):gsub(" ","%%20") end
    local text="take "..expected.ID.." "..esc(expected.Line).." "..esc(expected.Tour)..
      " "..(selected.trip-1).." "..(selected.first-1)
    if #text>120 then fail("namelimit");return end
    local ok=omsi.lan.send(1,text)
    if ok then issued=true else fail("host") end
  else
    local ok=omsi.duty.start(expected.Line,expected.Tour,selected.trip,1)
    if ok then issued,host_ack=true,true else fail("dispatch") end
  end
end
omsi.on("lan_message",function(from,text)
  if from~=1 then return end
  if text=="ready" then host_ready=true end
  if not issued then return end
  if text=="done "..expected.ID then host_ack=true end
  local prefix="no "..expected.ID.." "
  if text:sub(1,#prefix)==prefix then
    if text:find("Host update required",1,true) then fail("host") else fail("dispatch") end
  end
end)
omsi.every(0.1,function()
  if phase=="ready" or phase=="error" or not omsi.has_vehicle() then return end
  elapsed,resend=elapsed+0.1,resend-0.1
  if not vehicle_matches() then fail("vehicle");publish();return end
  if not selected then
    local why
    selected,why=choose()
    if not selected then fail(why);publish();return end
    phase="checking"
    if matches() then issued,host_ack=true,true end
  end
  if host_ack and matches() then
    phase="ready";publish()
    omsi.log("[BBS trip confirmed] spawned at selected BCS duty ready: "..
      expected.Line.." / "..expected.Tour.." / "..selected.name)
  elseif elapsed>=30 then fail(host_ready and "timeout" or "host");publish()
  elseif resend<=0 and not host_ack then dispatch();resend=2;publish()
  else publish() end
end)
