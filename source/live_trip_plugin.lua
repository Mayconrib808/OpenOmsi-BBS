-- Private per-launch companion. PeDePe's bbs.lua remains a separate plugin.
local token = "__BRIDGE_TOKEN__"
local language = "__BRIDGE_LANGUAGE__"
local function translated(pt, en, de)
  local l = language:lower():sub(1, 2)
  if l == "pt" then return pt elseif l == "de" then return de else return en end
end
local current, phase, problem, resolved = nil, "ready", nil, nil
local elapsed, resend, prepared_age, sign_ok = 0, 0, 0, false
local host_ready = false
local issued, host_ack = false, false
local function trim(s) return tostring(s or ""):match("^%s*(.-)%s*$") end
local function key(s) return trim(s):lower():gsub("\\", "/") end
local function name(s) return key(s):gsub("[%p%s]", "") end
local function clock(t) return math.floor((tonumber(t) or -100000) / 60) % 1440 end
local function esc(s) return tostring(s):gsub("%%", "%%25"):gsub(" ", "%%20") end
local function fail(s)
  phase, problem = "error", tostring(s)
  omsi.message(translated("BCS: troca de viagem não confirmada. Veja o diagnóstico no Setup.",
    "BCS: trip change not confirmed. Check the Setup diagnostics.",
    "BBS: Fahrtwechsel nicht bestätigt. Prüfe die Setup-Diagnose."), 8)
end
local function state()
  local d = omsi.duty.get()
  local destination, index = omsi.bus.destination()
  return { token = token, id = current and current.id or "", phase = phase,
    error = problem, sign_ok = sign_ok, destination = destination,
    map = (omsi.map.info() or {}).path, bus = omsi.bus.file(),
    line = d and d.line, tour = d and d.tour, trip = d and d.trip,
    trip_name = d and d.trip_name, departure = d and d.departure,
    selected_trip = resolved and resolved.trip, version = omsi.game.version() }
end
local function valid_vehicle(c)
  return key((omsi.map.info() or {}).path) == key(c.map) and
    key(omsi.bus.file()) == key(c.bus)
end
local function choose(c)
  local groups = {}
  for _, s in ipairs(omsi.timetable.stops(c.line, c.tour)) do
    local g = groups[s.trip]
    if not g then g = {trip = s.trip}; groups[s.trip] = g end
    if not g.first or s.station < g.first.station then g.first = s end
    if not g.last or s.station > g.last.station then g.last = s end
  end
  local chosen, count = nil, 0
  for _, g in pairs(groups) do
    if clock(g.first.departure) == c.minute and
       (not c.origin or c.origin == "" or name(g.first.name) == name(c.origin)) and
       (not c.terminus or c.terminus == "" or name(g.last.name) == name(c.terminus)) then
      chosen, count = g, count + 1
    end
  end
  if count ~= 1 then return nil, "Expected one matching BCS trip; found " .. count end
  return chosen
end
local function matches()
  local d = omsi.duty.get()
  return d and resolved and key(d.line) == key(current.line) and
    key(d.tour) == key(current.tour) and d.trip == resolved.trip and
    clock(d.departure) == current.minute
end
local function sign_matches()
  local d = omsi.duty.get()
  if not d then return false end
  local _, actual = omsi.bus.destination()
  local found, count = nil, 0
  for _, row in ipairs(omsi.bus.destinations()) do
    if name(row.name) == name(d.terminus) or
       (current.terminus and current.terminus ~= "" and name(row.name) == name(current.terminus)) then
      found, count = row.index, count + 1
    end
  end
  return count == 1 and actual == found
end
local function dispatch()
  local id, _, host = omsi.lan.me()
  if omsi.lan.active() and not host then
    local text = "take " .. current.id .. " " .. esc(current.line) .. " " ..
      esc(current.tour) .. " " .. (resolved.trip - 1) .. " " .. (resolved.first.station - 1)
    if #text > 120 then fail("The line/tour name exceeds the LAN request limit"); return end
    if not host_ready then
      omsi.lan.send(1, "hello")
      return
    end
    local ok, why = omsi.lan.send(1, text)
    if not ok then fail(why) else issued = true end
  else
    local ok, why = omsi.duty.start(current.line, current.tour, resolved.trip, 1)
    if not ok then fail(why) else issued, host_ack = true, true end
  end
end
function lan_message(from, text)
  if from ~= 1 then return end
  if text == "ready" then host_ready = true end
  if current and text == "done " .. current.id then host_ack = true end
  if current and text:sub(1, #("no " .. current.id .. " ")) == "no " .. current.id .. " " then
    fail(text)
  end
end
omsi.every(0.25, function()
  local raw = omsi.files.read("command.json")
  local c = raw and omsi.json.decode(raw)
  if c and c.token == token and type(c.id) == "string" then
    if c.phase == "cancel" and current and c.id == current.id then fail("BCS selection changed") end
    if c.phase == "prepare" and (not current or current.id ~= c.id) then
      current, resolved, problem, sign_ok = c, nil, nil, false
      elapsed, resend, prepared_age = 0, 0, 0
      issued, host_ack = false, false
      if not valid_vehicle(c) then fail("The loaded map/bus differs from the BCS selection")
      elseif math.abs(omsi.speed() or 0) > 0.5 then fail("Stop the bus before changing the BCS trip")
      else
        local why
        resolved, why = choose(c)
        if not resolved then fail(why)
        elseif not omsi.command("save") then fail("The game did not accept the evaluation save")
        else phase = "saving" end
      end
    end
    if c.phase == "apply" and current and c.id == current.id and phase == "prepared" then
      if not valid_vehicle(current) then fail("The loaded map/bus changed during preparation")
      else phase, elapsed, resend = "applying", 0, 0 end
    end
  end
  if phase == "saving" then
    prepared_age = prepared_age + 0.25
    if prepared_age >= 1.0 then phase = "prepared" end
  elseif phase == "applying" then
    elapsed, resend = elapsed + 0.25, resend - 0.25
    if not valid_vehicle(current) then fail("The loaded map/bus changed during application")
    elseif issued and host_ack and matches() then
      sign_ok = sign_matches()
      if sign_ok or elapsed >= 8 then
        phase = "applied"
        if not sign_ok then problem = "Timetable confirmed; the destination display could not be verified" end
        if sign_ok then
          omsi.message(translated("BCS: nova tabela e destino confirmados.",
            "BCS: new timetable and destination confirmed.",
            "BBS: neuer Fahrplan und neues Ziel bestätigt."), 5)
        else
          omsi.message(translated("BCS: tabela atualizada. Confira o letreiro do ônibus.",
            "BCS: timetable updated. Check the bus destination display.",
            "BBS: Fahrplan aktualisiert. Prüfe die Zielanzeige."), 8)
        end
      end
    elseif elapsed >= 20 then fail("No confirmed timetable change; check the host update and server version")
    elseif resend <= 0 then dispatch(); resend = 2 end
  end
  omsi.files.write("status.json", omsi.json.encode(state()))
end)
