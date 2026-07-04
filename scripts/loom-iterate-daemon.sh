#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

USAGE='用法：
  scripts/loom-iterate-daemon.sh [options]

选项:
  --backlog <path>         backlog 文件（默认 devkit/backlog.json）
  --max-rounds <N>         每轮迭代批次执行轮数上限（默认 20）
  --reflect-carrier <name>  反思模型（默认 minimax）
  --compact-model <name>    迭代时压缩模型（默认 deepseek）
  --allow-cache             允许后台自治复用精确响应缓存（默认关闭）
  --sleep <sec>            每轮间隔秒数（默认 60）
  --log-file <path>        日志文件（默认 devkit/logs/iterate-daemon.log）
  --once                    只跑一次 iterate，不进入 while 循环
  --no-compact              关闭上下文压缩（默认开启）
  -h, --help               查看帮助
'

BACKLOG="devkit/backlog.json"
MAX_ROUNDS=20
REFLECT_CARRIER="minimax"
COMPACT_MODEL="deepseek"
SLEEP_SECONDS=60
LOG_FILE="$ROOT/devkit/logs/iterate-daemon.log"
ONCE=0
NO_COMPACT=0
ALLOW_CACHE=0

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
    --allow-cache)
      ALLOW_CACHE=1; shift
      ;;
    --sleep)
      SLEEP_SECONDS="$2"; shift 2
      ;;
    --log-file)
      LOG_FILE="$2"; shift 2
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

mkdir -p "$(dirname "$LOG_FILE")"
if [[ ! -f "$BACKLOG" ]]; then
  echo "未找到 backlog 文件: $BACKLOG"
  exit 1
fi

export PYTHONUNBUFFERED=1

# Use the project's venv Python (Python 3.10+ required by devkit/__main__.py).
# Fall back to system python3 only if .venv is missing — and warn loudly.
PY="$ROOT/.venv/bin/python3"
if [[ ! -x "$PY" ]]; then
  PY="python3"
  echo "[$(date '+%F %T')] WARN: .venv/bin/python3 not found, falling back to system python3 (will fail on Python <3.10)" >&2
fi

run_once() {
  local compact_flag=(--compact-model "$COMPACT_MODEL")
  if [[ "$NO_COMPACT" -eq 1 ]]; then
    compact_flag=(--no-compact)
  fi

  if [[ "$ALLOW_CACHE" -eq 1 ]]; then
    "$PY" -m devkit iterate \
      --backlog "$BACKLOG" \
      --max-rounds "$MAX_ROUNDS" \
      --reflect-carrier "$REFLECT_CARRIER" \
      "${compact_flag[@]}"
  else
    LOOM_NO_CACHE=1 "$PY" -m devkit iterate \
      --backlog "$BACKLOG" \
      --max-rounds "$MAX_ROUNDS" \
      --reflect-carrier "$REFLECT_CARRIER" \
      "${compact_flag[@]}"
  fi
}

echo "[$(date '+%F %T')] 启动 Loom iterate 后台循环"
echo "backlog=$BACKLOG max-rounds=$MAX_ROUNDS reflect=$REFLECT_CARRIER compact=$COMPACT_MODEL cache=$([[ "$ALLOW_CACHE" -eq 1 ]] && echo on || echo off) sleep=${SLEEP_SECONDS}s"

while true; do
  {
    echo "[$(date '+%F %T')] 开始一轮 iterate"
    if run_once; then
      RC=0
    else
      RC=$?
    fi
    echo "[$(date '+%F %T')] 一轮完成 rc=$RC"
  } >>"$LOG_FILE" 2>&1

  if [[ "$ONCE" -eq 1 ]]; then
    exit 0
  fi

  # 若无就绪任务，iterate 会快速返回 0；避免空转，每轮等待后再试
  if [[ "${RC:-0}" -ne 0 ]]; then
    echo "[$(date '+%F %T')] 一轮失败，等待 ${SLEEP_SECONDS}s 后重试"
  else
    echo "[$(date '+%F %T')] 一轮结束，等待 ${SLEEP_SECONDS}s 后继续"
  fi
  sleep "$SLEEP_SECONDS"
done
