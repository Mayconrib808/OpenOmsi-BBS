"""Build Setup's Windows x86 COFF resources using only Python's standard library.

The bitmap and icon are derivatives of the project's existing MIT SVG artwork.
No resource compiler or image library is required to build the application.
"""
from __future__ import annotations

from pathlib import Path
import struct


def resources_from_assets(directory: Path) -> dict[int, dict[int, bytes]]:
    icon = (directory / "setup.ico").read_bytes()
    reserved, kind, count = struct.unpack_from("<HHH", icon)
    if reserved or kind != 1 or not 1 <= count <= 32:
        raise ValueError("Invalid Setup ICO header")
    images = {}
    group = bytearray(struct.pack("<HHH", 0, 1, count))
    for index in range(count):
        width, height, colors, reserved, planes, bits, size, offset = struct.unpack_from(
            "<BBBBHHII", icon, 6 + index * 16)
        if reserved or not size or offset < 6 + count * 16 or offset + size > len(icon):
            raise ValueError("Invalid Setup ICO image")
        images[index + 1] = icon[offset:offset + size]
        group += struct.pack("<BBBBHHIH", width, height, colors, 0, planes, bits, size, index + 1)
    bitmap = (directory / "setup-banner.bmp").read_bytes()
    if bitmap[:2] != b"BM" or len(bitmap) < 54:
        raise ValueError("Invalid Setup bitmap")
    # RT_BITMAP starts at BITMAPINFOHEADER, without the BMP file header.
    return {2: {201: bitmap[14:]}, 3: images, 14: {101: bytes(group)}}


def write_coff(path: Path, resources: dict[int, dict[int, bytes]]) -> None:
    data = bytearray()
    relocations = []

    def allocate(size: int) -> int:
        data.extend(b"\0" * (-len(data) % 4))
        offset = len(data)
        data.extend(b"\0" * size)
        return offset

    def directory(ids: list[int]) -> tuple[int, dict[int, int]]:
        offset = allocate(16 + 8 * len(ids))
        struct.pack_into("<IIHHHH", data, offset, 0, 0, 0, 0, 0, len(ids))
        entries = {}
        for i, key in enumerate(sorted(ids)):
            entry = offset + 16 + i * 8
            struct.pack_into("<I", data, entry, key)
            entries[key] = entry + 4
        return offset, entries

    _, root_entries = directory(list(resources))
    for kind, images in sorted(resources.items()):
        type_offset, type_entries = directory(list(images))
        struct.pack_into("<I", data, root_entries[kind], type_offset | 0x80000000)
        for name, payload in sorted(images.items()):
            lang_offset, lang_entries = directory([0])  # language neutral artwork
            struct.pack_into("<I", data, type_entries[name], lang_offset | 0x80000000)
            entry = allocate(16)
            struct.pack_into("<I", data, lang_entries[0], entry)
            start = allocate(len(payload))
            data[start:start + len(payload)] = payload
            struct.pack_into("<IIII", data, entry, start, len(payload), 0, 0)
            relocations.append(entry)

    # One read-only initialized .rsrc section; each IMAGE_RESOURCE_DATA_ENTRY
    # uses IMAGE_REL_I386_DIR32NB to turn its section offset into a PE RVA.
    raw_offset = 20 + 40
    relocation_offset = raw_offset + len(data)
    symbol_offset = relocation_offset + 10 * len(relocations)
    header = struct.pack("<HHIIIHH", 0x014C, 1, 0, symbol_offset, 1, 0, 0)
    section = struct.pack("<8sIIIIIIHHI", b".rsrc\0\0\0", 0, 0, len(data), raw_offset,
                          relocation_offset, 0, len(relocations), 0, 0x40000040)
    relocs = b"".join(struct.pack("<IIH", offset, 0, 0x0007) for offset in relocations)
    symbol = struct.pack("<8sIhHBB", b".rsrc\0\0\0", 0, 1, 0, 3, 0)
    path.write_bytes(header + section + data + relocs + symbol + struct.pack("<I", 4))


def verify_executable(path: Path, expected: dict[int, dict[int, bytes]]) -> None:
    """Verify the linked PE resources, including icon group IDs and exact pixels."""
    data = path.read_bytes()
    pe = struct.unpack_from("<I", data, 0x3C)[0]
    if data[pe:pe + 4] != b"PE\0\0":
        raise ValueError("Setup is not a PE executable")
    machine, sections = struct.unpack_from("<HH", data, pe + 4)
    optional_size = struct.unpack_from("<H", data, pe + 20)[0]
    optional = pe + 24
    if machine != 0x014C or struct.unpack_from("<H", data, optional)[0] != 0x10B:
        raise ValueError("Setup resource check expects Windows x86 PE32")
    resource_rva, _ = struct.unpack_from("<II", data, optional + 96 + 2 * 8)

    def file_offset(rva: int, size: int = 1) -> int:
        for i in range(sections):
            offset = optional + optional_size + i * 40
            _, virtual_size, address, raw_size, raw = struct.unpack_from("<8sIIII", data, offset)
            if address <= rva and rva + size <= address + raw_size:
                return raw + rva - address
        raise ValueError("Resource RVA is outside a PE section")

    root = file_offset(resource_rva)

    def entries(offset: int) -> dict[int, int]:
        named, count = struct.unpack_from("<HH", data, root + offset + 12)
        if named:
            raise ValueError("Unexpected named Setup resource")
        return dict(struct.unpack_from("<II", data, root + offset + 16 + i * 8) for i in range(count))

    actual = {}
    for kind, type_offset in entries(0).items():
        actual[kind] = {}
        for name, lang_offset in entries(type_offset & 0x7FFFFFFF).items():
            langs = entries(lang_offset & 0x7FFFFFFF)
            if set(langs) != {0}:
                raise ValueError("Unexpected Setup resource language")
            rva, size = struct.unpack_from("<II", data, root + langs[0])
            start = file_offset(rva, size)
            actual[kind][name] = data[start:start + size]
    if actual != expected:
        raise ValueError("Linked Setup artwork differs from the source assets")


def prepare(directory: Path, assets: Path) -> dict[int, dict[int, bytes]]:
    resources = resources_from_assets(assets)
    write_coff(directory / "setup_windows_386.syso", resources)
    return resources
