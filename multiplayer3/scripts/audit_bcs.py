"""Inspect a locally supplied official BCS JAR without modifying or redistributing it.

Optional Lua simulation executes the original plugin with a mocked simulator API.
It checks interoperability, not a live BCS trip or PeDePe endorsement.
"""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
import struct
import zipfile

EXPECTED_JAR = "bfbf51cbb9033b130ecd4bebd41e1d04b926d8ef6c578737f1363a3146b196c6"

def class_strings(data: bytes) -> set[str]:
    if data[:4] != b"\xca\xfe\xba\xbe":
        raise ValueError("Invalid Java class")
    pos, index = 10, 1
    count = struct.unpack_from(">H", data, 8)[0]
    result = set()
    sizes = {3:4, 4:4, 5:8, 6:8, 7:2, 8:2, 9:4, 10:4, 11:4, 12:4, 15:3, 16:2, 17:4, 18:4, 19:2, 20:2}
    while index < count:
        tag = data[pos]
        pos += 1
        if tag == 1:
            size = struct.unpack_from(">H", data, pos)[0]
            pos += 2
            result.add(data[pos:pos+size].decode("utf-8", errors="replace"))
            pos += size
        else:
            pos += sizes[tag]
            if tag in (5, 6):
                index += 1
        index += 1
    return result

def simulate_plugin(source: str) -> list[str]:
    from lupa import LuaRuntime
    lua = LuaRuntime(unpack_returned_tuples=True)
    lua.execute('''
      callbacks, events, starts, sent, files = {}, {}, {}, {}, {}
      vehicle, current_trip = false, ""
      omsi = {
        every=function(interval, fn) callbacks[interval]=fn end,
        on=function(event, fn) events[event]=fn end,
        has_vehicle=function() return vehicle end,
        log=function() end,
        var=function() return 0 end, str=function() return "" end,
        set_var=function() end, speed=function() return 0 end,
        time=function() return 3600 end, clock=function() return 3600 end,
        sys=function() return 0 end,
        info=function() return {line="810",tour="Duty 1",trip_name=current_trip} end,
        info_value=function(name) if name=="trip_name" then return current_trip end end,
        send=function(port, text) sent[#sent+1]={port=port,text=text} end,
        files={dir=function() return "mock-data-folder" end, read=function(name) return files[name] end},
        game={has=function() return true end, version=function() return "0.2.50" end, stats=function() return {} end},
        duty={
          start=function(line,tour,trip) starts[#starts+1]={line=line,tour=tour,trip=trip}; return true end,
          trips=function() return {
            {number=1,name="Pause",departure=3600},
            {number=2,name="Centro",departure=3600},
            {number=3,name="Terminal",departure=7200}
          } end
        }
      }
      -- A stale command from the preceding game must never be replayed.
      files["befehl.txt"]="old;810;Duty 1;60;Centro"
    ''')
    lua.execute(source)
    lua.execute('''
      callbacks[0.5]()
      assert(#starts==0, "replayed an old command")
      files["befehl.txt"]="new;810;Duty 1;60;Centro"
      callbacks[0.5]()
      assert(#starts==0, "started a duty without a real player vehicle")
      vehicle=true
      callbacks[0.5](); callbacks[0.5]()
      assert(#starts==2 and starts[2].trip==2, "selected the pause instead of the BCS trip")
      callbacks[0.5](); assert(#starts==2, "executed a command twice")
      files["befehl.txt"]="next;810;Duty 1;120;Terminal"
      callbacks[0.5](); callbacks[0.5]()
      assert(#starts==4 and starts[4].trip==3, "next trip was not selected")
      callbacks[0.1]()
      assert(sent[#sent].port==4041, "wrong BCS telemetry port")
      local fields={}
      for value in (sent[#sent].text.."#####"):gmatch("(.-)#####") do fields[#fields+1]=value end
      assert(#fields==62, "BCS telemetry field layout changed")
      assert(fields[59]=="next" and fields[60]=="ok", "missing command acknowledgment")
    ''')
    return ["stale_command_ignored", "real_vehicle_required", "pause_not_selected", "command_executed_once", "next_trip_without_restart", "62_field_udp_telemetry_and_ack"]

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--jar", type=Path, required=True)
    ap.add_argument("--simulate", action="store_true")
    ap.add_argument("--output", type=Path)
    args = ap.parse_args()
    before = args.jar.read_bytes()
    digest = hashlib.sha256(before).hexdigest()
    if digest != EXPECTED_JAR:
        raise SystemExit("This audit is pinned to the supplied official 5.0.2.0 build")
    with zipfile.ZipFile(args.jar) as archive:
        launch = archive.read("system/D.class")
        vendor = archive.read("system/m.class")
        plugin = archive.read("res/openomsi/bbs.lua")
    strings = class_strings(launch)
    required = {"--schedule", "--situation", "--line", "--tour", "--trip", "--driver", "game.log", "] spawned at ", "befehl.txt"}
    if not required <= strings or "5.0.2.0" not in class_strings(vendor):
        raise SystemExit("BCS native launch or command contract changed")
    result = {
        "bcs_version":"5.0.2.0", "jar_sha256":digest,
        "native_launch_class_sha256":hashlib.sha256(launch).hexdigest(),
        "native_lua_sha256":hashlib.sha256(plugin).hexdigest(),
        "launch_contract":"passed", "next_trip_interface":"native BCS command file and plugin acknowledgment",
        "jar_modified":False, "vendor_code_distributed":False,
        "live_gameplay_test":"requires Windows BCS/openOMSI and two PCs",
        "lua_simulation":simulate_plugin(plugin.decode("utf-8")) if args.simulate else "not run"
    }
    assert args.jar.read_bytes() == before
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, indent=2)+"\n", encoding="utf-8")
    print(json.dumps(result, indent=2))

if __name__ == "__main__":
    main()
