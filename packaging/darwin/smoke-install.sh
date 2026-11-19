#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/../.."
VERSION="$(tr -d '\r\n' < VERSION)"
sudo installer -pkg "dist/release/MyGittu-$VERSION-macOS.pkg" -target /
APP=/Applications/MyGittu.app
for command in desktop agent mygittu; do
  lipo "$APP/Contents/MacOS/$command" -verify_arch x86_64
  lipo "$APP/Contents/MacOS/$command" -verify_arch arm64
done
signature="$(codesign -dv "$APP" 2>&1)"
if ! grep -Fxq 'TeamIdentifier=not set' <<< "$signature"; then
  EXT="$APP/Contents/PlugIns/MyGittuFinder.appex"
  lipo "$EXT/Contents/MacOS/MyGittuFinder" -verify_arch x86_64
  lipo "$EXT/Contents/MacOS/MyGittuFinder" -verify_arch arm64
  [[ "$(plutil -extract NSExtension.NSExtensionPointIdentifier raw "$EXT/Contents/Info.plist")" == com.apple.FinderSync ]]
fi
codesign --verify --deep --strict "$APP"
mkdir -p .work
"$APP/Contents/MacOS/agent" > .work/installed-agent.log 2>&1 &
AGENT_PID=$!
trap 'kill "$AGENT_PID" 2>/dev/null || true' EXIT
sleep 1
"$APP/Contents/MacOS/mygittu" -method health.ping > .work/installed-health.json
python3 - "$VERSION" <<'PY'
import json,sys
from pathlib import Path
assert json.loads(Path('.work/installed-health.json').read_text())['app_version'] == sys.argv[1]
PY
echo "Installed Universal package and started native Agent on $(uname -m)"
