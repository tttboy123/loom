#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
harness="$repo_root/.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction.sh"

if [ ! -f "$harness" ] || [ ! -x "$harness" ]; then
  echo "RED: governed final exit transaction harness is missing" >&2
  exit 1
fi

assert_frozen_constant() {
  name=$1
  value=$2
  grep -Fqx "$name='$value'" "$harness"
}

assert_frozen_constant expected_candidate_loom \
  3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f
assert_frozen_constant expected_candidate_loomd \
  ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357
assert_frozen_constant expected_candidate_app \
  f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b
assert_frozen_constant expected_candidate_uuid \
  CE91F84E-4333-35DB-B493-88FADCBC6EC1
assert_frozen_constant expected_candidate_manifest \
  e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
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

fixture_parent=$(mktemp -d /private/tmp/loom-final-exit-test.XXXXXX)
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
  printf '%s\n' loom-final-exit-fixture-v1 >"$root/.loom-final-exit-fixture"
  printf '%s\n' "$scenario" >"$root/control/scenario"
  chmod 600 "$root/.loom-final-exit-fixture" "$root/control/scenario"
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
  "FINAL_EXIT_RESULT result=prebootstrap_failure initial_bootstrap_calls=0 consumed=0 rollback_count=1 restart_count=0"
[ ! -e "$prebootstrap_root/control/bootstrap.count" ]
[ "$(cat "$prebootstrap_root/control/rollback.count")" = 1 ]
[ ! -e "$prebootstrap_root/run" ]

readiness_root=$(make_fixture readiness readiness_failure)
capture_case "$readiness_root" 21
assert_exact_output "$readiness_root" \
  "FINAL_EXIT_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0"
[ "$(cat "$readiness_root/control/bootstrap.count")" = 1 ]
[ "$(cat "$readiness_root/control/rollback.count")" = 1 ]
[ ! -e "$readiness_root/run" ]

success_root=$(make_fixture success ready)
capture_case "$success_root" 0
assert_exact_output "$success_root" \
  "FINAL_EXIT_RESULT result=fixture_pass initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1"
[ "$(cat "$success_root/control/bootstrap.count")" = 2 ]
[ ! -e "$success_root/control/rollback.count" ]
[ "$(cat "$success_root/control/restart.count")" = 1 ]
[ ! -e "$success_root/run" ]

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
  LOOM_FINAL_EXIT_LIVE_ROOT \
  LOOM_FINAL_EXIT_SERVICE_LABEL \
  LOOM_FINAL_EXIT_PLIST \
  LOOM_FINAL_EXIT_SOCKET \
  LOOM_FINAL_EXIT_APP \
  LOOM_FINAL_EXIT_UID
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
  "FINAL_EXIT_RESULT result=fixture_pass initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=1" ]

clean_marker_bypass_root=$(make_fixture clean-marker-bypass ready)
if /usr/bin/env -i \
  PATH=/usr/bin:/bin:/usr/sbin:/sbin \
  LC_ALL=C \
  LOOM_FINAL_EXIT_ENVIRONMENT_READY=1 \
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

echo "final exit transaction fixture PASS"
