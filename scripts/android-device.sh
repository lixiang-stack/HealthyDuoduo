#!/usr/bin/env bash
# Small adb helper for manual Android smoke tests.
#
# Usage:
#   bash scripts/android-device.sh screenshot <path>
#   bash scripts/android-device.sh dump
#   bash scripts/android-device.sh tap-testid <testID>
#   bash scripts/android-device.sh tap-text <text>
#   bash scripts/android-device.sh reverse <port>
#   bash scripts/android-device.sh keyevent <code>
#   bash scripts/android-device.sh text <text>
#
# ADB defaults to the Android SDK platform-tools path; override with ADB=...
set -euo pipefail

ADB=${ADB:-$HOME/Library/Android/sdk/platform-tools/adb}
if ! command -v "$ADB" >/dev/null 2>&1; then
  echo "android-device: adb not found at $ADB" >&2
  exit 1
fi

command=${1:-}
case "$command" in
  screenshot)
    [ -n "${2:-}" ] || { echo "usage: $0 screenshot <path>" >&2; exit 2; }
    "$ADB" exec-out screencap -p > "$2"
    echo "$2"
    ;;
  dump)
    "$ADB" shell uiautomator dump /sdcard/window.xml >/dev/null 2>&1
    "$ADB" shell cat /sdcard/window.xml
    ;;
  tap-testid|tap-text)
    [ -n "${2:-}" ] || { echo "usage: $0 $command <value>" >&2; exit 2; }
    tmp=$(mktemp "${TMPDIR:-/tmp}/android-device.XXXXXX")
    trap 'rm -f "$tmp"' EXIT
    "$ADB" shell uiautomator dump /sdcard/window.xml >/dev/null 2>&1
    "$ADB" shell cat /sdcard/window.xml > "$tmp"
    coords=$(python3 - "$tmp" "$command" "$2" <<'PY'
import re
import sys
import xml.etree.ElementTree as ET

path, mode, value = sys.argv[1:4]
root = ET.parse(path).getroot()
target = None
for node in root.iter():
    if mode == "tap-testid" and node.get("resource-id") == value:
        target = node
        break
    if mode == "tap-text" and value in (node.get("text"), node.get("content-desc")):
        target = node
        break
if target is None:
    sys.exit(1)
match = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", target.get("bounds", ""))
if not match:
    sys.exit(1)
x1, y1, x2, y2 = map(int, match.groups())
print((x1 + x2) // 2, (y1 + y2) // 2)
PY
) || {
      echo "android-device: $command target not found: $2" >&2
      exit 1
    }
    read -r x y <<< "$coords"
    "$ADB" shell input tap "$x" "$y"
    echo "tapped $2 at $x,$y"
    ;;
  reverse)
    [ -n "${2:-}" ] || { echo "usage: $0 reverse <port>" >&2; exit 2; }
    "$ADB" reverse "tcp:$2" "tcp:$2"
    echo "reversed tcp:$2"
    ;;
  keyevent)
    [ -n "${2:-}" ] || { echo "usage: $0 keyevent <code>" >&2; exit 2; }
    "$ADB" shell input keyevent "$2"
    ;;
  text)
    [ -n "${2:-}" ] || { echo "usage: $0 text <text>" >&2; exit 2; }
    "$ADB" shell input text "$2"
    ;;
  *)
    echo "usage: $0 {screenshot <path>|dump|tap-testid <id>|tap-text <text>|reverse <port>|keyevent <code>|text <text>}" >&2
    exit 2
    ;;
esac
