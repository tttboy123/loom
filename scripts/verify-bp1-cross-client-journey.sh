#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: verify-bp1-cross-client-journey.sh ROOT" >&2; exit 2; }
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

for method in permissions_snapshot permissions_attention permissions_command; do
  count=$(/usr/bin/jq -sr --arg m "$method" '[.[] | select(.method == $m)] | length' "$root/ipc/request-response-summary.jsonl")
  [ "$count" -ge 1 ] || { echo "no IPC traffic for $method" >&2; exit 5; }
done

/usr/bin/grep -q 'Permissions ·' "$root/tui/transcript.txt" || { echo "no Permissions screen in TUI transcript" >&2; exit 5; }
/usr/bin/grep -q 'define_profile\|Profile ' "$root/tui/transcript.txt" || { echo "no profile define in TUI transcript" >&2; exit 5; }
/usr/bin/grep -q 'verdict\|approval required' "$root/tui/transcript.txt" || { echo "no validate_call verdict in TUI transcript" >&2; exit 5; }
/usr/bin/grep -q 'Permission ' "$root/tui/transcript.txt" || { echo "no permission attention item in TUI transcript" >&2; exit 5; }

[ "$(find "$root/gui/screenshots" -type f ! -type l | wc -l | tr -d ' ')" -gt 0 ] || { echo "no screenshots" >&2; exit 5; }
test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }

perm_events=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type LIKE 'Permission%' OR event_type = 'JobPermissionBound' OR stream_id LIKE 'permission-%';")
[ "$perm_events" -ge 3 ] || { echo "missing permission Journal facts ($perm_events)" >&2; exit 5; }
decision_facts=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type = 'PermissionDecisionRecorded';")
[ "$decision_facts" -ge 1 ] || { echo "missing PermissionDecisionRecorded fact" >&2; exit 5; }
approval_requested=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type = 'ApprovalRequested';")
[ "$approval_requested" -ge 1 ] || { echo "missing ApprovalRequested fact" >&2; exit 5; }
approval_decided=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type = 'ApprovalDecided';")
[ "$approval_decided" -ge 1 ] || { echo "missing ApprovalDecided fact" >&2; exit 5; }

test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true || { echo "projection mismatch" >&2; exit 5; }
for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done
[ ! -S "$root/loomd.sock" ] && [ ! -e "$root/loomd.sock.lock" ] || { echo "socket/lock residue" >&2; exit 5; }
if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' \
  "$root/result.md" "$root/gui/actions.jsonl" "$root/tui/keystrokes.jsonl" "$root/timeline.jsonl" \
  "$root/ipc/request-response-summary.jsonl" "$root/daemon/structured-log.jsonl"; then
  echo "secret-like material" >&2; exit 6
fi
echo "B-P1 cross-client journey verified: $jid"
