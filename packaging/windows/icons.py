"""Generate compact, transparent Windows Explorer status badges."""
import math
import struct
from pathlib import Path

TARGET = Path(__file__).resolve().parents[2] / "extensions" / "windows"
STATES = {
    "clean": ((17, 117, 88), [(22.7, 25.3), (24.6, 27.2), (28.2, 23.2)]),
    "modified": ((157, 88, 17), [(22.7, 27.7), (27.9, 22.5)]),
    "conflicted": ((174, 47, 64), [(25.4, 22.5), (25.4, 26.1)]),
}


def distance(x, y, a, b):
    dx, dy = b[0] - a[0], b[1] - a[1]
    t = min(1, max(0, ((x - a[0]) * dx + (y - a[1]) * dy) / (dx * dx + dy * dy)))
    return math.hypot(x - a[0] - t * dx, y - a[1] - t * dy)


def sample(x, y, state, color, points):
    radius = math.hypot(x - 25.3, y - 25.3)
    if radius > 6.0:
        return None
    if radius > 5.05:
        return (255, 255, 255)
    if any(distance(x, y, a, b) < 0.86 for a, b in zip(points, points[1:])):
        return (255, 255, 255)
    if state == "conflicted" and math.hypot(x - 25.4, y - 28.0) < 0.88:
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
    sizes = (16, 32, 48)
    header = bytearray(struct.pack("<HHH", 0, 1, len(sizes)))
    images = []
    offset = 6 + 16 * len(sizes)
    for size in sizes:
        image = bitmap(size, state, color, points)
        header.extend(struct.pack("<BBBBHHII", size, size, 0, 0, 1, 32, len(image), offset))
        images.append(image)
        offset += len(image)
    (TARGET / f"{state}.ico").write_bytes(header + b"".join(images))
