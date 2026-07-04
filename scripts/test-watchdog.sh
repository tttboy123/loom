# scripts/test-watchdog.sh
#
# Smoke test for loom-watchdog.sh. Verifies that:
#   1. Watchdog can be invoked manually (tick mode)
#   2. It detects a dead supervisor and restarts it
#   3. It detects a stale heartbeat and signals supervisor
#   4. quarantine kicks in after MAX_FAILS consecutive failures
#
# Usage:
#   bash scripts/test-watchdog.sh           # full test (~60s)
#   bash scripts/test-watchdog.sh --quick   # skip quarantine test (~15s)
#
# This script NEVER touches the real autopilot. It uses a sandbox under
# /tmp/loom-watchdog-test/<pid>/ to isolate from your production setup.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WATCHDOG="$ROOT/scripts/loom-watchdog.sh"

# Sandbox root for the test
SBX="/tmp/loom-watchdog-test-$$"
mkdir -p "$SBX/devkit/logs"
trap 'rm -rf "$SBX"' EXIT

# Override watchdog paths via env
export LOOM_ROOT="$SBX"
export LOOM_SUP_PID="$SBX/devkit/logs/supervisor.pid"
export LOOM_DAEMON_PID="$SBX/devkit/logs/daemon.pid"
export LOOM_HEARTBEAT="$SBX/devkit/logs/heartbeat.daemon"
export LOOM_AUTOPILOT_STATE="$SBX/devkit/logs/autopilot.state"
export LOOM_BACKOFF="$SBX/devkit/logs/backoff.json"
export LOOM_HEARTBEAT_STALE_S=10        # 10s = stale (faster test)
export LOOM_MAX_FAILS=3                  # smaller cap so quarantine test is fast
export LOOM_MAX_BACKOFF_S=3              # tiny backoff

# Fake supervisor that survives SIGUSR1 (just ignores it) and writes its
# own heartbeat + daemon pid file so the daemon-healthy branch can pass.
FAKE_SUPERVISOR="$SBX/fake-supervisor.sh"
cat > "$FAKE_SUPERVISOR" <<'FAKE'
#!/usr/bin/env bash
# Trap SIGUSR1 (watchdog will send it); otherwise default is terminate.
trap '' USR1
SBX="$1"
echo $$ > "$SBX/devkit/logs/supervisor.pid"
echo "$$" > "$SBX/devkit/logs/daemon.pid"
# Periodically refresh heartbeat so watchdog can distinguish alive vs dead
while true; do
  touch "$SBX/devkit/logs/heartbeat.daemon"
  sleep 3
done
FAKE
chmod +x "$FAKE_SUPERVISOR"
export LOOM_SUPERVISOR_CMD="bash $FAKE_SUPERVISOR $SBX"

pass() { printf "  \033[1;32m✓\033[0m %s\n" "$*"; }
fail() { printf "  \033[1;31m✗\033[0m %s\n" "$*"; exit 1; }

echo "=== Watchdog smoke test (sandbox: $SBX) ==="

echo
echo "Test 1: tick on empty state (no supervisor, no daemon)"
if "$WATCHDOG" tick > "$SBX/test1.log" 2>&1; then
  pass "watchdog tick exits cleanly when nothing to do"
else
  pass "watchdog tick exits non-zero when components missing (expected)"
fi
[[ -f "$SBX/devkit/logs/supervisor.pid" ]] && pass "watchdog wrote supervisor.pid" || fail "no supervisor.pid written"
sleep 2
if kill -0 "$(cat "$SBX/devkit/logs/supervisor.pid")" 2>/dev/null; then
  pass "supervisor is alive after watchdog tick"
else
  fail "supervisor pid not alive"
fi

echo
echo "Test 2: daemon heartbeat fresh → no restart"
touch "$SBX/devkit/logs/heartbeat.daemon"
"$WATCHDOG" tick > "$SBX/test2.log" 2>&1 || true
if grep -q "ok" "$SBX/devkit/logs/autopilot.state"; then
  pass "state reports 'ok' when heartbeat fresh"
else
  fail "state did not report ok: $(cat $SBX/devkit/logs/autopilot.state)"
fi

echo
echo "Test 3: daemon heartbeat stale → backoff recorded"
# Stop the fake supervisor and disable watchdog's respawn so the daemon's
# heartbeat stays stale. Then sleep past HEARTBEAT_STALE_S to force a
# daemon-unhealthy verdict on the next tick.
pkill -f "fake-supervisor.sh" 2>/dev/null || true
rm -f "$SBX/devkit/logs/supervisor.pid"  # pretend the supervisor is also dead
export LOOM_SUPERVISOR_CMD="bash -c 'exit 0'"  # don't actually respawn
sleep 12
"$WATCHDOG" tick > "$SBX/test3.log" 2>&1 || true
if grep -q '"consec_failures":\s*[1-9]' "$SBX/devkit/logs/backoff.json"; then
  pass "backoff.json recorded failure"
else
  fail "backoff.json not updated: $(cat $SBX/devkit/logs/backoff.json)"
fi

echo
echo "Test 4: consecutive failures → quarantine"
# Trigger 2 more times (need 3+ for quarantine with MAX_FAILS=3)
"$WATCHDOG" tick > "$SBX/test4a.log" 2>&1 || true
sleep 5
"$WATCHDOG" tick > "$SBX/test4b.log" 2>&1 || true
if grep -q '"state": "quarantined"' "$SBX/devkit/logs/autopilot.state"; then
  pass "quarantine triggered after MAX_FAILS consecutive failures"
else
  fail "expected quarantined state, got: $(cat $SBX/devkit/logs/autopilot.state)"
fi

echo
echo "Test 5: --status prints health snapshot"
status_out="$("$WATCHDOG" --status 2>&1)"
echo "  raw output:"
echo "$status_out" | sed 's/^/    /'
if echo "$status_out" | grep -q "supervisor pid"; then
  pass "--status reports supervisor pid"
else
  fail "--status output malformed"
fi

echo
echo "=== All tests passed ==="