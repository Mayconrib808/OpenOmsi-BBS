#!/usr/bin/env python3
"""Check x64 PE import names/ordinals against bundled DLLs, without executing them.

Usage: python3 audit_pe.py PACKAGE_DIRECTORY
Windows/system API availability and execution are outside this static check.
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
import struct
import sys


SYSTEM_DLLS = set("""
advapi32.dll bcrypt.dll bcryptprimitives.dll combase.dll comctl32.dll
crypt32.dll dinput8.dll dwmapi.dll dxgi.dll gdi32.dll imm32.dll kernel32.dll
msvcrt.dll ntdll.dll ole32.dll oleaut32.dll opengl32.dll propsys.dll
psapi.dll rpcrt4.dll setupapi.dll shell32.dll shlwapi.dll user32.dll
userenv.dll uxtheme.dll version.dll winmm.dll ws2_32.dll
""".split())


class PE:
    def __init__(self, path: Path):
        self.path = path
        self.data = path.read_bytes()
        if self.data[:2] != b"MZ":
            raise ValueError(f"Not PE: {path}")
        nt = self.u32(0x3C)
        if self.data[nt:nt + 4] != b"PE\0\0":
            raise ValueError(f"Invalid PE header: {path}")
        self.machine = self.u16(nt + 4)
        sections = self.u16(nt + 6)
        opt = nt + 24
        self.x64 = self.u16(opt) == 0x20B
        if not self.x64:
            raise ValueError(f"Expected PE32+: {path}")
        self.image_base = self.u64(opt + 24)
        self.header_size = self.u32(opt + 60)
        self.directories = [self.unpack("II", opt + 112 + i * 8)
                            for i in range(min(16, self.u32(opt + 108)))]
        first_section = opt + self.u16(nt + 20)
        self.sections = []
        for i in range(sections):
            section = first_section + i * 40
            virtual_size, address, raw_size, raw_offset = self.unpack("IIII", section + 8)
            self.sections.append((address, max(virtual_size, raw_size), raw_size, raw_offset))

    def unpack(self, fmt, offset):
        return struct.unpack_from("<" + fmt, self.data, offset)

    def u16(self, offset):
        return self.unpack("H", offset)[0]

    def u32(self, offset):
        return self.unpack("I", offset)[0]

    def u64(self, offset):
        return self.unpack("Q", offset)[0]

    def offset(self, rva):
        if rva < self.header_size:
            return rva
        for address, size, raw_size, raw_offset in self.sections:
            if address <= rva < address + size:
                if rva - address >= raw_size:
                    raise ValueError(f"RVA has no file bytes: {self.path}: {rva:x}")
                return raw_offset + rva - address
        raise ValueError(f"Unmapped RVA: {self.path}: {rva:x}")

    def string(self, rva):
        start = self.offset(rva)
        end = self.data.index(b"\0", start)
        return self.data[start:end].decode("ascii")

    def directory(self, index):
        return self.directories[index] if index < len(self.directories) else (0, 0)

    def thunk(self, rva):
        result = []
        if not rva:
            return result
        ptr = self.offset(rva)
        while (value := self.u64(ptr)):
            result.append(value & 0xFFFF if value & (1 << 63) else self.string(value + 2))
            ptr += 8
        return result

    def imports(self):
        result = []
        rva, _ = self.directory(1)
        if rva:
            ptr = self.offset(rva)
            while any(descriptor := self.unpack("IIIII", ptr)):
                original, _, _, name, first = descriptor
                result.append((self.string(name).lower(), self.thunk(original or first), False))
                ptr += 20
        rva, _ = self.directory(13)
        if rva:
            ptr = self.offset(rva)
            while any(descriptor := self.unpack("IIIIIIII", ptr)):
                attributes, name, _, iat, names, _, _, _ = descriptor
                def as_rva(value):
                    return value if attributes & 1 else value - self.image_base
                result.append((self.string(as_rva(name)).lower(),
                               self.thunk(as_rva(names or iat)), True))
                ptr += 32
        return result

    def exports(self):
        names, ordinals = set(), set()
        rva, _ = self.directory(0)
        if not rva:
            return names, ordinals
        ptr = self.offset(rva)
        base, count, name_count, functions, name_rvas, name_ordinals = self.unpack("IIIIII", ptr + 16)
        functions = self.offset(functions)
        for i in range(count):
            if self.u32(functions + i * 4):
                ordinals.add(base + i)
        if name_count:
            name_rvas, name_ordinals = self.offset(name_rvas), self.offset(name_ordinals)
            for i in range(name_count):
                if base + self.u16(name_ordinals + i * 2) in ordinals:
                    names.add(self.string(self.u32(name_rvas + i * 4)))
        return names, ordinals


def audit(root: Path):
    paths = sorted(p for p in root.rglob("*")
                   if p.is_file() and p.suffix.lower() in {".exe", ".dll"})
    binaries = {p.resolve(): PE(p) for p in paths}
    failures, edges, systems = [], [], set()
    for path, binary in binaries.items():
        relative = str(path.relative_to(root.resolve()))
        if binary.machine != 0x8664:
            failures.append({"file": relative, "machine": hex(binary.machine)})
        for dll, symbols, delayed in binary.imports():
            target = next((p for p in binaries
                           if p.parent == path.parent and p.name.lower() == dll), None)
            if target is None:
                if dll in SYSTEM_DLLS or dll.startswith(("api-ms-win-", "ext-ms-")):
                    systems.add(dll)
                else:
                    failures.append({"file": relative, "missing_dll": dll})
                continue
            exported_names, exported_ordinals = binaries[target].exports()
            missing = [str(symbol) for symbol in symbols
                       if symbol not in (exported_ordinals if isinstance(symbol, int) else exported_names)]
            edges.append({"file": relative, "dll": dll, "symbols": len(symbols),
                          "delayed": delayed, "missing_symbols": missing})
            if missing:
                failures.append(edges[-1])
    game = root / "programa/openomsi.exe"
    steam = root / "programa/steam_api64.dll"
    return {"check": "PE imports versus bundled DLL exports (names and ordinals, including delay imports)",
            "passed": not failures, "pe_files": len(paths), "bundled_edges": edges,
            "failures": failures, "system_dlls_unchecked_at_runtime": sorted(systems),
            "game_sha256": hashlib.sha256(game.read_bytes()).hexdigest(),
            "steam_sha256": hashlib.sha256(steam.read_bytes()).hexdigest(),
            "limitations": ["Static metadata check only; no Windows startup or game session was run.",
                            "Windows system DLL exports and runtime-loaded libraries were not validated.",
                            "Exported forwarder targets were not resolved."]}


if __name__ == "__main__":
    report = audit(Path(sys.argv[1]).resolve())
    print(json.dumps(report, ensure_ascii=False, indent=2))
    raise SystemExit(0 if report["passed"] else 1)
