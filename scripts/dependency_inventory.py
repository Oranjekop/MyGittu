"""Collect the actually imported Windows/macOS Go modules and their notices."""
import json
import os
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LICENSES = {
    "github.com/Microsoft/go-winio": "MIT",
    "github.com/egoist/mygo": "MIT",
    "github.com/ebitengine/purego": "Apache-2.0",
    "github.com/fsnotify/fsnotify": "BSD-3-Clause",
    "github.com/go-text/typesetting": "Unlicense OR BSD-3-Clause",
}


def objects(text):
    decoder = json.JSONDecoder()
    while text.strip():
        obj, end = decoder.raw_decode(text.lstrip())
        yield obj
        text = text.lstrip()[end:]


modules = {}
notices = {}
for target, arch in [("windows", "amd64"), ("darwin", "arm64")]:
    env = dict(os.environ, GOOS=target, GOARCH=arch, CGO_ENABLED="0")
    output = subprocess.check_output(["go", "list", "-deps", "-json", "./cmd/..."], cwd=ROOT, env=env, text=True, encoding="utf-8")
    for package in objects(output):
        module = package.get("Module", {})
        if not module or module.get("Main"):
            continue
        name = module["Path"]
        modules[name] = module
        base = Path(module["Dir"])
        directory = Path(package["Dir"])
        while directory.is_relative_to(base):
            for file in directory.iterdir():
                if file.is_file() and (file.name.upper().startswith(("LICENSE", "COPYING", "NOTICE")) or file.name.upper() == "OFL.TXT"):
                    key = name + "/" + file.relative_to(base).as_posix()
                    notices[key] = file.read_text(encoding="utf-8", errors="replace")
            if directory == base:
                break
            directory = directory.parent

components = []
for name, module in sorted(modules.items()):
    license_id = LICENSES.get(name, "BSD-3-Clause" if name.startswith("golang.org/x/") else "NOASSERTION")
    if license_id == "NOASSERTION":
        raise SystemExit("Review new dependency license: " + name)
    components.append({
        "type": "library", "name": name, "version": module["Version"],
        "purl": "pkg:golang/" + name + "@" + module["Version"],
        "licenses": [{"expression": license_id}],
        "properties": [{"name": "go:moduleSum", "value": module.get("Sum", "")}],
    })

goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
notices["Go standard library/LICENSE"] = (goroot / "LICENSE").read_text(encoding="utf-8")
notices["NSIS/LICENSE"] = (
    "NSIS source code is available from https://nsis.sourceforge.io/Download.\n\n"
    + (ROOT / "packaging/windows/LICENSE.nsis.txt").read_text(encoding="utf-8")
)
sbom = {"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
        "metadata": {"component": {"type": "application", "name": "MyGittu", "version": "0.0.1"}},
        "components": components}
(ROOT / "docs" / "sbom.cdx.json").write_text(json.dumps(sbom, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
text = "# Third-party notices\n\nImported Go modules for Windows amd64 and macOS arm64. System Git is an external prerequisite.\n"
for name, license_text in sorted(notices.items()):
    text += "\n## " + name + "\n\n" + license_text.rstrip() + "\n"
(ROOT / "THIRD_PARTY_NOTICES.md").write_text(text, encoding="utf-8")
print(f"Inventoried {len(components)} modules and {len(notices)} license/notice files")
