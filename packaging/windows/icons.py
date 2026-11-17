"""Generate small deterministic Explorer overlay resources without external libraries."""
import struct
from pathlib import Path

target = Path(__file__).resolve().parents[2] / "extensions" / "windows"
for state, color in [("clean", (54, 154, 83)), ("modified", (224, 153, 36)), ("conflicted", (205, 64, 64))]:
    pixels = bytearray()
    for y in reversed(range(32)):
        for x in range(32):
            inside = (x - 25) ** 2 + (y - 25) ** 2 <= 6 ** 2
            mark = False
            if state == "clean":
                mark = (21 <= x <= 24 and abs(y - (x - 1)) <= 1) or (24 <= x <= 29 and abs(y - (-x + 49)) <= 1)
            elif state == "modified":
                mark = 22 <= x <= 28 and abs(y - (31 - x)) <= 1
            else:
                mark = (24 <= x <= 26 and 20 <= y <= 27) or (24 <= x <= 26 and 29 <= y <= 30)
            r, g, b = (255, 255, 255) if mark else color
            pixels.extend((b, g, r, 255 if inside else 0))
    mask = bytes(4 * 32)
    bmp = struct.pack("<IIIHHIIIIII", 40, 32, 64, 1, 32, 0, len(pixels), 0, 0, 0, 0) + pixels + mask
    ico = struct.pack("<HHH", 0, 1, 1) + struct.pack("<BBBBHHII", 32, 32, 0, 0, 1, 32, len(bmp), 22) + bmp
    (target / (state + ".ico")).write_bytes(ico)
