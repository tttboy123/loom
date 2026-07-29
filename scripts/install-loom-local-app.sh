#!/bin/sh
set -eu

usage() {
  echo "usage: install-loom-local-app.sh [--dry-run] --app PATH --destination /absolute/path/Loom.app" >&2
  echo "       install-loom-local-app.sh --rollback --destination /absolute/path/Loom.app" >&2
  exit 2
}

dry_run=0
rollback=0
source_app=
destination=

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      dry_run=1
      shift
      ;;
    --rollback)
      rollback=1
      shift
      ;;
    --app)
      [ "$#" -ge 2 ] || usage
      source_app=$2
      shift 2
      ;;
    --destination)
      [ "$#" -ge 2 ] || usage
      destination=$2
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done
[ -n "$destination" ] || usage
case "$destination" in
  /*/Loom.app) ;;
  *) usage ;;
esac
[ "$destination" = "${destination%/}" ] || usage

effective_uid=$(/usr/bin/id -u)
destination_parent=$(/usr/bin/dirname "$destination")
[ -d "$destination_parent" ] && [ ! -L "$destination_parent" ] || {
  echo "invalid destination parent" >&2
  exit 3
}
[ "$(/usr/bin/stat -f '%u' "$destination_parent")" = "$effective_uid" ] || {
  echo "destination parent owner mismatch" >&2
  exit 3
}
parent_mode=$(/usr/bin/stat -f '%Lp' "$destination_parent")
[ $(((parent_mode % 10) & 2)) -eq 0 ] || {
  echo "world-writable destination parent" >&2
  exit 3
}

previous="$destination.previous"
for controlled in "$destination" "$previous"
do
  [ ! -L "$controlled" ] || {
    echo "symlinked app target" >&2
    exit 3
  }
  if [ -e "$controlled" ]; then
    [ -d "$controlled" ] || {
      echo "invalid app target" >&2
      exit 3
    }
    [ "$(/usr/bin/stat -f '%u' "$controlled")" = "$effective_uid" ] || {
      echo "app target owner mismatch" >&2
      exit 3
    }
  fi
done
if [ -d "$previous" ] && [ ! -d "$destination" ]; then
  echo "rollback app exists without current app" >&2
  exit 3
fi

validate_bundle() {
  bundle=$1
  [ -d "$bundle" ] && [ ! -L "$bundle" ] || return 1
  [ -z "$(find "$bundle" -type l -print -quit)" ] || return 1
  [ "$(/usr/bin/stat -f '%u' "$bundle")" = "$effective_uid" ] || return 1
  executable="$bundle/Contents/MacOS/LoomLocalApp"
  plist="$bundle/Contents/Info.plist"
  [ -f "$executable" ] && [ ! -L "$executable" ] || return 1
  [ -f "$plist" ] && [ ! -L "$plist" ] || return 1
  [ "$(/usr/bin/stat -f '%Lp' "$bundle")" = 700 ] || return 1
  [ "$(/usr/bin/stat -f '%Lp' "$executable")" = 700 ] || return 1
  [ "$(/usr/bin/stat -f '%Lp' "$plist")" = 600 ] || return 1
  [ -z "$(
    find "$bundle" -exec /usr/bin/stat -f '%u' {} \; |
      /usr/bin/grep -Ev "^${effective_uid}$" || true
  )" ] || return 1
  [ -z "$(
    find "$bundle" -type d -exec /usr/bin/stat -f '%Lp' {} \; |
      /usr/bin/grep -Ev '^700$' || true
  )" ] || return 1
  [ -z "$(
    find "$bundle" -type f ! -path "$executable" \
      -exec /usr/bin/stat -f '%Lp' {} \; |
      /usr/bin/grep -Ev '^600$' || true
  )" ] || return 1
  [ "$(
    /usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$plist"
  )" = com.earendilworks.loom.local ] || return 1
  /usr/bin/lipo -archs "$executable" |
    /usr/bin/grep -Eq '(^| )arm64( |$)' || return 1
  /usr/bin/codesign --verify --strict "$bundle" || return 1
}

transaction="$destination_parent/.Loom.app.transaction.$$"
incoming="$destination_parent/.Loom.app.incoming.$$"
transaction_mode=none
had_current=0
had_previous=0
new_installed=0

restore_active_transaction() {
  case "$transaction_mode" in
    install)
      if [ -d "$transaction/current" ] &&
        [ ! -L "$transaction/current" ]
      then
        if [ -d "$destination" ] && [ ! -L "$destination" ]; then
          /bin/chmod -R u+rwX "$destination" 2>/dev/null || true
          /bin/rm -rf "$destination"
        fi
        /bin/mv "$transaction/current" "$destination" || true
      elif [ "$had_current" -eq 0 ] &&
        [ "$new_installed" -eq 1 ] &&
        [ -d "$destination" ] &&
        [ ! -L "$destination" ]
      then
        /bin/chmod -R u+rwX "$destination" 2>/dev/null || true
        /bin/rm -rf "$destination"
      fi
      if [ -d "$transaction/previous" ] &&
        [ ! -L "$transaction/previous" ] &&
        [ ! -e "$previous" ] &&
        [ ! -L "$previous" ]
      then
        /bin/mv "$transaction/previous" "$previous" || true
      fi
      ;;
    rollback)
      if [ -d "$transaction/current" ] &&
        [ -d "$destination" ] &&
        [ ! -L "$destination" ] &&
        [ ! -e "$previous" ] &&
        [ ! -L "$previous" ]
      then
        /bin/mv "$destination" "$previous" || true
      fi
      if [ -d "$transaction/current" ] &&
        [ ! -L "$transaction/current" ]
      then
        /bin/mv "$transaction/current" "$destination" || true
      fi
      ;;
  esac
  transaction_mode=none
}

cleanup() {
  if [ "$transaction_mode" != none ]; then
    restore_active_transaction
  fi
  if [ -d "$incoming" ] && [ ! -L "$incoming" ]; then
    /bin/chmod -R u+rwX "$incoming" 2>/dev/null || true
    /bin/rm -rf "$incoming"
  fi
  if [ -d "$transaction" ] && [ ! -L "$transaction" ]; then
    /bin/chmod -R u+rwX "$transaction" 2>/dev/null || true
    /bin/rm -rf "$transaction"
  fi
}
handle_signal() {
  trap '' HUP INT TERM
  restore_active_transaction
  cleanup
  trap - EXIT
  exit 98
}
trap cleanup EXIT
trap handle_signal HUP INT TERM

if [ "$rollback" -eq 1 ]; then
  [ "$dry_run" -eq 0 ] && [ -z "$source_app" ] || usage
  validate_bundle "$destination" && validate_bundle "$previous" || {
    echo "rollback unavailable" >&2
    exit 3
  }
  /bin/mkdir -m 700 "$transaction"
  transaction_mode=rollback
  /bin/mv "$destination" "$transaction/current"
  /bin/mv "$previous" "$destination"
  /bin/mv "$transaction/current" "$previous"
  transaction_mode=none
  /bin/rmdir "$transaction"
  trap - EXIT HUP INT TERM
  exit 0
fi

[ -n "$source_app" ] || usage
case "$source_app" in
  /*/Loom.app) ;;
  *) usage ;;
esac
validate_bundle "$source_app" || {
  echo "invalid source app" >&2
  exit 3
}

if [ "$dry_run" -eq 1 ]; then
  exit 0
fi

/bin/cp -pR "$source_app" "$incoming"
validate_bundle "$incoming" || {
  echo "copied app failed validation" >&2
  exit 3
}

/bin/mkdir -m 700 "$transaction"
transaction_mode=install
if [ -d "$destination" ]; then
  /bin/mv "$destination" "$transaction/current"
  had_current=1
fi
if [ -d "$previous" ]; then
  /bin/mv "$previous" "$transaction/previous"
  had_previous=1
fi

if ! /bin/mv "$incoming" "$destination"; then
  exit 3
fi
new_installed=1
if [ "${LOOM_APP_INSTALL_TEST_FAIL_STEP:-}" = after_swap ]; then
  echo "injected app installation failure" >&2
  exit 97
fi
if [ "${LOOM_APP_INSTALL_TEST_SIGNAL_STEP:-}" = after_swap ]; then
  kill -TERM "$$"
  exit 98
fi
if ! validate_bundle "$destination"; then
  echo "installed app failed validation" >&2
  exit 3
fi

if [ "$had_current" -eq 1 ]; then
  /bin/mv "$transaction/current" "$previous"
fi
transaction_mode=none
if [ "$had_previous" -eq 1 ] && [ -d "$transaction/previous" ]; then
  /bin/chmod -R u+rwX "$transaction/previous"
  /bin/rm -rf "$transaction/previous"
fi
/bin/rmdir "$transaction"
trap - EXIT HUP INT TERM
