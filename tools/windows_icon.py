"""Build a deterministic x64 COFF icon resource from the approved app PNG.

Only the build output is generated; no runtime dependencies or downloads.
Run from the repository root before compiling the Windows agent.
"""
import struct
from pathlib import Path


def build(png):
    assert png[:8] == b'\x89PNG\r\n\x1a\n'
    w, h = struct.unpack_from('>II', png, 16)
    assert 1 <= w <= 256 and 1 <= h <= 256
    group = struct.pack('<HHHBBBBHHIH', 0, 1, 1, w % 256, h % 256, 0, 0, 1, 32, len(png), 1)
    resource = bytearray(160)

    def directory(offset, entries):
        struct.pack_into('<IIHHHH', resource, offset, 0, 0, 0, 0, 0, len(entries))
        for i, (ident, target) in enumerate(entries):
            struct.pack_into('<II', resource, offset + 16 + i * 8, ident, target)

    # RT_ICON (1) and RT_GROUP_ICON (101), language en-US.
    directory(0, [(3, 0x80000020), (14, 0x80000038)])
    directory(32, [(1, 0x80000050)])
    directory(56, [(101, 0x80000068)])
    directory(80, [(0x409, 128)])
    directory(104, [(0x409, 144)])
    resource.extend(png)
    resource.extend(b'\x00' * (-len(resource) % 4))
    group_offset = len(resource)
    resource.extend(group)
    resource.extend(b'\x00' * (-len(resource) % 4))
    struct.pack_into('<IIII', resource, 128, 160, len(png), 0, 0)
    struct.pack_into('<IIII', resource, 144, group_offset, len(group), 0, 0)
    reloc_offset = 60 + len(resource)
    symbol_offset = reloc_offset + 20
    header = struct.pack('<HHIIIHH', 0x8664, 1, 0, symbol_offset, 1, 0, 0)
    section = struct.pack('<8sIIIIIIHHI', b'.rsrc', 0, 0, len(resource), 60, reloc_offset, 0, 2, 0, 0x40300040)
    relocs = struct.pack('<IIHIIH', 128, 0, 3, 144, 0, 3)
    symbol = struct.pack('<8sIhHBB', b'.rsrc', 0, 1, 0, 3, 0)
    return header + section + resource + relocs + symbol + struct.pack('<I', 4)


if __name__ == '__main__':
    png = Path('internal/core/static/apple-touch-icon.png').read_bytes()
    Path('cmd/agent/icon_windows_amd64.syso').write_bytes(build(png))
