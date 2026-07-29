#!/bin/sh
set -eu

p2a_w1_verify_process_environment() {
  if [ "$#" -ne 2 ]; then
    printf '%s\n' invalid_pid
    return 1
  fi
  ps_command=$1
  process_id=$2
  case "$process_id" in
    ''|*[!0-9]*)
      printf '%s\n' invalid_pid
      return 1
      ;;
  esac
  case "$process_id" in
    *[!0]*) ;;
    *)
      printf '%s\n' invalid_pid
      return 1
      ;;
  esac

  if ! process_row=$(
    "$ps_command" eww -p "$process_id" -o command= 2>/dev/null
  )
  then
    printf '%s\n' target_unavailable
    return 1
  fi
  row_count=$(
    /usr/bin/printf '%s\n' "$process_row" |
      /usr/bin/awk 'NF { count++ } END { print count + 0 }'
  )
  if [ "$row_count" -ne 1 ]; then
    printf '%s\n' target_unavailable
    return 1
  fi
  if /usr/bin/printf '%s\n' "$process_row" |
    /usr/bin/grep -Eiq \
      'api[_-]?key|authorization|bearer|credential|secret|token|OPENAI|MINIMAX|DEEPSEEK|STEPFUN'
  then
    printf '%s\n' forbidden_marker
    return 1
  fi
  printf '%s\n' clean
}

if [ "${LOOM_P2A_W1_SOURCE_ONLY:-}" = 1 ]; then
  return 0 2>/dev/null || exit 0
fi

if [ "$#" -ne 1 ]; then
  printf '%s\n' invalid_pid
  exit 1
fi
p2a_w1_verify_process_environment /bin/ps "$1"
