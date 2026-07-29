#!/bin/sh
set -eu

usage() {
  echo "usage: install-loom-local-product.sh [--dry-run] --loom PATH --loomd PATH --root PATH" >&2
  echo "       install-loom-local-product.sh --rollback --root PATH" >&2
  exit 2
}

dry_run=0
rollback=0
loom_source=
loomd_source=
install_root=

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
    --loom)
      [ "$#" -ge 2 ] || usage
      loom_source=$2
      shift 2
      ;;
    --loomd)
      [ "$#" -ge 2 ] || usage
      loomd_source=$2
      shift 2
      ;;
    --root)
      [ "$#" -ge 2 ] || usage
      install_root=$2
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done

[ -n "$install_root" ] || usage
case "$install_root" in
  /*) ;;
  *) usage ;;
esac
[ "$install_root" = "${install_root%/}" ] || usage
case "$install_root" in
  /|/Users|/Users/*/Library|/Applications|/Library|/tmp|/private/tmp)
    echo "unsafe install root" >&2
    exit 3
    ;;
esac
case "$install_root" in
  *'
'*|*'"'*|*'\'*)
    echo "unsupported install root" >&2
    exit 3
    ;;
esac

effective_uid=$(/usr/bin/id -u)
install_parent=$(/usr/bin/dirname "$install_root")
[ -d "$install_parent" ] || {
  echo "install parent unavailable" >&2
  exit 3
}
[ ! -L "$install_parent" ] || {
  echo "symlinked install parent" >&2
  exit 3
}
[ "$(/usr/bin/stat -f '%u' "$install_parent")" = "$effective_uid" ] || {
  echo "install parent owner mismatch" >&2
  exit 3
}
parent_mode=$(/usr/bin/stat -f '%Lp' "$install_parent")
other_digit=$((parent_mode % 10))
[ $((other_digit & 2)) -eq 0 ] || {
  echo "world-writable install parent" >&2
  exit 3
}

if [ -e "$install_root" ] || [ -L "$install_root" ]; then
  [ -d "$install_root" ] && [ ! -L "$install_root" ] || {
    echo "invalid install root" >&2
    exit 3
  }
  [ "$(/usr/bin/stat -f '%u' "$install_root")" = "$effective_uid" ] || {
    echo "install root owner mismatch" >&2
    exit 3
  }
fi

for controlled_path in \
  "$install_root/bin" \
  "$install_root/bin/loom" \
  "$install_root/bin/loomd" \
  "$install_root/bin/loom.previous" \
  "$install_root/bin/loomd.previous" \
  "$install_root/Loom.command" \
  "$install_root/Loom.command.previous"
do
  [ ! -L "$controlled_path" ] || {
    echo "symlinked install target" >&2
    exit 3
  }
done
[ ! -e "$install_root/bin" ] || [ -d "$install_root/bin" ] || {
  echo "invalid binary directory" >&2
  exit 3
}
current_loom=0
current_loomd=0
current_launcher=0
previous_loom=0
previous_loomd=0
previous_launcher=0
[ ! -f "$install_root/bin/loom" ] || current_loom=1
[ ! -f "$install_root/bin/loomd" ] || current_loomd=1
[ ! -f "$install_root/Loom.command" ] || current_launcher=1
[ ! -f "$install_root/bin/loom.previous" ] || previous_loom=1
[ ! -f "$install_root/bin/loomd.previous" ] || previous_loomd=1
[ ! -f "$install_root/Loom.command.previous" ] || previous_launcher=1

if [ "$current_loom" -ne "$current_loomd" ] ||
  { [ "$current_launcher" -eq 1 ] && [ "$current_loom" -ne 1 ]; }
then
  echo "incomplete current installation" >&2
  exit 3
fi
if [ "$previous_loom" -ne "$previous_loomd" ] ||
  { [ "$previous_launcher" -eq 1 ] && [ "$previous_loom" -ne 1 ]; }
then
  echo "incomplete rollback installation" >&2
  exit 3
fi

validate_source() {
  source_path=$1
  expected_name=$2
  case "$source_path" in
    /*) ;;
    *) echo "source path must be absolute" >&2; exit 3 ;;
  esac
  [ "$(/usr/bin/basename "$source_path")" = "$expected_name" ] || {
    echo "unexpected executable name" >&2
    exit 3
  }
  [ -f "$source_path" ] && [ ! -L "$source_path" ] || {
    echo "invalid executable source" >&2
    exit 3
  }
  [ "$(/usr/bin/stat -f '%u' "$source_path")" = "$effective_uid" ] || {
    echo "executable owner mismatch" >&2
    exit 3
  }
}

if [ "$rollback" -eq 1 ]; then
  [ "$dry_run" -eq 0 ] && [ -z "$loom_source" ] && [ -z "$loomd_source" ] || usage
  binary_dir="$install_root/bin"
  [ "$current_loom" -eq 1 ] &&
    [ "$previous_loom" -eq 1 ] || {
      echo "rollback unavailable" >&2
      exit 3
    }
  rollback_root="$install_root/.rollback.$$"
  /bin/mkdir -m 700 "$rollback_root"
  rollback_active=0
  restore_rollback() {
    /bin/cp -p "$rollback_root/current-loom" "$binary_dir/loom"
    /bin/cp -p "$rollback_root/current-loomd" "$binary_dir/loomd"
    /bin/cp -p "$rollback_root/previous-loom" "$binary_dir/loom.previous"
    /bin/cp -p "$rollback_root/previous-loomd" "$binary_dir/loomd.previous"
    if [ "$current_launcher" -eq 1 ]; then
      /bin/cp -p "$rollback_root/current-launcher" "$install_root/Loom.command"
    else
      /bin/rm -f "$install_root/Loom.command"
    fi
    if [ "$previous_launcher" -eq 1 ]; then
      /bin/cp -p \
        "$rollback_root/previous-launcher" \
        "$install_root/Loom.command.previous"
    else
      /bin/rm -f "$install_root/Loom.command.previous"
    fi
  }
  rollback_cleanup() {
    if [ "$rollback_active" -eq 1 ]; then
      restore_rollback || true
      rollback_active=0
    fi
    /bin/rm -f "$rollback_root"/*
    /bin/rmdir "$rollback_root" 2>/dev/null || true
  }
  handle_rollback_signal() {
    trap '' HUP INT TERM
    rollback_cleanup
    trap - EXIT
    exit 98
  }
  inject_rollback_test_signal() {
    step=$1
    if [ "${LOOM_ROLLBACK_TEST_SIGNAL_STEP:-}" = "$step" ] &&
      [ -f "$install_root/.loom-installer-test-sentinel" ] &&
      [ ! -L "$install_root/.loom-installer-test-sentinel" ]
    then
      kill -TERM "$$"
    fi
  }
  trap rollback_cleanup EXIT
  trap handle_rollback_signal HUP INT TERM
  /bin/cp -p "$binary_dir/loom" "$rollback_root/current-loom"
  /bin/cp -p "$binary_dir/loomd" "$rollback_root/current-loomd"
  /bin/cp -p "$binary_dir/loom.previous" "$rollback_root/previous-loom"
  /bin/cp -p "$binary_dir/loomd.previous" "$rollback_root/previous-loomd"
  if [ "$current_launcher" -eq 1 ]; then
    /bin/cp -p "$install_root/Loom.command" "$rollback_root/current-launcher"
  fi
  if [ "$previous_launcher" -eq 1 ]; then
    /bin/cp -p \
      "$install_root/Loom.command.previous" \
      "$rollback_root/previous-launcher"
  fi
  rollback_active=1

  /bin/cp -p "$rollback_root/previous-loom" "$binary_dir/loom"
  inject_rollback_test_signal after_current_loom
  /bin/cp -p "$rollback_root/previous-loomd" "$binary_dir/loomd"
  if [ "$previous_launcher" -eq 1 ]; then
    /bin/cp -p \
      "$rollback_root/previous-launcher" \
      "$install_root/Loom.command"
  else
    /bin/rm -f "$install_root/Loom.command"
  fi

  /bin/cp -p "$rollback_root/current-loom" "$binary_dir/loom.previous"
  /bin/cp -p "$rollback_root/current-loomd" "$binary_dir/loomd.previous"
  if [ "$current_launcher" -eq 1 ]; then
    /bin/cp -p \
      "$rollback_root/current-launcher" \
      "$install_root/Loom.command.previous"
  else
    /bin/rm -f "$install_root/Loom.command.previous"
  fi
  for controlled_path in \
    "$binary_dir/loom" \
    "$binary_dir/loomd" \
    "$binary_dir/loom.previous" \
    "$binary_dir/loomd.previous" \
    "$install_root/Loom.command" \
    "$install_root/Loom.command.previous"
  do
    [ ! -f "$controlled_path" ] || /bin/chmod 700 "$controlled_path"
  done
  rollback_active=0
  trap - EXIT HUP INT TERM
  rollback_cleanup
  exit 0
fi

[ -n "$loom_source" ] && [ -n "$loomd_source" ] || usage
validate_source "$loom_source" loom
validate_source "$loomd_source" loomd

if [ "$dry_run" -eq 1 ]; then
  exit 0
fi

/bin/mkdir -p "$install_root/bin"
/bin/chmod 700 "$install_root" "$install_root/bin"

binary_dir="$install_root/bin"
loom_temp="$binary_dir/.loom.new.$$"
loomd_temp="$binary_dir/.loomd.new.$$"
launcher_temp="$install_root/.Loom.command.new.$$"
transaction_root="$install_root/.transaction.$$"
transaction_active=0
cleanup() {
  if [ "$transaction_active" -eq 1 ]; then
    restore_transaction || true
    transaction_active=0
  fi
  /bin/rm -f \
    "$loom_temp" \
    "$loomd_temp" \
    "$launcher_temp" \
    "$binary_dir/loom.previous.new.$$" \
    "$binary_dir/loomd.previous.new.$$" \
    "$install_root/.Loom.command.previous.new.$$"
  if [ -d "$transaction_root" ] && [ ! -L "$transaction_root" ]; then
    /bin/rm -f "$transaction_root"/*
    /bin/rmdir "$transaction_root" 2>/dev/null || true
  fi
}
handle_install_signal() {
  trap '' HUP INT TERM
  cleanup
  trap - EXIT
  exit 98
}
trap cleanup EXIT
trap handle_install_signal HUP INT TERM

/bin/cp "$loom_source" "$loom_temp"
/bin/cp "$loomd_source" "$loomd_temp"
/bin/chmod 700 "$loom_temp" "$loomd_temp"

{
  printf '%s\n' '#!/bin/sh'
  printf 'exec "%s" "$@"\n' "$binary_dir/loom"
} >"$launcher_temp"
/bin/chmod 700 "$launcher_temp"

/bin/mkdir -m 700 "$transaction_root"
had_loom=0
had_loomd=0
had_launcher=0
had_previous_loom=0
had_previous_loomd=0
had_previous_launcher=0
backup_if_present() {
  source_path=$1
  backup_name=$2
  if [ -f "$source_path" ] && [ ! -L "$source_path" ]; then
    /bin/cp -p "$source_path" "$transaction_root/$backup_name"
    return 0
  fi
  return 1
}
if backup_if_present "$binary_dir/loom" current-loom; then had_loom=1; fi
if backup_if_present "$binary_dir/loomd" current-loomd; then had_loomd=1; fi
if backup_if_present "$install_root/Loom.command" current-launcher; then
  had_launcher=1
fi
if backup_if_present "$binary_dir/loom.previous" previous-loom; then
  had_previous_loom=1
fi
if backup_if_present "$binary_dir/loomd.previous" previous-loomd; then
  had_previous_loomd=1
fi
if backup_if_present \
  "$install_root/Loom.command.previous" \
  previous-launcher
then
  had_previous_launcher=1
fi

restore_path() {
  target=$1
  backup_name=$2
  existed=$3
  if [ "$existed" -eq 1 ]; then
    /bin/cp -p "$transaction_root/$backup_name" "$target"
  else
    /bin/rm -f "$target"
  fi
}
restore_transaction() {
  restore_path "$binary_dir/loom" current-loom "$had_loom"
  restore_path "$binary_dir/loomd" current-loomd "$had_loomd"
  restore_path "$install_root/Loom.command" current-launcher "$had_launcher"
  restore_path \
    "$binary_dir/loom.previous" \
    previous-loom \
    "$had_previous_loom"
  restore_path \
    "$binary_dir/loomd.previous" \
    previous-loomd \
    "$had_previous_loomd"
  restore_path \
    "$install_root/Loom.command.previous" \
    previous-launcher \
    "$had_previous_launcher"
}
fail_transaction() {
  restore_transaction || true
  transaction_active=0
  echo "installation transaction failed and was rolled back" >&2
  exit 4
}
test_failure() {
  step=$1
  if [ "${LOOM_INSTALL_TEST_FAIL_STEP:-}" = "$step" ] &&
    [ -f "$install_root/.loom-installer-test-sentinel" ] &&
    [ ! -L "$install_root/.loom-installer-test-sentinel" ]
  then
    return 0
  fi
  return 1
}
inject_install_test_signal() {
  step=$1
  if [ "${LOOM_INSTALL_TEST_SIGNAL_STEP:-}" = "$step" ] &&
    [ -f "$install_root/.loom-installer-test-sentinel" ] &&
    [ ! -L "$install_root/.loom-installer-test-sentinel" ]
  then
    kill -TERM "$$"
  fi
}

transaction_active=1
/bin/mv -f "$loom_temp" "$binary_dir/loom" || fail_transaction
inject_install_test_signal after_loom
if test_failure after_loom; then fail_transaction; fi
/bin/mv -f "$loomd_temp" "$binary_dir/loomd" || fail_transaction
inject_install_test_signal after_loomd
if test_failure after_loomd; then fail_transaction; fi
/bin/mv -f "$launcher_temp" "$install_root/Loom.command" || fail_transaction
inject_install_test_signal after_launcher

if [ "$had_loom" -eq 1 ]; then
  /bin/cp -p "$transaction_root/current-loom" "$binary_dir/loom.previous.new.$$" ||
    fail_transaction
  /bin/mv -f "$binary_dir/loom.previous.new.$$" "$binary_dir/loom.previous" ||
    fail_transaction
fi
if [ "$had_loomd" -eq 1 ]; then
  /bin/cp -p "$transaction_root/current-loomd" "$binary_dir/loomd.previous.new.$$" ||
    fail_transaction
  /bin/mv -f "$binary_dir/loomd.previous.new.$$" "$binary_dir/loomd.previous" ||
    fail_transaction
fi
if [ "$had_launcher" -eq 1 ]; then
  /bin/cp -p \
    "$transaction_root/current-launcher" \
    "$install_root/.Loom.command.previous.new.$$" ||
    fail_transaction
  /bin/mv -f \
    "$install_root/.Loom.command.previous.new.$$" \
    "$install_root/Loom.command.previous" ||
    fail_transaction
fi
/bin/chmod 700 "$binary_dir/loom" "$binary_dir/loomd" "$install_root/Loom.command"

transaction_active=0
trap - EXIT HUP INT TERM
cleanup
exit 0
