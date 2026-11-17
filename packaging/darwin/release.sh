#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/../.."
VERSION="${1:-$(tr -d '\r\n' < VERSION)}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 2
APP="$PWD/dist/macos/root/Applications/MyGittu.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" .work/universal dist/release
for command in desktop agent mygittu; do
  for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -ldflags "-s -w -X mygittu/internal/buildinfo.Version=$VERSION" -o ".work/universal/$command-$arch" "./cmd/$command"
  done
  lipo -create ".work/universal/$command-amd64" ".work/universal/$command-arm64" -output "$APP/Contents/MacOS/$command"
  lipo "$APP/Contents/MacOS/$command" -verify_arch x86_64 arm64
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
for command in agent mygittu; do codesign --force --sign - "$APP/Contents/MacOS/$command"; done
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"
plutil -lint "$APP/Contents/Info.plist"
pkgbuild --root dist/macos/root --identifier dev.mygittu.desktop --version "$VERSION" --install-location / --component-plist packaging/darwin/components.plist .work/universal/MyGittu-component.pkg
sed "s/@VERSION@/$VERSION/g" packaging/darwin/Distribution.xml > .work/universal/Distribution.xml
productbuild --distribution .work/universal/Distribution.xml --resources packaging/darwin/resources --package-path .work/universal "dist/release/MyGittu-$VERSION-macOS.pkg"
pkgutil --payload-files .work/universal/MyGittu-component.pkg | grep 'Applications/MyGittu.app/Contents/MacOS/desktop'
