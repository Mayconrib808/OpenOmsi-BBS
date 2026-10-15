-- Private per-launch companion. PeDePe's bbs.lua remains a separate plugin.
local token = "__BRIDGE_TOKEN__"
local language = "__BRIDGE_LANGUAGE__"
local function translated(pt, en, de)
  local l = language:lower():sub(1, 2)
  if l == "pt" then return pt elseif l == "de" then return de else return en end
end
local current, phase, problem, resolved = nil, "ready", nil, nil
local elapsed, resend, prepared_age, sign_ok = 0, 0, 0, false
local sign_elapsed, sign_attempts, sign_retry_at = 0, 0, 3
local sign_expected, sign_issue = nil, nil
local host_ready = false
local issued, host_ack = false, false
local dispatch_problem = nil
local function trim(s) return tostring(s or ""):match("^%s*(.-)%s*$") end
local function key(s) return trim(s):lower():gsub("\\", "/") end
local function name(s) return key(s):gsub("[%p%s]", "") end
local function clock(t) return math.floor((tonumber(t) or -100000) / 60) % 1440 end
local function esc(s) return tostring(s):gsub("%%", "%%25"):gsub(" ", "%%20") end
local function fail(s)
  if phase == "error" then return end
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
    destination_index = index, expected_destination_index = sign_expected,
    sign_attempts = sign_attempts, sign_issue = sign_issue,
    map = (omsi.map.info() or {}).path, bus = omsi.bus.file(),
    line = d and d.line, tour = d and d.tour, trip = d and d.trip,
    trip_name = d and d.trip_name, departure = d and d.departure,
    selected_trip = resolved and resolved.trip, version = omsi.game.version(),
    host_ready = host_ready, issued = issued, host_ack = host_ack,
    dispatch_warning = dispatch_problem }
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
local function desired_sign()
  local d = omsi.duty.get()
  if not d then return nil, "The current timetable is unavailable" end
  local found, count = nil, 0
  local duty_name, selected_name = name(d.terminus), name(current.terminus)
  for _, row in ipairs(omsi.bus.destinations()) do
    local row_name = name(row.name)
    if row_name ~= "" and ((duty_name ~= "" and row_name == duty_name) or
       (selected_name ~= "" and row_name == selected_name)) then
      found, count = row.index, count + 1
    end
  end
  if count == 0 then return nil, "No matching destination in the loaded HOF" end
  if count ~= 1 then return nil, "More than one matching destination in the loaded HOF" end
  if type(found) ~= "number" or found < 1 or found ~= math.floor(found) then
    return nil, "The HOF destination index is unavailable"
  end
  return found
end
local function sign_matches()
  sign_expected, sign_issue = desired_sign()
  local _, actual = omsi.bus.destination()
  return sign_expected ~= nil and actual == sign_expected
end
local function sign_confirmed()
  phase, problem, sign_issue = "applied", nil, nil
  omsi.log("BCS next trip: destination confirmed " .. current.id .. " / HOF " .. sign_expected)
  omsi.message(translated("BCS: nova tabela e destino confirmados.",
    "BCS: new timetable and destination confirmed.",
    "BBS: neuer Fahrplan und neues Ziel bestätigt."), 5)
end
local function confirm_sign()
  sign_elapsed = sign_elapsed + 0.25
  sign_ok = sign_matches()
  if sign_ok then sign_confirmed(); return end
  -- Give the native IBIS time to finish first. A setter accepting a request is
  -- not confirmation: only a later reading of the actual display can confirm it.
  -- Never guess among duplicate HOF rows or change another bus/the timetable.
  if sign_expected and sign_attempts < 2 and sign_elapsed >= sign_retry_at then
    if math.abs(omsi.speed() or 0) > 0.5 then
      sign_issue = "Stop the bus to correct the destination display"
    elseif type(omsi.bus.set_destination) ~= "function" then
      sign_issue = "This game does not expose destination correction"
    else
      sign_attempts = sign_attempts + 1
      sign_retry_at = sign_elapsed + 4
      local ok, accepted = pcall(omsi.bus.set_destination, sign_expected)
      sign_issue = ok and accepted == true and "Waiting for the destination display" or
        "The bus did not accept destination correction"
      omsi.log("BCS next trip: destination correction " .. current.id .. " / HOF " ..
        sign_expected .. " / attempt " .. sign_attempts .. " / " .. sign_issue)
    end
  end
  if sign_elapsed >= 15 then
    -- The new timetable is already real. Keep its BCS counters working while
    -- reporting the sign separately; this is never the confirmed/applied state.
    phase = "applied_unverified"
    problem = "Timetable confirmed; destination unconfirmed: " ..
      (sign_issue or "The displayed destination still differs")
    omsi.log("BCS next trip: " .. problem)
    omsi.message(translated("BCS: tabela atualizada, mas o destino não foi confirmado. Confira o letreiro; o registro da viagem continua ativo.",
      "BCS: timetable updated, but the destination is unconfirmed. Check the display; trip recording remains active.",
      "BBS: Fahrplan aktualisiert, Ziel aber nicht bestätigt. Prüfe die Zielanzeige; die Fahrtaufzeichnung bleibt aktiv."), 10)
  end
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
    if not ok then fail(why) else
      if not issued then omsi.log("BCS next trip: requested " .. current.id .. " / " .. current.line .. " / " .. current.tour) end
      issued = true
    end
  else
    local ok, why = omsi.duty.start(current.line, current.tour, resolved.trip, 1)
    if not ok then fail(why) else issued, host_ack = true, true end
  end
end
-- The game invokes registered handlers or on_<event>, never a bare
-- function named after the event. Both ready and done must reach this handler.
omsi.on("lan_message", function(from, text)
  if from ~= 1 then return end
  if text == "ready" and not host_ready then
    host_ready = true
    omsi.log("BCS next trip: native host ready")
  end
  if not current or phase ~= "applying" or not issued then return end
  if text == "done " .. current.id and not host_ack then
    host_ack, dispatch_problem = true, nil
    omsi.log("BCS next trip: native client confirmed " .. current.id)
  end
  local prefix = "no " .. current.id .. " "
  if not host_ack and text:sub(1, #prefix) == prefix then
    local reason = text:sub(#prefix + 1)
    -- host.2 expects HTTP 200, but the native admin endpoint returns 202.
    -- A submission error is not the client's duty-no: wait for its actual
    -- duty-ok and verify duty.get(), in either LAN message order.
    if reason == "Native dispatch rejected" or reason == "Native dispatch unavailable" then
      if dispatch_problem ~= reason then
        omsi.log("BCS next trip: waiting for native confirmation after " .. reason)
      end
      dispatch_problem = reason
    else fail(text) end
  end
end)
omsi.every(0.25, function()
  local raw = omsi.files.read("command.json")
  local c = raw and omsi.json.decode(raw)
  if c and c.token == token and type(c.id) == "string" then
    if c.phase == "cancel" and current and c.id == current.id then fail("BCS selection changed") end
    if c.phase == "prepare" and (not current or current.id ~= c.id) then
      current, resolved, problem, sign_ok = c, nil, nil, false
      phase = "preparing"
      elapsed, resend, prepared_age = 0, 0, 0
      sign_elapsed, sign_attempts, sign_retry_at = 0, 0, 3
      sign_expected, sign_issue = nil, nil
      issued, host_ack = false, false
      dispatch_problem = nil
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
      confirm_sign()
    elseif elapsed >= 20 then
      if omsi.lan.active() and not host_ready then fail("The native host did not answer the next-trip handshake")
      elseif not host_ack then fail("The native client did not confirm the requested duty" ..
        (dispatch_problem and ": " .. dispatch_problem or ""))
      else fail("The loaded timetable does not match the selected BCS trip") end
    elseif resend <= 0 and not host_ack then dispatch(); resend = 2 end
  elseif phase == "applied_unverified" then
    -- A driver may correct the sign by hand after the warning. Read it without
    -- issuing further corrections or repeating the BCS trip handoff.
    if valid_vehicle(current) and matches() then
      sign_ok = sign_matches()
      if sign_ok then sign_confirmed() end
    end
  end
  omsi.files.write("status.json", omsi.json.encode(state()))
end)
