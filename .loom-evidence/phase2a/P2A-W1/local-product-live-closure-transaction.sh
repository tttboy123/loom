#!/bin/sh
set -eu
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH
LC_ALL=C
export LC_ALL
exec 2>/dev/null

result_prefix=LOCAL_PRODUCT_CLOSURE_RESULT

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

emit_pre_ready_failure() {
  phase=$1
  recorded=$2
  printf 'LOCAL_PRODUCT_CLOSURE_FAILURE phase=%s reason_recorded=%s\n' \
    "$phase" \
    "$recorded"
}

reject_ambient_overrides() {
  [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_LIVE_ROOT+x}" ] &&
    [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_SERVICE_LABEL+x}" ] &&
    [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_PLIST+x}" ] &&
    [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_SOCKET+x}" ] &&
    [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_APP+x}" ] &&
    [ -z "${LOOM_LOCAL_PRODUCT_CLOSURE_UID+x}" ]
}

establish_clean_environment() {
  case "${LOOM_LOCAL_PRODUCT_CLOSURE_ENVIRONMENT_READY-}" in
    "")
      exec /usr/bin/env -i \
        PATH=/usr/bin:/bin:/usr/sbin:/sbin \
        LC_ALL=C \
        LOOM_LOCAL_PRODUCT_CLOSURE_ENVIRONMENT_READY=1 \
        /bin/sh "$0" "$@"
      ;;
    1)
      /usr/bin/env |
        while IFS='=' read -r name _
        do
          case "$name" in
            LC_ALL|LOOM_LOCAL_PRODUCT_CLOSURE_ENVIRONMENT_READY|PATH|PWD|SHLVL|_) ;;
            *) exit 1 ;;
          esac
        done || exit 2
      ;;
    *)
      exit 2
      ;;
  esac
  unset LOOM_LOCAL_PRODUCT_CLOSURE_ENVIRONMENT_READY
}

exact_pid=
exact_stop_exit=0
stop_exact_path_candidate() {
  candidate_pid=${exact_pid-}
  [ -n "$candidate_pid" ] || return 0

  kill -TERM "$candidate_pid" 2>/dev/null || true
  for _ in $(jot 80)
  do
    kill -0 "$candidate_pid" 2>/dev/null || break
    sleep 0.05
  done
  if kill -0 "$candidate_pid" 2>/dev/null; then
    kill -KILL "$candidate_pid" 2>/dev/null || true
  fi

  exact_stop_exit=0
  wait "$candidate_pid" || exact_stop_exit=$?
  exact_pid=
  return 0
}

wait_for_running() {
  attempts=$1
  delay=$2
  probe=$3
  for _ in $(jot "$attempts")
  do
    if "$probe"; then
      return 0
    fi
    if [ "$delay" != 0 ]; then
      sleep "$delay"
    fi
  done
  return 1
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

write_closed_reason_json() {
  target=$1
  classification=$2
  failure_phase=$3
  launchd_state=$4
  launchd_runs=$5
  launchd_exit_code=$6
  initial_bootstrap_calls=$7
  reason_consumed=$8
  reason_rollback_count=$9
  reason_restart_count=${10}
  expected_uid=${11}

  [ ! -e "$target" ] && [ ! -L "$target" ] || return 1
  parent=${target%/*}
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  temporary=$(mktemp "$parent/.local-product-reason.XXXXXX") || return 1
  chmod 600 "$temporary" || {
    rm -f "$temporary"
    return 1
  }
  if ! python3 - \
    "$temporary" \
    "$classification" \
    "$failure_phase" \
    "$launchd_state" \
    "$launchd_runs" \
    "$launchd_exit_code" \
    "$initial_bootstrap_calls" \
    "$reason_consumed" \
    "$reason_rollback_count" \
    "$reason_restart_count" <<'PY'
import json
import os
import sys

(
    target,
    classification,
    failure_phase,
    launchd_state,
    launchd_runs,
    launchd_exit_code,
    initial_bootstrap_calls,
    consumed,
    rollback_count,
    restart_count,
) = sys.argv[1:]

allowed_classifications = {
    "daemon unavailable",
    "daemon failed: observer",
    "daemon failed: local_ipc",
    "daemon failed: shutdown",
    "daemon failed: result",
}
allowed_phases = {
    "bootstrap",
    "readiness",
    "run_root",
    "socket",
    "process_identity",
    "product_identity",
    "app_identity",
    "status_snapshot",
    "journal",
    "crash_inventory",
    "staging",
    "unclassified",
}
allowed_states = {"running", "not_running", "exited", "unknown"}
if classification not in allowed_classifications:
    raise SystemExit(2)
if failure_phase not in allowed_phases:
    raise SystemExit(2)
if launchd_state not in allowed_states:
    raise SystemExit(2)

def bounded_count(value):
    parsed = int(value)
    if parsed < 0 or parsed > 999999999:
        raise ValueError("count")
    return parsed

if launchd_exit_code == "null":
    parsed_exit_code = None
else:
    parsed_exit_code = int(launchd_exit_code)
    if parsed_exit_code < -999999999 or parsed_exit_code > 999999999:
        raise ValueError("exit")

value = {
    "schema_version": 1,
    "classification": classification,
    "failure_phase": failure_phase,
    "launchd_state": launchd_state,
    "launchd_runs": bounded_count(launchd_runs),
    "launchd_exit_code": parsed_exit_code,
    "initial_bootstrap_calls": bounded_count(initial_bootstrap_calls),
    "consumed": bounded_count(consumed),
    "rollback_count": bounded_count(rollback_count),
    "restart_count": bounded_count(restart_count),
}
encoded = json.dumps(
    value,
    ensure_ascii=True,
    separators=(",", ":"),
    sort_keys=True,
).encode("ascii") + b"\n"
with open(target, "wb", buffering=0) as handle:
    handle.write(encoded)
    os.fsync(handle.fileno())
PY
  then
    rm -f "$temporary"
    return 1
  fi
  chmod 600 "$temporary"
  [ "$(stat -f '%u' "$temporary")" = "$expected_uid" ] &&
    [ "$(stat -f '%Lp' "$temporary")" = 600 ] || {
      rm -f "$temporary"
      return 1
    }
  if ! ln "$temporary" "$target"; then
    rm -f "$temporary"
    return 1
  fi
  rm -f "$temporary"
  [ -f "$target" ] &&
    [ ! -L "$target" ] &&
    [ "$(stat -f '%u' "$target")" = "$expected_uid" ] &&
    [ "$(stat -f '%Lp' "$target")" = 600 ]
}

reject_ambient_overrides || exit 2
establish_clean_environment "$@"

run_fixture() {
  fixture_root=$1
  fixture_uid=$(id -u)

  case "$fixture_root" in
    /private/tmp/loom-local-product-closure-test.*/*) ;;
    *) exit 2 ;;
  esac
  validate_owned_private_directory "$fixture_root" "$fixture_uid" || exit 2

  sentinel="$fixture_root/.loom-local-product-closure-fixture"
  scenario_file="$fixture_root/control/scenario"
  validate_owned_private_directory "$fixture_root/control" "$fixture_uid" ||
    exit 2
  [ -f "$sentinel" ] &&
    [ ! -L "$sentinel" ] &&
    [ "$(stat -f '%Lp' "$sentinel")" = 600 ] &&
    [ "$(stat -f '%u' "$sentinel")" = "$fixture_uid" ] &&
    [ "$(cat "$sentinel")" = loom-local-product-closure-fixture-v1 ] || exit 2
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
  fixture_original_absent=0
  fixture_exact_path_preflight=0
  fixture_candidate_installed=0
  fixture_order="$fixture_root/control/order"

  append_fixture_order() {
    step=$1
    printf '%s\n' "$step" >>"$fixture_order"
    chmod 600 "$fixture_order"
  }

  fixture_mark_original_absent() {
    [ "$fixture_original_absent" -eq 0 ] || return 1
    fixture_original_absent=1
    write_fixture_counter original-absent.count 1
    append_fixture_order original_absent
  }

  fixture_run_exact_path_preflight() {
    [ "$fixture_original_absent" -eq 1 ] &&
      [ "$fixture_exact_path_preflight" -eq 0 ] || return 1
    fixture_exact_path_preflight=1
    write_fixture_counter exact-path-preflight.count 1
    append_fixture_order exact_path_preflight
  }

  fixture_install_candidate() {
    [ "$fixture_original_absent" -eq 1 ] &&
      [ "$fixture_exact_path_preflight" -eq 1 ] &&
      [ "$fixture_candidate_installed" -eq 0 ] || return 1
    fixture_candidate_installed=1
    write_fixture_counter candidate-install.count 1
    append_fixture_order candidate_install
  }

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
    [ "$fixture_original_absent" -eq 1 ] &&
      [ "$fixture_exact_path_preflight" -eq 1 ] &&
      [ "$fixture_candidate_installed" -eq 1 ] || return 1
    validate_owned_private_directory "$run_root" "$fixture_uid"
    fixture_bootstrap_calls=$((fixture_bootstrap_calls + 1))
    write_fixture_counter bootstrap.count "$fixture_bootstrap_calls"
    append_fixture_order bootstrap
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
      failure_phase_*)
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
        append_fixture_order rollback
        ;;
      restart)
        fixture_restart_calls=$((fixture_restart_calls + 1))
        write_fixture_counter restart.count "$fixture_restart_calls"
        append_fixture_order restart
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

  fixture_signal_exit() {
    trap '' HUP INT TERM
    stop_exact_path_candidate
    append_fixture_order exact_path_candidate_stopped
    fixture_rollback
    emit_result \
      signal_rolled_back \
      "$initial_calls" \
      "$consumed" \
      "$rollback_count" \
      "$restart_count"
    exit 98
  }

  fixture_fail_pre_ready() {
    phase=$1
    reason_recorded=0
    if write_closed_reason_json \
      "$fixture_root/control/reason.json" \
      "daemon failed: local_ipc" \
      "$phase" \
      exited \
      1 \
      4 \
      "$initial_calls" \
      "$consumed" \
      1 \
      "$restart_count" \
      "$fixture_uid"
    then
      reason_recorded=1
    fi
    emit_pre_ready_failure "$phase" "$reason_recorded"
    fixture_rollback
    emit_result \
      rolled_back \
      "$initial_calls" \
      "$consumed" \
      "$rollback_count" \
      "$restart_count"
    exit 21
  }

  fixture_unclassified_exit() {
    trap - EXIT
    fixture_fail_pre_ready unclassified
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
      fixture_mark_original_absent
      create_fixture_run_root
      fixture_run_exact_path_preflight
      fixture_install_candidate
      fixture_bootstrap
      initial_calls=1
      consumed=1
      if [ -S "$run_root/loomd.sock" ]; then
        exit 2
      fi
      append_fixture_order reason_recorded
      fixture_fail_pre_ready readiness
      ;;
    ready)
      fixture_mark_original_absent
      create_fixture_run_root
      fixture_run_exact_path_preflight
      fixture_install_candidate
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
    exact_path_signal)
      fixture_mark_original_absent
      create_fixture_run_root
      append_fixture_order exact_path_preflight_started
      sleep 300 &
      exact_pid=$!
      trap fixture_signal_exit HUP INT TERM
      write_fixture_counter exact-child.pid "$exact_pid"
      write_fixture_counter exact-child.ready 1
      wait "$exact_pid"
      exit 2
      ;;
    failure_phase_*)
      phase=${scenario#failure_phase_}
      case "$phase" in
        bootstrap|readiness|run_root|socket|process_identity|product_identity|app_identity|status_snapshot|journal|crash_inventory|staging|unclassified) ;;
        *) exit 2 ;;
      esac
      fixture_mark_original_absent
      create_fixture_run_root
      fixture_run_exact_path_preflight
      fixture_install_candidate
      fixture_bootstrap
      initial_calls=1
      consumed=1
      if [ "$phase" = unclassified ]; then
        trap fixture_unclassified_exit EXIT
        false
        exit 2
      fi
      fixture_fail_pre_ready "$phase"
      ;;
    delayed_recovery)
      recovery_probes=0
      fixture_recovery_probe() {
        recovery_probes=$((recovery_probes + 1))
        [ "$recovery_probes" -eq 81 ]
      }
      wait_for_running 240 0 fixture_recovery_probe
      write_fixture_counter recovery-probes.count "$recovery_probes"
      emit_result delayed_recovery_pass 0 0 0 0
      exit 20
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
candidate_parent='/private/tmp/loom-local-product-repair.8ptDJ8'
candidate_root='/private/tmp/loom-local-product-repair.8ptDJ8/candidate'
support='/Users/lune/Library/Application Support/Loom'
install_root="$support/demo-resident"
binary_root="$install_root/bin"
run_root="$support/run"
socket_path="$run_root/loomd.sock"
plist='/Users/lune/Library/LaunchAgents/com.earendilworks.loom.runtime-observer.plist'
app='/Users/lune/Applications/Loom.app'
label='com.earendilworks.loom.runtime-observer'
activation_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-replacement-activation-audit.md"
candidate_manifest_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-candidate-manifest.json"
source_lock_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-source-lock.json"
reason_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-reason.json"
uid=501

expected_candidate_record='ec03f11e6b46512994ce2d9c389b019102feadb11398a986ff346b6f3d234167'
expected_source_lock='8c75b532951de6ce8372fb76584d8246d625e310b7300e0af9c022b8cd903abb'
expected_local_product_read='3add36a2f8e27b2b8858c0181195bd8447bcb0a87e6e2874aa5aebe3f101ed00'
expected_local_product_read_test='7266b7f5e56a92d1bb3f8f36af3e670a70ef3f09f663bf860a19cf8bf73560c2'
expected_product_daemon_test='da65d75c4655b88bf233a001e42ef075d728ba76fec3389232e8768810e5ddea'
expected_swift_contract_test='8672343979b8c0a418af623644f0d71ef6878a39b96b2b63b35aa4616875a136'
expected_candidate_loom='b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29'
expected_candidate_loomd='af29fbb9cfa46ac0b97321a3240c9e7fd4048f0f64ce55cfd1b139a1a691d6e6'
expected_candidate_app='3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba'
expected_candidate_uuid='93E3FC61-0A19-3379-BFDB-8CA7726F59F6'
expected_candidate_manifest='bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58'
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

read_activation_count() {
  field=$1
  python3 - "$activation_record" "$field" <<'PY'
import re
import sys

path, field = sys.argv[1:]
if field not in {"Resident PID", "Resident Runs"}:
    raise SystemExit(2)
with open(path, "rb") as handle:
    data = handle.read(65537)
if len(data) > 65536:
    raise SystemExit(2)
pattern = re.compile(
    rb"^\*\*" + re.escape(field.encode("ascii")) + rb"\*\*: `([0-9]{1,9})`$"
)
matches = [
    pattern.fullmatch(line)
    for line in data.splitlines()
]
values = [match.group(1) for match in matches if match is not None]
if len(values) != 1:
    raise SystemExit(2)
value = int(values[0])
if value < 1 or value > 999999999:
    raise SystemExit(2)
print(value)
PY
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

service_runs() {
  value=$(
    launchctl print "gui/$uid/$label" 2>/dev/null |
      awk '/^[[:space:]]*runs =/{print $3; exit}'
  )
  case "$value" in
    ""|*[!0-9]*) printf '0\n' ;;
    *)
      if [ "${#value}" -le 9 ]; then
        printf '%s\n' "$value"
      else
        printf '0\n'
      fi
      ;;
  esac
}

service_exit_code() {
  value=$(
    launchctl print "gui/$uid/$label" 2>/dev/null |
      awk '/^[[:space:]]*last exit code =/{
        sub(/^[[:space:]]*last exit code = /, "")
        print
        exit
      }'
  )
  if printf '%s\n' "$value" |
    grep -Eq '^-?[0-9]{1,9}$'
  then
    printf '%s\n' "$value"
  else
    printf 'null\n'
  fi
}

write_failure_reason() {
  reason_phase=$1
  [ "$initial_calls" -eq 1 ] &&
    [ "$consumed" -eq 1 ] &&
    [ ! -e "$reason_record" ] &&
    [ ! -L "$reason_record" ] || return 1

  classification='daemon failed: shutdown'
  if [ -f "$candidate_stderr" ] &&
    [ ! -L "$candidate_stderr" ] &&
    [ "$(stat -f '%u' "$candidate_stderr")" = "$uid" ] &&
    [ "$(stat -f '%Lp' "$candidate_stderr")" = 600 ] &&
    [ "$(wc -l <"$candidate_stderr" | tr -d ' ')" = 1 ]
  then
    candidate_message=$(cat "$candidate_stderr")
    case "$candidate_message" in
      "daemon unavailable" | \
        "daemon failed: observer" | \
        "daemon failed: local_ipc" | \
        "daemon failed: shutdown" | \
        "daemon failed: result")
        classification=$candidate_message
        ;;
    esac
  fi

  if service_running; then
    launchd_state=running
  elif launchctl print "gui/$uid/$label" >/dev/null 2>&1; then
    launchd_state=exited
  else
    launchd_state=not_running
  fi
  launchd_runs=$(service_runs)
  launchd_exit_code=$(service_exit_code)
  write_closed_reason_json \
    "$reason_record" \
    "$classification" \
    "$reason_phase" \
    "$launchd_state" \
    "$launchd_runs" \
    "$launchd_exit_code" \
    "$initial_calls" \
    "$consumed" \
    1 \
    "$restart_count" \
    "$uid"
}

failure_emitted=0
pre_ready_complete=0
record_pre_ready_failure() {
  failure_phase=$1
  reason_recorded=0
  if write_failure_reason "$failure_phase"; then
    reason_recorded=1
  fi
  emit_pre_ready_failure "$failure_phase" "$reason_recorded"
  failure_emitted=1
}

fail_pre_ready() {
  failure_phase=$1
  record_pre_ready_failure "$failure_phase"
  exit 21
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
  wait_for_running 240 0.25 service_running
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
assert data["runtimes"][0]["model_ids"] == []
assert data["runtimes"][0]["observed_capabilities"] == []
assert len(data["teams"]) == 0
assert len(data["runs"]) == 0
assert len(data["evidence"]) == 0
assert len(data["attention"]) == 0
PY
}

run_exact_path_preflight() {
  exact_root="$live_root/exact-path-preflight"
  exact_state="$exact_root/loom.sqlite"
  exact_isolation="$exact_root/isolation"
  exact_stdout="$exact_root/daemon.stdout"
  exact_stderr="$exact_root/daemon.stderr"
  exact_status_stdout="$exact_root/status.stdout"
  exact_status_stderr="$exact_root/status.stderr"
  exact_pid=
  exact_ready=0
  exact_running_processes=0
  exact_status_exit=99
  exact_timed_out=0

  [ ! -e "$exact_root" ] && [ ! -L "$exact_root" ] || return 1
  mkdir -m 700 "$exact_root" "$exact_isolation" || return 1
  cp -R "$install_root/isolation/." "$exact_isolation/" || return 1
  [ -z "$(find "$exact_isolation" -type l -print -quit)" ] || return 1
  cp "$install_root/loom.sqlite" "$exact_state" || return 1
  chmod 600 "$exact_state"
  : >"$exact_stdout"
  : >"$exact_stderr"
  : >"$exact_status_stdout"
  : >"$exact_status_stderr"
  chmod 600 \
    "$exact_stdout" \
    "$exact_stderr" \
    "$exact_status_stdout" \
    "$exact_status_stderr"

  /usr/bin/env -i \
    PATH=/usr/bin:/bin:/usr/sbin:/sbin \
    LC_ALL=C \
    "$candidate_root/loomd" \
    --state "$exact_state" \
    --isolation-root "$exact_isolation" \
    --runtime-dir "$support/runtimes/pi/0.82.1/node_modules/.bin" \
    --runtime-dir '/Users/lune/Documents/Codex/devtools/node/bin' \
    --probe-id 'probe.pi.resident-demo' \
    --instance-id 'runtime.pi.earendil-works.0.82.1' \
    --device-id 'device.local' \
    --display-name 'Pi 0.82.1 Resident Demo' \
    --interval 10s \
    --process-timeout 10s \
    --max-cycles 1 \
    --socket "$socket_path" \
    >"$exact_stdout" \
    2>"$exact_stderr" &
  exact_pid=$!

  for _ in $(jot 200)
  do
    if [ -S "$socket_path" ] && [ ! -L "$socket_path" ]; then
      exact_ready=1
      break
    fi
    kill -0 "$exact_pid" 2>/dev/null || break
    sleep 0.05
  done

  if [ "$exact_ready" -eq 1 ]; then
    exact_running_processes=$(
      pgrep -f "^$candidate_root/loomd( |$)" 2>/dev/null |
        wc -l |
        tr -d ' '
    )
    set +e
    /usr/bin/env -i \
      PATH=/usr/bin:/bin:/usr/sbin:/sbin \
      LC_ALL=C \
      "$candidate_root/loom" \
      status \
      --socket "$socket_path" \
      >"$exact_status_stdout" \
      2>"$exact_status_stderr"
    exact_status_exit=$?
    set -e
  fi

  for _ in $(jot 400)
  do
    kill -0 "$exact_pid" 2>/dev/null || break
    sleep 0.05
  done
  if kill -0 "$exact_pid" 2>/dev/null; then
    exact_timed_out=1
    stop_exact_path_candidate
    exact_daemon_exit=$exact_stop_exit
  else
    set +e
    wait "$exact_pid"
    exact_daemon_exit=$?
    set -e
    exact_pid=
  fi

  exact_processes=$(
    pgrep -f "^$candidate_root/loomd( |$)" 2>/dev/null |
      wc -l |
      tr -d ' '
  )
  exact_ok=1
  [ "$exact_ready" -eq 1 ] || exact_ok=0
  [ "$exact_running_processes" -eq 1 ] || exact_ok=0
  [ "$exact_status_exit" -eq 0 ] || exact_ok=0
  [ "$exact_timed_out" -eq 0 ] || exact_ok=0
  [ "$exact_daemon_exit" -eq 0 ] || exact_ok=0
  [ "$exact_processes" -eq 0 ] || exact_ok=0
  [ "$(stat -f '%z' "$exact_stderr")" -eq 0 ] || exact_ok=0
  [ "$(stat -f '%z' "$exact_status_stderr")" -eq 0 ] || exact_ok=0
  [ "$(sqlite3 "$exact_state" 'pragma integrity_check;')" = ok ] ||
    exact_ok=0
  python3 - "$exact_status_stdout" <<'PY' || exact_ok=0
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    value = json.load(handle)
assert value["command"] == "status"
assert value["source_mode"] == "daemon_api"
assert isinstance(value["schema_version"], int)
assert isinstance(value["view_version"], str)
for key in ("runtimes", "teams", "runs", "evidence", "attention"):
    assert isinstance(value[key], list)
PY

  if [ -e "$socket_path" ] || [ -L "$socket_path" ]; then
    [ -S "$socket_path" ] && [ ! -L "$socket_path" ] || exact_ok=0
    rm -f "$socket_path"
  fi
  if [ -e "$socket_path.lock" ] || [ -L "$socket_path.lock" ]; then
    [ -f "$socket_path.lock" ] &&
      [ ! -L "$socket_path.lock" ] || exact_ok=0
    rm -f "$socket_path.lock"
  fi
  chmod -R u+rwX "$exact_root" 2>/dev/null || true
  rm -rf "$exact_root"
  [ "$exact_ok" -eq 1 ]
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
  rollback_ok=1
  rollback_count=$((rollback_count + 1))
  stop_exact_path_candidate || rollback_ok=0
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
  if [ "$initial_calls" -eq 1 ] &&
    [ "$consumed" -eq 1 ] &&
    [ "$pre_ready_complete" -eq 0 ] &&
    [ "$failure_emitted" -eq 0 ]
  then
    record_pre_ready_failure unclassified
  fi
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
  'P2A-W1 Local Product Launch Failure Replacement and one controlled native-window canary' \
  "$activation_record" ||
  production_preflight_failure
activation_resident_pid=$(read_activation_count 'Resident PID') ||
  production_preflight_failure
activation_resident_runs=$(read_activation_count 'Resident Runs') ||
  production_preflight_failure
[ ! -e "$reason_record" ] && [ ! -L "$reason_record" ] ||
  production_preflight_failure

[ -f "$candidate_manifest_record" ] &&
  [ ! -L "$candidate_manifest_record" ] &&
  [ "$(sha256 "$candidate_manifest_record")" = "$expected_candidate_record" ] ||
  production_preflight_failure
[ -f "$source_lock_record" ] &&
  [ ! -L "$source_lock_record" ] &&
  [ "$(sha256 "$source_lock_record")" = "$expected_source_lock" ] ||
  production_preflight_failure
[ "$(sha256 "$repo/internal/api/local_product_read.go")" = "$expected_local_product_read" ] ||
  production_preflight_failure
[ "$(sha256 "$repo/internal/api/local_product_read_test.go")" = "$expected_local_product_read_test" ] ||
  production_preflight_failure
[ "$(sha256 "$repo/cmd/loomd/product_daemon_test.go")" = "$expected_product_daemon_test" ] ||
  production_preflight_failure
[ "$(sha256 "$repo/internal/localipc/swift_contract_test.go")" = "$expected_swift_contract_test" ] ||
  production_preflight_failure

[ -d "$candidate_parent" ] &&
  [ ! -L "$candidate_parent" ] &&
  validate_owned_private_directory "$candidate_parent" "$uid" ||
  production_preflight_failure
[ -d "$candidate_root/Loom.app" ] &&
  [ ! -L "$candidate_root/Loom.app" ] ||
  production_preflight_failure
validate_owned_private_directory "$candidate_root" "$uid" ||
  production_preflight_failure
[ -z "$(find "$candidate_root" -type l -print -quit)" ] ||
  production_preflight_failure
[ -z "$(
  find "$candidate_root" \
    \( ! -user "$uid" -o -type d ! -perm 0700 \) \
    -print -quit
)" ] ||
  production_preflight_failure
[ -f "$candidate_root/loom" ] &&
  [ ! -L "$candidate_root/loom" ] &&
  [ "$(stat -f '%u' "$candidate_root/loom")" = "$uid" ] &&
  [ "$(stat -f '%Lp' "$candidate_root/loom")" = 700 ] ||
  production_preflight_failure
[ -f "$candidate_root/loomd" ] &&
  [ ! -L "$candidate_root/loomd" ] &&
  [ "$(stat -f '%u' "$candidate_root/loomd")" = "$uid" ] &&
  [ "$(stat -f '%Lp' "$candidate_root/loomd")" = 700 ] ||
  production_preflight_failure
[ "$(stat -f '%Lp' "$candidate_root/Loom.app/Contents/MacOS/LoomLocalApp")" = 700 ] ||
  production_preflight_failure
[ -z "$(
  find "$candidate_root/Loom.app" \
    -type f \
    ! -path '*/Contents/MacOS/LoomLocalApp' \
    ! -perm 0600 \
    -print -quit
)" ] ||
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
[ "$original_pid" = "$activation_resident_pid" ] &&
  [ "$(service_runs)" = "$activation_resident_runs" ] ||
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

live_root=$(mktemp -d /private/tmp/loom-native-local-product-closure.XXXXXX)
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
candidate_stdout="$live_root/candidate.stdout"
candidate_stderr="$live_root/candidate.stderr"

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
: >"$candidate_stdout"
: >"$candidate_stderr"
chmod 600 "$candidate_stdout" "$candidate_stderr"
/usr/libexec/PlistBuddy \
  -c 'Add :ProgramArguments:21 string --socket' \
  "$candidate_plist" >/dev/null
/usr/libexec/PlistBuddy \
  -c "Add :ProgramArguments:22 string $socket_path" \
  "$candidate_plist" >/dev/null
/usr/libexec/PlistBuddy \
  -c "Set :StandardOutPath $candidate_stdout" \
  "$candidate_plist" >/dev/null
/usr/libexec/PlistBuddy \
  -c "Set :StandardErrorPath $candidate_stderr" \
  "$candidate_plist" >/dev/null
/usr/libexec/PlistBuddy \
  -c 'Set :KeepAlive false' \
  "$candidate_plist" >/dev/null
chmod 600 "$candidate_plist"
python3 - \
  "$candidate_plist" \
  "$socket_path" \
  "$candidate_stdout" \
  "$candidate_stderr" <<'PY'
import plistlib
import sys

with open(sys.argv[1], "rb") as handle:
    value = plistlib.load(handle)
arguments = value["ProgramArguments"]
assert len(arguments) == 23
assert arguments[-2:] == ["--socket", sys.argv[2]]
assert value["StandardOutPath"] == sys.argv[3]
assert value["StandardErrorPath"] == sys.argv[4]
assert value["KeepAlive"] is False
PY

[ "$(service_pid)" = "$activation_resident_pid" ] &&
  [ "$(service_runs)" = "$activation_resident_runs" ] ||
  production_preflight_failure
rollback_needed=1
preparation_active=0
trap on_exit EXIT
trap 'exit 98' HUP INT TERM

launchctl bootout "gui/$uid/$label" >/dev/null
wait_service_absent "$original_pid"

mkdir -m 700 "$run_root"
validate_owned_private_directory "$run_root" "$uid"
run_exact_path_preflight

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

initial_calls=1
consumed=1
if ! /usr/bin/env -i \
  PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  /bin/launchctl bootstrap "gui/$uid" "$plist" >/dev/null
then
  fail_pre_ready bootstrap
fi

if ! wait_candidate_running; then
  fail_pre_ready readiness
fi
if ! validate_owned_private_directory "$run_root" "$uid"; then
  fail_pre_ready run_root
fi
if ! {
  [ "$(stat -f '%Lp' "$socket_path")" = 600 ] &&
    [ "$(stat -f '%u' "$socket_path")" = "$uid" ]
}; then
  fail_pre_ready socket
fi
candidate_pid=$(service_pid)
if ! {
  [ -n "$candidate_pid" ] &&
    [ "$(target_marker_count "$candidate_pid")" = 0 ]
}; then
  fail_pre_ready process_identity
fi
if ! {
  [ "$(sha256 "$binary_root/loom")" = "$expected_candidate_loom" ] &&
    [ "$(sha256 "$binary_root/loomd")" = "$expected_candidate_loomd" ] &&
    [ "$(stat -f '%Lp' "$binary_root/loom")" = 700 ] &&
    [ "$(stat -f '%Lp' "$binary_root/loomd")" = 700 ]
}; then
  fail_pre_ready product_identity
fi
if ! {
  [ "$(sha256 "$app/Contents/MacOS/LoomLocalApp")" = "$expected_candidate_app" ] &&
    [ "$(bundle_manifest_digest "$app")" = "$expected_candidate_manifest" ] &&
    codesign --verify --strict "$app" >/dev/null 2>&1
}; then
  fail_pre_ready app_identity
fi
if ! validate_status_snapshot "$status_json"; then
  fail_pre_ready status_snapshot
fi
if ! validate_journal; then
  fail_pre_ready journal
fi
if ! validate_crash_inventory; then
  fail_pre_ready crash_inventory
fi
if ! git -C "$repo" diff --cached --quiet; then
  fail_pre_ready staging
fi

pre_ready_complete=1
printf 'LOCAL_PRODUCT_CLOSURE_READY initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=0\n'

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
printf 'LOCAL_PRODUCT_CLOSURE_RESTART_READY initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1\n'

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
