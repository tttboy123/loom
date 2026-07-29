#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
harness="$repo_root/.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction.sh"

if [ ! -f "$harness" ] || [ ! -x "$harness" ]; then
  echo "RED: governed local product closure transaction harness is missing" >&2
  exit 1
fi

grep -Fqx \
  'activation_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-replacement-activation-audit.md"' \
  "$harness"
if grep -F \
  'activation_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-live-closure-activation-audit.md"' \
  "$harness" >/dev/null 2>&1
then
  echo "transaction retained the consumed activation-record path" >&2
  exit 1
fi
grep -Fqx \
  "candidate_parent='/private/tmp/loom-local-product-repair.8ptDJ8'" \
  "$harness"
grep -Fqx \
  "candidate_root='/private/tmp/loom-local-product-repair.8ptDJ8/candidate'" \
  "$harness"
grep -Fqx \
  'candidate_manifest_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-candidate-manifest.json"' \
  "$harness"
grep -Fqx \
  'source_lock_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-source-lock.json"' \
  "$harness"
if grep -F \
  '/private/tmp/loom-local-product-closure.aqt1x7' \
  "$harness" >/dev/null 2>&1
then
  echo "transaction retained the failed Candidate" >&2
  exit 1
fi

red_requirements=0
require_harness_literal() {
  literal=$1
  message=$2
  if ! grep -F -- "$literal" "$harness" >/dev/null 2>&1; then
    echo "RED: $message" >&2
    red_requirements=$((red_requirements + 1))
  fi
}

require_harness_literal \
  'reason_record="$repo/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-reason.json"' \
  'transaction does not own the closed failure-reason path'
require_harness_literal \
  'run_exact_path_preflight() {' \
  'transaction has no exact-path preflight boundary'
require_harness_literal \
  'stop_exact_path_candidate() {' \
  'transaction has no bounded exact-path Candidate cleanup helper'
require_harness_literal \
  'stop_exact_path_candidate || rollback_ok=0' \
  'rollback does not terminate and join the exact-path Candidate'
require_harness_literal \
  "activation_resident_pid=\$(read_activation_count 'Resident PID')" \
  'transaction does not strictly bind the audited resident PID'
require_harness_literal \
  "activation_resident_runs=\$(read_activation_count 'Resident Runs')" \
  'transaction does not strictly bind the audited resident run count'
require_harness_literal \
  '[ "$(service_pid)" = "$activation_resident_pid" ] &&' \
  'transaction does not recheck the audited PID immediately before bootout'
require_harness_literal \
  '[ "$(service_runs)" = "$activation_resident_runs" ] ||' \
  'transaction does not recheck the audited run count immediately before bootout'
require_harness_literal \
  'emit_pre_ready_failure() {' \
  'transaction cannot preserve a closed primary pre-READY phase'
require_harness_literal \
  'LOCAL_PRODUCT_CLOSURE_FAILURE phase=%s reason_recorded=%s' \
  'transaction has no closed pre-READY failure line'
require_harness_literal \
  'wait_for_running 240 0.25 service_running' \
  'original-service recovery envelope is shorter than sixty seconds'
for required_phase in \
  bootstrap \
  readiness \
  run_root \
  socket \
  process_identity \
  product_identity \
  app_identity \
  status_snapshot \
  journal \
  crash_inventory \
  staging
do
  require_harness_literal \
    "fail_pre_ready $required_phase" \
    "transaction does not route $required_phase through closed attribution"
done
require_harness_literal \
  'record_pre_ready_failure unclassified' \
  'transaction does not route unclassified through closed attribution'
require_harness_literal \
  'write_failure_reason() {' \
  'transaction cannot atomically record a closed pre-ready reason'
require_harness_literal \
  '-c "Set :StandardOutPath $candidate_stdout"' \
  'transaction does not bind Candidate stdout to the private evidence root'
require_harness_literal \
  '-c "Set :StandardErrorPath $candidate_stderr"' \
  'transaction does not bind Candidate stderr to the private evidence root'
require_harness_literal \
  "-c 'Set :KeepAlive false'" \
  'transaction allows launchd to hide a pre-ready retry'
if grep -F \
  'Add :StandardOutPath' \
  "$harness" >/dev/null 2>&1 ||
  grep -F \
    'Add :StandardErrorPath' \
    "$harness" >/dev/null 2>&1
then
  echo "RED: transaction adds plist log keys that already exist" >&2
  red_requirements=$((red_requirements + 1))
fi

original_absent_line=$(
  grep -n 'wait_service_absent "$original_pid"' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
candidate_install_line=$(
  grep -n '"$repo/scripts/install-loom-local-product.sh"' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
exact_path_call_line=$(
  grep -n '^run_exact_path_preflight$' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
initial_bootstrap_line=$(
  grep -n '^initial_calls=1$' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
resident_recheck_line=$(
  grep -nF '[ "$(service_pid)" = "$activation_resident_pid" ] &&' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
rollback_arm_line=$(
  grep -n '^rollback_needed=1$' "$harness" |
    tail -n 1 |
    cut -d: -f1
)
if [ -z "$original_absent_line" ] ||
  [ -z "$candidate_install_line" ] ||
  [ "$original_absent_line" -ge "$candidate_install_line" ]
then
  echo "RED: Candidate installation occurs before original-service absence" >&2
  red_requirements=$((red_requirements + 1))
fi
if [ -z "$exact_path_call_line" ] ||
  [ -z "$initial_bootstrap_line" ] ||
  [ "$original_absent_line" -ge "$exact_path_call_line" ] ||
  [ "$exact_path_call_line" -ge "$candidate_install_line" ] ||
  [ "$candidate_install_line" -ge "$initial_bootstrap_line" ]
then
  echo "RED: exact-path preflight is not ordered before Candidate bootstrap" >&2
  red_requirements=$((red_requirements + 1))
fi
if [ -z "$resident_recheck_line" ] ||
  [ -z "$rollback_arm_line" ] ||
  [ "$resident_recheck_line" -ge "$rollback_arm_line" ]
then
  echo "RED: audited resident identity is not checked before rollback arm and bootout" >&2
  red_requirements=$((red_requirements + 1))
fi

if [ "$red_requirements" -ne 0 ]; then
  exit 1
fi

assert_frozen_constant() {
  name=$1
  value=$2
  grep -Fqx "$name='$value'" "$harness"
}

assert_frozen_constant expected_candidate_record \
  ec03f11e6b46512994ce2d9c389b019102feadb11398a986ff346b6f3d234167
assert_frozen_constant expected_source_lock \
  8c75b532951de6ce8372fb76584d8246d625e310b7300e0af9c022b8cd903abb
assert_frozen_constant expected_local_product_read \
  3add36a2f8e27b2b8858c0181195bd8447bcb0a87e6e2874aa5aebe3f101ed00
assert_frozen_constant expected_local_product_read_test \
  7266b7f5e56a92d1bb3f8f36af3e670a70ef3f09f663bf860a19cf8bf73560c2
assert_frozen_constant expected_product_daemon_test \
  da65d75c4655b88bf233a001e42ef075d728ba76fec3389232e8768810e5ddea
assert_frozen_constant expected_swift_contract_test \
  8672343979b8c0a418af623644f0d71ef6878a39b96b2b63b35aa4616875a136
assert_frozen_constant expected_candidate_loom \
  b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29
assert_frozen_constant expected_candidate_loomd \
  af29fbb9cfa46ac0b97321a3240c9e7fd4048f0f64ce55cfd1b139a1a691d6e6
assert_frozen_constant expected_candidate_app \
  3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba
assert_frozen_constant expected_candidate_uuid \
  93E3FC61-0A19-3379-BFDB-8CA7726F59F6
assert_frozen_constant expected_candidate_manifest \
  bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58
assert_frozen_constant expected_original_loom \
  60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
assert_frozen_constant expected_original_loomd \
  e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
assert_frozen_constant expected_original_wrapper \
  ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
assert_frozen_constant expected_original_plist \
  2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
assert_frozen_constant expected_journal \
  91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
assert_frozen_constant expected_view \
  6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
assert_frozen_constant expected_report_one \
  f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
assert_frozen_constant expected_report_two \
  4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54

source_lock="$repo_root/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-source-lock.json"
candidate_record="$repo_root/.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-candidate-manifest.json"
[ "$(shasum -a 256 "$source_lock" | awk '{print $1}')" = \
  8c75b532951de6ce8372fb76584d8246d625e310b7300e0af9c022b8cd903abb ]
[ "$(shasum -a 256 "$candidate_record" | awk '{print $1}')" = \
  ec03f11e6b46512994ce2d9c389b019102feadb11398a986ff346b6f3d234167 ]
python3 - "$repo_root" "$source_lock" "$candidate_record" <<'PY'
import hashlib
import json
import os
import subprocess
import sys

repo, lock_path, candidate_path = sys.argv[1:]
with open(lock_path, "r", encoding="utf-8") as handle:
    source_lock = json.load(handle)
with open(candidate_path, "r", encoding="utf-8") as handle:
    candidate = json.load(handle)

with open(lock_path, "rb") as handle:
    lock_digest = hashlib.sha256(handle.read()).hexdigest()
assert candidate["source_lock_sha256"] == lock_digest

raw = subprocess.check_output(
    ["go", "list", "-deps", "-test", "-json", "./..."],
    cwd=repo,
    text=True,
)
decoder = json.JSONDecoder()
offset = 0
packages = []
while offset < len(raw):
    while offset < len(raw) and raw[offset].isspace():
        offset += 1
    if offset >= len(raw):
        break
    package, offset = decoder.raw_decode(raw, offset)
    packages.append(package)

fields = (
    "GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles",
    "FFiles", "SFiles", "SwigFiles", "SwigCXXFiles", "SysoFiles",
    "TestGoFiles", "XTestGoFiles", "EmbedFiles", "TestEmbedFiles",
    "XTestEmbedFiles",
)
repo_prefix = repo + os.sep
go_inputs = set()
for package in packages:
    directory = package.get("Dir", "")
    if directory != repo and not directory.startswith(repo_prefix):
        continue
    for field in fields:
        for name in package.get(field, []):
            absolute = os.path.realpath(os.path.join(directory, name))
            if absolute != repo and not absolute.startswith(repo_prefix):
                continue
            assert os.path.isfile(absolute)
            go_inputs.add(os.path.relpath(absolute, repo))

locked = source_lock["locked_inputs"]
assert go_inputs <= set(locked)
assert source_lock["input_closure"]["go_input_count"] == len(go_inputs)
assert source_lock["input_closure"]["locked_input_count"] == len(locked)
for relative, expected in locked.items():
    absolute = os.path.join(repo, relative)
    assert os.path.isfile(absolute)
    with open(absolute, "rb") as handle:
        assert hashlib.sha256(handle.read()).hexdigest() == expected
PY

fixture_parent=$(mktemp -d /private/tmp/loom-local-product-closure-test.XXXXXX)
chmod 700 "$fixture_parent"
cleanup() {
  if [ -d "$fixture_parent" ] && [ ! -L "$fixture_parent" ]; then
    chmod -R u+rwX "$fixture_parent" 2>/dev/null || true
    rm -rf "$fixture_parent"
  fi
}
trap cleanup EXIT HUP INT TERM

make_fixture() {
  name=$1
  scenario=$2
  root="$fixture_parent/$name"
  mkdir -m 700 "$root" "$root/control"
  printf '%s\n' loom-local-product-closure-fixture-v1 >"$root/.loom-local-product-closure-fixture"
  printf '%s\n' "$scenario" >"$root/control/scenario"
  chmod 600 "$root/.loom-local-product-closure-fixture" "$root/control/scenario"
  printf '%s\n' "$root"
}

capture_case() {
  root=$1
  expected_status=$2
  output="$root/control/output"
  set +e
  "$harness" --fixture-root "$root" >"$output" 2>&1
  status=$?
  set -e
  chmod 600 "$output"
  [ "$status" -eq "$expected_status" ]
  if grep -E \
    'API_KEY|PASSWORD|PRIVATE_KEY|BEARER|AUTHORIZATION|(^|[^a-z])sk-' \
    "$output" >/dev/null 2>&1
  then
    echo "fixture output exposed a prohibited credential surface" >&2
    exit 1
  fi
}

assert_exact_output() {
  root=$1
  expected=$2
  [ "$(cat "$root/control/output")" = "$expected" ]
}

prebootstrap_root=$(make_fixture prebootstrap before_bootstrap)
capture_case "$prebootstrap_root" 20
assert_exact_output "$prebootstrap_root" \
  "LOCAL_PRODUCT_CLOSURE_RESULT result=prebootstrap_failure initial_bootstrap_calls=0 consumed=0 rollback_count=1 restart_count=0"
[ ! -e "$prebootstrap_root/control/bootstrap.count" ]
[ "$(cat "$prebootstrap_root/control/rollback.count")" = 1 ]
[ ! -e "$prebootstrap_root/run" ]

readiness_root=$(make_fixture readiness readiness_failure)
capture_case "$readiness_root" 21
assert_exact_output "$readiness_root" \
  "LOCAL_PRODUCT_CLOSURE_FAILURE phase=readiness reason_recorded=1
LOCAL_PRODUCT_CLOSURE_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0"
[ "$(cat "$readiness_root/control/bootstrap.count")" = 1 ]
[ "$(cat "$readiness_root/control/rollback.count")" = 1 ]
[ "$(cat "$readiness_root/control/original-absent.count")" = 1 ]
[ "$(cat "$readiness_root/control/exact-path-preflight.count")" = 1 ]
[ "$(cat "$readiness_root/control/candidate-install.count")" = 1 ]
[ "$(cat "$readiness_root/control/order")" = \
  "original_absent
exact_path_preflight
candidate_install
bootstrap
reason_recorded
rollback" ]
[ -f "$readiness_root/control/reason.json" ]
[ ! -L "$readiness_root/control/reason.json" ]
[ "$(stat -f '%u' "$readiness_root/control/reason.json")" = "$(id -u)" ]
[ "$(stat -f '%Lp' "$readiness_root/control/reason.json")" = 600 ]
python3 - "$readiness_root/control/reason.json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="ascii") as handle:
    value = json.load(handle)
assert value == {
    "classification": "daemon failed: local_ipc",
    "consumed": 1,
    "failure_phase": "readiness",
    "initial_bootstrap_calls": 1,
    "launchd_exit_code": 4,
    "launchd_runs": 1,
    "launchd_state": "exited",
    "restart_count": 0,
    "rollback_count": 1,
    "schema_version": 1,
}
PY
[ ! -e "$readiness_root/run" ]

success_root=$(make_fixture success ready)
capture_case "$success_root" 0
assert_exact_output "$success_root" \
  "LOCAL_PRODUCT_CLOSURE_RESULT result=fixture_pass initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1"
[ "$(cat "$success_root/control/bootstrap.count")" = 2 ]
[ ! -e "$success_root/control/rollback.count" ]
[ "$(cat "$success_root/control/restart.count")" = 1 ]
[ "$(cat "$success_root/control/original-absent.count")" = 1 ]
[ "$(cat "$success_root/control/exact-path-preflight.count")" = 1 ]
[ "$(cat "$success_root/control/candidate-install.count")" = 1 ]
[ "$(cat "$success_root/control/order")" = \
  "original_absent
exact_path_preflight
candidate_install
bootstrap
restart
bootstrap" ]
[ ! -e "$success_root/control/reason.json" ]
[ ! -e "$success_root/run" ]

signal_root=$(make_fixture exact-path-signal exact_path_signal)
signal_output="$signal_root/control/output"
"$harness" --fixture-root "$signal_root" >"$signal_output" 2>&1 &
signal_harness_pid=$!
signal_ready=0
for _ in $(jot 100)
do
  if [ -f "$signal_root/control/exact-child.pid" ] &&
    [ -f "$signal_root/control/exact-child.ready" ]
  then
    signal_ready=1
    break
  fi
  kill -0 "$signal_harness_pid" 2>/dev/null || break
  sleep 0.02
done
[ "$signal_ready" -eq 1 ]
exact_child_pid=$(cat "$signal_root/control/exact-child.pid")
kill -0 "$exact_child_pid"
kill -TERM "$signal_harness_pid"
set +e
wait "$signal_harness_pid"
signal_status=$?
set -e
[ "$signal_status" -eq 98 ]
chmod 600 "$signal_output"
for _ in $(jot 100)
do
  kill -0 "$exact_child_pid" 2>/dev/null || break
  sleep 0.02
done
if kill -0 "$exact_child_pid" 2>/dev/null; then
  echo "signal path orphaned the exact-path Candidate child" >&2
  exit 1
fi
[ "$(cat "$signal_root/control/rollback.count")" = 1 ]
[ "$(cat "$signal_root/control/order")" = \
  "original_absent
exact_path_preflight_started
exact_path_candidate_stopped
rollback" ]
[ "$(cat "$signal_output")" = \
  "LOCAL_PRODUCT_CLOSURE_RESULT result=signal_rolled_back initial_bootstrap_calls=0 consumed=0 rollback_count=1 restart_count=0" ]
[ ! -e "$signal_root/run" ]

for failure_phase in \
  bootstrap \
  readiness \
  run_root \
  socket \
  process_identity \
  product_identity \
  app_identity \
  status_snapshot \
  journal \
  crash_inventory \
  staging \
  unclassified
do
  phase_root=$(make_fixture "phase-$failure_phase" "failure_phase_$failure_phase")
  capture_case "$phase_root" 21
  assert_exact_output "$phase_root" \
    "LOCAL_PRODUCT_CLOSURE_FAILURE phase=$failure_phase reason_recorded=1
LOCAL_PRODUCT_CLOSURE_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0"
  [ "$(cat "$phase_root/control/bootstrap.count")" = 1 ]
  [ "$(cat "$phase_root/control/rollback.count")" = 1 ]
  [ ! -e "$phase_root/run" ]
  python3 - "$phase_root/control/reason.json" "$failure_phase" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="ascii") as handle:
    value = json.load(handle)
assert set(value) == {
    "classification",
    "consumed",
    "failure_phase",
    "initial_bootstrap_calls",
    "launchd_exit_code",
    "launchd_runs",
    "launchd_state",
    "restart_count",
    "rollback_count",
    "schema_version",
}
assert value["failure_phase"] == sys.argv[2]
PY
done

delayed_recovery_root=$(make_fixture delayed-recovery delayed_recovery)
capture_case "$delayed_recovery_root" 20
assert_exact_output "$delayed_recovery_root" \
  "LOCAL_PRODUCT_CLOSURE_RESULT result=delayed_recovery_pass initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0"
[ "$(cat "$delayed_recovery_root/control/recovery-probes.count")" = 81 ]

existing_reason_root=$(make_fixture existing-reason failure_phase_readiness)
printf '%s\n' unchanged >"$existing_reason_root/control/reason.json"
chmod 600 "$existing_reason_root/control/reason.json"
existing_reason_digest=$(
  shasum -a 256 "$existing_reason_root/control/reason.json" |
    awk '{print $1}'
)
capture_case "$existing_reason_root" 21
assert_exact_output "$existing_reason_root" \
  "LOCAL_PRODUCT_CLOSURE_FAILURE phase=readiness reason_recorded=0
LOCAL_PRODUCT_CLOSURE_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0"
[ "$existing_reason_digest" = "$(
  shasum -a 256 "$existing_reason_root/control/reason.json" |
    awk '{print $1}'
)" ]

reason_symlink_root=$(make_fixture reason-symlink failure_phase_readiness)
reason_symlink_target="$fixture_parent/reason-symlink-target"
printf '%s\n' unchanged >"$reason_symlink_target"
chmod 600 "$reason_symlink_target"
reason_symlink_digest=$(
  shasum -a 256 "$reason_symlink_target" |
    awk '{print $1}'
)
ln -s "$reason_symlink_target" "$reason_symlink_root/control/reason.json"
capture_case "$reason_symlink_root" 21
assert_exact_output "$reason_symlink_root" \
  "LOCAL_PRODUCT_CLOSURE_FAILURE phase=readiness reason_recorded=0
LOCAL_PRODUCT_CLOSURE_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0"
[ "$reason_symlink_digest" = "$(
  shasum -a 256 "$reason_symlink_target" |
    awk '{print $1}'
)" ]

missing_sentinel_root="$fixture_parent/missing-sentinel"
mkdir -m 700 "$missing_sentinel_root"
if "$harness" --fixture-root "$missing_sentinel_root" >/dev/null 2>&1; then
  echo "fixture without sentinel was accepted" >&2
  exit 1
fi

wrong_mode_root=$(make_fixture wrong-mode before_bootstrap)
chmod 755 "$wrong_mode_root"
if "$harness" --fixture-root "$wrong_mode_root" >/dev/null 2>&1; then
  echo "non-private fixture root was accepted" >&2
  exit 1
fi
chmod 700 "$wrong_mode_root"

symlink_target=$(make_fixture symlink-target before_bootstrap)
ln -s "$symlink_target" "$fixture_parent/symlink-root"
if "$harness" --fixture-root "$fixture_parent/symlink-root" >/dev/null 2>&1; then
  echo "symlinked fixture root was accepted" >&2
  exit 1
fi

control_symlink_root=$(make_fixture control-symlink before_bootstrap)
mv "$control_symlink_root/control" "$control_symlink_root/control-real"
ln -s "$control_symlink_root/control-real" "$control_symlink_root/control"
if "$harness" --fixture-root "$control_symlink_root" >/dev/null 2>&1; then
  echo "symlinked fixture control directory was accepted" >&2
  exit 1
fi

run_symlink_root=$(make_fixture run-symlink before_bootstrap)
mkdir -m 700 "$run_symlink_root/external-run"
ln -s "$run_symlink_root/external-run" "$run_symlink_root/run"
if "$harness" --fixture-root "$run_symlink_root" >/dev/null 2>&1; then
  echo "symlinked fixture run directory was accepted" >&2
  exit 1
fi

assert_counter_symlink_rejected() {
  name=$1
  scenario=$2
  counter=$3
  root=$(make_fixture "$name" "$scenario")
  outside="$fixture_parent/$name-outside"
  printf '%s\n' unchanged >"$outside"
  chmod 600 "$outside"
  outside_digest=$(shasum -a 256 "$outside" | awk '{print $1}')
  ln -s "$outside" "$root/control/$counter"
  if "$harness" --fixture-root "$root" >/dev/null 2>&1; then
    echo "symlinked fixture counter was accepted" >&2
    exit 1
  fi
  [ "$outside_digest" = "$(shasum -a 256 "$outside" | awk '{print $1}')" ]
}

assert_counter_symlink_rejected \
  bootstrap-counter-symlink \
  readiness_failure \
  bootstrap.count
assert_counter_symlink_rejected \
  rollback-counter-symlink \
  before_bootstrap \
  rollback.count
assert_counter_symlink_rejected \
  restart-counter-symlink \
  ready \
  restart.count

escape_root=$(make_fixture escape before_bootstrap)
escape_spelling="$fixture_parent/escape/../escape"
if "$harness" --fixture-root "$escape_spelling" >/dev/null 2>&1; then
  echo "non-canonical fixture spelling was accepted" >&2
  exit 1
fi

unknown_root=$(make_fixture unknown unknown)
if "$harness" --fixture-root "$unknown_root" >/dev/null 2>&1; then
  echo "unknown fixture decision was accepted" >&2
  exit 1
fi

override_root=$(make_fixture ambient-override before_bootstrap)
for override_name in \
  LOOM_LOCAL_PRODUCT_CLOSURE_LIVE_ROOT \
  LOOM_LOCAL_PRODUCT_CLOSURE_SERVICE_LABEL \
  LOOM_LOCAL_PRODUCT_CLOSURE_PLIST \
  LOOM_LOCAL_PRODUCT_CLOSURE_SOCKET \
  LOOM_LOCAL_PRODUCT_CLOSURE_APP \
  LOOM_LOCAL_PRODUCT_CLOSURE_UID
do
  if /usr/bin/env "$override_name=blocked" \
    "$harness" --fixture-root "$override_root" >/dev/null 2>&1
  then
    echo "ambient production override was accepted" >&2
    exit 1
  fi
done

ambient_root=$(make_fixture ambient-command-controls ready)
ambient_python_dir="$fixture_parent/ambient-python"
ambient_python_marker="$fixture_parent/ambient-python-executed"
ambient_shell_marker="$fixture_parent/ambient-shell-executed"
ambient_shell_init="$fixture_parent/ambient-shell-init"
mkdir -m 700 "$ambient_python_dir"
printf '%s\n' \
  'from pathlib import Path' \
  "Path('$ambient_python_marker').write_text('executed')" \
  >"$ambient_python_dir/sitecustomize.py"
printf '%s\n' \
  "printf '%s\\n' executed >'$ambient_shell_marker'" \
  >"$ambient_shell_init"
chmod 600 "$ambient_python_dir/sitecustomize.py" "$ambient_shell_init"
ambient_output="$ambient_root/control/ambient-output"
/usr/bin/env \
  BASH_ENV="$ambient_shell_init" \
  ENV="$ambient_shell_init" \
  PYTHONPATH="$ambient_python_dir" \
  GIT_INDEX_FILE="$fixture_parent/ambient-index" \
  LOOM_ROLLBACK_TEST_SIGNAL_STEP=after_current_loom \
  "$harness" --fixture-root "$ambient_root" >"$ambient_output" 2>&1
chmod 600 "$ambient_output"
[ ! -e "$ambient_python_marker" ] && [ ! -L "$ambient_python_marker" ]
[ ! -e "$ambient_shell_marker" ] && [ ! -L "$ambient_shell_marker" ]
[ "$(cat "$ambient_output")" = \
  "LOCAL_PRODUCT_CLOSURE_RESULT result=fixture_pass initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1" ]

clean_marker_bypass_root=$(make_fixture clean-marker-bypass ready)
if /usr/bin/env -i \
  PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  LC_ALL=C \
  LOOM_LOCAL_PRODUCT_CLOSURE_ENVIRONMENT_READY=1 \
  PYTHONPATH="$ambient_python_dir" \
  "$harness" --fixture-root "$clean_marker_bypass_root" >/dev/null 2>&1
then
  echo "forged clean-environment marker accepted an injected control" >&2
  exit 1
fi
[ ! -e "$ambient_python_marker" ] && [ ! -L "$ambient_python_marker" ]

if "$harness" --fixture-root "$success_root" --unknown >/dev/null 2>&1; then
  echo "unknown harness argument was accepted" >&2
  exit 1
fi

foreign_root=$(make_fixture foreign-owner before_bootstrap)
if chown 0 "$foreign_root" >/dev/null 2>&1; then
  if "$harness" --fixture-root "$foreign_root" >/dev/null 2>&1; then
    echo "foreign-owned fixture root was accepted" >&2
    exit 1
  fi
  chown "$(id -u)" "$foreign_root"
fi

echo "local product closure transaction fixture PASS"
