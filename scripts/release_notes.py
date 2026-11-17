"""Extract the exact tag's notes; fail if the changelog was not prepared."""
import re
import sys
from pathlib import Path

version = sys.argv[1]
if not re.fullmatch(r"\d+\.\d+\.\d+", version):
    raise SystemExit("invalid version")
text = (Path(__file__).resolve().parents[1] / "CHANGELOG.md").read_text(encoding="utf-8")
match = re.search(r"^## v" + re.escape(version) + r"\s*\n(.*?)(?=^## |\Z)", text, re.M | re.S)
if not match:
    raise SystemExit("missing version in CHANGELOG.md")
print(match.group(1).strip())
