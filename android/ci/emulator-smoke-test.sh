#!/usr/bin/env bash
set -Eeuo pipefail

APK="android/app/build/outputs/apk/debug/app-debug.apk"
PACKAGE="com.xiaoyi.xinyuenovel"
ACTIVITY=".MainActivity"
READY_LOG="/tmp/xinyue-ready.log"
ACTIVITY_LOG="/tmp/xinyue-activity.txt"
READY_TIMEOUT_SECONDS="${XINYUE_READY_TIMEOUT_SECONDS:-60}"

printf '%s\n' '[smoke] Installing APK...'
adb install -r "$APK"

printf '%s\n' '[smoke] Clearing logcat and starting app...'
adb logcat -c
adb shell am force-stop "$PACKAGE"
adb shell am start -n "$PACKAGE/$ACTIVITY"

printf '[smoke] Waiting for JavaScript/UI readiness marker (max %s seconds)...\n' "$READY_TIMEOUT_SECONDS"
ready=0
for ((i=1; i<=READY_TIMEOUT_SECONDS; i++)); do
  adb logcat -d -s 'XinyueNovel:I' '*:S' > "$READY_LOG" 2>&1 || true
  if grep -Fq 'XINYUE_APP_READY' "$READY_LOG"; then
    ready=1
    printf '[smoke] Ready marker received after %s second(s).\n' "$i"
    break
  fi
  sleep 1
done

if [[ "$ready" -ne 1 ]]; then
  printf '[smoke][ERROR] App did not report JavaScript/UI readiness within %s seconds.\n' "$READY_TIMEOUT_SECONDS"
  adb shell dumpsys activity activities > "$ACTIVITY_LOG" 2>&1 || true
  grep -E 'xinyuenovel|mResumedActivity|topResumedActivity' "$ACTIVITY_LOG" || true
  adb logcat -d -t 400 || true
  exit 1
fi

printf '%s\n' '[smoke] Verifying process and foreground activity...'
if ! adb shell pidof "$PACKAGE" > /tmp/xinyue-pid.txt 2>&1; then
  printf '%s\n' '[smoke][ERROR] App process is not running.'
  cat /tmp/xinyue-pid.txt || true
  exit 1
fi

adb shell dumpsys activity activities > "$ACTIVITY_LOG" 2>&1 || true
if ! grep -Fq "$PACKAGE/$ACTIVITY" "$ACTIVITY_LOG"; then
  printf '%s\n' '[smoke][ERROR] MainActivity is not present in activity state.'
  grep -E 'xinyuenovel|mResumedActivity|topResumedActivity' "$ACTIVITY_LOG" || true
  exit 1
fi

printf '%s\n' 'XinyueNovel startup smoke test: PASS'
