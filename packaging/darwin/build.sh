#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/../.."
: "${MYGITTU_APP_GROUP:?Set MYGITTU_APP_GROUP to your provisioned app group identifier}"
: "${SIGN_IDENTITY:?Set SIGN_IDENTITY to your Developer ID Application identity}"
APP="$PWD/bin/MyGittu.app"
EXT="$APP/Contents/PlugIns/MyGittuFinder.appex"
mkdir -p "$APP/Contents/MacOS" "$EXT/Contents/MacOS" .work/darwin
mkdir -p "$APP/Contents/Resources"
cp packaging/darwin/MyGittu.icns "$APP/Contents/Resources/MyGittu.icns"
GOOS=darwin GOARCH=arm64 go build -o "$APP/Contents/MacOS/desktop" ./cmd/desktop
GOOS=darwin GOARCH=arm64 go build -o "$APP/Contents/MacOS/agent" ./cmd/agent
go build -o bin/mygittu ./cmd/mygittu
cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>dev.mygittu.desktop</string><key>CFBundleExecutable</key><string>desktop</string><key>CFBundleName</key><string>MyGittu</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleVersion</key><string>0.0.1</string><key>CFBundleShortVersionString</key><string>0.0.1</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>NSHighResolutionCapable</key><true/>
<key>LSEnvironment</key><dict><key>MYGITTU_APP_GROUP</key><string>$MYGITTU_APP_GROUP</string></dict>
<key>CFBundleIconFile</key><string>MyGittu.icns</string>
</dict></plist>
EOF
cat > "$EXT/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>dev.mygittu.desktop.finder</string><key>CFBundleExecutable</key><string>MyGittuFinder</string><key>CFBundleName</key><string>MyGittu Finder</string><key>CFBundlePackageType</key><string>XPC!</string><key>CFBundleVersion</key><string>0.0.1</string><key>CFBundleShortVersionString</key><string>0.0.1</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>MyGittuAppGroup</key><string>$MYGITTU_APP_GROUP</string>
<key>NSExtension</key><dict><key>NSExtensionPointIdentifier</key><string>com.apple.FinderSync</string><key>NSExtensionPrincipalClass</key><string>FinderSync</string></dict>
</dict></plist>
EOF
cat > .work/darwin/host.entitlements <<EOF
<?xml version="1.0"?><plist version="1.0"><dict><key>com.apple.security.application-groups</key><array><string>$MYGITTU_APP_GROUP</string></array><key>com.apple.security.cs.allow-jit</key><true/></dict></plist>
EOF
cat > .work/darwin/finder.entitlements <<EOF
<?xml version="1.0"?><plist version="1.0"><dict><key>com.apple.security.app-sandbox</key><true/><key>com.apple.security.application-groups</key><array><string>$MYGITTU_APP_GROUP</string></array></dict></plist>
EOF
xcrun swiftc -target arm64-apple-macosx13.0 -module-name MyGittuFinder -emit-executable -application-extension -framework Cocoa -framework FinderSync -Xlinker -e -Xlinker _NSExtensionMain extensions/darwin/FinderSync.swift -o "$EXT/Contents/MacOS/MyGittuFinder"
codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" --entitlements .work/darwin/host.entitlements "$APP/Contents/MacOS/agent"
codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" --entitlements .work/darwin/finder.entitlements "$EXT"
codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" --entitlements .work/darwin/host.entitlements "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
echo "Built $APP. Provisioning profile, notarization and Finder activation must be validated on macOS."
