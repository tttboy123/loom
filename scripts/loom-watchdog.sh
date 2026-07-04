#!/usr/bin/env bash
# =============================================================================
# scripts/loom-watchdog.sh
#
# Layer-1 watchdog for the Loom autopilot. Runs every 60s (via launchd /
# systemd timer — see scripts/install-watchdog.sh).
#
# Responsibilities (k8s-inspired; see docs/loom-autopilot-self-healing-2026-07-05.md):
#   1. Verify supervisor is alive (PID file + actual process)
#   2. Verify daemon heartbeat is fresh (< HEARTBEAT_STALE_S)
#   3. Restart failed components with exponential backoff (CrashLoopBackOff)
#   4. Quarantine (no auto-restart) after MAX_FAILS consecutive failures
#   5. Clean up orphan PID files
#
# This script is **the smallest possible external supervisor**. It does NOT
# contain any Loom domain knowledge — just process liveness + heartbeat.
#
# Usage:
#   scripts/loom-watchdog.sh                # run one tick (default)
#   scripts/loom-watchdog.sh --install      # install as launchd/systemd unit
#   scripts/loom-watchdog.sh --uninstall    # remove the installed unit
#   scripts/loom-watchdog.sh --status       # print current health snapshot
# =============================================================================
set -euo pipefail

# ----- Defaults (override via env) -----
ROOT="${LOOM_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
LOG="$ROOT/devkit/logs"
mkdir -p "$LOG"

# IMPORTANT: the supervisor writes the worker's pid to `loom-iterate-daemon.pid`
# (see scripts/loom-iterate-supervisor.sh:46). We must use the same file, not
# a different name like `daemon.pid`, otherwise the watchdog thinks the
# daemon never started.
SUP_PID_FILE="${LOOM_SUP_PID:-$LOG/loom-iterate-supervisor.pid}"
DAEMON_PID_FILE="${LOOM_DAEMON_PID:-$LOG/loom-iterate-daemon.pid}"
HEARTBEAT="${LOOM_HEARTBEAT:-$LOG/heartbeat.daemon}"
STATE="${LOOM_AUTOPILOT_STATE:-$LOG/autopilot.state}"
BACKOFF="${LOOM_BACKOFF:-$LOG/backoff.json}"

SUPERVISOR_CMD="${LOOM_SUPERVISOR_CMD:-bash scripts/loom-iterate-supervisor.sh --backlog devkit/backlog.json --max-rounds 20 --reflect-carrier minimax --compact-model deepseek --sleep 60}"

HEARTBEAT_STALE_S="${LOOM_HEARTBEAT_STALE_S:-120}"   # 2min stale = dead
MAX_FAILS="${LOOM_MAX_FAILS:-5}"
MAX_BACKOFF_S="${LOOM_MAX_BACKOFF_S:-300}"          # 5min cap

# ----- Helpers -----
ts() { date '+%Y-%m-%d %H:%M:%S %z'; }
log() { printf '[%s] %s\n' "$(ts)" "$*" | tee -a "$LOG/watchdog.log" >&2; }

# macOS stat syntax differs from Linux; support both
file_mtime() {
  if stat -f %m "$1" 2>/dev/null | grep -q '^[0-9]'; then
    stat -f %m "$1"
  else
    stat -c %Y "$1"
  fi
}

now_epoch() { date +%s; }

pid_alive() {
  local pid_file="$1"
  [[ -f "$pid_file" ]] || return 1
  local pid
  pid=$(cat "$pid_file" 2>/dev/null || true)
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

read_backoff() {
  if [[ -f "$BACKOFF" ]]; then
    python3 -c "import json,sys
try:
  d=json.load(open('$BACKOFF'))
  print(int(d.get('consec_failures',0)))
except Exception:
  print(0)
" 2>/dev/null || echo 0
  else
    echo 0
  fi
}

write_backoff() {
  local n="$1" reason="$2"
  python3 - "$BACKOFF" "$n" "$reason" <<'PY'
import json, sys
from datetime import datetime
path, n, reason = sys.argv[1], int(sys.argv[2]), sys.argv[3]
data = {
  "consec_failures": n,
  "last_reason": reason,
  "last_attempt_at": datetime.now().isoformat(),
}
try:
  with open(path, "w") as f:
    json.dump(data, f, indent=2)
except Exception as exc:
  print(f"watchdog: failed to write backoff: {exc}", file=sys.stderr)
PY
}

backoff_seconds() {
  # 1 → 1, 2 → 2, 3 → 4, 4 → 8, 5+ → 16 capped at MAX_BACKOFF_S
  local n="$1"
  if (( n <= 1 )); then echo 1; return; fi
  local s=$(( 2 ** (n - 1) ))
  (( s > MAX_BACKOFF_S )) && s=$MAX_BACKOFF_S
  echo "$s"
}

write_state() {
  # Used to expose current autopilot health to ./loom doctor
  local state="$1" reason="$2"
  python3 - "$STATE" "$state" "$reason" "$SUP_PID_FILE" "$DAEMON_PID_FILE" "$HEARTBEAT" <<'PY'
import json, sys, os, time
from datetime import datetime
path, state, reason, sup_pid_f, daemon_pid_f, hb = sys.argv[1:7]
sup_pid = open(sup_pid_f).read().strip() if os.path.exists(sup_pid_f) else ""
daemon_pid = open(daemon_pid_f).read().strip() if os.path.exists(daemon_pid_f) else ""
last_hb = ""
if os.path.exists(hb):
    last_hb = datetime.fromtimestamp(os.path.getmtime(hb)).isoformat()
data = {
  "state": state,
  "reason": reason,
  "since": datetime.now().isoformat(),
  "supervisor_pid": sup_pid,
  "daemon_pid": daemon_pid,
  "last_heartbeat": last_hb,
}
try:
  with open(path, "w") as f:
    json.dump(data, f, indent=2)
except Exception as exc:
  print(f"watchdog: failed to write state: {exc}", file=sys.stderr)
PY
}

is_quarantined() {
  [[ -f "$STATE" ]] && grep -q '"state": "quarantined"' "$STATE" 2>/dev/null
}

# ----- Core logic -----
check_supervisor() {
  if pid_alive "$SUP_PID_FILE"; then
    return 0
  fi
  # Don't try to spawn a brand-new supervisor if the file just doesn't exist
  # yet (e.g. fresh install). Only respawn if there IS a pid file but the
  # process is dead — that means the previous supervisor crashed.
  if [[ ! -f "$SUP_PID_FILE" ]]; then
    log "supervisor pid file missing ($SUP_PID_FILE) — autopilot not yet started, skipping respawn"
    return 1
  fi
  log "supervisor not running (pid file: $SUP_PID_FILE)"
  (
    cd "$ROOT"
    nohup bash -c "$SUPERVISOR_CMD" >> "$LOG/supervisor.restart.log" 2>&1 &
    echo $! > "$SUP_PID_FILE"
  )
  sleep 2
  if pid_alive "$SUP_PID_FILE"; then
    log "supervisor restarted, new pid=$(cat "$SUP_PID_FILE")"
    return 0
  fi
  return 1
}

check_daemon() {
  local reason="ok"
  if ! pid_alive "$DAEMON_PID_FILE"; then
    reason="daemon pid file stale (pid not alive)"
  elif [[ ! -f "$HEARTBEAT" ]]; then
    reason="daemon heartbeat missing"
  else
    local age=$(( $(now_epoch) - $(file_mtime "$HEARTBEAT") ))
    if (( age > HEARTBEAT_STALE_S )); then
      reason="daemon heartbeat stale (${age}s > ${HEARTBEAT_STALE_S}s)"
    fi
  fi

  if [[ "$reason" == "ok" ]]; then
    write_backoff 0 "ok"
    write_state "running" "ok"
    return 0
  fi

  log "daemon unhealthy: $reason"

  # Kill stale daemon if pid exists
  if [[ -f "$DAEMON_PID_FILE" ]]; then
    local dp
    dp=$(cat "$DAEMON_PID_FILE" 2>/dev/null || true)
    if [[ -n "$dp" ]]; then
      kill -9 "$dp" 2>/dev/null || true
    fi
    rm -f "$DAEMON_PID_FILE"
  fi

  # Bump backoff
  local n
  n=$(read_backoff)
  n=$(( n + 1 ))
  write_backoff "$n" "$reason"

  if (( n >= MAX_FAILS )); then
    log "MAX_FAILS reached (n=$n) — quarantining, no auto-restart"
    write_state "quarantined" "$reason"
    return 1
  fi

  # Apply backoff
  local delay
  delay=$(backoff_seconds "$n")
  log "backoff: sleep ${delay}s before signaling supervisor (attempt $n)"
  sleep "$delay"

  # Tell supervisor to restart its worker (SIGUSR1 = please restart worker)
  if pid_alive "$SUP_PID_FILE"; then
    local sp
    sp=$(cat "$SUP_PID_FILE")
    kill -USR1 "$sp" 2>/dev/null || true
    log "sent SIGUSR1 to supervisor pid=$sp"
  else
    log "supervisor also dead — restart loop will pick it up next tick"
  fi

  write_state "degraded" "$reason"
  return 1
}

# ----- Entry -----
cmd="${1:-tick}"
case "$cmd" in
  --install)
    exec bash "$ROOT/scripts/install-watchdog.sh" install
    ;;
  --uninstall)
    exec bash "$ROOT/scripts/install-watchdog.sh" uninstall
    ;;
  --status)
    echo "Loom autopilot status:"
    if [[ -f "$STATE" ]]; then
      cat "$STATE"
      echo
    fi
    echo "supervisor pid: $(cat "$SUP_PID_FILE" 2>/dev/null || echo 'NONE')"
    echo "daemon pid:     $(cat "$DAEMON_PID_FILE" 2>/dev/null || echo 'NONE')"
    if [[ -f "$HEARTBEAT" ]]; then
      echo "heartbeat age:  $(( $(now_epoch) - $(file_mtime "$HEARTBEAT") ))s"
    else
      echo "heartbeat:      MISSING"
    fi
    exit 0
    ;;
  tick|"")
    if is_quarantined; then
      log "quarantined: not auto-restarting. Run ./loom doctor for details."
      exit 1
    fi
    check_supervisor || log "supervisor check failed"
    check_daemon     || log "daemon check failed"
    ;;
  *)
    echo "Usage: $0 [tick|--install|--uninstall|--status]"
    exit 2
    ;;
esac