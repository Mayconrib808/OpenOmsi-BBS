"""Truthful Windows identity for the source-backed 32-bit BCS plugin host."""
from __future__ import annotations

from pathlib import Path
import re
import struct

import setup_resources


def version_numbers(version: str) -> tuple[int, int, int, int]:
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)(?:-dev\.(\d+))?", version)
    if not match:
        raise ValueError("Unsupported helper version")
    numbers = tuple(int(n or 0) for n in match.groups())
    if any(n > 65535 for n in numbers):
        raise ValueError("Windows version components must fit in WORDs")
    return numbers


def version_strings(version: str) -> dict[str, str]:
    return {
        "CompanyName": "Mayconrib808",
        "FileDescription": "OpenOMSI BCS 32-bit plugin host",
        "FileVersion": version,
        "InternalName": "OpenOMSI_BCS_PluginHost32",
        "LegalCopyright": "Copyright (c) 2026 Mayconrib808. MIT License.",
        "OriginalFilename": "omsi-plugin-host32.exe",
        "ProductName": "OpenOmsi + BBS",
        "ProductVersion": version,
        "Comments": "Loads the locally installed BCS plugin through the bridge pipe protocol.",
    }


def version_resource(version: str) -> bytes:
    """Create standard VS_VERSION_INFO, with DWORD-aligned UTF-16 children."""
    def block(key: str, kind: int, value: bytes = b"", children: tuple[bytes, ...] = ()) -> bytes:
        value_length = len(value) // 2 if kind == 1 else len(value)
        data = bytearray(struct.pack("<HHH", 0, value_length, kind))
        data += (key + "\0").encode("utf-16-le")
        data += b"\0" * (-len(data) % 4)
        data += value
        for child in children:
            data += b"\0" * (-len(data) % 4)
            data += child
        struct.pack_into("<H", data, 0, len(data))
        return bytes(data)

    major, minor, patch, revision = version_numbers(version)
    ms, ls = major << 16 | minor, patch << 16 | revision
    fixed = struct.pack("<13I", 0xFEEF04BD, 0x10000, ms, ls, ms, ls,
                        0x3F, 0x02 if "-dev." in version else 0, 0x40004, 1, 0, 0, 0)
    strings = tuple(block(key, 1, (value + "\0").encode("utf-16-le"))
                    for key, value in version_strings(version).items())
    string_info = block("StringFileInfo", 1, children=(block("040904B0", 1, children=strings),))
    translation = block("VarFileInfo", 1, children=(block("Translation", 0, struct.pack("<HH", 0x0409, 1200)),))
    return block("VS_VERSION_INFO", 0, fixed, (string_info, translation))


def resources_from_assets(assets: Path, version: str) -> dict[int, dict[int, bytes]]:
    artwork = setup_resources.resources_from_assets(assets)
    return {3: artwork[3], 14: artwork[14], 16: {1: version_resource(version)}}


def prepare(directory: Path, assets: Path, version: str) -> dict[int, dict[int, bytes]]:
    resources = resources_from_assets(assets, version)
    setup_resources.write_coff(directory / "plugin_host_windows_386.syso", resources)
    return resources


def verify_native_version(path: Path, version: str) -> None:
    """Read linked metadata through Windows version.dll, independently of our writer."""
    import ctypes
    from ctypes import wintypes

    api = ctypes.WinDLL("version", use_last_error=True)
    api.GetFileVersionInfoSizeW.argtypes = [wintypes.LPCWSTR, ctypes.POINTER(wintypes.DWORD)]
    api.GetFileVersionInfoSizeW.restype = wintypes.DWORD
    api.GetFileVersionInfoW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p]
    api.GetFileVersionInfoW.restype = wintypes.BOOL
    api.VerQueryValueW.argtypes = [ctypes.c_void_p, wintypes.LPCWSTR, ctypes.POINTER(ctypes.c_void_p), ctypes.POINTER(wintypes.UINT)]
    api.VerQueryValueW.restype = wintypes.BOOL
    handle = wintypes.DWORD()
    size = api.GetFileVersionInfoSizeW(str(path), ctypes.byref(handle))
    if not size:
        raise ctypes.WinError(ctypes.get_last_error())
    data = ctypes.create_string_buffer(size)
    if not api.GetFileVersionInfoW(str(path), 0, size, data):
        raise ctypes.WinError(ctypes.get_last_error())

    def query(key: str) -> tuple[int, int]:
        pointer, length = ctypes.c_void_p(), wintypes.UINT()
        if not api.VerQueryValueW(data, key, ctypes.byref(pointer), ctypes.byref(length)) or not pointer.value:
            raise ValueError(f"Windows cannot read helper metadata: {key}")
        return pointer.value, length.value

    pointer, length = query("\\")
    if length != 52:
        raise ValueError("Invalid Windows fixed version size")
    fixed = struct.unpack("<13I", ctypes.string_at(pointer, length))
    major, minor, patch, revision = version_numbers(version)
    ms, ls = major << 16 | minor, patch << 16 | revision
    if fixed[:6] != (0xFEEF04BD, 0x10000, ms, ls, ms, ls) or fixed[7] != (2 if "-dev." in version else 0) or fixed[8:10] != (0x40004, 1):
        raise ValueError("Windows reads an incorrect helper version or file type")
    pointer, length = query("\\VarFileInfo\\Translation")
    if ctypes.string_at(pointer, length) != struct.pack("<HH", 0x0409, 1200):
        raise ValueError("Windows reads an incorrect helper language/code page")
    for key, expected in version_strings(version).items():
        pointer, length = query("\\StringFileInfo\\040904B0\\" + key)
        if ctypes.wstring_at(pointer, length).rstrip("\0") != expected:
            raise ValueError(f"Windows reads an incorrect helper field: {key}")

