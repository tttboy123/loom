# Loom Autopilot 自愈方案（k8s-inspired）

> **作者**：Mavis (mavis Agent Team)
> **时间**：2026-07-05
> **背景**：现场发现 autopilot 守护链（daemon + supervisor）死了 31 小时没人拉起；178 个 task 因为 backlog 路径过期无限重试失败。
> **目标**：让 Loom autopilot 在本地单机上达到"无人值守 24h+" 的可靠性。

---

## TL;DR

把 k8s 自愈的核心模式（**Reconciliation Loop + CrashLoopBackOff + 多级监督 + 健康探针 + 熔断器**）按 Loom 的单机本地约束**简化**成 4 层结构。**第一层（watchdog）是缺失的兜底，必须做**——它是当前可靠性缺口的最大单一原因。

---

## 1. 问题回顾（现场证据）

| 现象 | 根因 |
|---|---|
| Daemon 死 24h+ | supervisor 31h 前死了，没人拉 |
| 178 个 failed task | backlog 路径过期，validator 拒绝 |
| Supervisor 死了没自愈 | **没有外部 watcher**（无 launchd / 无 systemd / 无 cron） |
| 同一 task 反复失败 12+ 小时 | `--max-rounds 20` 只数成功轮；失败无上限 |
| 失败归因不清晰 | daemon 只输出"一轮失败，等待 90s 后重试" |

---

## 2. 从 k8s 借鉴哪些模式

k8s 自愈体系的核心：**期望状态 vs 实际状态 → 不停 reconcile**。具体可拆 6 个 pattern：

| k8s 模式 | k8s 做什么 | Loom 对应什么 |
|---|---|---|
| **Reconciliation Loop** | Controller 不断 observe → diff → act，直到 desired == actual | Daemon 跑任务循环：pick ready task → run → 标记状态 |
| **CrashLoopBackOff** | 容器反复 crash 时指数退避：10s→30s→90s→...→5min | Daemon 失败时同样应该退避，避免雪崩 |
| **Liveness Probe** | kubelet HTTP GET /healthz，失败就重启 Pod | **缺失** —— 需要 heartbeat 文件 |
| **Readiness Probe** | Pod 启动慢时不立刻 liveness-kill | Daemon 启动期间不被 watchdog 误判 |
| **Multi-level supervision** | systemd → kubelet → container | **缺失** —— 当前只有 supervisor → daemon 两层，少了最外层 |
| **Circuit breaker (de facto)** | BackoffLimit / restart count 上限 | **缺失** —— 同一 task 失败 N 次后应跳过 |

---

## 3. 4 层结构设计

```
┌──────────────────────────────────────────────────────────┐
│ Layer 0  外部 watcher（launchd / systemd / cron）           │   ← 用户配置一次
│          唯一职责：supervisor 死了拉起 supervisor            │
│          不依赖 Loom 代码，纯 shell                       │
├──────────────────────────────────────────────────────────┤
│ Layer 1  Loom Watchdog (loom-watchdog.sh)                │   ← 新增
│          - heartbeat 监控（supervisor + daemon）            │
│          - 失败计数 + 指数退避（CrashLoopBackOff）            │
│          - 孤儿 PID 清理（daemon 死了但 PID 文件还在）        │
│          - 通知：N 次失败后写 STATE 文件 + 触发人类门         │
├──────────────────────────────────────────────────────────┤
│ Layer 2  Domain Supervisor (loom-iterate-supervisor.sh)  │   ← 已有，需改造
│          - 5 分钟巡检 daemon                                   │
│          - 转发 Watchdog 的退避信号                           │
│          - 现在的 300s 巡检保留                                │
├──────────────────────────────────────────────────────────┤
│ Layer 3  Worker Daemon (loom-iterate-daemon.sh)           │   ← 已有，需改造
│          - 任务循环（已有）                                    │
│          - 熔断器：同 task_id 失败 N 次后跳过                    │
│          - 写 heartbeat + STATE                              │
│          - 失败时给 Watchdog 写退避信号                       │
└──────────────────────────────────────────────────────────┘
```

### 3.1 借鉴映射表（具体落地）

| k8s 概念 | Loom 实现 |
|---|---|
| Pod | Daemon 进程 |
| Liveness probe | `devkit/logs/heartbeat.daemon`（每 30s 写一次 mtime） |
| Readiness probe | Daemon 启动后写一个 `READY` 标志位 |
| CrashLoopBackOff | `devkit/logs/backoff.json`（`{"consec_failures": N, "next_restart_at": ts}`） |
| restartPolicy: Always | Watchdog 永远重启 |
| restartPolicy: OnFailure | 不在这里——Watchdog 一律拉起 |
| BackoffLimit | `MAX_CONSEC_FAILS=5`，超过写 `STATE=quarantined` 等待人类门 |
| PodDisruptionBudget | 不适用（单机） |
| Controller (reconcile) | Backlog janitor：定期扫描 backlog，识别 stale task |
| TerminationGracePeriod | Daemon 收到 SIGTERM 后清理 PID + 心跳 + STATE |

---

## 4. 关键模块设计

### 4.1 Watchdog（新增，~80 行 bash）

**职责**：
1. 检查 supervisor 是否活（PID 文件 + 进程存在）
2. 检查 daemon 是否活（PID 文件 + 进程存在 + heartbeat 新鲜）
3. 失败计数 + 指数退避
4. 清理孤儿 PID
5. N 次失败后写 STATE 让人介入

**核心伪代码**：

```bash
#!/usr/bin/env bash
# scripts/loom-watchdog.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG="$ROOT/devkit/logs"
SUP_PID_FILE="$LOG/supervisor.pid"     # supervisor 写
DAEMON_PID_FILE="$LOG/daemon.pid"      # daemon 写
HEARTBEAT="$LOG/heartbeat.daemon"
STATE="$LOG/autopilot.state"
BACKOFF="$LOG/backoff.json"

MAX_FAILS=5
HEARTBEAT_STALE_S=120      # heartbeat >120s 没更新 = 死了

# 退避：1s → 2s → 4s → 8s → ... → cap 300s
backoff_seconds() {
  local n="$1"
  local cap=300
  local s=$(( 2 ** (n - 1) ))
  [ $s -gt $cap ] && s=$cap
  echo $s
}

# 检查 supervisor
if ! pgrep -F "$SUP_PID_FILE" >/dev/null 2>&1; then
  bash "$ROOT/scripts/loom-iterate-supervisor.sh" \
       --backlog "$ROOT/devkit/backlog.json" \
       --max-rounds 20 \
       --reflect-carrier minimax \
       --compact-model deepseek \
       --sleep 90 \
       >>"$LOG/watchdog.log" 2>&1 &
  echo $! > "$SUP_PID_FILE"
  echo "$(date): supervisor 重启 (was dead)" >> "$LOG/watchdog.log"
fi

# 检查 daemon + heartbeat
NEED_RESTART=false
if ! pgrep -F "$DAEMON_PID_FILE" >/dev/null 2>&1; then
  NEED_RESTART=true
  REASON="daemon dead (PID stale)"
elif [ ! -f "$HEARTBEAT" ] || [ $(( $(date +%s) - $(stat -f %m "$HEARTBEAT") )) -gt $HEARTBEAT_STALE_S ]; then
  NEED_RESTART=true
  REASON="daemon heartbeat stale (>${HEARTBEAT_STALE_S}s)"
fi

if $NEED_RESTART; then
  CONSEC=$(jq -r '.consec_failures // 0' "$BACKOFF" 2>/dev/null || echo 0)
  CONSEC=$((CONSEC + 1))
  NEXT_RESTART_AT=$(date -v +${BACKOFF_S}s +%s 2>/dev/null \
                    || date -d "+${BACKOFF_S} seconds" +%s)
  jq -n --argjson c $CONSEC --arg r "$REASON" --arg ts "$(date -Iseconds)" \
        '{consec_failures:$c, last_reason:$r, last_restart_at:$ts}' > "$BACKOFF"

  if [ $CONSEC -ge $MAX_FAILS ]; then
    jq -n --arg r "$REASON" --arg ts "$(date -Iseconds)" \
       '{state:"quarantined", reason:$r, since:$ts, action:"需要人类介入"}' > "$STATE"
    exit 0  # 不再自动重启，让人类看
  fi

  # 强制杀 daemon（孤儿清理）
  if [ -f "$DAEMON_PID_FILE" ]; then
    kill -9 "$(cat "$DAEMON_PID_FILE")" 2>/dev/null || true
    /Users/lune/.mavis/bin/mavis-trash -- "$DAEMON_PID_FILE"
  fi

  # 退避等待
  DELAY=$(backoff_seconds $CONSEC)
  echo "$(date): 退避 ${DELAY}s 后重启 daemon ($REASON)" >> "$LOG/watchdog.log"
  sleep "$DELAY"

  # 通知 supervisor（SIGUSR1 让它重启 worker）
  if [ -f "$SUP_PID_FILE" ]; then
    kill -USR1 "$(cat "$SUP_PID_FILE")" 2>/dev/null || true
  fi
fi
```

### 4.2 Daemon 自保护改造

**新增写**（每 30s）：
- `devkit/logs/heartbeat.daemon`（mtime 即心跳）
- `devkit/logs/backoff.json`（累计失败 / 退避状态）

**新增逻辑**：
```bash
# 启动后立即写一次心跳
touch "$LOG/heartbeat.daemon"

# 任务循环里加熔断器
declare -A TASK_FAIL_COUNT
MAX_TASK_FAILS=3   # 同一 task_id 失败 3 次后 circuit-break

while true; do
  TASK=$(pick_next_task "$BACKLOG")
  if [ -z "$TASK" ]; then sleep 90; continue; fi

  TASK_ID=$(echo "$TASK" | jq -r '.id')
  COUNT=${TASK_FAIL_COUNT[$TASK_ID]:-0}

  if [ $COUNT -ge $MAX_TASK_FAILS ]; then
    echo "$(date): circuit-break $TASK_ID (连续失败 $COUNT 次)"
    mark_task_circuit_broken "$TASK_ID" "$BACKLOG"
    continue
  fi

  if ! run_task "$TASK"; then
    TASK_FAIL_COUNT[$TASK_ID]=$((COUNT + 1))
    echo "$(date): task $TASK_ID 失败 (第 $((COUNT+1)) 次)"
  else
    TASK_FAIL_COUNT[$TASK_ID]=0
    jq -n '{consec_failures:0, last_reason:"ok"}' > "$LOG/backoff.json"
  fi

  touch "$LOG/heartbeat.daemon"   # 每次循环都写心跳
  sleep 90
done
```

### 4.3 Backlog Janitor（新增，~40 行 python）

**职责**：定期清理过期路径的 task + 自动归档过老的 failed。

```python
# devkit/backlog_janitor.py
"""Reconciliation loop for backlog.json — k8s controller-style."""
import json, re
from pathlib import Path
from datetime import datetime, timedelta

ROOT = Path(__file__).resolve().parent.parent
BACKLOG = ROOT / "devkit" / "backlog.json"
AUDIT_DIR = ROOT / "devkit" / "backlog_audits"

def stale_path_target(task: dict) -> bool:
    """Detect tasks whose target_path no longer exists."""
    path = task.get("target_path") or task.get("path")
    if not path:
        return False
    abs_path = ROOT / path
    return not abs_path.exists()

def too_old_failed(task: dict, hours=24) -> bool:
    if task.get("status") != "failed":
        return False
    last_fail = task.get("last_failure_at") or task.get("updated_at")
    if not last_fail:
        return False
    try:
        t = datetime.fromisoformat(last_fail)
    except ValueError:
        return False
    return datetime.now() - t > timedelta(hours=hours)

def reconcile(dry_run: bool = False) -> dict:
    data = json.loads(BACKLOG.read_text())
    tasks = data if isinstance(data, list) else data.get("tasks", [])
    
    stale_paths = [t for t in tasks if stale_path_target(t)]
    old_failed = [t for t in tasks if too_old_failed(t)]
    
    report = {
        "stale_paths": len(stale_paths),
        "old_failed": len(old_failed),
        "examples": [t.get("id") for t in stale_paths[:3]],
    }
    
    if not dry_run and (stale_paths or old_failed):
        # 归档
        AUDIT_DIR.mkdir(exist_ok=True)
        archive = {
            "at": datetime.now().isoformat(),
            "stale_paths": stale_paths,
            "old_failed": old_failed,
        }
        (AUDIT_DIR / f"janitor-{datetime.now().strftime('%Y%m%d-%H%M%S')}.json").write_text(
            json.dumps(archive, indent=2, ensure_ascii=False)
        )
        # 过滤
        bad_ids = {t["id"] for t in stale_paths + old_failed}
        clean = [t for t in tasks if t.get("id") not in bad_ids]
        out = clean if isinstance(data, list) else {"tasks": clean}
        BACKLOG.write_text(json.dumps(out, indent=2, ensure_ascii=False) + "\n")
    
    return report

if __name__ == "__main__":
    import sys
    print(reconcile(dry_run="--dry-run" in sys.argv))
```

**调用**：`scripts/loom-iterate-supervisor.sh` 每 30 分钟跑一次 janitor。

### 4.4 State 文件契约

**`devkit/logs/autopilot.state`** — 顶层状态：
```json
{
  "state": "running" | "degraded" | "quarantined",
  "since": "2026-07-05T01:00:40+08:00",
  "supervisor_pid": 41164,
  "daemon_pid": 41181,
  "last_heartbeat": "2026-07-05T01:05:12+08:00",
  "consec_failures": 0
}
```

`./loom doctor` 读这个文件给人类门提示。

---

## 5. 部署（Layer 0）

macOS 用 launchd：

```xml
<!-- ~/Library/LaunchAgents/com.loom.watchdog.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.loom.watchdog</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/scripts/loom-watchdog.sh</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StartInterval</key><integer>60</integer>  <!-- 每 60s 跑一次 -->
  <key>StandardOutPath</key><string>/tmp/loom-watchdog.log</string>
  <key>StandardErrorPath</key><string>/tmp/loom-watchdog.err</string>
</dict>
</plist>
```

Linux 用 systemd：

```ini
# ~/.config/systemd/user/loom-watchdog.service
[Unit]
Description=Loom autopilot watchdog
After=network.target

[Service]
Type=oneshot
ExecStart=/path/to/loom-watchdog.sh
```

```ini
# ~/.config/systemd/user/loom-watchdog.timer
[Unit]
Description=Run Loom watchdog every 60s

[Timer]
OnBootSec=30
OnUnitActiveSec=60

[Install]
WantedBy=timers.target
```

---

## 6. 关键指标（怎么知道自愈在工作）

| 指标 | 健康 | 异常 |
|---|---|---|
| `consec_failures` | 0 | ≥3 看趋势，≥5 触发 quarantined |
| `heartbeat.daemon` mtime | < 90s 前 | > 120s → daemon 假死 |
| `autopilot.state.state` | `running` | `degraded` / `quarantined` |
| Backlog failed 数 / day | ≤ 5 | ≥ 20 → 系统性问题 |
| 任务平均时长 | < 2min | > 10min → 模型慢/死锁 |

加进 `./loom doctor` 的输出：

```bash
echo "自愈状态:"
[ -f devkit/logs/autopilot.state ] && cat devkit/logs/autopilot.state | jq .
echo "心跳:"
[ -f devkit/logs/heartbeat.daemon ] && echo "  daemon: $(($(date +%s) - $(stat -f %m devkit/logs/heartbeat.daemon)))s ago" || echo "  daemon: 无心跳"
```

---

## 7. 实施 PR 拆分

```
PR-A: 基础设施（约 200 行）
  - scripts/loom-watchdog.sh
  - scripts/install-watchdog.sh（生成 plist / systemd unit）
  - launchd plist + systemd unit 示例
  - 测试：人工 kill -9 supervisor，watchdog 拉起

PR-B: Daemon 自保护（约 100 行）
  - heartbeat 写入
  - backoff.json 状态机
  - 同 task_id 熔断器
  - SIGTERM 优雅退出（清 PID + STATE）

PR-C: Backlog Janitor（约 80 行）
  - devkit/backlog_janitor.py
  - 接入 supervisor 周期任务
  - 测试：故意制造 stale_path，验证被清理

PR-D: ./loom doctor 自愈面板（约 50 行）
  - 读取 STATE / heartbeat
  - 提示 quarantine
  - 一键恢复（清 STATE + 触发 watchdog）
```

总工作量约 1.5 天。可以一次性 PR 也可以分。

---

## 8. 借鉴但**不**做的事

| k8s 概念 | 不做的原因 |
|---|---|
| Leader election | 单机，没必要 |
| Multi-region failover | 单机 |
| Init container | Loom 没有容器化 daemon |
| Service mesh | 单进程足够 |
| HPA（horizontal scaling） | 单机 |
| Admission webhook | 没有多租户 |

---

## 9. 与现有组件的兼容性

| 现有 | 影响 |
|---|---|
| `loom-iterate-supervisor.sh` | 加 SIGUSR1 监听（watchdog 通知重启 worker） |
| `loom-iterate-daemon.sh` | 加 heartbeat / 熔断器 / 优雅退出 |
| `evaluate_final_gate` | 不变 |
| 现有 backlog.json schema | 不变（janitor 只删不改 schema） |
| `loom doctor` | 加自愈状态面板 |
| `loom autopilot` | 不变（用户入口不变） |

---

## 10. 一句话总结

> **没有外部 watcher 的 autopilot 永远不可靠**。加 80 行 watchdog + launchd/systemd 单文件 + 状态文件契约，能把"24h 无人值守"从不可能变成默认。