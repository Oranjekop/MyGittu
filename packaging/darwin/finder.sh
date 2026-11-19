# Sourced by release.sh after validating the signing configuration.
build_finder_extension() {
  local app="$1" version="$2" work="$3" group="$4" arch
  FINDER_EXTENSION="$app/Contents/PlugIns/MyGittuFinder.appex"
  mkdir -p "$FINDER_EXTENSION/Contents/MacOS"
cat > "$FINDER_EXTENSION/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>dev.mygittu.desktop.finder</string><key>CFBundleExecutable</key><string>MyGittuFinder</string><key>CFBundleName</key><string>MyGittu Finder</string><key>CFBundlePackageType</key><string>XPC!</string><key>CFBundleVersion</key><string>$version</string><key>CFBundleShortVersionString</key><string>$version</string><key>LSMinimumSystemVersion</key><string>13.0</string><key>MyGittuAppGroup</key><string>$group</string>
<key>NSExtension</key><dict><key>NSExtensionPointIdentifier</key><string>com.apple.FinderSync</string><key>NSExtensionPrincipalClass</key><string>FinderSync</string></dict>
</dict></plist>
EOF
cat > "$work/group.entitlements" <<EOF
<?xml version="1.0"?><plist version="1.0"><dict><key>com.apple.security.application-groups</key><array><string>$group</string></array></dict></plist>
EOF
cat > "$work/finder.entitlements" <<EOF
<?xml version="1.0"?><plist version="1.0"><dict><key>com.apple.security.app-sandbox</key><true/><key>com.apple.security.application-groups</key><array><string>$group</string></array></dict></plist>
EOF
  for arch in x86_64 arm64; do
    xcrun swiftc -target "$arch-apple-macosx13.0" -module-name MyGittuFinder \
      -emit-executable -application-extension -framework Cocoa -framework FinderSync \
      -Xlinker -e -Xlinker _NSExtensionMain extensions/darwin/FinderSync.swift \
      -o "$work/finder-$arch"
  done
  local executable="$FINDER_EXTENSION/Contents/MacOS/MyGittuFinder"
  lipo -create "$work/finder-x86_64" "$work/finder-arm64" -output "$executable"
  lipo "$executable" -verify_arch x86_64
  lipo "$executable" -verify_arch arm64
  plutil -lint "$FINDER_EXTENSION/Contents/Info.plist"
}
