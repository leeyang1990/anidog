#!/usr/bin/env bash
# 与 CI 共用的原生打包入口；版本同时写入后端与应用包元数据。
set -euo pipefail
release_tag="${1:?usage: package-desktop.sh vX.Y.Z darwin|windows|linux arm64|amd64}"
desktop_os="${2:?missing OS}"
desktop_arch="${3:?missing architecture}"
[[ "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 1
[[ "$desktop_arch" = arm64 || "$desktop_arch" = amd64 ]] || exit 1
[[ "$desktop_os" = darwin || "$desktop_os" = windows || "$desktop_os" = linux ]] || exit 1
repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_dir/backend/cmd/anidog-desktop"
config_backup="$(mktemp)"
cp wails.json "$config_backup"
trap 'cp "$config_backup" wails.json; rm -f "$config_backup"' EXIT
export RELEASE_TAG="$release_tag"
node <<'JS'
const fs = require('fs');
const config = JSON.parse(fs.readFileSync('wails.json', 'utf8'));
config.info.productVersion = process.env.RELEASE_TAG.slice(1);
fs.writeFileSync('wails.json', JSON.stringify(config, null, 2) + '\n');
JS
build_tags=desktop,nosqlite
[[ "$desktop_os" != linux ]] || build_tags+=,webkit2_41
wails_bin="$(go env GOPATH)/bin/wails"
[[ "$desktop_os" != windows ]] || wails_bin+=.exe
# 前端通过 window.go 调用桥接，不需要生成 JS wrapper；避免绑定生成执行主程序。
"$wails_bin" build -clean -skipbindings -platform "$desktop_os/$desktop_arch" -tags "$build_tags" \
  -ldflags "-X github.com/anidog/anidog-go/internal/config.Version=$release_tag"
mkdir -p "$repo_dir/release"
case "$desktop_os" in
  darwin)
    app_path="$PWD/build/bin/AniDog.app"
    test -s "$app_path/Contents/MacOS/AniDog"
    test "$(/usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' "$app_path/Contents/Info.plist")" = "${release_tag#v}"
    expected_arch="$desktop_arch"
    [[ "$desktop_arch" != amd64 ]] || expected_arch=x86_64
    lipo "$app_path/Contents/MacOS/AniDog" -verify_arch "$expected_arch"
    codesign --verify --deep --strict "$app_path"
    if otool -L "$app_path/Contents/MacOS/AniDog" | grep -E '/opt/homebrew/|/usr/local/'; then
      echo 'Application depends on non-portable local libraries' >&2
      exit 1
    fi
    ditto -c -k --sequesterRsrc --keepParent "$app_path" \
      "$repo_dir/release/AniDog-$release_tag-macos-$desktop_arch.zip"
    ;;
  windows)
    test -s build/bin/AniDog.exe
    (cd build/bin && 7z a -tzip "$repo_dir/release/AniDog-$release_tag-windows-$desktop_arch.zip" AniDog.exe)
    ;;
  linux)
    test -x build/bin/AniDog
    ldd build/bin/AniDog
    if ldd build/bin/AniDog | grep 'not found'; then exit 1; fi
    cp "$repo_dir/docs/desktop-install.md" build/bin/README.md
    tar -C build/bin -czf "$repo_dir/release/AniDog-$release_tag-linux-$desktop_arch.tar.gz" AniDog README.md
    ;;
esac
