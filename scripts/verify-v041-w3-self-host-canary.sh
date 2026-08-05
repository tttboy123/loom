#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: verify-v041-w3-self-host-canary.sh ROOT" >&2; exit 2; }
root=$1
case "$root" in /*) ;; *) exit 2 ;; esac
[ -d "$root" ] && [ ! -L "$root" ] || exit 3
[ "$(/usr/bin/stat -f '%Lp' "$root")" = 700 ] || exit 3

for directory in manifest source state isolation ipc daemon gui gui/screenshots tui journal projection artifacts processes assertions; do
  [ -d "$root/$directory" ] && [ ! -L "$root/$directory" ] || { echo "missing dir $directory" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$directory")" = 700 ] || { echo "non-private dir $directory" >&2; exit 4; }
done

for relative in result.md gui/actions.jsonl tui/transcript.txt tui/keystrokes.jsonl timeline.jsonl \
  ipc/request-response-summary.jsonl daemon/structured-log.jsonl projection/summary.json \
  artifacts/digest-verification.json processes/preflight.json processes/postflight.json processes/cleanup-proof.txt \
  state/loom.db; do
  [ -f "$root/$relative" ] && [ ! -L "$root/$relative" ] || { echo "missing evidence $relative" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$relative")" = 600 ] || { echo "non-private evidence $relative" >&2; exit 4; }
done

jid=$(/usr/bin/jq -er '.journey_id' "$root/manifest/journey-harness.json")
case "$jid" in ????????-????-4???-[89ab]???-????????????) ;; *) echo "invalid journey id" >&2; exit 4 ;; esac

[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/daemon/structured-log.jsonl")" = 0 ] || { echo "daemon journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/ipc/request-response-summary.jsonl")" = 0 ] || { echo "ipc journey drift" >&2; exit 5; }
kinds=$(/usr/bin/jq -sr '[.[].client_kind] | unique | join(",")' "$root/ipc/request-response-summary.jsonl")
case "$kinds" in *gui*|*tui*) ;; *) echo "missing dual client traffic: $kinds" >&2; exit 5 ;; esac
app_rows=$(/usr/bin/jq -sr '[.[] | select(.request_id | test("^loom-swift-[0-9a-f]{8}-"))] | length' "$root/ipc/request-response-summary.jsonl")
[ "$app_rows" -ge 2 ] || { echo "production app did not connect (app rows $app_rows)" >&2; exit 5; }
/usr/bin/grep -q 'Loom ·' "$root/tui/transcript.txt" || { echo "no TUI transcript" >&2; exit 5; }
/usr/bin/grep -q 'Integration ·' "$root/tui/transcript.txt" || { echo "no Integration screen" >&2; exit 5; }
screenshots=$(find "$root/gui/screenshots" -type f ! -type l | wc -l | tr -d ' ')
[ "$screenshots" -ge 2 ] || { echo "insufficient screenshots ($screenshots)" >&2; exit 5; }
/usr/bin/grep -q 'restart_reconnect' "$root/gui/actions.jsonl" || { echo "no restart checkpoint" >&2; exit 5; }
test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }
test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true || { echo "projection mismatch" >&2; exit 5; }

# W3 canary specifics.
test "$(/usr/bin/jq -er '.canary_runs' "$root/projection/summary.json")" = 2 || { echo "expected two canary runs" >&2; exit 5; }
test "$(/usr/bin/jq -er '.canary_projection_entries' "$root/projection/summary.json")" = 4 || { echo "expected four canary projection entries" >&2; exit 5; }

db="$root/state/loom.db"
counts=$(/usr/bin/sqlite3 -readonly "$db" \
  "SELECT event_type || '=' || COUNT(*) FROM events WHERE correlation_id='$jid' GROUP BY event_type ORDER BY event_type;")
for want in "CanaryStarted=2" "CanaryCompleted=2"
do
  printf '%s\n' "$counts" | /usr/bin/grep -qx "$want" || { echo "missing journal fact $want" >&2; exit 5; }
done
runs=$(/usr/bin/sqlite3 -readonly "$db" \
  "SELECT json_extract(payload_json,'\$.run_id') FROM events WHERE event_type='CanaryStarted' ORDER BY rowid;")
[ "$(printf '%s\n' "$runs" | /usr/bin/grep -cx 'v041-w3-canary-1')" = 1 ] || { echo "canary run 1 missing" >&2; exit 5; }
[ "$(printf '%s\n' "$runs" | /usr/bin/grep -cx 'v041-w3-canary-2')" = 1 ] || { echo "canary run 2 missing" >&2; exit 5; }
bindings=$(/usr/bin/sqlite3 -readonly "$db" \
  "SELECT json_extract(payload_json,'\$.runtime_instance_id')||'|'||json_extract(payload_json,'\$.model_id')||'|'||json_extract(payload_json,'\$.skill_digest') FROM events WHERE event_type='CanaryCompleted' ORDER BY rowid;")
[ "$(printf '%s\n' "$bindings" | /usr/bin/grep -cx 'runtime.pi.earendil-works.0.82.1|loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m|0795cedc586127a14c03361e864d7a7d694cabdde518acc5afebc3bb1e965f73')" = 2 ] || { echo "canary exact bindings mismatch" >&2; exit 5; }

# Idempotent duplicate rejection: exactly one failed integration_command
# with conflict, and no extra Canary facts beyond the two runs.
conflicts=$(/usr/bin/jq -sr '[.[] | select(.method=="integration_command" and .response_ok==false and .error_code=="conflict")] | length' "$root/ipc/request-response-summary.jsonl")
[ "$conflicts" -ge 1 ] || { echo "missing idempotent duplicate rejection" >&2; exit 5; }

# Projection-failure preserve: the controlled fault was toggled on and off
# through the production client (two projection_failure_test commands, both
# accepted).
fault_ops=$(/usr/bin/jq -sr '[.[] | select(.method=="integration_command" and .response_ok==true)] | length' "$root/ipc/request-response-summary.jsonl")
[ "$fault_ops" -ge 3 ] || { echo "missing projection-fault client operations" >&2; exit 5; }

for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done
[ ! -S "$root/loomd.sock" ] && [ ! -e "$root/loomd.sock.lock" ] || { echo "socket/lock residue" >&2; exit 5; }
if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' \
  "$root/result.md" "$root/gui/actions.jsonl" "$root/tui/keystrokes.jsonl" "$root/timeline.jsonl" \
  "$root/ipc/request-response-summary.jsonl" "$root/daemon/structured-log.jsonl"; then
  echo "secret-like material" >&2; exit 6
fi
echo "v0.4.1-W3 self-host canary verified: $jid"
