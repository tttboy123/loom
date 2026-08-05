#!/bin/sh
set -eu

[ "$#" -eq 1 ] || { echo "usage: verify-phase3b-canary.sh ROOT" >&2; exit 2; }
root=$1
case "$root" in /*) ;; *) exit 2 ;; esac
[ -d "$root" ] && [ ! -L "$root" ] || exit 3
[ "$(/usr/bin/stat -f '%Lp' "$root")" = 700 ] || exit 3

for directory in manifest state journal processes; do
  [ -d "$root/$directory" ] && [ ! -L "$root/$directory" ] || { echo "missing dir $directory" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$directory")" = 700 ] || { echo "non-private dir $directory" >&2; exit 4; }
done

for relative in result.md manifest/journey-harness.json journal/facts-summary.json \
  processes/preflight.json processes/postflight.json processes/cleanup-proof.txt \
  state/loom.db; do
  [ -f "$root/$relative" ] && [ ! -L "$root/$relative" ] || { echo "missing evidence $relative" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$relative")" = 600 ] || { echo "non-private evidence $relative" >&2; exit 4; }
done

jid=$(/usr/bin/jq -er '.journey_id' "$root/manifest/journey-harness.json")
case "$jid" in ????????-????-4???-[89ab]???-????????????) ;; *) echo "invalid journey id" >&2; exit 4 ;; esac
test "$(/usr/bin/jq -er '.scenario_id' "$root/manifest/journey-harness.json")" = phase3b-sandbox-canary || { echo "wrong scenario" >&2; exit 4; }

/usr/bin/grep -q 'Phase 3B Sandbox Canary' "$root/result.md" || { echo "no canary result header" >&2; exit 5; }
test "$(/usr/bin/jq -er '.all_ok' "$root/journal/facts-summary.json")" = true || { echo "canary checks not all ok" >&2; exit 5; }
for check in isolated-exec no-secret-leak generation-fencing cancel-terminates reconcile-cleanup journal-authority; do
  /usr/bin/grep -q "\[PASS\] $check" "$root/result.md" || { echo "missing PASS marker $check" >&2; exit 5; }
done

test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }

for event_type in SandboxInstanceCreated SandboxExecRequested SandboxExecCompleted SandboxCancelled SandboxDestroyed; do
  count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" "SELECT COUNT(*) FROM events WHERE event_type='$event_type';")
  [ "$count" -ge 1 ] || { echo "missing $event_type fact" >&2; exit 5; }
done

# The loopback backend must not leave any instance workspace behind.
[ "$(find "$root/state/sandbox-work" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ')" = 0 ] || { echo "sandbox workspace residue" >&2; exit 5; }
for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done

if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|canary-secret-value' \
  "$root/result.md" "$root/journal/facts-summary.json" "$root/manifest/journey-harness.json"; then
  echo "secret-like material" >&2; exit 6
fi
echo "Phase 3B sandbox canary verified: $jid"
