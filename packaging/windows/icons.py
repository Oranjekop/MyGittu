"""Generate compact, transparent Windows Explorer status badges."""
import math
import struct
from pathlib import Path

TARGET = Path(__file__).resolve().parents[2] / "extensions" / "windows"
SIZES = (16, 20, 24, 28, 32, 40, 48, 56, 64, 72, 96, 128, 256)
STATES = {
    "clean": ((17, 117, 88), [(19.2, 23.3), (21.8, 26), (27, 20.4)]),
    "modified": ((157, 88, 17), [(19.5, 26.6), (26.6, 19.5)]),
    "conflicted": ((174, 47, 64), [(23, 18.7), (23, 23.4)]),
}


def distance(x, y, a, b):
    dx, dy = b[0] - a[0], b[1] - a[1]
    t = min(1, max(0, ((x - a[0]) * dx + (y - a[1]) * dy) / (dx * dx + dy * dy)))
    return math.hypot(x - a[0] - t * dx, y - a[1] - t * dy)


def sample(x, y, state, color, points):
    radius = math.hypot(x - 23, y - 23)
    if radius > 8.5:
        return None
    if radius > 7.15:
        return (255, 255, 255)
    if any(distance(x, y, a, b) < 1.25 for a, b in zip(points, points[1:])):
        return (255, 255, 255)
    if state == "conflicted" and math.hypot(x - 23, y - 27.2) < 1.25:
        return (255, 255, 255)
    return color


def bitmap(size, state, color, points):
    pixels = bytearray()
    mask = bytearray()
    stride = ((size + 31) // 32) * 4
    for y in reversed(range(size)):
        row = bytearray(stride)
        for x in range(size):
            samples = [
                pixel for j in range(4) for i in range(4)
                if (pixel := sample((x + (i + 0.5) / 4) * 32 / size,
                                    (y + (j + 0.5) / 4) * 32 / size,
                                    state, color, points)) is not None
            ]
            if samples:
                r, g, b = (round(sum(ch) / len(samples)) for ch in zip(*samples))
                pixels.extend((b, g, r, round(255 * len(samples) / 16)))
            else:
                pixels.extend((0, 0, 0, 0))
                row[x // 8] |= 0x80 >> (x % 8)
        mask.extend(row)
    header = struct.pack("<IIIHHIIIIII", 40, size, size * 2, 1, 32, 0, len(pixels), 0, 0, 0, 0)
    return header + pixels + mask


for state, (color, points) in STATES.items():
    header = bytearray(struct.pack("<HHH", 0, 1, len(SIZES)))
    images = []
    offset = 6 + 16 * len(SIZES)
    for size in SIZES:
        image = bitmap(size, state, color, points)
        # ICO encodes a 256-pixel dimension as zero.
        dimension = size % 256
        header.extend(struct.pack("<BBBBHHII", dimension, dimension, 0, 0, 1, 32, len(image), offset))
        images.append(image)
        offset += len(image)
    (TARGET / f"{state}.ico").write_bytes(header + b"".join(images))
