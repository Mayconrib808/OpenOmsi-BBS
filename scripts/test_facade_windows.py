#!/usr/bin/env python3
"""Exercise the actual x86 facade, BCS log encodings and GUI-thread handshake."""
from __future__ import annotations
import ctypes
from ctypes import wintypes
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
ACK = "INFO 23:17:35:264 OMSI Startmenue: Karte laedt, 500 ms nach dem Erscheinen\r\n"
TRANSITION = "-> WAIT_TIMETABLE"

def wait_for(predicate, process, timeout=8):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise AssertionError(f"Facade exited prematurely: {process.returncode}")
        if predicate():
            return
        time.sleep(0.01)
    raise AssertionError("Facade handshake timed out")

def windows(pid):
    user = ctypes.WinDLL("user32", use_last_error=True)
    user.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
    user.GetClassNameW.argtypes = [wintypes.HWND, wintypes.LPWSTR, ctypes.c_int]
    user.IsWindowVisible.argtypes = [wintypes.HWND]
    callback_type = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
    user.EnumWindows.argtypes = [callback_type, wintypes.LPARAM]
    found = {}
    @callback_type
    def visit(hwnd, _):
        owner = wintypes.DWORD()
        user.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value == pid:
            name = ctypes.create_unicode_buffer(256)
            user.GetClassNameW(hwnd, name, len(name))
            found[name.value] = bool(user.IsWindowVisible(hwnd))
        return True
    user.EnumWindows(visit, 0)
    return found

def scenario(executable, encoding, ack_first):
    with tempfile.TemporaryDirectory(prefix="bcs-facade-handshake-") as temp:
        folder = Path(temp)
        exe = folder / "Omsi.exe"
        shutil.copy2(executable, exe)
        root = folder / "root"
        root.mkdir()
        native = root / "bbs.odr"
        opened = folder / "driver.odr"
        data = (ROOT / "source/testdata/bbs-before.odr").read_bytes()
        # Zero ratings make both field orders identical; synthetic data only.
        data = data.replace(b"0.25", b"0")
        native.write_bytes(data)
        opened.write_bytes(data)
        bcs = root / "bcs.txt"
        baseline = "INFO Schicht ID: 1001\r\n"
        def encode(text):
            bom = {"utf-16-be": b"\xfe\xff", "utf-16-le": b"\xff\xfe"}.get(encoding, b"")
            return bom + text.encode(encoding)
        bcs.write_bytes(encode(baseline))
        log = folder / "compat-facade-v1.1.3.log"
        ready = folder / "openomsi-ready-v1.1.3.flag"
        read = lambda: log.read_text(errors="replace") if log.exists() else ""
        process = subprocess.Popen([str(exe), "0", str(root), "maps/Sample/global.cfg", "522", "1", str(bcs), str(opened), str(native), "1001"])
        try:
            # The earliest published window is enough to let BCS answer. In
            # particular this does not wait until the log watcher is armed.
            wait_for(lambda: "Tform_start" in windows(process.pid), process)
            if ack_first:
                bcs.write_bytes(encode(baseline + ACK))
            else:
                ready.write_text("ready\r\n")
            wait_for(lambda: ("BCS start-menu acknowledged" if ack_first else "openOMSI ready=true") in read(), process)
            assert TRANSITION not in read(), "Map finished before both sides acknowledged"
            if ack_first:
                ready.write_text("ready\r\n")
            else:
                bcs.write_bytes(encode(baseline + ACK))
            wait_for(lambda: TRANSITION in read(), process)
            wait_for(lambda: windows(process.pid).get("Tform_settt") is True and windows(process.pid).get("Tform_start") is False, process)
            assert read().count(TRANSITION) == 1, "Duplicate GUI transition"
            print(f"Native facade passed: {encoding}, ack_first={ack_first}", flush=True)
        finally:
            process.kill()
            process.wait(timeout=5)

if __name__ == "__main__":
    if sys.platform != "win32":
        raise SystemExit("This regression requires native Windows")
    for encoding, ack_first in [("utf-16-be", True), ("utf-16-le", False), ("utf-8", True)]:
        scenario(Path(sys.argv[1]).resolve(), encoding, ack_first)
