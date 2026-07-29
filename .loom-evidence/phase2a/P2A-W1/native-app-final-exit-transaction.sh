#!/bin/sh
set -eu
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH
LC_ALL=C
export LC_ALL
exec 2>/dev/null

result_prefix=FINAL_EXIT_RESULT

emit_result() {
  result=$1
  initial_calls=$2
  consumed=$3
  rollback_count=$4
  restart_count=$5
  printf '%s result=%s initial_bootstrap_calls=%s consumed=%s rollback_count=%s restart_count=%s\n' \
    "$result_prefix" \
    "$result" \
    "$initial_calls" \
    "$consumed" \
    "$rollback_count" \
    "$restart_count"
}

reject_ambient_overrides() {
  [ -z "${LOOM_FINAL_EXIT_LIVE_ROOT+x}" ] &&
    [ -z "${LOOM_FINAL_EXIT_SERVICE_LABEL+x}" ] &&
    [ -z "${LOOM_FINAL_EXIT_PLIST+x}" ] &&
    [ -z "${LOOM_FINAL_EXIT_SOCKET+x}" ] &&
    [ -z "${LOOM_FINAL_EXIT_APP+x}" ] &&
    [ -z "${LOOM_FINAL_EXIT_UID+x}" ]
}

establish_clean_environment() {
  case "${LOOM_FINAL_EXIT_ENVIRONMENT_READY-}" in
    "")
      exec /usr/bin/env -i \
        PATH=/usr/bin:/bin:/usr/sbin:/sbin \
        LC_ALL=C \
        LOOM_FINAL_EXIT_ENVIRONMENT_READY=1 \
        /bin/sh "$0" "$@"
      ;;
    1)
      /usr/bin/env |
        while IFS='=' read -r name _
        do
          case "$name" in
            LC_ALL|LOOM_FINAL_EXIT_ENVIRONMENT_READY|PATH|PWD|SHLVL|_) ;;
            *) exit 1 ;;
          esac
        done || exit 2
      ;;
    *)
      exit 2
      ;;
  esac
  unset LOOM_FINAL_EXIT_ENVIRONMENT_READY
}

validate_owned_private_directory() {
  path=$1
  expected_uid=$2
  [ -d "$path" ] &&
    [ ! -L "$path" ] &&
    [ "$(stat -f '%Lp' "$path")" = 700 ] &&
    [ "$(stat -f '%u' "$path")" = "$expected_uid" ] &&
    [ "$(CDPATH= cd -- "$path" && pwd -P)" = "$path" ]
}

reject_ambient_overrides || exit 2
establish_clean_environment "$@"

run_fixture() {
  fixture_root=$1
  fixture_uid=$(id -u)

  case "$fixture_root" in
    /private/tmp/loom-final-exit-test.*/*) ;;
    *) exit 2 ;;
  esac
  validate_owned_private_directory "$fixture_root" "$fixture_uid" || exit 2

  sentinel="$fixture_root/.loom-final-exit-fixture"
  scenario_file="$fixture_root/control/scenario"
  validate_owned_private_directory "$fixture_root/control" "$fixture_uid" ||
    exit 2
  [ -f "$sentinel" ] &&
    [ ! -L "$sentinel" ] &&
    [ "$(stat -f '%Lp' "$sentinel")" = 600 ] &&
    [ "$(stat -f '%u' "$sentinel")" = "$fixture_uid" ] &&
    [ "$(cat "$sentinel")" = loom-final-exit-fixture-v1 ] || exit 2
  [ -f "$scenario_file" ] &&
    [ ! -L "$scenario_file" ] &&
    [ "$(stat -f '%Lp' "$scenario_file")" = 600 ] &&
    [ "$(stat -f '%u' "$scenario_file")" = "$fixture_uid" ] || exit 2

  scenario=$(cat "$scenario_file")
  run_root="$fixture_root/run"
  initial_calls=0
  consumed=0
  rollback_count=0
  restart_count=0
  fixture_bootstrap_calls=0
  fixture_rollback_calls=0
  fixture_restart_calls=0

  create_fixture_run_root() {
    [ ! -e "$run_root" ] && [ ! -L "$run_root" ] || return 1
    mkdir -m 700 "$run_root"
    validate_owned_private_directory "$run_root" "$fixture_uid"
  }

  remove_fixture_run_root() {
    if [ -e "$run_root/loomd.sock" ] || [ -L "$run_root/loomd.sock" ]; then
      [ -S "$run_root/loomd.sock" ] &&
        [ ! -L "$run_root/loomd.sock" ] || return 1
      rm -f "$run_root/loomd.sock"
    fi
    [ ! -e "$run_root/loomd.sock.lock" ] ||
      rm -f "$run_root/loomd.sock.lock"
    rmdir "$run_root"
  }

  write_fixture_counter() {
    counter=$1
    value=$2
    target="$fixture_root/control/$counter"
    if [ -e "$target" ] || [ -L "$target" ]; then
      [ -f "$target" ] &&
        [ ! -L "$target" ] &&
        [ "$(stat -f '%u' "$target")" = "$fixture_uid" ] &&
        [ "$(stat -f '%Lp' "$target")" = 600 ] ||
        return 1
    fi
    temporary=$(mktemp "$fixture_root/control/.counter.XXXXXX")
    printf '%s\n' "$value" >"$temporary"
    chmod 600 "$temporary"
    mv -f "$temporary" "$target"
    [ -f "$target" ] &&
      [ ! -L "$target" ] &&
      [ "$(stat -f '%u' "$target")" = "$fixture_uid" ] &&
      [ "$(stat -f '%Lp' "$target")" = 600 ]
  }

  fixture_bootstrap() {
    validate_owned_private_directory "$run_root" "$fixture_uid"
    fixture_bootstrap_calls=$((fixture_bootstrap_calls + 1))
    write_fixture_counter bootstrap.count "$fixture_bootstrap_calls"
    case "$scenario" in
      ready)
        python3 - "$run_root/loomd.sock" <<'PY'
import os
import socket
import sys

path = sys.argv[1]
server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
server.bind(path)
os.chmod(path, 0o600)
server.close()
PY
        ;;
      readiness_failure)
        ;;
      *)
        return 1
        ;;
    esac
  }

  fixture_bootout() {
    kind=$1
    case "$kind" in
      rollback)
        fixture_rollback_calls=$((fixture_rollback_calls + 1))
        write_fixture_counter rollback.count "$fixture_rollback_calls"
        ;;
      restart)
        fixture_restart_calls=$((fixture_restart_calls + 1))
        write_fixture_counter restart.count "$fixture_restart_calls"
        ;;
      *) return 1 ;;
    esac
    rm -f "$run_root/loomd.sock" "$run_root/loomd.sock.lock"
  }

  fixture_rollback() {
    fixture_bootout rollback
    rollback_count=$((rollback_count + 1))
    remove_fixture_run_root
  }

  case "$scenario" in
    before_bootstrap)
      create_fixture_run_root
      fixture_rollback
      emit_result \
        prebootstrap_failure \
        "$initial_calls" \
        "$consumed" \
        "$rollback_count" \
        "$restart_count"
      exit 20
      ;;
    readiness_failure)
      create_fixture_run_root
      fixture_bootstrap
      initial_calls=1
      consumed=1
      if [ -S "$run_root/loomd.sock" ]; then
        exit 2
      fi
      fixture_rollback
      emit_result \
        rolled_back \
        "$initial_calls" \
        "$consumed" \
        "$rollback_count" \
        "$restart_count"
      exit 21
      ;;
    ready)
      create_fixture_run_root
      fixture_bootstrap
      initial_calls=1
      consumed=1
      [ -S "$run_root/loomd.sock" ] &&
        [ ! -L "$run_root/loomd.sock" ] &&
        [ "$(stat -f '%Lp' "$run_root/loomd.sock")" = 600 ] ||
        exit 2

      fixture_bootout restart
      restart_count=1
      [ ! -e "$run_root/loomd.sock" ] ||
        exit 2
      fixture_bootstrap
      [ -S "$run_root/loomd.sock" ] &&
        [ ! -L "$run_root/loomd.sock" ] &&
        [ "$(stat -f '%Lp' "$run_root/loomd.sock")" = 600 ] ||
        exit 2
      rm -f "$run_root/loomd.sock"
      rmdir "$run_root"
      emit_result \
        fixture_pass \
        "$initial_calls" \
        "$consumed" \
        "$rollback_count" \
        "$restart_count"
      exit 0
      ;;
    *)
      exit 2
      ;;
  esac
}

if [ "$#" -eq 2 ] && [ "$1" = --fixture-root ]; then
  run_fixture "$2"
fi
[ "$#" -eq 0 ] || exit 2

repo='/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild'
candidate_root='/private/tmp/loom-replacement-audit.i4X6PR/candidate'
support='/Users/lune/Library/Application Support/Loom'
install_root="$support/demo-resident"
binary_root="$install_root/bin"
run_root="$support/run"
socket_path="$run_root/loomd.sock"
plist='/Users/lune/Library/LaunchAgents/com.earendilworks.loom.runtime-observer.plist'
app='/Users/lune/Applications/Loom.app'
label='com.earendilworks.loom.runtime-observer'
activation_record="$repo/.loom-evidence/phase2a/P2A-W1/native-app-final-exit-post-review-audit.md"
uid=501

expected_candidate_loom='3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f'
expected_candidate_loomd='ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357'
expected_candidate_app='f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b'
expected_candidate_uuid='CE91F84E-4333-35DB-B493-88FADCBC6EC1'
expected_candidate_manifest='e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd'
expected_original_loom='60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698'
expected_original_loomd='e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a'
expected_original_wrapper='ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4'
expected_original_plist='2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4'
expected_journal='91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4'
expected_view='6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05'
expected_report_one='f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0'
expected_report_two='4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54'
report_root='/Users/lune/Library/Logs/DiagnosticReports'

initial_calls=0
consumed=0
rollback_count=0
restart_count=0
rollback_needed=0
product_installed=0

sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

bundle_manifest_digest() {
  bundle=$1
  find "$bundle" -type f -print0 |
    sort -z |
    while IFS= read -r -d '' file
    do
      relative_path=${file#"$bundle/"}
      mode=$(stat -f '%Sp' "$file")
      digest=$(sha256 "$file")
      printf '%s\000%s\000%s\n' "$relative_path" "$mode" "$digest"
    done |
    shasum -a 256 |
    awk '{print $1}'
}

record_crash_inventory() {
  destination=$1
  find "$report_root" \
    -maxdepth 1 \
    -type f \
    -name 'LoomLocalApp-*.ips' \
    -print0 |
    sort -z |
    while IFS= read -r -d '' report
    do
      printf '%s %s\n' "${report##*/}" "$(sha256 "$report")"
    done >"$destination"
  chmod 600 "$destination"
}

service_pid() {
  launchctl print "gui/$uid/$label" 2>/dev/null |
    awk '/^[[:space:]]*pid =/{print $3; exit}'
}

service_program() {
  launchctl print "gui/$uid/$label" 2>/dev/null |
    awk '/^[[:space:]]*program =/{
      sub(/^[[:space:]]*program = /, "")
      print
      exit
    }'
}

service_running() {
  launchctl print "gui/$uid/$label" 2>/dev/null |
    awk '/state = running/{found=1} END{exit !found}'
}

wait_service_absent() {
  old_pid=$1
  for _ in $(jot 80)
  do
    if ! launchctl print "gui/$uid/$label" >/dev/null 2>&1; then
      if [ -z "$old_pid" ] || ! kill -0 "$old_pid" 2>/dev/null; then
        return 0
      fi
    fi
    sleep 0.25
  done
  return 1
}

wait_candidate_running() {
  for _ in $(jot 80)
  do
    if service_running &&
      [ -S "$socket_path" ] &&
      [ ! -L "$socket_path" ]
    then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

wait_original_running() {
  for _ in $(jot 80)
  do
    if service_running; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

target_marker_count() {
  target_pid=$1
  python3 - "$target_pid" <<'PY'
import subprocess
import sys

process = subprocess.run(
    ["/bin/ps", "eww", "-p", sys.argv[1], "-o", "command="],
    stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL,
    check=False,
)
if process.returncode != 0:
    raise SystemExit(2)
s = process.stdout.decode("utf-8", errors="strict")
names=("DEEPSEEK_BASE_URL","STEPFUN_BASE_URL","MINIMAX_BASE_URL","DEEPSEEK_API_KEY","STEPFUN_API_KEY")
print(sum(1 for name in names if name in s))
PY
}

quit_native_app() {
  app_pid=$(pgrep -x LoomLocalApp 2>/dev/null || true)
  if [ -n "$app_pid" ]; then
    kill -TERM "$app_pid" 2>/dev/null || true
    for _ in $(jot 40)
    do
      if ! pgrep -x LoomLocalApp >/dev/null 2>&1; then
        return 0
      fi
      sleep 0.25
    done
    return 1
  fi
}

remove_candidate_app() {
  if [ -d "$app" ] && [ ! -L "$app" ]; then
    [ "$(sha256 "$app/Contents/MacOS/LoomLocalApp")" = "$expected_candidate_app" ]
    [ "$(bundle_manifest_digest "$app")" = "$expected_candidate_manifest" ]
    chmod -R u+rwX "$app"
    rm -rf "$app"
  fi
  [ ! -e "$app" ] && [ ! -L "$app" ]
  [ ! -e "$app.previous" ] && [ ! -L "$app.previous" ]
}

remove_product_run_root() {
  validate_owned_private_directory "$run_root" "$uid" || return 1
  if [ -e "$socket_path" ] || [ -L "$socket_path" ]; then
    [ -S "$socket_path" ] && [ ! -L "$socket_path" ] || return 1
    rm -f "$socket_path"
  fi
  lock_path="$socket_path.lock"
  if [ -e "$lock_path" ] || [ -L "$lock_path" ]; then
    [ -f "$lock_path" ] && [ ! -L "$lock_path" ] || return 1
    rm -f "$lock_path"
  fi
  rmdir "$run_root"
}

validate_status_snapshot() {
  status_path=$1
  "$binary_root/loom" status --socket "$socket_path" >"$status_path"
  chmod 600 "$status_path"
  python3 - "$status_path" "$expected_view" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
assert data["source_mode"] == "daemon_api"
assert data["view_version"] == sys.argv[2]
assert len(data["runtimes"]) == 1
assert data["runtimes"][0]["display_name"] == "Pi 0.82.1 Resident Demo"
assert len(data["teams"]) == 0
assert len(data["runs"]) == 0
assert len(data["evidence"]) == 0
assert len(data["attention"]) == 0
PY
}

validate_journal() {
  [ "$(sha256 "$install_root/loom.sqlite")" = "$expected_journal" ] &&
    [ "$(sqlite3 "$install_root/loom.sqlite" 'pragma integrity_check;')" = ok ] &&
    [ "$(sqlite3 "$install_root/loom.sqlite" 'select count(*) from events;')" = 1 ]
}

validate_crash_inventory() {
  current="$live_root/current-crash-reports"
  record_crash_inventory "$current"
  cmp -s "$original_reports" "$current"
}

restore_original_service() {
  cp -p "$original_plist_copy" "$plist"
  chmod 600 "$plist"
  chmod 755 "$binary_root/loom" "$binary_root/loomd"
  chmod 700 "$binary_root/loomd-clean"
  xattr -d com.apple.provenance "$binary_root/loomd" 2>/dev/null || true
  xattr -d com.apple.provenance "$binary_root/loomd-clean" 2>/dev/null || true
  /usr/bin/env -i \
    PATH=/usr/bin:/bin:/usr/sbin:/sbin \
    /bin/launchctl bootstrap "gui/$uid" "$plist" >/dev/null
  wait_original_running
  xattr -wx \
    com.apple.provenance \
    "$(tr -d ' \n' <"$original_loomd_provenance")" \
    "$binary_root/loomd"
  xattr -wx \
    com.apple.provenance \
    "$(tr -d ' \n' <"$original_wrapper_provenance")" \
    "$binary_root/loomd-clean"
}

rollback() {
  rollback_needed=0
  trap '' HUP INT TERM
  set +e
  rollback_count=$((rollback_count + 1))
  quit_native_app
  current_pid=$(service_pid 2>/dev/null || true)
  launchctl bootout "gui/$uid/$label" >/dev/null 2>&1 || true
  wait_service_absent "$current_pid" || true
  remove_candidate_app || true

  if [ "$product_installed" -eq 1 ]; then
    "$repo/scripts/install-loom-local-product.sh" \
      --rollback \
      --root "$install_root" >/dev/null 2>&1 || true
  fi

  cp -p "$original_loom_copy" "$binary_root/loom"
  cp -p "$original_loomd_copy" "$binary_root/loomd"
  cp -p "$original_wrapper_copy" "$binary_root/loomd-clean"
  chmod 755 "$binary_root/loom" "$binary_root/loomd"
  chmod 700 "$binary_root/loomd-clean"

  if [ -f "$binary_root/loom.previous" ] &&
    [ "$(sha256 "$binary_root/loom.previous")" = "$expected_candidate_loom" ]
  then
    rm -f "$binary_root/loom.previous"
  fi
  if [ -f "$binary_root/loomd.previous" ] &&
    [ "$(sha256 "$binary_root/loomd.previous")" = "$expected_candidate_loomd" ]
  then
    rm -f "$binary_root/loomd.previous"
  fi
  rm -f \
    "$install_root/Loom.command" \
    "$install_root/Loom.command.previous"

  if [ -d "$run_root" ] && [ ! -L "$run_root" ]; then
    remove_product_run_root || true
  fi

  restore_original_service || true
  restored_pid=$(service_pid 2>/dev/null || true)
  rollback_ok=1
  [ "$(sha256 "$binary_root/loom")" = "$expected_original_loom" ] ||
    rollback_ok=0
  [ "$(stat -f '%Lp' "$binary_root/loom")" = 755 ] ||
    rollback_ok=0
  [ "$(sha256 "$binary_root/loomd")" = "$expected_original_loomd" ] ||
    rollback_ok=0
  [ "$(stat -f '%Lp' "$binary_root/loomd")" = 755 ] ||
    rollback_ok=0
  [ "$(sha256 "$binary_root/loomd-clean")" = "$expected_original_wrapper" ] ||
    rollback_ok=0
  [ "$(stat -f '%Lp' "$binary_root/loomd-clean")" = 700 ] ||
    rollback_ok=0
  [ "$(sha256 "$plist")" = "$expected_original_plist" ] ||
    rollback_ok=0
  [ "$(stat -f '%Lp' "$plist")" = 600 ] ||
    rollback_ok=0
  [ -n "$restored_pid" ] && service_running ||
    rollback_ok=0
  [ "$(service_program)" = "$binary_root/loomd-clean" ] ||
    rollback_ok=0
  [ -n "$restored_pid" ] &&
    [ "$(target_marker_count "$restored_pid")" = 0 ] ||
    rollback_ok=0
  [ "$(
    xattr -px com.apple.provenance "$binary_root/loomd" 2>/dev/null |
      tr -d ' \n'
  )" = "$(tr -d ' \n' <"$original_loomd_provenance")" ] ||
    rollback_ok=0
  [ "$(
    xattr -px com.apple.provenance "$binary_root/loomd-clean" 2>/dev/null |
      tr -d ' \n'
  )" = "$(tr -d ' \n' <"$original_wrapper_provenance")" ] ||
    rollback_ok=0
  validate_journal || rollback_ok=0
  validate_crash_inventory || rollback_ok=0
  [ ! -e "$app" ] && [ ! -L "$app" ] ||
    rollback_ok=0
  [ ! -e "$app.previous" ] && [ ! -L "$app.previous" ] ||
    rollback_ok=0
  [ ! -e "$run_root" ] && [ ! -L "$run_root" ] ||
    rollback_ok=0
  [ ! -e "$install_root/Loom.command" ] &&
    [ ! -L "$install_root/Loom.command" ] ||
    rollback_ok=0
  [ ! -e "$install_root/Loom.command.previous" ] &&
    [ ! -L "$install_root/Loom.command.previous" ] ||
    rollback_ok=0
  [ ! -e "$binary_root/loom.previous" ] &&
    [ ! -L "$binary_root/loom.previous" ] ||
    rollback_ok=0
  [ ! -e "$binary_root/loomd.previous" ] &&
    [ ! -L "$binary_root/loomd.previous" ] ||
    rollback_ok=0
  git -C "$repo" diff --cached --quiet ||
    rollback_ok=0

  rm -rf "$live_root"
  if [ "$rollback_ok" -eq 1 ]; then
    emit_result \
      rolled_back \
      "$initial_calls" \
      "$consumed" \
      "$rollback_count" \
      "$restart_count"
    return 0
  fi
  emit_result \
    rollback_incomplete \
    "$initial_calls" \
    "$consumed" \
    "$rollback_count" \
    "$restart_count"
  return 1
}

on_exit() {
  exit_code=$?
  if [ "$rollback_needed" -eq 1 ]; then
    rollback || exit_code=1
  fi
  exit "$exit_code"
}

production_preflight_failure() {
  emit_result preflight_failure 0 0 0 0
  exit 20
}

[ "$(id -u)" = "$uid" ] || production_preflight_failure
[ -f "$activation_record" ] &&
  [ ! -L "$activation_record" ] ||
  production_preflight_failure
grep -Fqx '**Status**: `PASS`' "$activation_record" ||
  production_preflight_failure
grep -Fqx '**Allowance**: `1`' "$activation_record" ||
  production_preflight_failure
grep -Fqx \
  'P2A-W1 Final Native Launch Transaction Closure and one controlled native-window canary' \
  "$activation_record" ||
  production_preflight_failure

[ -d "$candidate_root/Loom.app" ] &&
  [ ! -L "$candidate_root/Loom.app" ] ||
  production_preflight_failure
validate_owned_private_directory "$candidate_root" "$uid" ||
  production_preflight_failure
[ -f "$candidate_root/loom" ] &&
  [ ! -L "$candidate_root/loom" ] &&
  [ "$(stat -f '%u' "$candidate_root/loom")" = "$uid" ] ||
  production_preflight_failure
[ -f "$candidate_root/loomd" ] &&
  [ ! -L "$candidate_root/loomd" ] &&
  [ "$(stat -f '%u' "$candidate_root/loomd")" = "$uid" ] ||
  production_preflight_failure
[ "$(sha256 "$candidate_root/loom")" = "$expected_candidate_loom" ] ||
  production_preflight_failure
[ "$(sha256 "$candidate_root/loomd")" = "$expected_candidate_loomd" ] ||
  production_preflight_failure
[ "$(sha256 "$candidate_root/Loom.app/Contents/MacOS/LoomLocalApp")" = "$expected_candidate_app" ] ||
  production_preflight_failure
[ "$(
  dwarfdump --uuid "$candidate_root/Loom.app/Contents/MacOS/LoomLocalApp" |
    awk '{print $2}'
)" = "$expected_candidate_uuid" ] ||
  production_preflight_failure
[ "$(bundle_manifest_digest "$candidate_root/Loom.app")" = "$expected_candidate_manifest" ] ||
  production_preflight_failure
codesign --verify --strict "$candidate_root/Loom.app" >/dev/null 2>&1 ||
  production_preflight_failure

[ "$(sha256 "$binary_root/loom")" = "$expected_original_loom" ] ||
  production_preflight_failure
[ "$(stat -f '%Lp' "$binary_root/loom")" = 755 ] ||
  production_preflight_failure
[ "$(sha256 "$binary_root/loomd")" = "$expected_original_loomd" ] ||
  production_preflight_failure
[ "$(stat -f '%Lp' "$binary_root/loomd")" = 755 ] ||
  production_preflight_failure
[ "$(sha256 "$binary_root/loomd-clean")" = "$expected_original_wrapper" ] ||
  production_preflight_failure
[ "$(stat -f '%Lp' "$binary_root/loomd-clean")" = 700 ] ||
  production_preflight_failure
[ "$(sha256 "$plist")" = "$expected_original_plist" ] ||
  production_preflight_failure
[ "$(stat -f '%Lp' "$plist")" = 600 ] ||
  production_preflight_failure
original_pid=$(service_pid)
[ -n "$original_pid" ] && service_running ||
  production_preflight_failure
[ "$(service_program)" = "$binary_root/loomd-clean" ] ||
  production_preflight_failure
[ "$(target_marker_count "$original_pid")" = 0 ] ||
  production_preflight_failure
validate_journal ||
  production_preflight_failure
[ ! -e "$app" ] && [ ! -L "$app" ] ||
  production_preflight_failure
[ ! -e "$app.previous" ] && [ ! -L "$app.previous" ] ||
  production_preflight_failure
[ ! -e "$run_root" ] && [ ! -L "$run_root" ] ||
  production_preflight_failure
[ ! -e "$install_root/Loom.command" ] &&
  [ ! -L "$install_root/Loom.command" ] ||
  production_preflight_failure
[ ! -e "$install_root/Loom.command.previous" ] &&
  [ ! -L "$install_root/Loom.command.previous" ] ||
  production_preflight_failure
[ ! -e "$binary_root/loom.previous" ] &&
  [ ! -L "$binary_root/loom.previous" ] ||
  production_preflight_failure
[ ! -e "$binary_root/loomd.previous" ] &&
  [ ! -L "$binary_root/loomd.previous" ] ||
  production_preflight_failure
! pgrep -x LoomLocalApp >/dev/null 2>&1 ||
  production_preflight_failure
git -C "$repo" diff --cached --quiet ||
  production_preflight_failure
[ "$(sha256 "$report_root/LoomLocalApp-2026-07-28-213614.ips")" = "$expected_report_one" ] ||
  production_preflight_failure
[ "$(sha256 "$report_root/LoomLocalApp-2026-07-28-213628.ips")" = "$expected_report_two" ] ||
  production_preflight_failure

live_root=$(mktemp -d /private/tmp/loom-native-final-exit.XXXXXX)
chmod 700 "$live_root"
preparation_active=1
cleanup_preparation() {
  if [ "$preparation_active" -eq 1 ] &&
    [ -d "$live_root" ] &&
    [ ! -L "$live_root" ] &&
    [ "$(stat -f '%u' "$live_root")" = "$uid" ]
  then
    chmod -R u+rwX "$live_root" 2>/dev/null || true
    rm -rf "$live_root"
  fi
}
handle_preparation_signal() {
  trap '' HUP INT TERM
  cleanup_preparation
  trap - EXIT
  exit 98
}
trap cleanup_preparation EXIT
trap handle_preparation_signal HUP INT TERM
candidate_plist="$live_root/candidate.plist"
original_plist_copy="$live_root/original.plist"
original_loom_copy="$live_root/original-loom"
original_loomd_copy="$live_root/original-loomd"
original_wrapper_copy="$live_root/original-wrapper"
original_loomd_provenance="$live_root/original-loomd-provenance.hex"
original_wrapper_provenance="$live_root/original-wrapper-provenance.hex"
original_reports="$live_root/original-crash-reports"
status_json="$live_root/status.json"
restart_status_json="$live_root/restart-status.json"

record_crash_inventory "$original_reports"
[ "$(wc -l <"$original_reports" | tr -d ' ')" = 2 ] ||
  production_preflight_failure
cp -p "$plist" "$original_plist_copy"
cp -p "$binary_root/loom" "$original_loom_copy"
cp -p "$binary_root/loomd" "$original_loomd_copy"
cp -p "$binary_root/loomd-clean" "$original_wrapper_copy"
chmod 600 \
  "$original_plist_copy" \
  "$original_loom_copy" \
  "$original_loomd_copy" \
  "$original_wrapper_copy"
xattr -px com.apple.provenance "$binary_root/loomd" >"$original_loomd_provenance" ||
  production_preflight_failure
xattr -px com.apple.provenance "$binary_root/loomd-clean" >"$original_wrapper_provenance" ||
  production_preflight_failure
chmod 600 "$original_loomd_provenance" "$original_wrapper_provenance"

cp -p "$plist" "$candidate_plist"
/usr/libexec/PlistBuddy \
  -c 'Add :ProgramArguments:21 string --socket' \
  "$candidate_plist" >/dev/null
/usr/libexec/PlistBuddy \
  -c "Add :ProgramArguments:22 string $socket_path" \
  "$candidate_plist" >/dev/null
chmod 600 "$candidate_plist"
python3 - "$candidate_plist" "$socket_path" <<'PY'
import plistlib
import sys

with open(sys.argv[1], "rb") as handle:
    value = plistlib.load(handle)
arguments = value["ProgramArguments"]
assert len(arguments) == 23
assert arguments[-2:] == ["--socket", sys.argv[2]]
PY

rollback_needed=1
preparation_active=0
trap on_exit EXIT
trap 'exit 98' HUP INT TERM

mkdir -m 700 "$run_root"
validate_owned_private_directory "$run_root" "$uid"

"$repo/scripts/install-loom-local-product.sh" \
  --loom "$candidate_root/loom" \
  --loomd "$candidate_root/loomd" \
  --root "$install_root" >/dev/null
product_installed=1
"$repo/scripts/install-loom-local-app.sh" \
  --app "$candidate_root/Loom.app" \
  --destination "$app" >/dev/null

cp -p "$candidate_plist" "$plist"
chmod 600 "$plist"
launchctl bootout "gui/$uid/$label" >/dev/null
wait_service_absent "$original_pid"

/usr/bin/env -i \
  PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  /bin/launchctl bootstrap "gui/$uid" "$plist" >/dev/null
initial_calls=1
consumed=1

wait_candidate_running
validate_owned_private_directory "$run_root" "$uid"
[ "$(stat -f '%Lp' "$socket_path")" = 600 ]
[ "$(stat -f '%u' "$socket_path")" = "$uid" ]
candidate_pid=$(service_pid)
[ -n "$candidate_pid" ]
[ "$(target_marker_count "$candidate_pid")" = 0 ]
[ "$(sha256 "$binary_root/loom")" = "$expected_candidate_loom" ]
[ "$(sha256 "$binary_root/loomd")" = "$expected_candidate_loomd" ]
[ "$(stat -f '%Lp' "$binary_root/loom")" = 700 ]
[ "$(stat -f '%Lp' "$binary_root/loomd")" = 700 ]
[ "$(sha256 "$app/Contents/MacOS/LoomLocalApp")" = "$expected_candidate_app" ]
[ "$(bundle_manifest_digest "$app")" = "$expected_candidate_manifest" ]
codesign --verify --strict "$app" >/dev/null 2>&1
validate_status_snapshot "$status_json"
validate_journal
validate_crash_inventory
git -C "$repo" diff --cached --quiet

printf 'FINAL_EXIT_READY initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=0\n'

IFS= read -r decision || decision=eof
[ "$decision" = ui_verified ] || exit 22
! pgrep -x LoomLocalApp >/dev/null 2>&1 || exit 22
validate_journal
validate_crash_inventory

pre_restart_pid=$(service_pid)
launchctl bootout "gui/$uid/$label" >/dev/null
wait_service_absent "$pre_restart_pid"
[ ! -e "$socket_path" ] && [ ! -L "$socket_path" ]
[ ! -e "$socket_path.lock" ] && [ ! -L "$socket_path.lock" ]
/usr/bin/env -i \
  PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  /bin/launchctl bootstrap "gui/$uid" "$plist" >/dev/null
restart_count=1
wait_candidate_running
restart_pid=$(service_pid)
[ -n "$restart_pid" ]
[ "$restart_pid" != "$pre_restart_pid" ]
[ "$(target_marker_count "$restart_pid")" = 0 ]
validate_status_snapshot "$restart_status_json"
validate_journal
validate_crash_inventory
printf 'FINAL_EXIT_RESTART_READY initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1\n'

IFS= read -r decision || decision=eof
[ "$decision" = result_review_pass ] || exit 22
service_running
validate_owned_private_directory "$run_root" "$uid"
[ -S "$socket_path" ] && [ ! -L "$socket_path" ]
validate_status_snapshot "$restart_status_json"
validate_journal
validate_crash_inventory
git -C "$repo" diff --cached --quiet

validate_owned_private_directory "$live_root" "$uid"
rollback_needed=0
trap - EXIT HUP INT TERM
rm -rf "$live_root"
emit_result preserved "$initial_calls" "$consumed" "$rollback_count" "$restart_count"
