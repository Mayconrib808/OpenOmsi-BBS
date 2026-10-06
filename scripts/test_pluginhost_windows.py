#!/usr/bin/env python3
"""Exercise the real PE32 host with an original synthetic stdcall DLL on Windows."""
from __future__ import annotations
import os
from pathlib import Path
import queue
import shutil
import struct
import subprocess
import sys
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[1]

def compiler_environment() -> dict[str, str]:
    vswhere = Path(os.environ.get("ProgramFiles(x86)", r"C:\Program Files (x86)")) / "Microsoft Visual Studio/Installer/vswhere.exe"
    location = subprocess.check_output([str(vswhere), "-latest", "-products", "*", "-requires",
        "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-property", "installationPath"], text=True).strip()
    if not location:
        raise RuntimeError("Native Windows tests require Visual Studio C++ x86 tools.")
    vcvars = Path(location) / "VC/Auxiliary/Build/vcvarsall.bat"
    # Send batch commands on stdin. Passing the embedded quoted VS path through
    # Python's Windows argument-list escaping gives cmd.exe literal backslashes.
    command = f'@echo off\ncall "{vcvars}" x86 >nul\nif errorlevel 1 exit 1\nset\nexit 0\n'
    output = subprocess.check_output(["cmd.exe", "/d", "/q"], input=command, text=True)
    env = os.environ.copy()
    for line in output.splitlines():
        if "=" in line and not line.startswith("="):
            key, value = line.split("=", 1)
            env[key.upper()] = value
    return env

class Host:
    def __init__(self, executable: Path, dll: Path, cwd: Path):
        self.error = open(cwd / "test-host-stderr.txt", "wb")
        self.process = subprocess.Popen([str(executable), str(dll)], cwd=cwd,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=self.error)
        self.bytes = queue.Queue()
        def read():
            while True:
                b = self.process.stdout.read(1)
                self.bytes.put(b)
                if not b: return
        threading.Thread(target=read, daemon=True).start()
    def send(self, b: bytes) -> None:
        self.process.stdin.write(b)
        self.process.stdin.flush()
    def read(self, n: int) -> bytes:
        result = bytearray()
        deadline = time.monotonic() + 10
        for _ in range(n):
            b = self.bytes.get(timeout=max(0.01, deadline - time.monotonic()))
            if not b: raise RuntimeError("host ended before replying")
            result.extend(b)
        return bytes(result)
    def close(self) -> None:
        if self.process.poll() is None: self.process.kill()
        self.process.wait(timeout=10)
        self.error.close()

def string(s: str) -> bytes:
    encoded = s.encode("utf-16-le")
    return struct.pack("<H", len(encoded) // 2) + encoded

def main() -> None:
    if os.name != "nt": raise RuntimeError("Run the native test on Windows.")
    built_host = Path(sys.argv[1]).resolve()
    env = compiler_environment()
    with tempfile.TemporaryDirectory(prefix="bridge-native-") as temporary:
        root = Path(temporary) / "OMSI teste ç"
        plugins = root / "Plugins"
        plugins.mkdir(parents=True)
        host = root / "OpenOMSI_BCS_PluginHost32.exe"
        shutil.copy2(built_host, host)
        (root / "Omsi.exe").write_bytes(b"synthetic original game path")
        marker = b"synthetic marker owned by the mock plugin test"
        (root / "bbs.start").write_bytes(marker)
        fixture = ROOT / "source/pluginhost/testdata/plugin_fixture.c"
        definition = ROOT / "source/pluginhost/testdata/plugin_fixture.def"
        obj = root / "fixture.obj"
        dll = plugins / "bbs.dll"
        cl = shutil.which("cl.exe", path=env["PATH"])
        linker = shutil.which("link.exe", path=env["PATH"])
        if not cl or not linker: raise RuntimeError("Visual Studio compiler/linker not found")
        subprocess.run([cl, "/nologo", "/c", "/GS-", "/TC", str(fixture), "/Fo" + str(obj)], env=env, cwd=root, check=True)
        subprocess.run([linker, "/NOLOGO", "/DLL", "/NOENTRY", "/MACHINE:X86", "/NODEFAULTLIB",
            "/DEF:" + str(definition), "/OUT:" + str(dll), str(obj), "kernel32.lib", "user32.lib"], env=env, cwd=root, check=True)
        session = Host(host, dll, plugins)
        try:
            session.send(b"\x01")
            assert session.read(2) == b"\x01\x0f", "START/export flags mismatch"
            request = b"\x02" + struct.pack("<HHfHf", 2, 0, 1.5, 3, 2.5)
            request += struct.pack("<HHfHf", 2, 1, 2.0, 2, 4.0)
            request += struct.pack("<HH", 2, 0) + string("Zoo € 🚌") + struct.pack("<H", 1) + string("unchanged")
            request += struct.pack("<HHH", 2, 2, 3)
            session.send(request)
            floats = [struct.unpack("<Bf", session.read(5)) for _ in range(4)]
            assert floats == [(1, 11.5), (0, 0.0), (1, 4.0), (0, 0.0)], floats
            strings = []
            for _ in range(2):
                write = session.read(1)[0]
                count = struct.unpack("<H", session.read(2))[0]
                strings.append((write, session.read(count * 2).decode("utf-16-le")))
            assert strings == [(1, "Pronto € 🚌"), (0, "")], strings
            assert session.read(2) == b"\x01\x00", "message pump/thread/context failed"
            session.send(b"\x03")
            assert session.read(1) == b"\x01"
            assert session.process.wait(timeout=10) == 0
            assert (root / "test-finalized.txt").read_bytes() == b"finalized"
            assert (root / "bbs.start").read_bytes() == marker, "host changed marker"
        finally: session.close()
        (root / "test-finalized.txt").unlink()
        session = Host(host, dll, plugins)
        try:
            session.send(b"\x01")
            assert session.read(2) == b"\x01\x0f"
            time.sleep(0.05)
            session.process.stdin.close()
            assert session.process.wait(timeout=10) == 0
            assert (root / "test-finalized.txt").read_bytes() == b"finalized", "EOF cleanup failed"
        finally: session.close()
        other = plugins / "other.dll"
        shutil.copy2(dll, other)
        session = Host(host, other, plugins)
        try:
            session.send(b"\x01")
            assert session.read(2) == b"\x00\x00"
            assert session.process.wait(timeout=10) != 0
        finally: session.close()
    print("Native PE32/stdcall DLL: startup context, flags, float/string/trigger ABI, Unicode, Windows messages, finalization, EOF cleanup and non-BCS rejection passed.")

if __name__ == "__main__": main()
