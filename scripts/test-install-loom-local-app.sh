#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
builder="$repo_root/scripts/build-loom-local-app.sh"
installer="$repo_root/scripts/install-loom-local-app.sh"

if [ ! -f "$builder" ] || [ ! -f "$installer" ]; then
  echo "RED: native app build/install transaction is missing" >&2
  exit 1
fi

private_root=$(mktemp -d "${TMPDIR:-/tmp}/loom-native-install.XXXXXX")
trap 'chmod -R u+rwX "$private_root" 2>/dev/null || true; rm -rf "$private_root"' EXIT HUP INT TERM
chmod 700 "$private_root"

source_parent="$private_root/source"
install_parent="$private_root/Applications"
mkdir -m 700 "$source_parent" "$install_parent"
source_app="$source_parent/Loom.app"
destination="$install_parent/Loom.app"

"$builder" --output "$source_app"
before=$(find "$private_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
"$installer" --dry-run --app "$source_app" --destination "$destination"
after=$(find "$private_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
test "$before" = "$after"

"$installer" --app "$source_app" --destination "$destination"
/usr/bin/codesign --verify --strict "$destination"
cmp "$source_app/Contents/MacOS/LoomLocalApp" \
  "$destination/Contents/MacOS/LoomLocalApp"

first_digest=$(find "$destination" -type f -exec shasum -a 256 {} \; |
  LC_ALL=C sort)
replacement="$source_parent/replacement/Loom.app"
mkdir -m 700 "$source_parent/replacement"
cp -R "$source_app" "$replacement"
printf '%s\n' fixture-v2 >"$replacement/Contents/Resources/fixture-version"
chmod 600 "$replacement/Contents/Resources/fixture-version"
/usr/bin/codesign --force --sign - --timestamp=none "$replacement"

"$installer" --app "$replacement" --destination "$destination"
test -f "$destination/Contents/Resources/fixture-version"
test -d "$destination.previous"

controlled_digest() {
  find "$destination" "$destination.previous" -type f \
    -exec shasum -a 256 {} \; | LC_ALL=C sort
}
before_failure=$(controlled_digest)
if LOOM_APP_INSTALL_TEST_FAIL_STEP=after_swap "$installer" \
  --app "$source_app" \
  --destination "$destination" >/dev/null 2>&1
then
  echo "injected app installation failure unexpectedly succeeded" >&2
  exit 1
fi
test "$before_failure" = "$(controlled_digest)"

before_signal=$(controlled_digest)
if LOOM_APP_INSTALL_TEST_SIGNAL_STEP=after_swap "$installer" \
  --app "$source_app" \
  --destination "$destination" >/dev/null 2>&1
then
  echo "injected app installation signal unexpectedly succeeded" >&2
  exit 1
fi
test "$before_signal" = "$(controlled_digest)"

"$installer" --rollback --destination "$destination"
test "$first_digest" = \
  "$(find "$destination" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)"

attack_parent="$private_root/attack"
outside="$private_root/outside"
mkdir -m 700 "$attack_parent" "$outside"
ln -s "$outside" "$attack_parent/Loom.app"
if "$installer" \
  --app "$source_app" \
  --destination "$attack_parent/Loom.app" >/dev/null 2>&1
then
  echo "symlinked app destination was accepted" >&2
  exit 1
fi
test -z "$(find "$outside" -mindepth 1 -print -quit)"

echo "native app installer fixture PASS"
