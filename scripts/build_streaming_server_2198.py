#!/usr/bin/env python3
"""Produce the v0.3.39/protocol-14 native dynamic host used with Bridge 2.1.9.8.

This builds a DIFFERENT dedicated openOMSI runtime, not the 2.1.x Bridge itself.
The source is pinned to the exact signed Git tag commit. Windows CI compiles,
smoke-tests and packages it; no fake radius=2 workaround is used.
"""
from __future__ import annotations
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import urllib.request
import zipfile

COMMIT = "9eaf95d4caafabe041edcdfc37931b5259c30c8a"
VERSION = "0.3.39-bbs-stream.1"
REPO = "openOMSI-org/openOMSI"

def patch_file(root: Path, relative: str, changes: list[tuple[str, str]]) -> None:
    path = root / relative
    src = path.read_text(encoding="utf-8")
    for before, after in changes:
        count = src.count(before)
        if count != 1:
            raise RuntimeError(f"{relative}: expected unique anchor, got {count}: {before[:80]!r}")
        src = src.replace(before, after, 1)
    path.write_text(src, encoding="utf-8")
    print(f"patched {relative}", flush=True)

def prepare(directory: Path) -> None:
    url = f"https://codeload.github.com/{REPO}/zip/{COMMIT}"
    with urllib.request.urlopen(url, timeout=180) as response:
        payload = response.read()
    print("upstream source sha256", hashlib.sha256(payload).hexdigest(), flush=True)
    with zipfile.ZipFile(io.BytesIO(payload)) as archive:
        members = archive.namelist()
        prefix = members[0].split("/")[0] + "/"
        if not prefix.startswith("openOMSI-"):
            raise RuntimeError(f"unexpected source prefix: {prefix}")
        for member in members:
            if not member.startswith(prefix) or member.endswith("/"):
                continue
            relative = Path(member[len(prefix):])
            if ".." in relative.parts or relative.is_absolute():
                raise RuntimeError(f"unsafe file: {member}")
            target = directory / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(archive.read(member))
    patch_file(directory, "crates/omsi-app/src/offscreen.rs", [
        ("    world: World,\n", "    world: Arc<World>,\n    /// Streaming regions around every connected player on a managed dedicated server.\n    streamer: Option<tiles::Streamer>,\n"),
    ])
    patch_file(directory, "crates/omsi-app/src/offscreen/setup.rs", [
        (
            "        let (world, camera) = lan::answering_while(&mut lan_off, args.bus.as_deref(), || load_world(args, &renderer, &mut scene))?;\n",
            """        // The ordinary game's window streams tiles, but upstream --server loads
        // the entire map (radius=0 becomes radius=99). Use exactly the same
        // streamer on the dedicated host instead of reducing a FIXED radius:
        // a player spawning on the opposite side of a map must get roads,
        // AI, stops and collision there too.
        let dynamic_server = args.server.is_some() && args.radius == Some(99);
        let (world, camera, streamer) = if dynamic_server {
            let (w, cam, entry) = open_world(args)?;
            let w = Arc::new(w);
            let centers = [entry];
            let mut stream = tiles::Streamer::new(
                w.clone(), &centers, 1350.0, 700.0, crate::release_free_memory,
            );
            let began = Instant::now();
            lan::answering_while(&mut lan_off, args.bus.as_deref(), || -> Result<()> {
                // Bootstrap only the first neighborhood. Additional neighborhoods
                // are loaded as players join/drive; never wait for every tile.
                while stream.initial_progress().is_some() {
                    stream.update(&renderer, &mut scene, &centers,
                        std::time::Duration::from_millis(10), None);
                    std::thread::sleep(std::time::Duration::from_millis(3));
                }
                Ok(())
            })?;
            log::info!("BBS dynamic server: initial region ready after {:.2} s, {} tiles (entire map NOT preloaded)",
                began.elapsed().as_secs_f32(), w.loaded_tiles().len());
            (w, cam, Some(stream))
        } else {
            let (w, cam) = lan::answering_while(&mut lan_off, args.bus.as_deref(),
                || load_world(args, &renderer, &mut scene))?;
            (Arc::new(w), cam, None)
        };
""",
        ),
        (
            "            scene,\n            world,\n            camera,\n",
            "            scene,\n            world,\n            streamer,\n            camera,\n",
        ),
    ])
    patch_file(directory, "crates/omsi-app/src/offscreen/step.rs", [
        (
            "        Ok(crate::frame_stages::run(&mut step, stages::OFFSCREEN_STEP)? == crate::frame_stages::Flow::Go)\\n",
            "        let running = crate::frame_stages::run(&mut step, stages::OFFSCREEN_STEP)? == crate::frame_stages::Flow::Go;\\n        if running { self.stream_server_step(); }\\n        Ok(running)\\n",
        ),
        (
            "    /// The cabin air, the tyres' spray and the roads' wetness, frame by frame as the window's\n",
            """    /// Dedicated-server equivalent of the ordinary multiplayer host's tile
    /// streaming. Centers are the LIVE positions of all peers, not the entry
    /// point fixed at server startup. Headless renderer uploads are no-op.
    /// Called after LAN receives positions so a new player's region is queued
    /// even when the players are several kilometers apart.
    pub(super) fn stream_server_step(&mut self) {
        let Some(stream) = self.streamer.as_mut() else { return; };
        let mut centers: Vec<DVec3> = self.remotes_off.remotes.values()
            .map(|r| r.vehicle().position).collect();
        if let Some(lan) = self.lan_off.as_ref() {
            centers.extend(lan.peers().filter(|p| p.has_pose)
                .map(|p| DVec3::new(p.pose.x, p.pose.y, p.pose.z)));
        }
        if let Some(p) = self.player.as_ref() { centers.push(p.vehicle.position); }
        if centers.is_empty() { centers.push(self.center); }
        let changed = stream.update(&self.renderer, &mut self.scene, &centers,
            std::time::Duration::from_millis(6), None);
        if changed {
            if let Some(t) = self.traffic.as_mut() {
                // Without this the streamed tiles have scenery but no usable
                // road lanes, lights, AI traffic or timetable routes.
                t.add_tiles(&self.world.traffic_inbox);
            } else {
                let _ = self.world.traffic_inbox.take();
            }
        }
    }

    /// The cabin air, the tyres' spray and the roads' wetness, frame by frame as the window's
""",
        ),
    ])
    print("dynamic streaming source prepared", flush=True)

def build(source: Path, output: Path) -> None:
    env = dict(os.environ, OPENOMSI_VERSION=VERSION)
    subprocess.run(["cargo", "build", "--locked", "--release", "-p", "omsi-app"],
        cwd=source, env=env, check=True)
    binary = source / "target/release/openomsi.exe"
    if not binary.is_file():
        raise FileNotFoundError(binary)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copy2(binary, output / "openomsi.exe")
    for name in ["steam_api64.dll"]:
        src = source / "target/release" / name
        if src.is_file(): shutil.copy2(src, output / name)
    v = subprocess.run([str((output / "openomsi.exe").resolve()), "--version"],
        capture_output=True, text=True, timeout=20, check=True)
    if VERSION not in v.stdout + v.stderr:
        raise RuntimeError("version mismatch: "+v.stdout+v.stderr)
    h = subprocess.run([str((output / "openomsi.exe").resolve()), "--help"],
        capture_output=True, text=True, timeout=20, check=True)
    if "--server" not in h.stdout+h.stderr: raise RuntimeError("server capability missing")
    shutil.copy2(source / "LICENSE", output / "OpenOMSI-LICENSE.txt")
    info = {"bridge_line":"2.1.9.8","native_version": VERSION,
            "source_commit":COMMIT,"protocol":14,
            "runtime_files":{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in output.iterdir() if p.is_file()}}
    (output / "BBS-DYNAMIC-SERVER.json").write_text(json.dumps(info,indent=2))
    print("Native server built and version verified",flush=True)

if __name__ == "__main__":
    ap=argparse.ArgumentParser()
    ap.add_argument("--source",type=Path,required=True)
    ap.add_argument("--output",type=Path,required=True)
    ap.add_argument("--prepare-only",action="store_true")
    args=ap.parse_args()
    prepare(args.source)
    if not args.prepare_only:
        build(args.source,args.output)
