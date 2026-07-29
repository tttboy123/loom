#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
builder="$repo_root/scripts/build-loom-local-app.sh"

if [ ! -f "$builder" ]; then
  echo "RED: native app builder is missing" >&2
  exit 1
fi
! grep -Eq -- '-no_uuid|-random_uuid' "$builder"
test "$(grep -c -- '-Xlinker -reproducible' "$builder")" = 2

private_root=$(mktemp -d "${TMPDIR:-/tmp}/loom-native-build.XXXXXX")
chmod 700 "$private_root"
smoke_pid=
cleanup() {
  if [ -n "$smoke_pid" ]; then
    kill -TERM "$smoke_pid" 2>/dev/null || true
    wait "$smoke_pid" 2>/dev/null || true
    smoke_pid=
  fi
  chmod -R u+rwX "$private_root" 2>/dev/null || true
  rm -rf "$private_root"
}
trap cleanup EXIT HUP INT TERM

crash_report_inventory() {
  reports_root=${HOME:?}/Library/Logs/DiagnosticReports
  if [ ! -d "$reports_root" ]; then
    return 0
  fi
  find "$reports_root" \
    -maxdepth 1 \
    -type f \
    -name 'LoomLocalApp-*.ips' \
    -print0 |
    sort -z |
    while IFS= read -r -d '' report
    do
      report_name=${report##*/}
      report_digest=$(shasum -a 256 "$report" | awk '{print $1}')
      printf '%s %s\n' "$report_name" "$report_digest"
    done
}

record_crash_inventory() {
  inventory_path=$1
  crash_report_inventory >"$inventory_path"
  chmod 600 "$inventory_path"
}

assert_no_new_crash_report() {
  before_path=$1
  label=$2
  after_path="$private_root/$label-crash-reports-after"
  record_crash_inventory "$after_path"
  if ! cmp -s "$before_path" "$after_path"; then
    echo "unexpected LoomLocalApp crash report after $label" >&2
    comm -13 "$before_path" "$after_path" >&2 || true
    exit 1
  fi
}

first="$private_root/first/Loom.app"
second="$private_root/second/Loom.app"
mkdir -m 700 "$private_root/first" "$private_root/second"
red_crash_inventory="$private_root/red-crash-reports-before"
record_crash_inventory "$red_crash_inventory"

shadow="$private_root/shadow"
mkdir -m 700 \
  "$shadow" \
  "$shadow/scripts" \
  "$shadow/apps" \
  "$shadow/apps/macos" \
  "$shadow/apps/macos/Sources" \
  "$shadow/apps/macos/Sources/LoomLocalApp"
cp "$builder" "$shadow/scripts/build-loom-local-app.sh"
chmod 700 "$shadow/scripts/build-loom-local-app.sh"
printf '%s\n' 'let forbidden = Process()' \
  >"$shadow/apps/macos/Sources/LoomLocalApp/Forbidden.swift"
chmod 600 "$shadow/apps/macos/Sources/LoomLocalApp/Forbidden.swift"
if "$shadow/scripts/build-loom-local-app.sh" \
  --output "$shadow/Loom.app" \
  >"$private_root/forbidden-build-output" 2>&1
then
  echo "builder accepted forbidden native source" >&2
  exit 1
fi
grep -q 'forbidden native app source surface' \
  "$private_root/forbidden-build-output"

(
  cd "$repo_root/apps/macos"
  /usr/bin/swift package reset
)
"$builder" --output "$first"
(
  cd "$repo_root/apps/macos"
  /usr/bin/swift package reset
)
"$builder" --output "$second"

first_uuid=
second_uuid=
for bundle in "$first" "$second"
do
  test -d "$bundle"
  test ! -L "$bundle"
  test "$(stat -f '%Lp' "$bundle")" = 700
  test "$(stat -f '%Lp' "$bundle/Contents/MacOS/LoomLocalApp")" = 700
  test "$(stat -f '%Lp' "$bundle/Contents/Info.plist")" = 600
  test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$bundle/Contents/Info.plist")" = com.earendilworks.loom.local
  test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleName' "$bundle/Contents/Info.plist")" = Loom
  /usr/bin/codesign --verify --strict "$bundle"
  /usr/bin/lipo -archs "$bundle/Contents/MacOS/LoomLocalApp" |
    grep -Eq '(^| )arm64( |$)'
  uuid_output=$(
    /usr/bin/dwarfdump --uuid "$bundle/Contents/MacOS/LoomLocalApp"
  )
  if [ -z "$uuid_output" ]; then
    assert_no_new_crash_report "$red_crash_inventory" red
    echo "native app executable is missing LC_UUID" >&2
    exit 1
  fi
  test "$(printf '%s\n' "$uuid_output" | wc -l | tr -d ' ')" = 1
  uuid_value=$(printf '%s\n' "$uuid_output" | awk '{print $2}')
  printf '%s\n' "$uuid_value" |
    grep -Eq '^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$'
  test "$uuid_value" != 00000000-0000-0000-0000-000000000000
  test "$(
    /usr/bin/nm -pa "$bundle/Contents/MacOS/LoomLocalApp" |
      awk '$2 == "-" { count++ } END { print count + 0 }'
  )" = 0
  case "$bundle" in
    "$first") first_uuid=$uuid_value ;;
    "$second") second_uuid=$uuid_value ;;
    *) exit 1 ;;
  esac
  ! find "$bundle" -type l -print | grep .
done

test "$first_uuid" = "$second_uuid"
test "$(shasum -a 256 "$first/Contents/MacOS/LoomLocalApp" | awk '{print $1}')" = \
  "$(shasum -a 256 "$second/Contents/MacOS/LoomLocalApp" | awk '{print $1}')"
test "$(shasum -a 256 "$first/Contents/Info.plist" | awk '{print $1}')" = \
  "$(shasum -a 256 "$second/Contents/Info.plist" | awk '{print $1}')"
bundle_manifest_digest() {
  bundle=$1
  find "$bundle" -type f -print0 |
    sort -z |
    while IFS= read -r -d '' file
    do
      relative_path=${file#"$bundle/"}
      mode=$(stat -f '%Sp' "$file")
      digest=$(shasum -a 256 "$file" | awk '{print $1}')
      printf '%s\000%s\000%s\n' "$relative_path" "$mode" "$digest"
    done |
    shasum -a 256 |
    awk '{print $1}'
}
first_manifest_digest=$(bundle_manifest_digest "$first")
second_manifest_digest=$(bundle_manifest_digest "$second")
test "$first_manifest_digest" = "$second_manifest_digest"
test "$(
  find "$first" -type f -exec shasum -a 256 {} \; |
    sed "s#  $first/#  #" |
    LC_ALL=C sort
)" = "$(
  find "$second" -type f -exec shasum -a 256 {} \; |
    sed "s#  $second/#  #" |
  LC_ALL=C sort
)"

smoke_crash_inventory="$private_root/smoke-crash-reports-before"
record_crash_inventory "$smoke_crash_inventory"
smoke_output="$private_root/native-launch-smoke-output"
: >"$smoke_output"
chmod 600 "$smoke_output"
"$first/Contents/MacOS/LoomLocalApp" >"$smoke_output" 2>&1 &
smoke_pid=$!
sleep 1
if ! kill -0 "$smoke_pid" 2>/dev/null; then
  wait "$smoke_pid" 2>/dev/null || true
  smoke_pid=
  assert_no_new_crash_report "$smoke_crash_inventory" smoke
  echo "native app did not survive the launch smoke window" >&2
  exit 1
fi
smoke_command=$(ps -p "$smoke_pid" -o command=)
test "$smoke_command" = "$first/Contents/MacOS/LoomLocalApp"
kill -TERM "$smoke_pid"
wait "$smoke_pid" 2>/dev/null || true
smoke_pid=
assert_no_new_crash_report "$smoke_crash_inventory" smoke

app_sources="$repo_root/apps/macos/Sources/LoomLocalApp"
core_sources="$repo_root/apps/macos/Sources/LoomLocalAppCore"
! grep -REn \
  'Process[[:space:]]*[(]|NSTask|UserDefaults|CacheStore|CoreData|WebView|WKWebView|launchctl|loom-cockpit-bridge|LOOM_WORKSPACE' \
  "$app_sources" "$core_sources"
! grep -REn \
  'getenv|ProcessInfo\\.processInfo\\.environment' \
  "$app_sources"
test "$(grep -c 'unsetenv(name)' "$app_sources/LoomLocalApp.swift")" = 1

echo "native app build fixture PASS"
