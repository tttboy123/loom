#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
installer="$repo_root/scripts/install-loom-local-product.sh"

if [ ! -f "$installer" ]; then
  echo "RED: local product installer is missing" >&2
  exit 1
fi

private_root=$(mktemp -d "${TMPDIR:-/tmp}/loom-p2a-w1-installer.XXXXXX")
short_run_root=$(mktemp -d /tmp/loom-w1-run.XXXXXX)
short_run_root=$(CDPATH= cd -- "$short_run_root" && pwd -P)
trap 'chmod -R u+rwX "$private_root" "$short_run_root" 2>/dev/null || true; rm -rf "$private_root" "$short_run_root"' EXIT HUP INT TERM
chmod 700 "$private_root"
chmod 700 "$short_run_root"

source_dir="$private_root/source"
install_root="$private_root/install"
run_dir="$short_run_root"
mkdir -m 700 "$source_dir" "$install_root"
(
  cd "$repo_root"
  go build -o "$source_dir/loom" ./cmd/loom
  go build -o "$source_dir/loomd" ./cmd/loomd
)
chmod 700 "$source_dir/loom" "$source_dir/loomd"

legacy_root="$private_root/legacy-install"
legacy_bin="$legacy_root/bin"
mkdir -m 700 "$legacy_root" "$legacy_bin"
printf '#!/bin/sh\nexit 41\n' >"$legacy_bin/loom"
printf '#!/bin/sh\nexit 42\n' >"$legacy_bin/loomd"
chmod 700 "$legacy_bin/loom" "$legacy_bin/loomd"
legacy_loom_digest=$(shasum -a 256 "$legacy_bin/loom" | awk '{print $1}')
legacy_loomd_digest=$(shasum -a 256 "$legacy_bin/loomd" | awk '{print $1}')

legacy_before=$(find "$legacy_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
"$installer" \
  --dry-run \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$legacy_root"
legacy_after_dry_run=$(find "$legacy_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
test "$legacy_before" = "$legacy_after_dry_run"

"$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$legacy_root"
cmp "$source_dir/loom" "$legacy_bin/loom"
cmp "$source_dir/loomd" "$legacy_bin/loomd"
test -x "$legacy_root/Loom.command"
test "$legacy_loom_digest" = "$(
  shasum -a 256 "$legacy_bin/loom.previous" | awk '{print $1}'
)"
test "$legacy_loomd_digest" = "$(
  shasum -a 256 "$legacy_bin/loomd.previous" | awk '{print $1}'
)"
test ! -e "$legacy_root/Loom.command.previous"

"$installer" --rollback --root "$legacy_root"
test "$legacy_loom_digest" = "$(
  shasum -a 256 "$legacy_bin/loom" | awk '{print $1}'
)"
test "$legacy_loomd_digest" = "$(
  shasum -a 256 "$legacy_bin/loomd" | awk '{print $1}'
)"
test ! -e "$legacy_root/Loom.command"
cmp "$source_dir/loom" "$legacy_bin/loom.previous"
cmp "$source_dir/loomd" "$legacy_bin/loomd.previous"
test -x "$legacy_root/Loom.command.previous"

"$installer" --rollback --root "$legacy_root"
cmp "$source_dir/loom" "$legacy_bin/loom"
cmp "$source_dir/loomd" "$legacy_bin/loomd"
test -x "$legacy_root/Loom.command"
test "$legacy_loom_digest" = "$(
  shasum -a 256 "$legacy_bin/loom.previous" | awk '{print $1}'
)"
test "$legacy_loomd_digest" = "$(
  shasum -a 256 "$legacy_bin/loomd.previous" | awk '{print $1}'
)"
test ! -e "$legacy_root/Loom.command.previous"

: >"$legacy_root/.loom-installer-test-sentinel"
chmod 600 "$legacy_root/.loom-installer-test-sentinel"
legacy_before_signal=$(find "$legacy_root" -type f \
  ! -name '.loom-installer-test-sentinel' \
  -exec shasum -a 256 {} \; | LC_ALL=C sort)
if LOOM_ROLLBACK_TEST_SIGNAL_STEP=after_current_loom "$installer" \
  --rollback \
  --root "$legacy_root" >/dev/null 2>&1
then
  echo "injected rollback signal unexpectedly succeeded" >&2
  exit 1
else
  rollback_signal_status=$?
fi
test "$rollback_signal_status" -eq 98
legacy_after_signal=$(find "$legacy_root" -type f \
  ! -name '.loom-installer-test-sentinel' \
  -exec shasum -a 256 {} \; | LC_ALL=C sort)
test "$legacy_before_signal" = "$legacy_after_signal"
rm "$legacy_root/.loom-installer-test-sentinel"

before=$(find "$private_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
"$installer" \
  --dry-run \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$install_root"
after_dry_run=$(find "$private_root" -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
test "$before" = "$after_dry_run"

"$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$install_root"

test -x "$install_root/bin/loom"
test -x "$install_root/bin/loomd"
test -x "$install_root/Loom.command"
test "$(stat -f '%Lp' "$install_root")" = 700
test "$(stat -f '%Lp' "$install_root/bin/loom")" = 700
cmp "$source_dir/loom" "$install_root/bin/loom"
cmp "$source_dir/loomd" "$install_root/bin/loomd"
! grep -E 'API_KEY|TOKEN|SECRET|PASSWORD' "$install_root/Loom.command"
tui_capture="$private_root/tui-output"
LOOM_TEST_COMMAND="$install_root/Loom.command" \
LOOM_TEST_SOCKET="$run_dir/loomd.sock" \
LOOM_TEST_CAPTURE="$tui_capture" \
TERM=xterm \
/usr/bin/expect -c '
  set timeout 5
  log_user 0
  log_file -noappend $env(LOOM_TEST_CAPTURE)
  spawn $env(LOOM_TEST_COMMAND) app --socket $env(LOOM_TEST_SOCKET)
  expect {
    -exact "\033]11;?\033\\\033\[6n" {
      send -- "\033]11;rgb:1212/3434/5656\007\033\[24;80R"
    }
    timeout {
      send -- "\003"
      exit 1
    }
  }
  expect {
    -re "Loom" {}
    timeout {
      send -- "\003"
      exit 1
    }
  }
  send -- "q"
  expect {
    eof {}
    timeout {
      send -- "\003"
      expect eof
      exit 1
    }
  }
  set result [wait]
  exit [lindex $result 3]
'
! grep -E 'API_KEY|TOKEN|SECRET|PASSWORD' "$tui_capture"

cp "$source_dir/loom" "$source_dir/loom.first"
cp "$source_dir/loomd" "$source_dir/loomd.first"
cp "$install_root/Loom.command" "$source_dir/Loom.command.first"
printf '#!/bin/sh\nexit 7\n' >"$source_dir/loom"
printf '#!/bin/sh\nexit 8\n' >"$source_dir/loomd"
chmod 700 "$source_dir/loom" "$source_dir/loomd"
"$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$install_root"
cmp "$source_dir/loom" "$install_root/bin/loom"

controlled_digest() {
  shasum -a 256 \
    "$install_root/bin/loom" \
    "$install_root/bin/loomd" \
    "$install_root/Loom.command" \
    "$install_root/bin/loom.previous" \
    "$install_root/bin/loomd.previous" \
    "$install_root/Loom.command.previous"
}
before_failed_update=$(controlled_digest)
printf '#!/bin/sh\nexit 17\n' >"$source_dir/loom"
printf '#!/bin/sh\nexit 18\n' >"$source_dir/loomd"
chmod 700 "$source_dir/loom" "$source_dir/loomd"
: >"$install_root/.loom-installer-test-sentinel"
chmod 600 "$install_root/.loom-installer-test-sentinel"
if LOOM_INSTALL_TEST_FAIL_STEP=after_loom "$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$install_root" >/dev/null 2>&1
then
  echo "injected mid-transaction failure unexpectedly succeeded" >&2
  exit 1
fi
after_failed_update=$(controlled_digest)
test "$before_failed_update" = "$after_failed_update"

if LOOM_INSTALL_TEST_SIGNAL_STEP=after_loom "$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$install_root" >/dev/null 2>&1
then
  echo "injected install signal unexpectedly succeeded" >&2
  exit 1
else
  install_signal_status=$?
fi
test "$install_signal_status" -eq 98
after_signaled_update=$(controlled_digest)
test "$before_failed_update" = "$after_signaled_update"

"$installer" --rollback --root "$install_root"
cmp "$source_dir/loom.first" "$install_root/bin/loom"
cmp "$source_dir/loomd.first" "$install_root/bin/loomd"
cmp "$source_dir/Loom.command.first" "$install_root/Loom.command"
test -x "$install_root/bin/loom"
test -x "$install_root/bin/loomd"
test -x "$install_root/Loom.command"

attack_root="$private_root/attack"
outside_root="$private_root/outside"
mkdir -m 700 "$attack_root" "$outside_root"
printf '%s\n' 'outside-unchanged' >"$outside_root/marker"
outside_digest=$(shasum -a 256 "$outside_root/marker")
ln -s "$outside_root" "$attack_root/bin"
if "$installer" \
  --loom "$source_dir/loom" \
  --loomd "$source_dir/loomd" \
  --root "$attack_root" >/dev/null 2>&1
then
  echo "symlinked install target was accepted" >&2
  exit 1
fi
test "$outside_digest" = "$(shasum -a 256 "$outside_root/marker")"

echo "installer fixture PASS"
