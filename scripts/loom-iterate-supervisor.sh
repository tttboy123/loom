#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

USAGE='用法：
  scripts/loom-iterate-supervisor.sh [options]

说明：
  该脚本是 iterate 守护进程的监督器，默认每 300 秒巡检一次。
  - 子进程退出 / 停住：自动重启 iterate
  - 发现明显报错（如 traceback / exception / selected model is at capacity）：尝试重部署并重启 iterate

选项:
  --backlog <path>             backlog 文件（默认 devkit/backlog.json）
  --max-rounds <N>             每轮执行任务数上限（默认 20）
  --reflect-carrier <name>      反思模型（默认 minimax）
  --compact-model <name>        压缩模型（默认 deepseek）
  --sleep <sec>                 每轮间隔秒数（默认 60）
  --log-file <path>             iterate 工作日志（默认 devkit/logs/iterate-daemon.log）
  --supervisor-log-file <path>   监督器日志（默认 devkit/logs/iterate-supervisor.log）
  --check-interval <sec>        健康检查间隔（默认 300）
  --check-error-lines <N>       每次扫描日志行数（默认 120）
  --restart-cooldown <sec>      重启冷却（默认 90）
  --max-restarts <N>            5 分钟内最大重启次数（默认 15）
  --once                        只执行一次 iterate，不启动监督循环
  --no-compact                  关闭上下文压缩（默认开启）
  -h, --help                   查看帮助
'

BACKLOG="devkit/backlog.json"
MAX_ROUNDS=20
REFLECT_CARRIER="minimax"
COMPACT_MODEL="deepseek"
SLEEP_SECONDS=60
ITERATE_LOG_FILE="$ROOT/devkit/logs/iterate-daemon.log"
SUPERVISOR_LOG_FILE="$ROOT/devkit/logs/iterate-supervisor.log"
CHECK_INTERVAL=300
CHECK_ERROR_LINES=120
RESTART_COOLDOWN=90
MAX_RESTARTS=15
ONCE=0
NO_COMPACT=0
WORKER_PID_FILE="$ROOT/devkit/logs/loom-iterate-daemon.pid"
WORKER_OUT_FILE="$ROOT/devkit/logs/loom-iterate-daemon.out"
LAST_LOG_LINES_FILE="$ROOT/devkit/logs/iterate-supervisor-log.state"
LAST_RESTART_TS_FILE="$ROOT/devkit/logs/iterate-supervisor.state"
WORKER_CMD="$ROOT/scripts/loom-iterate-daemon.sh"
TASK_QUEUE_STATUS_SCRIPT="$ROOT/scripts/loom-task-queue-status.py"
TASK_QUEUE_STATUS_LOG_FILE="$ROOT/devkit/logs/task-queue-status.log"
ITERATE_CMD=()

ERROR_PATTERN='(Traceback|Exception:|selected model is at capacity|Selected model is at capacity|Model.*is at capacity|RuntimeError|AssertionError|FATAL|fatal error)'

log() {
  local msg="$1"
  printf '[%s] %s\n' "$(date '+%F %T')" "$msg" >> "$SUPERVISOR_LOG_FILE"
}

# ---------- Signal handlers ----------
# SIGUSR1 = "watchdog wants me to restart the worker NOW" (skips cooldown).
# SIGUSR2 = reserved for future use (e.g. dump state).
# SIGTERM = clean shutdown.
WATCHDOG_RESTART_REQUESTED=0
WATCHDOG_RESTART_REASON=""

on_sigusr1() {
  WATCHDOG_RESTART_REQUESTED=1
  WATCHDOG_RESTART_REASON="watchdog SIGUSR1"
}
on_sigterm() {
  log "收到 SIGTERM，停止 supervise 并 stop_worker"
  stop_worker || true
  exit 0
}
trap on_sigusr1 USR1
trap on_sigterm TERM

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --backlog)
      BACKLOG="$2"; shift 2
      ;;
    --max-rounds)
      MAX_ROUNDS="$2"; shift 2
      ;;
    --reflect-carrier)
      REFLECT_CARRIER="$2"; shift 2
      ;;
    --compact-model)
      COMPACT_MODEL="$2"; shift 2
      ;;
    --sleep)
      SLEEP_SECONDS="$2"; shift 2
      ;;
    --log-file)
      ITERATE_LOG_FILE="$2"; shift 2
      ;;
    --supervisor-log-file)
      SUPERVISOR_LOG_FILE="$2"; shift 2
      ;;
    --check-interval)
      CHECK_INTERVAL="$2"; shift 2
      ;;
    --check-error-lines)
      CHECK_ERROR_LINES="$2"; shift 2
      ;;
    --restart-cooldown)
      RESTART_COOLDOWN="$2"; shift 2
      ;;
    --max-restarts)
      MAX_RESTARTS="$2"; shift 2
      ;;
    --once)
      ONCE=1; shift
      ;;
    --no-compact)
      NO_COMPACT=1; shift
      ;;
    -h|--help)
      printf "%s\n" "$USAGE"
      exit 0
      ;;
    *)
      echo "未知参数：$1"
      printf "%s\n" "$USAGE"
      exit 1
      ;;
  esac
done

mkdir -p "$(dirname "$ITERATE_LOG_FILE")" "$(dirname "$SUPERVISOR_LOG_FILE")"
if [[ ! -f "$BACKLOG" ]]; then
  echo "未找到 backlog 文件: $BACKLOG"
  exit 1
fi

# 启动监督器时，将日志扫描水位线对齐到当前文件末尾，避免把旧 traceback 当成新故障。
if [[ -f "$ITERATE_LOG_FILE" ]]; then
  wc -l < "$ITERATE_LOG_FILE" | tr -d '[:space:]' > "$LAST_LOG_LINES_FILE"
else
  echo "0" > "$LAST_LOG_LINES_FILE"
fi

daemon_cmd() {
  local args=(
    --backlog "$BACKLOG"
    --max-rounds "$MAX_ROUNDS"
    --reflect-carrier "$REFLECT_CARRIER"
    --compact-model "$COMPACT_MODEL"
    --sleep "$SLEEP_SECONDS"
    --log-file "$ITERATE_LOG_FILE"
  )
  if [[ "$NO_COMPACT" -eq 1 ]]; then
    args+=(--no-compact)
  fi
  if [[ "$ONCE" -eq 1 ]]; then
    args+=(--once)
  fi
  ITERATE_CMD=("${args[@]}")
}

iter_pid=""
restart_count=0
window_start=$(date +%s)
window_end=$(( window_start + 300 ))

read_worker_pid() {
  if [[ -f "$WORKER_PID_FILE" ]]; then
    iter_pid="$(cat "$WORKER_PID_FILE")"
  else
    iter_pid=""
  fi
}

is_worker_running() {
  read_worker_pid
  if [[ -z "${iter_pid:-}" ]]; then
    return 1
  fi
  if kill -0 "$iter_pid" 2>/dev/null; then
    return 0
  fi
  return 1
}

stop_worker() {
  is_worker_running || return 0
  log "停止旧 iterate 守护进程 $iter_pid"
  kill -TERM "$iter_pid" 2>/dev/null || true
  sleep 5
  if kill -0 "$iter_pid" 2>/dev/null; then
    kill -KILL "$iter_pid" 2>/dev/null || true
  fi
  rm -f "$WORKER_PID_FILE"
  iter_pid=""
}

build_iterate_cmd() {
  daemon_cmd
}

start_worker() {
  local now
  now=$(date '+%F %T')
  if is_worker_running; then
    return
  fi
  stop_worker
  build_iterate_cmd
  log "启动 iterate worker [$now]：scripts/loom-iterate-daemon.sh ${ITERATE_CMD[*]}"
  mkdir -p "$(dirname "$WORKER_OUT_FILE")"
  if nohup bash "$WORKER_CMD" "${ITERATE_CMD[@]}" >>"$WORKER_OUT_FILE" 2>&1 & then
    iter_pid=$!
    echo "$iter_pid" > "$WORKER_PID_FILE"
    log "iterate worker 启动成功，PID=$iter_pid"
  else
    log "iterate worker 启动失败"
    return 1
  fi
}

detect_backlog_stuck() {
  python3 - "$BACKLOG" <<'PY'
import json
import sys

path = sys.argv[1]
try:
    with open(path, 'r', encoding='utf-8') as fp:
        data = json.load(fp)
except Exception:
    print("0")
    raise SystemExit(0)

items = data if isinstance(data, list) else data.get("tasks", [])
if not isinstance(items, list):
    print("0")
    raise SystemExit(0)

running = [it for it in items if isinstance(it, dict) and str(it.get("status", "")).lower() == "running"]
if not running:
    print("0")
    raise SystemExit(0)

stuck = all(int(it.get("_attempts", 0) or 0) >= 2 for it in running)
print("1" if stuck else "0")
PY
}

has_backlog_stuck() {
  if [[ "$(detect_backlog_stuck)" == "1" ]]; then
    return 0
  fi
  return 1
}

detect_recent_error() {
  if [[ ! -f "$ITERATE_LOG_FILE" ]]; then
    return 1
  fi
  local last_lines current_lines
  current_lines="$(wc -l < "$ITERATE_LOG_FILE" | tr -d '[:space:]')"
  last_lines="$(cat "$LAST_LOG_LINES_FILE" 2>/dev/null | tr -d '[:space:]' || echo 0)"
  if [[ ! "$current_lines" =~ ^[0-9]+$ ]] || [[ ! "$last_lines" =~ ^[0-9]+$ ]]; then
    last_lines=0
  fi
  if (( current_lines < last_lines )); then
    last_lines=0
  fi
  if (( current_lines > last_lines )); then
    if tail -n +$((last_lines + 1)) "$ITERATE_LOG_FILE" 2>/dev/null | tail -n "$CHECK_ERROR_LINES" | grep -Eiq "$ERROR_PATTERN"; then
      echo "$current_lines" > "$LAST_LOG_LINES_FILE"
      return 0
    fi
    echo "$current_lines" > "$LAST_LOG_LINES_FILE"
  fi
  return 1
}

self_heal() {
  local reason="$1"
  local now_ts_epoch last_ts
  now_ts_epoch=$(date +%s)
  last_ts=$(cat "$LAST_RESTART_TS_FILE" 2>/dev/null || echo 0)
  if ! [[ "$last_ts" =~ ^[0-9]+$ ]]; then
    last_ts=0
  fi

  if (( now_ts_epoch - last_ts < RESTART_COOLDOWN )); then
    log "跳过修复（冷却中）：$reason"
    return 0
  fi
  echo "$now_ts_epoch" > "$LAST_RESTART_TS_FILE"

  if (( now_ts_epoch > window_end )); then
    window_start=$(date +%s)
    window_end=$(( window_start + 300 ))
    restart_count=0
  fi

  restart_count=$((restart_count + 1))
  if (( restart_count > MAX_RESTARTS )); then
    log "重启次数超过阈值（$MAX_RESTARTS）跳过，稍后重试：$reason"
    return 1
  fi

  log "检测到异常（$reason），进入自动修复，重启序号=$restart_count"
  stop_worker
  log "执行自检修复：./loom up"
  cd "$ROOT"
  ./loom up >/dev/null 2>&1 || true
  ./loom doctor >/tmp/loom-autopilot-last-doctor.log 2>&1 || true
  start_worker || return 1
  return 0
}

LAST_RESTART_TS_FILE="$ROOT/devkit/logs/iterate-supervisor.state"
: > "$SUPERVISOR_LOG_FILE"
log "Loom iterate 监督器启动"
log "backlog=$BACKLOG max_rounds=$MAX_ROUNDS reflect=$REFLECT_CARRIER compact=$COMPACT_MODEL sleep=${SLEEP_SECONDS}s check_interval=${CHECK_INTERVAL}s"

# Write our own pid so the external watchdog (loom-watchdog.sh) can find us
# even if it runs before the worker pid file is created.
echo $$ > "$ROOT/devkit/logs/loom-iterate-supervisor.pid"

if [[ "$ONCE" -eq 1 ]]; then
  daemon_cmd
  bash "$WORKER_CMD" "${ITERATE_CMD[@]}"
  exit 0
fi

start_worker

while true; do
  if (( WATCHDOG_RESTART_REQUESTED )); then
    log "watchdog 请求立即重启 worker：$WATCHDOG_RESTART_REASON"
    self_heal "$WATCHDOG_RESTART_REASON" || true
    WATCHDOG_RESTART_REQUESTED=0
    WATCHDOG_RESTART_REASON=""
  elif ! is_worker_running; then
    self_heal "iterate worker 未运行"
  elif has_backlog_stuck; then
    self_heal "backlog 运行任务连续卡住（attempts >=2）"
  elif detect_recent_error; then
    self_heal "iterate 日志出现异常"
  else
    log "定期巡检（${CHECK_INTERVAL}s）：iterate 运行中，PID=$iter_pid"
  fi
  if [[ -f "$TASK_QUEUE_STATUS_SCRIPT" ]]; then
    if python3 "$TASK_QUEUE_STATUS_SCRIPT" --backlog "$BACKLOG" --log-file "$TASK_QUEUE_STATUS_LOG_FILE" --run-root "$ROOT" >/dev/null 2>&1; then
      log "任务队列快照已记录：$TASK_QUEUE_STATUS_LOG_FILE"
    else
      log "任务队列快照失败：脚本执行异常"
    fi
  else
    log "任务队列快照脚本缺失：$TASK_QUEUE_STATUS_SCRIPT"
  fi
  # Sleep in short slices so SIGUSR1 (or SIGTERM) is observed within ~5s
  # instead of waiting for the full CHECK_INTERVAL to elapse.
  elapsed=0
  while (( elapsed < CHECK_INTERVAL )); do
    if (( WATCHDOG_RESTART_REQUESTED )); then
      break
    fi
    sleep 5
    elapsed=$(( elapsed + 5 ))
  done
done
