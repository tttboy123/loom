#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: verify-wautonomy-cross-client-journey.sh ROOT" >&2; exit 2; }
root=$1
case "$root" in /*) ;; *) exit 2 ;; esac
[ -d "$root" ] && [ ! -L "$root" ] || exit 3
[ "$(/usr/bin/stat -f '%Lp' "$root")" = 700 ] || exit 3

for directory in manifest state source isolation ipc daemon gui gui/screenshots tui journal projection artifacts processes assertions bin app; do
  [ -d "$root/$directory" ] && [ ! -L "$root/$directory" ] || { echo "missing dir $directory" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$directory")" = 700 ] || { echo "non-private dir $directory" >&2; exit 4; }
done

for relative in result.md manifest/journey-harness.json journal/facts-summary.json \
  assertions/summary.json projection/summary.json artifacts/digest-verification.json \
  ipc/request-response-summary.jsonl daemon/structured-log.jsonl timeline.jsonl \
  gui/actions.jsonl tui/transcript.txt tui/keystrokes.jsonl \
  processes/preflight.json processes/postflight.json processes/cleanup-proof.txt \
  state/loom.db; do
  [ -f "$root/$relative" ] && [ ! -L "$root/$relative" ] || { echo "missing evidence $relative" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$relative")" = 600 ] || { echo "non-private evidence $relative" >&2; exit 4; }
done

jid=$(/usr/bin/jq -er '.journey_id' "$root/manifest/journey-harness.json")
case "$jid" in ????????-????-4???-[89ab]???-????????????) ;; *) echo "invalid journey id" >&2; exit 4 ;; esac
test "$(/usr/bin/jq -er '.scenario_id' "$root/manifest/journey-harness.json")" = wautonomy-cross-client-journey || { echo "wrong scenario" >&2; exit 4; }

[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/ipc/request-response-summary.jsonl")" = 0 ] || { echo "ipc journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/daemon/structured-log.jsonl")" = 0 ] || { echo "daemon journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/timeline.jsonl")" = 0 ] || { echo "timeline journey drift" >&2; exit 5; }

autonomy_rows=$(/usr/bin/jq -sr --arg k autonomy '[.[] | select(.client_kind == $k)] | length' "$root/ipc/request-response-summary.jsonl")
[ "$autonomy_rows" -ge 6 ] || { echo "not enough autonomy client traffic (rows $autonomy_rows)" >&2; exit 5; }
gui_rows=$(/usr/bin/jq -sr --arg k gui '[.[] | select(.client_kind == $k)] | length' "$root/gui/actions.jsonl")
[ "$gui_rows" -ge 1 ] || { echo "no gui observe traffic" >&2; exit 5; }

test "$(/usr/bin/jq -er '.all_pass' "$root/assertions/summary.json")" = true || { echo "step assertions failed" >&2; exit 5; }
test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true || { echo "projection mismatch" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] define' "$root/result.md" || { echo "define marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] activate' "$root/result.md" || { echo "activate marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] dispatch_1' "$root/result.md" || { echo "dispatch_1 marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] dispatch_2' "$root/result.md" || { echo "dispatch_2 marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] dispatch_3.*budget' "$root/result.md" || { echo "budget stop marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] revoke' "$root/result.md" || { echo "revoke marker missing" >&2; exit 5; }

test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }

for event_type in StandingOrderDefined StandingOrderActivated StandingOrderDispatch StandingOrderRevoked BudgetConsumed ToolExecutionProposed ToolExecutionAllowed ToolExecutionCompleted; do
  count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='$event_type';")
  [ "$count" -ge 1 ] || { echo "missing $event_type fact" >&2; exit 5; }
done
dispatch_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='StandingOrderDispatch';")
budget_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='BudgetConsumed';")
completed_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='ToolExecutionCompleted';")
[ "$dispatch_count" = 2 ] || { echo "expected 2 dispatches, got $dispatch_count" >&2; exit 5; }
[ "$budget_count" = 2 ] || { echo "expected 2 budget consumptions, got $budget_count" >&2; exit 5; }
[ "$completed_count" = 2 ] || { echo "expected 2 real executions, got $completed_count" >&2; exit 5; }

execution_evidence=$(find "$root/state/execution-evidence" -type f ! -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$execution_evidence" -ge 1 ] || { echo "no execution evidence artifacts" >&2; exit 5; }
test "$(/usr/bin/jq -er '.verified' "$root/artifacts/digest-verification.json")" = true || { echo "artifact verification failed" >&2; exit 5; }

for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done

if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' \
  "$root/result.md" "$root/assertions/summary.json" "$root/journal/facts-summary.json" \
  "$root/ipc/request-response-summary.jsonl" "$root/daemon/structured-log.jsonl"; then
  echo "secret-like material" >&2; exit 6
fi
echo "W-AUTONOMY cross-client journey verified: $jid"
