#!/usr/bin/env bash
set -euo pipefail
APK="${1:-app/build/outputs/apk/debug/app-debug.apk}"
PKG="com.xiaoyi.xinyuenovel"
ACT=".MainActivity"
adb install -r "$APK"
adb logcat -c
adb shell am force-stop "$PKG" || true
adb shell monkey -p "$PKG" -c android.intent.category.LAUNCHER 1 >/dev/null
for i in $(seq 1 60); do
  if adb logcat -d -s XinyueNovel:I '*:S' | grep -q 'XINYUE_APP_READY'; then
    echo 'XinyueNovel startup smoke test: PASS'
    exit 0
  fi
  sleep 1
done
echo 'XinyueNovel startup smoke test: FAIL'
adb logcat -d -t 300
exit 1
