"""Exercise the actual dedicated P15 executable's readiness and clock HTTP path."""
import argparse, json, os, pathlib, socket, subprocess, tempfile, time, urllib.request

def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]

def main():
    p=argparse.ArgumentParser();p.add_argument('--exe',type=pathlib.Path,required=True);p.add_argument('--out',type=pathlib.Path,required=True);a=p.parse_args()
    a.out.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='coop12-clock-') as folder:
        root=pathlib.Path(folder);maps=root/'maps/Test';maps.mkdir(parents=True)
        # Synthetic layout for the normal startup validator; no original game
        # assets or vehicle models are used by this empty-map regression.
        for name in ['Vehicles/MAN_SD200','Vehicles/MAN_SD202','Vehicles/MAN_NL_NG','Sceneryobjects','Splines','Texture','Fonts','Humans','Weather','Inputs']:
            (root/name).mkdir(parents=True,exist_ok=True)
        for name in ['Omsi.exe','maps/Grundorf/global.cfg','maps/Berlin-Spandau/global.cfg','envir.cfg']:
            path=root/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text('')
        (maps/'global.cfg').write_text('[name]\nClock startup regression\n[map]\n0\n0\ntile_0_0.map\n')
        (maps/'tile_0_0.map').write_text('[terrain]\n')
        web,udp=port(),port()
        cfg=root/'server.cfg'
        cfg.write_text(f'name = Clock regression\nmap = maps/Test/global.cfg\ndate = 2026-10-24\ntime = 05:50:00\ntraffic = 0\ntimetable = 0\npassengers = 0\ntunnel = 0\nmetar_sync = 0\nreal_time = 0\nport = {udp}\nweb_port = {web}\nadmin_password = ci-clock-only\nadmin_web = off\nmax_players = 4\nshare_maps = 0\nvoice = 0\npublic_server = 0\n')
        log=a.out/'native-clock-runtime.log'
        with log.open('w') as output:
            process=subprocess.Popen([str(a.exe.resolve()),'--root',str(root),'--server',str(cfg),'--size','64x36'],cwd=a.exe.resolve().parent,env=dict(os.environ,USERPROFILE=str(root),HOME=str(root),RUST_LOG='info'),stdout=output,stderr=subprocess.STDOUT)
            base=f'http://127.0.0.1:{web}'
            def status():
                with urllib.request.urlopen(base+'/status',timeout=2) as r:return json.load(r)
            try:
                until=time.monotonic()+90;ready=None
                while time.monotonic()<until:
                    if process.poll() is not None:raise RuntimeError('Native server exited: '+log.read_text(errors='replace')[-5000:])
                    try:
                        s=status()
                        if isinstance(s.get('world'),dict):ready=s;break
                    except (OSError,ValueError):pass
                    time.sleep(.1)
                assert ready is not None,'Native simulation never published readiness'
                assert ready['protocol']==15,ready
                corrections=[]
                for target,want in [(21600,'06:00'),(21640,'06:00'),(21670,'06:01')]:
                    req=urllib.request.Request(base+'/admin',data=f'speed 1\nclock {target}\n'.encode(),headers={'X-Admin-Password':'ci-clock-only','Content-Type':'text/plain'},method='POST')
                    started=time.monotonic()
                    with urllib.request.urlopen(req,timeout=3) as r:assert r.status==202
                    until=time.monotonic()+4;observed=None
                    while time.monotonic()<until:
                        observed=status()['time']
                        if observed.startswith(want):break
                        time.sleep(.05)
                    assert observed.startswith(want),(target,observed)
                    corrections.append(dict(target_seconds=target,observed=observed,latency_seconds=round(time.monotonic()-started,3)))
                report=dict(passed=True,protocol=15,readiness_after_initial_simulation=True,clock_updates=corrections)
            finally:
                process.terminate()
                try:process.wait(timeout=5)
                except subprocess.TimeoutExpired:process.kill();process.wait(timeout=5)
    runtime=log.read_text(errors='replace')
    assert 'BBS server ready:' in runtime
    assert runtime.count('BBS server clock confirmed:')>=3, runtime[-4000:]
    (a.out/'native-clock-regression.json').write_text(json.dumps(report,indent=2)+'\n')
    print('NATIVE_CLOCK='+json.dumps(report))

if __name__=='__main__':main()
