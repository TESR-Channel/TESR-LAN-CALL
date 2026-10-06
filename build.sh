#!/usr/bin/env bash
# สร้างแพ็กเกจติดตั้งทุกระบบในครั้งเดียว (รันบน Linux)
# ต้องมี: go 1.22+, nsis (makensis), llvm (llvm-lipo), dpkg-deb, zip
#   Ubuntu: sudo apt install -y golang-go nsis llvm zip
#   และ python3 + pillow (pip install pillow)
# ผลลัพธ์: ../dist/TESR-LAN-Call-v<ver>/  และ  ../dist/TESR-LAN-Call-v<ver>.zip
set -euo pipefail
cd "$(dirname "$0")"
SRC="$PWD"
VER=$(sed -n 's/^\s*version\s*=\s*"\(.*\)"/\1/p' main.go | head -1)
DIST="${DIST:-$(cd .. && pwd)/dist}"
NAME="TESR-LAN-Call-v$VER"
OUT="$DIST/$NAME"
B="$SRC/build"
rm -rf "$B" "$OUT" "$DIST/$NAME.zip"
mkdir -p "$B" "$OUT"/{For-Windows,For-Mac,For-Linux,For-Raspberry-Pi,For-iPad-Mobile,For-Robot,Resources,Source-Code}
export CGO_ENABLED=0
[ -d vendor ] && export GOFLAGS=-mod=vendor

echo "== เตรียมฟอนต์ โลโก้ ไอคอน"
FONTS=assets/fonts
for f in chakrapetch/ChakraPetch-SemiBold.ttf chakrapetch/ChakraPetch-Bold.ttf \
         ibmplexsansthai/IBMPlexSansThai-Regular.ttf ibmplexsansthai/IBMPlexSansThai-SemiBold.ttf \
         ibmplexsansthai/IBMPlexSansThai-Bold.ttf; do
  [ -f "$FONTS/$(basename "$f")" ] || curl -fsSL -o "$FONTS/$(basename "$f")" "https://raw.githubusercontent.com/google/fonts/main/ofl/$f"
done
python3 tools/make_placeholder_logo.py          # สร้างเฉพาะไฟล์ที่ยังไม่มี
python3 tools/make_icons.py                     # ไอคอนทุกขนาดจาก assets/icon-1024.png
SYSO=packaging/windows/rsrc_windows_amd64.syso  # ไอคอนที่ฝังใน .exe
if ! command -v rsrc >/dev/null; then
  GOFLAGS= go install github.com/akavel/rsrc@v0.10.2
  export PATH="$PATH:$(go env GOPATH)/bin"
fi
rsrc -ico assets/icon.ico -arch amd64 -o "$SYSO"

gobuild() { # os arch out [goarm] [extra ldflags]
  GOOS=$1 GOARCH=$2 GOARM=${4:-} go build -trimpath -ldflags "-s -w ${5:-}" -o "$3" .
}

# แปลงคู่มือเป็นไฟล์ .txt ที่เปิดบน Windows/Mac/Linux ได้ (UTF-8 BOM + CRLF)
guide() { # source dest
  { printf '\xEF\xBB\xBF'; sed "s/__VERSION__/$VER/g; s/\$/\r/" "packaging/guides/$1"; } > "$2"
}

echo "== Windows"
mkdir -p "$B/win"
cp packaging/windows/rsrc_windows_amd64.syso .
gobuild windows amd64 "$B/win/TESR-LAN-Call.exe" "" "-H windowsgui"
rm -f rsrc_windows_amd64.syso
cp assets/icon.ico assets/installer-side.bmp assets/installer-header.bmp "$B/win/"
makensis -V2 -DVERSION="$VER" -DSRC="$B/win" -DOUT="$OUT/For-Windows/TESR-LAN-Call-Setup.exe" packaging/windows/installer.nsi
guide windows.txt "$OUT/For-Windows/วิธีติดตั้ง.txt"

echo "== macOS (Universal: Apple Silicon + Intel)"
APP="$B/mac/TESR LAN Call.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
gobuild darwin amd64 "$B/mac/amd64"
gobuild darwin arm64 "$B/mac/arm64"
LIPO=$(command -v lipo || command -v llvm-lipo || ls /usr/bin/llvm-lipo-* 2>/dev/null | head -1)
"$LIPO" -create -output "$APP/Contents/MacOS/TESR-LAN-Call" "$B/mac/amd64" "$B/mac/arm64"
chmod 755 "$APP/Contents/MacOS/TESR-LAN-Call"
sed "s/__VERSION__/$VER/g" packaging/macos/Info.plist > "$APP/Contents/Info.plist"
cp assets/icon.icns "$APP/Contents/Resources/icon.icns"
printf 'APPL????' > "$APP/Contents/PkgInfo"
(cd "$B/mac" && zip -qry "$OUT/For-Mac/TESR-LAN-Call-Mac.zip" "TESR LAN Call.app")
guide mac.txt "$OUT/For-Mac/วิธีติดตั้ง.txt"

echo "== Linux / Raspberry Pi (.deb)"
mkdeb() { # debarch goarch goarm destdir
  local P="$B/deb-$1"
  mkdir -p "$P/DEBIAN" "$P/opt/tesr-lan-call" "$P/usr/bin" "$P/usr/share/applications" "$P/usr/share/pixmaps"
  gobuild linux "$2" "$P/opt/tesr-lan-call/tesr-lan-call" "$3"
  ln -s /opt/tesr-lan-call/tesr-lan-call "$P/usr/bin/tesr-lan-call"
  for s in 512 256 128 64 32; do
    mkdir -p "$P/usr/share/icons/hicolor/${s}x${s}/apps"
    cp "assets/icon-$s.png" "$P/usr/share/icons/hicolor/${s}x${s}/apps/tesr-lan-call.png"
  done
  cp assets/icon-256.png "$P/usr/share/pixmaps/tesr-lan-call.png"
  cat > "$P/usr/share/applications/tesr-lan-call.desktop" << DESK
[Desktop Entry]
Type=Application
Name=TESR LAN Call
Comment=โทรวิดีโอภายในองค์กรผ่าน LAN
Exec=/opt/tesr-lan-call/tesr-lan-call
Icon=tesr-lan-call
Terminal=false
Categories=Network;VideoConference;
StartupNotify=false
DESK
  cat > "$P/DEBIAN/control" << CTRL
Package: tesr-lan-call
Version: $VER
Architecture: $1
Maintainer: TESR Co., Ltd. <support@tesrshop.com>
Recommends: chromium | chromium-browser | google-chrome-stable
Section: net
Priority: optional
Homepage: https://tesrshop.com
Description: TESR LAN Call - video call on the local network
 Video call between computers, Raspberry Pi, robots, phones and tablets
 on the same LAN. No internet or account needed.
CTRL
  cat > "$P/DEBIAN/postinst" << 'POST'
#!/bin/sh
set -e
command -v update-desktop-database >/dev/null && update-desktop-database -q || true
command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q -f /usr/share/icons/hicolor || true
# วางไอคอนบน Desktop ของผู้ใช้ทุกคนในเครื่อง
getent passwd | while IFS=: read -r u _ uid _ _ home _; do
  [ "$uid" -ge 1000 ] 2>/dev/null && [ "$uid" -lt 60000 ] && [ -d "$home" ] || continue
  d=$(runuser -u "$u" -- xdg-user-dir DESKTOP 2>/dev/null || echo "$home/Desktop")
  [ -d "$d" ] || continue
  install -o "$u" -m 755 /usr/share/applications/tesr-lan-call.desktop "$d/tesr-lan-call.desktop"
  runuser -u "$u" -- gio set "$d/tesr-lan-call.desktop" metadata::trusted true 2>/dev/null || true
done
# เปิดไฟร์วอลล์ถ้าใช้ ufw
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow 47800/tcp >/dev/null; ufw allow 47843/tcp >/dev/null; ufw allow 47801/udp >/dev/null
fi
exit 0
POST
  cat > "$P/DEBIAN/postrm" << 'RM'
#!/bin/sh
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
  for f in /home/*/Desktop/tesr-lan-call.desktop /home/*/*/tesr-lan-call.desktop; do [ -f "$f" ] && rm -f "$f"; done
fi
exit 0
RM
  chmod 755 "$P/DEBIAN/postinst" "$P/DEBIAN/postrm"
  dpkg-deb --build --root-owner-group "$P" "$OUT/$4/tesr-lan-call_${VER}_$1.deb" >/dev/null
}
mkdeb amd64 amd64 "" For-Linux
mkdeb arm64 arm64 "" For-Raspberry-Pi
mkdeb armhf arm 7 For-Raspberry-Pi
guide linux.txt "$OUT/For-Linux/วิธีติดตั้ง.txt"
guide pi.txt "$OUT/For-Raspberry-Pi/วิธีติดตั้ง.txt"

echo "== iPad / Mobile, Robot, Resources"
guide ipad.txt "$OUT/For-iPad-Mobile/วิธีใช้.txt"
guide robot.txt "$OUT/For-Robot/วิธีตั้งค่า.txt"
guide resources.txt "$OUT/Resources/อ่านก่อน.txt"
cp assets/icon.ico assets/icon.icns assets/icon-*.png assets/logo.png assets/installer-*.bmp "$OUT/Resources/"
guide 00-README.txt "$OUT/อ่านก่อน-README.txt"

echo "== Source code"
tar --exclude=./build --exclude=./dist --exclude=./.git -cf - . | (cd "$OUT/Source-Code" && tar -xf -)

rm -f "$DIST/$NAME.zip" && python3 tools/zipdir.py "$OUT" "$DIST/$NAME.zip"
echo "เสร็จแล้ว: $DIST/$NAME.zip"
