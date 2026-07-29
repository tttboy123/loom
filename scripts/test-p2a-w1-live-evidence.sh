#!/bin/sh
set -eu

repository_root=$(
  CDPATH= cd -- "$(dirname -- "$0")/.." &&
    pwd -P
)
helper="$repository_root/scripts/p2a-w1-live-evidence.sh"

if [ ! -f "$helper" ]; then
  printf '%s\n' "missing reviewed live-evidence helper" >&2
  exit 1
fi

test_root=$(/usr/bin/mktemp -d /tmp/loom-p2a-w1-evidence-test.XXXXXX)
/bin/chmod 700 "$test_root"
trap '/bin/rm -R "$test_root"' EXIT HUP INT TERM

fake_ps="$test_root/ps"
args_log="$test_root/args"
/usr/bin/printf '%s\n' \
  '#!/bin/sh' \
  'set -eu' \
  ': "${FAKE_PS_MODE:?}"' \
  ': "${FAKE_PS_ARGS_LOG:?}"' \
  'printf "%s\n" "$*" >"$FAKE_PS_ARGS_LOG"' \
  'case "$FAKE_PS_MODE" in' \
  '  clean)' \
  '    printf "%s\n" "/safe/loomd HOME=/Users/lune PATH=/usr/bin:/bin"' \
  '    ;;' \
  '  forbidden)' \
  '    printf "%s\n" "/safe/loomd MINIMAX_API_KEY=sentinel-sensitive-value"' \
  '    ;;' \
  '  multi)' \
  '    printf "%s\n" "/safe/loomd one" "/unrelated/process two"' \
  '    ;;' \
  '  zero)' \
  '    ;;' \
  '  unavailable)' \
  '    exit 1' \
  '    ;;' \
  '  *)' \
  '    exit 2' \
  '    ;;' \
  'esac' >"$fake_ps"
/bin/chmod 700 "$fake_ps"

LOOM_P2A_W1_SOURCE_ONLY=1
export LOOM_P2A_W1_SOURCE_ONLY
. "$helper"

run_case() {
  mode=$1
  expected_status=$2
  expected_exit=$3
  FAKE_PS_MODE=$mode
  FAKE_PS_ARGS_LOG=$args_log
  export FAKE_PS_MODE FAKE_PS_ARGS_LOG
  set +e
  output=$(p2a_w1_verify_process_environment "$fake_ps" 4242)
  code=$?
  set -e
  if [ "$code" -ne "$expected_exit" ] || [ "$output" != "$expected_status" ]; then
    printf '%s\n' \
      "$mode: code=$code status=$output, want $expected_exit/$expected_status" \
      >&2
    exit 1
  fi
  if [ "$(/bin/cat "$args_log")" != "eww -p 4242 -o command=" ]; then
    printf '%s\n' "$mode: unexpected ps argument vector" >&2
    exit 1
  fi
}

run_case clean clean 0
run_case forbidden forbidden_marker 1
run_case multi target_unavailable 1
run_case zero target_unavailable 1
run_case unavailable target_unavailable 1

for invalid_pid in "" 0 00 000000 -1 +1 " 1" "1 " 1a
do
  set +e
  output=$(p2a_w1_verify_process_environment "$fake_ps" "$invalid_pid")
  code=$?
  set -e
  if [ "$code" -ne 1 ] || [ "$output" != invalid_pid ]; then
    printf '%s\n' \
      "invalid PID: code=$code status=$output" \
      >&2
    exit 1
  fi
done

set +e
forbidden_output=$(
  FAKE_PS_MODE=forbidden \
    FAKE_PS_ARGS_LOG="$args_log" \
    p2a_w1_verify_process_environment "$fake_ps" 4242 2>&1
)
forbidden_code=$?
set -e
if [ "$forbidden_code" -ne 1 ] ||
  [ "$forbidden_output" != forbidden_marker ] ||
  /usr/bin/grep -q 'sentinel-sensitive-value' <<EOF
$forbidden_output
EOF
then
  printf '%s\n' "forbidden value escaped closed status" >&2
  exit 1
fi

if /usr/bin/grep -q -- '-eww' "$helper"; then
  printf '%s\n' "former all-process ps form remains" >&2
  exit 1
fi

printf '%s\n' "P2A-W1 live evidence helper tests: PASS"
