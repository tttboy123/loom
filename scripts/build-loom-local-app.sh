#!/bin/sh
set -eu

usage() {
  echo "usage: build-loom-local-app.sh --output /absolute/path/Loom.app" >&2
  exit 2
}

output=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output)
      [ "$#" -ge 2 ] || usage
      output=$2
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done

[ -n "$output" ] || usage
case "$output" in
  /*/Loom.app) ;;
  *) usage ;;
esac
[ "$output" = "${output%/}" ] || usage
[ ! -e "$output" ] && [ ! -L "$output" ] || {
  echo "output already exists" >&2
  exit 3
}

effective_uid=$(/usr/bin/id -u)
output_parent=$(/usr/bin/dirname "$output")
[ -d "$output_parent" ] && [ ! -L "$output_parent" ] || {
  echo "invalid output parent" >&2
  exit 3
}
[ "$(/usr/bin/stat -f '%u' "$output_parent")" = "$effective_uid" ] || {
  echo "output parent owner mismatch" >&2
  exit 3
}
parent_mode=$(/usr/bin/stat -f '%Lp' "$output_parent")
[ $(((parent_mode % 10) & 2)) -eq 0 ] || {
  echo "world-writable output parent" >&2
  exit 3
}

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
package_root="$repo_root/apps/macos"
plist_source="$package_root/Resources/Info.plist"
agent_plist_source="$package_root/Resources/com.earendilworks.loom.local.daemon.plist"

for daemon_source in "$repo_root"/cmd/loomd/*.go
do
  case "$daemon_source" in
    *_test.go|*credential_migration*) continue ;;
  esac
  if /usr/bin/grep -q 'NewProductKeychainStore' "$daemon_source"
  then
    echo "implicit Keychain credential boundary" >&2
    exit 3
  fi
done

for scan_root in \
  "$package_root/Sources/LoomLocalApp" \
  "$package_root/Sources/LoomLocalAppCore"
do
  if [ -d "$scan_root" ] && /usr/bin/grep -REq \
    'NSTask|UserDefaults|CacheStore|CoreData|WebView|WKWebView|launchctl|loom-cockpit-bridge|LOOM_WORKSPACE|getenv|ProcessInfo[.]processInfo[.]environment' \
    "$scan_root"
  then
    echo "forbidden native app source surface" >&2
    exit 3
  fi
  if [ -d "$scan_root" ] &&
    /usr/bin/grep -REn 'Process[[:space:]]*[(]' "$scan_root" |
      /usr/bin/grep -Ev '/LocalServiceProcessHost[.]swift:' >/dev/null
  then
    echo "forbidden native app source surface" >&2
    exit 3
  fi
  if [ -d "$scan_root" ] && /usr/bin/grep -REq \
    'sk-[A-Za-z0-9_-]{12,}|BEGIN[[:space:]].*PRIVATE[[:space:]]KEY' \
    "$scan_root"
  then
    echo "forbidden native app source surface" >&2
    exit 3
  fi
done

service_host="$package_root/Sources/LoomLocalApp/LocalServiceProcessHost.swift"
[ -f "$service_host" ] && [ ! -L "$service_host" ] || {
  echo "bundled service host unavailable" >&2
  exit 3
}
test "$(/usr/bin/grep -Ec 'Process[[:space:]]*[(]' "$service_host")" = 1
! /usr/bin/grep -Eq \
  '/bin/(sh|bash|zsh)|launchctl|executableURL[[:space:]]*=.*(URL|user)|arguments[[:space:]]*=.*input' \
  "$service_host"

[ -f "$plist_source" ] && [ ! -L "$plist_source" ] || {
  echo "invalid Info.plist source" >&2
  exit 3
}
[ -f "$agent_plist_source" ] && [ ! -L "$agent_plist_source" ] || {
  echo "invalid LaunchAgent plist source" >&2
  exit 3
}

(
  cd "$package_root"
  /usr/bin/swift build \
    -c release \
    --arch arm64 \
    -Xlinker -reproducible \
    -Xlinker -no_adhoc_codesign
)
bin_path=$(
  cd "$package_root"
  /usr/bin/swift build \
    -c release \
    --arch arm64 \
    -Xlinker -reproducible \
    -Xlinker -no_adhoc_codesign \
    --show-bin-path
)
executable_source="$bin_path/LoomLocalApp"
[ -f "$executable_source" ] && [ ! -L "$executable_source" ] || {
  echo "native executable unavailable" >&2
  exit 3
}

service_path=/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin
for dependency in node codex opencode claude
do
  dependency_path=$(command -v "$dependency" 2>/dev/null || true)
  if [ -n "$dependency_path" ]; then
    dependency_dir=$(/usr/bin/dirname "$dependency_path")
    case ":$service_path:" in
      *":$dependency_dir:"*) ;;
      *) service_path="$dependency_dir:$service_path" ;;
    esac
  fi
done

temporary="$output_parent/.Loom.app.new.$$"
[ ! -e "$temporary" ] && [ ! -L "$temporary" ] || {
  echo "temporary output collision" >&2
  exit 3
}
cleanup() {
  if [ -d "$temporary" ] && [ ! -L "$temporary" ]; then
    /bin/chmod -R u+rwX "$temporary" 2>/dev/null || true
    /bin/rm -rf "$temporary"
  fi
}
trap cleanup EXIT HUP INT TERM

/bin/mkdir -m 700 \
  "$temporary" \
  "$temporary/Contents" \
  "$temporary/Contents/MacOS" \
  "$temporary/Contents/Library" \
  "$temporary/Contents/Library/Helpers" \
  "$temporary/Contents/Library/LaunchAgents" \
  "$temporary/Contents/Resources"
/bin/cp "$plist_source" "$temporary/Contents/Info.plist"
/bin/cp "$executable_source" "$temporary/Contents/MacOS/LoomLocalApp"
/bin/cp \
  "$agent_plist_source" \
  "$temporary/Contents/Library/LaunchAgents/com.earendilworks.loom.local.daemon.plist"
/usr/libexec/PlistBuddy \
  -c "Set :EnvironmentVariables:PATH $service_path" \
  "$temporary/Contents/Library/LaunchAgents/com.earendilworks.loom.local.daemon.plist"
(
  cd "$repo_root"
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
    /usr/bin/env go build \
      -trimpath \
      -buildvcs=false \
      -o "$temporary/Contents/Library/Helpers/loomd" \
      ./cmd/loomd
)
/usr/bin/strip -S "$temporary/Contents/MacOS/LoomLocalApp"
/usr/bin/strip -S "$temporary/Contents/Library/Helpers/loomd"
/bin/chmod 600 "$temporary/Contents/Info.plist"
/bin/chmod 700 "$temporary/Contents/MacOS/LoomLocalApp"
/bin/chmod 700 "$temporary/Contents/Library/Helpers/loomd"
/bin/chmod 600 \
  "$temporary/Contents/Library/LaunchAgents/com.earendilworks.loom.local.daemon.plist"

/usr/bin/codesign \
  --force \
  --sign - \
  --timestamp=none \
  "$temporary/Contents/Library/Helpers/loomd"

/usr/bin/codesign \
  --force \
  --sign - \
  --timestamp=none \
  "$temporary"

find "$temporary" -type d -exec /bin/chmod 700 {} +
find "$temporary" -type f \
  ! -path '*/Contents/MacOS/LoomLocalApp' \
  ! -path '*/Contents/Library/Helpers/loomd' \
  -exec /bin/chmod 600 {} +
/bin/chmod 700 "$temporary/Contents/MacOS/LoomLocalApp"
/bin/chmod 700 "$temporary/Contents/Library/Helpers/loomd"

[ -z "$(find "$temporary" -type l -print -quit)" ] || {
  echo "symlink in built bundle" >&2
  exit 3
}
test "$(
  /usr/libexec/PlistBuddy \
    -c 'Print :CFBundleIdentifier' \
    "$temporary/Contents/Info.plist"
)" = com.earendilworks.loom.local
/usr/bin/lipo -archs "$temporary/Contents/MacOS/LoomLocalApp" |
  /usr/bin/grep -Eq '(^| )arm64( |$)'
/usr/bin/codesign --verify --deep --strict "$temporary"

/bin/mv "$temporary" "$output"
trap - EXIT HUP INT TERM
