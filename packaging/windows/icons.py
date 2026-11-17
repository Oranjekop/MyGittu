"""Generate tiny deterministic overlay resources without external libraries."""
import math
import struct
from pathlib import Path

target = Path(__file__).resolve().parents[2] / "extensions" / "windows"
for state, color in [("clean", (54, 154, 83)), ("modified", (224, 153, 36)), ("conflicted", (205, 64, 64))]:
    pixels = bytearray()
    for y in reversed(range(32)):
        for x in range(32):
            inside = (x - 16) ** 2 + (y - 16) ** 2 <= 14 ** 2
            mark = False
            if state == "clean":
                mark = (7 <= x <= 14 and abs(y - (x + 8)) <= 2) or (13 <= x <= 24 and abs(y - (-x + 34)) <= 2)
            elif state == "modified":
                mark = 9 <= x <= 23 and abs(y - (32 - x)) <= 2
            else:
                mark = (14 <= x <= 18 and 7 <= y <= 19) or (14 <= x <= 18 and 23 <= y <= 26)
            r, g, b = (255, 255, 255) if mark else color
            pixels.extend((b, g, r, 255 if inside else 0))
    mask = bytes(4 * 32)
    bmp = struct.pack("<IIIHHIIIIII", 40, 32, 64, 1, 32, 0, len(pixels), 0, 0, 0, 0) + pixels + mask
    ico = struct.pack("<HHH", 0, 1, 1) + struct.pack("<BBBBHHII", 32, 32, 0, 0, 1, 32, len(bmp), 22) + bmp
    (target / (state + ".ico")).write_bytes(ico)
