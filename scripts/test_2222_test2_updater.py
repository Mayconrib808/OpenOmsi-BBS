"""Exercise the real Windows updater source list, process handoff and file locks."""
import os
from pathlib import Path
import subprocess
import build_bridge_update as updater

def main():
    payload=updater.SOURCE/'bridge_update_payload.zip'
    assert not payload.exists()
    payload.write_bytes(b'test-only embedded payload; never distributed')
    try:
        env=dict(os.environ,GO111MODULE='off',CGO_ENABLED='0',GOOS='windows',GOARCH='386',GO386='sse2',GOTOOLCHAIN='local')
        tests=['bridge_update_ready_test.go','bridge_update_setup_test.go','bridge_update_windows_test.go','update_process_windows_test.go']
        subprocess.run(['gofmt','-w',*updater.UPDATER_SOURCES,*tests],cwd=updater.SOURCE,env=env,check=True)
        subprocess.run(['go','test','-count=1','-v',*updater.UPDATER_SOURCES,*tests],cwd=updater.SOURCE,env=env,check=True)
        subprocess.run(['go','vet','-unsafeptr=false',*updater.UPDATER_SOURCES,*tests],cwd=updater.SOURCE,env=env,check=True)
    finally:
        payload.unlink(missing_ok=True)

if __name__=='__main__':main()
