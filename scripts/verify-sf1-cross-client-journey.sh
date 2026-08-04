#!/bin/sh
set -eu

# Verify an SF-W1 cross-client journey root (queue admission / conflict /
# gap convergence). Mirrors the frozen §8 evidence contract.

[ "$#" -eq 1 ] || { echo "usage: verify-sf1-cross-client-journey.sh ROOT" >&2; exit 2; }
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
  state/loom.db manifest.json journal/event-summary.json journal/stream-heads.json journal/sqlite-summary.json \
  source/source-lock.json source/runtime-fixture.json manifest/journey-context.json \
  manifest/preflight-source-lock.json manifest/tui-plan.json; do
  [ -f "$root/$relative" ] && [ ! -L "$root/$relative" ] || { echo "missing evidence $relative" >&2; exit 4; }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$relative")" = 600 ] || { echo "non-private evidence $relative" >&2; exit 4; }
done

jid=$(/usr/bin/jq -er '.journey_id' "$root/manifest/journey-harness.json")
case "$jid" in ????????-????-4???-[89ab]???-????????????) ;; *) echo "invalid journey id" >&2; exit 4 ;; esac

[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/daemon/structured-log.jsonl")" = 0 ] || { echo "daemon journey drift" >&2; exit 5; }
[ "$(/usr/bin/jq -sr --arg j "$jid" '[.[] | select(.journey_id != $j)] | length' "$root/ipc/request-response-summary.jsonl")" = 0 ] || { echo "ipc journey drift" >&2; exit 5; }
gui_kinds=$(/usr/bin/jq -sr '[.[].client_kind] | unique | join(",")' "$root/ipc/request-response-summary.jsonl")
case "$gui_kinds" in *gui*|*tui*) ;; *) echo "missing dual client traffic: $gui_kinds" >&2; exit 5 ;; esac
app_rows=$(/usr/bin/jq -sr '[.[] | select(.request_id | test("^loom-swift-[0-9a-f]{8}-"))] | length' "$root/ipc/request-response-summary.jsonl")
[ "$app_rows" -ge 2 ] || { echo "production app did not connect (app rows $app_rows)" >&2; exit 5; }
/usr/bin/grep -q 'Loom ·' "$root/tui/transcript.txt" || { echo "no TUI transcript" >&2; exit 5; }
[ "$(find "$root/gui/screenshots" -type f ! -type l | wc -l | tr -d ' ')" -gt 0 ] || { echo "no screenshots" >&2; exit 5; }
test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok || { echo "sqlite integrity" >&2; exit 5; }
dup=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT (SELECT COUNT(*)-COUNT(DISTINCT id) FROM events) + (SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events);')
[ "$dup" = 0 ] || { echo "duplicate event/idempotency identity" >&2; exit 5; }
gaps=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) n,MAX(seq) h FROM events GROUP BY stream_id HAVING n != h);')
[ "$gaps" = 0 ] || { echo "stream gaps" >&2; exit 5; }
test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true || { echo "projection mismatch" >&2; exit 5; }
manifest_digest=$(/usr/bin/jq -er '.manifest_digest' "$root/manifest.json")
computed_manifest_digest=$(/usr/bin/jq -cS 'del(.manifest_digest)' "$root/manifest.json" | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}')
[ "$manifest_digest" = "$computed_manifest_digest" ] || { echo "manifest digest mismatch" >&2; exit 5; }
event_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM events;')
[ "$(/usr/bin/jq -er '.event_count' "$root/journal/event-summary.json")" = "$event_count" ] || { echo "event-summary count mismatch" >&2; exit 5; }
[ "$(/usr/bin/jq -er '.event_count' "$root/journal/sqlite-summary.json")" = "$event_count" ] || { echo "sqlite-summary count mismatch" >&2; exit 5; }
[ "$(/usr/bin/jq -er '.integrity_check' "$root/journal/sqlite-summary.json")" = ok ] || { echo "sqlite-summary integrity" >&2; exit 5; }
for key in schema_version journey_id; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k]' "$root/journal/event-summary.json")" = "$(/usr/bin/jq -er --arg k "$key" '.[$k]' "$root/journal/sqlite-summary.json")" || { echo "journal summary identity drift" >&2; exit 5; }
done
test "$(/usr/bin/jq -er '.journey_id' "$root/manifest/journey-context.json")" = "$jid" || { echo "journey-context drift" >&2; exit 5; }
test "$(/usr/bin/jq -er '.network_allowed' "$root/manifest/journey-context.json")" = false || { echo "network_allowed" >&2; exit 5; }
test "$(/usr/bin/jq -er '.provider_credentials_present' "$root/manifest/journey-context.json")" = false || { echo "provider credentials" >&2; exit 5; }
for kind in daemon gui tui; do
  path=bin/loomd
  [ "$kind" = gui ] && path=app/Loom.app/Contents/MacOS/LoomLocalApp
  [ "$kind" = tui ] && path=bin/loom
  expected=$(/usr/bin/jq -er --arg k "$kind" '.production_components[] | select(.kind == $k) | .sha256' "$root/manifest.json")
  actual=$(/usr/bin/shasum -a 256 "$root/$path" | /usr/bin/awk '{print $1}')
  [ "$expected" = "$actual" ] || { echo "binary digest mismatch $kind" >&2; exit 5; }
  [ "$expected" = "$(/usr/bin/jq -er --arg k "$kind" '.[$k+"_binary_digest"]' "$root/manifest/journey-context.json")" ] || { echo "journey-context binary mismatch $kind" >&2; exit 5; }
done
expected_view_version=$(/usr/bin/jq -r '.streams[] | [.stream_id, (.sequence|tostring), .event_id] | @tsv' "$root/journal/stream-heads.json" | LC_ALL=C sort | while IFS="$(printf '\t')" read -r sid seq eid; do printf '%s\0%s\0%s\n' "$sid" "$seq" "$eid"; done | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}')
[ "$(/usr/bin/jq -er '.view_version' "$root/projection/summary.json")" = "$expected_view_version" ] || { echo "view_version mismatch" >&2; exit 5; }
/usr/bin/jq -r '.evidence_files[] | .relative_path' "$root/manifest.json" | while IFS= read -r relative; do
  path="$root/$relative"
  [ -f "$path" ] && [ ! -L "$path" ] || { echo "manifest evidence missing $relative" >&2; exit 5; }
  actual=$(/usr/bin/shasum -a 256 "$path" | /usr/bin/awk '{print $1}')
  expected=$(/usr/bin/jq -er --arg p "$relative" '.evidence_files[] | select(.relative_path == $p) | .sha256' "$root/manifest.json")
  [ "$actual" = "$expected" ] || { echo "manifest evidence digest mismatch $relative" >&2; exit 5; }
  actual_mode=$(/usr/bin/stat -f '%Lp' "$path")
  expected_mode=$(/usr/bin/jq -er --arg p "$relative" '.evidence_files[] | select(.relative_path == $p) | .mode' "$root/manifest.json")
  [ "$actual_mode" = "$expected_mode" ] || { echo "manifest evidence mode mismatch $relative" >&2; exit 5; }
done
for key in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg k "$key" '.[$k] | length' "$root/processes/postflight.json")" = 0 || { echo "postflight $key" >&2; exit 5; }
done
[ ! -S "$root/loomd.sock" ] && [ ! -e "$root/loomd.sock.lock" ] || { echo "socket/lock residue" >&2; exit 5; }
if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' \
  "$root/result.md" "$root/gui/actions.jsonl" "$root/tui/keystrokes.jsonl" "$root/timeline.jsonl" \
  "$root/ipc/request-response-summary.jsonl" "$root/daemon/structured-log.jsonl"; then
  echo "secret-like material" >&2; exit 6
fi
echo "SF-W1 cross-client journey verified: $jid"
