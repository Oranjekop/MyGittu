#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/../.."
VERSION="${1:-$(tr -d '\r\n' < VERSION)}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 2
APP="$PWD/dist/macos/root/Applications/MyGittu.app"
if [[ "${2:-}" == "--app-only" ]]; then
  APP="$PWD/bin/MyGittu.app"
fi
WORK="$PWD/.work/universal"
sign=(--force --sign -)
group_flags=""
if [[ -n "${SIGN_IDENTITY:-}" || -n "${MYGITTU_APP_GROUP:-}" ]]; then
  : "${SIGN_IDENTITY:?Set SIGN_IDENTITY to your Developer ID Application identity}"
  : "${MYGITTU_APP_GROUP:?Set MYGITTU_APP_GROUP to TEAMID.mygittu}"
  [[ "$MYGITTU_APP_GROUP" =~ ^[A-Z0-9]{10}\.[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*$ ]] || {
    echo "MYGITTU_APP_GROUP must start with the signing Team ID" >&2
    exit 2
  }
  sign=(--force --options runtime --timestamp --sign "$SIGN_IDENTITY")
  group_flags="-X mygittu/internal/platform.appGroup=$MYGITTU_APP_GROUP"
fi
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" .work/universal dist/release
for command in desktop agent mygittu; do
  for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -ldflags "-s -w -X mygittu/internal/buildinfo.Version=$VERSION $group_flags" -o ".work/universal/$command-$arch" "./cmd/$command"
  done
  lipo -create ".work/universal/$command-amd64" ".work/universal/$command-arm64" -output "$APP/Contents/MacOS/$command"
  lipo "$APP/Contents/MacOS/$command" -verify_arch x86_64
  lipo "$APP/Contents/MacOS/$command" -verify_arch arm64
done
cp packaging/darwin/MyGittu.icns "$APP/Contents/Resources/"
cp LICENSE THIRD_PARTY_NOTICES.md "$APP/Contents/Resources/"
cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>dev.mygittu.desktop</string><key>CFBundleExecutable</key><string>desktop</string>
<key>CFBundleName</key><string>MyGittu</string><key>CFBundleDisplayName</key><string>MyGittu</string><key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>$VERSION</string><key>CFBundleShortVersionString</key><string>$VERSION</string><key>CFBundleIconFile</key><string>MyGittu.icns</string>
<key>LSMinimumSystemVersion</key><string>13.0</string><key>NSHighResolutionCapable</key><true/><key>CFBundleDevelopmentRegion</key><string>zh_CN</string>
</dict></plist>
EOF
rm -rf "$APP/Contents/PlugIns/MyGittuFinder.appex"
if [[ -n "$group_flags" ]]; then
  source packaging/darwin/finder.sh
  build_finder_extension "$APP" "$VERSION" "$WORK" "$MYGITTU_APP_GROUP"
  for command in agent mygittu; do
    codesign "${sign[@]}" --entitlements "$WORK/group.entitlements" "$APP/Contents/MacOS/$command"
  done
  codesign "${sign[@]}" --entitlements "$WORK/finder.entitlements" "$FINDER_EXTENSION"
  codesign "${sign[@]}" --entitlements "$WORK/group.entitlements" "$APP"
  signature="$(codesign -dv "$APP" 2>&1)"
  if ! grep -Fxq "TeamIdentifier=${MYGITTU_APP_GROUP%%.*}" <<< "$signature"; then
    echo "App Group prefix does not match the signing Team ID" >&2
    exit 2
  fi
else
  for command in agent mygittu; do codesign "${sign[@]}" "$APP/Contents/MacOS/$command"; done
  codesign "${sign[@]}" "$APP"
fi
codesign --verify --deep --strict "$APP"
plutil -lint "$APP/Contents/Info.plist"
if [[ "${2:-}" == "--app-only" ]]; then
  exit 0
fi
pkgbuild --root dist/macos/root --identifier dev.mygittu.desktop --version "$VERSION" --install-location / --component-plist packaging/darwin/components.plist --scripts packaging/darwin/pkg-scripts .work/universal/MyGittu-component.pkg
sed "s/@VERSION@/$VERSION/g" packaging/darwin/Distribution.xml > .work/universal/Distribution.xml
productbuild --distribution .work/universal/Distribution.xml --resources packaging/darwin/resources --package-path .work/universal "dist/release/MyGittu-$VERSION-macOS.pkg"
pkgutil --payload-files .work/universal/MyGittu-component.pkg | grep 'Applications/MyGittu.app/Contents/MacOS/desktop'
if [[ -n "$group_flags" ]]; then
  pkgutil --payload-files "$WORK/MyGittu-component.pkg" | grep 'Applications/MyGittu.app/Contents/PlugIns/MyGittuFinder.appex/Contents/MacOS/MyGittuFinder'
fi
