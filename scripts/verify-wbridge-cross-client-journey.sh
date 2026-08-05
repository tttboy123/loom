#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: verify-wbridge-cross-client-journey.sh ROOT" >&2; exit 2; }
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
test "$(/usr/bin/jq -er '.scenario_id' "$root/manifest/journey-harness.json")" = wbridge-cross-runtime-journey || { echo "wrong scenario" >&2; exit 4; }

[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/ipc/request-response-summary.jsonl")" = 0 ] || { echo "ipc journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/daemon/structured-log.jsonl")" = 0 ] || { echo "daemon journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/timeline.jsonl")" = 0 ] || { echo "timeline journey drift" >&2; exit 5; }

bridge_rows=$(/usr/bin/jq -sr --arg k bridge '[.[] | select(.client_kind == $k)] | length' "$root/ipc/request-response-summary.jsonl")
[ "$bridge_rows" -ge 4 ] || { echo "not enough bridge runtime traffic (rows $bridge_rows)" >&2; exit 5; }
gui_rows=$(/usr/bin/jq -sr --arg k gui '[.[] | select(.client_kind == $k)] | length' "$root/gui/actions.jsonl")
[ "$gui_rows" -ge 1 ] || { echo "no gui observe traffic" >&2; exit 5; }

test "$(/usr/bin/jq -er '.all_pass' "$root/assertions/summary.json")" = true || { echo "leg assertions failed" >&2; exit 5; }
test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true || { echo "projection mismatch" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] leg=allow verdict=allow executed=true' "$root/result.md" || { echo "allow marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] leg=ask verdict=allow executed=true' "$root/result.md" || { echo "ask/resume marker missing" >&2; exit 5; }
/usr/bin/grep -q '\[PASS\] leg=deny verdict=deny executed=false' "$root/result.md" || { echo "deny marker missing" >&2; exit 5; }

test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }

for event_type in ToolExecutionProposed ToolExecutionAllowed ToolExecutionCompleted ToolExecutionDenied ApprovalRequested ApprovalDecided; do
  count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='$event_type';")
  [ "$count" -ge 1 ] || { echo "missing $event_type fact" >&2; exit 5; }
done
allowed_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='ToolExecutionAllowed';")
completed_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='ToolExecutionCompleted';")
denied_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='ToolExecutionDenied';")
[ "$allowed_count" = 2 ] || { echo "expected 2 allowed (allow + ask resume), got $allowed_count" >&2; exit 5; }
[ "$completed_count" = 2 ] || { echo "expected 2 completed, got $completed_count" >&2; exit 5; }
[ "$denied_count" = 1 ] || { echo "expected 1 denied, got $denied_count" >&2; exit 5; }

/usr/bin/grep -q 'verdict=allow' "$root/tui/transcript.txt" || { echo "audit allow entry missing" >&2; exit 5; }
/usr/bin/grep -q 'verdict=ask' "$root/tui/transcript.txt" || { echo "audit ask entry missing" >&2; exit 5; }
/usr/bin/grep -q 'verdict=deny' "$root/tui/transcript.txt" || { echo "audit deny entry missing" >&2; exit 5; }
/usr/bin/grep -q 'denial_reason="denied by permission' "$root/tui/transcript.txt" || { echo "deny reason missing in audit" >&2; exit 5; }

execution_evidence=$(find "$root/state/execution-evidence" -type f ! -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$execution_evidence" -ge 1 ] || { echo "no execution evidence artifacts" >&2; exit 5; }
test "$(/usr/bin/jq -er '.verified' "$root/artifacts/digest-verification.json")" = true || { echo "artifact verification failed" >&2; exit 5; }

for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done
[ ! -S "$root/loomd.sock" ] 2>/dev/null || { echo "socket residue" >&2; exit 5; }

if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|loom_grant_v1' \
  "$root/result.md" "$root/assertions/summary.json" "$root/journal/facts-summary.json" \
  "$root/tui/transcript.txt" "$root/ipc/request-response-summary.jsonl" \
  "$root/daemon/structured-log.jsonl" "$root/timeline.jsonl"; then
  echo "secret-like material" >&2; exit 6
fi
echo "W-BRIDGE cross-runtime journey verified: $jid"
