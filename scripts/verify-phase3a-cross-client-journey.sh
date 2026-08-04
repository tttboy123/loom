#!/bin/sh
set -eu

[ "$#" -eq 1 ] || {
  echo "usage: verify-phase3a-cross-client-journey.sh ROOT" >&2
  exit 2
}
root=$1
case "$root" in /*) ;; *) exit 2 ;; esac
[ -d "$root" ] && [ ! -L "$root" ] || exit 3
[ "$(/usr/bin/stat -f '%Lp' "$root")" = 700 ] || exit 3

for directory in manifest source state isolation ipc daemon gui gui/screenshots tui journal projection artifacts processes assertions; do
  [ -d "$root/$directory" ] && [ ! -L "$root/$directory" ] || {
    echo "missing private evidence directory: $directory" >&2
    exit 4
  }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$directory")" = 700 ] || {
    echo "non-private evidence directory: $directory" >&2
    exit 4
  }
done

required_files='manifest.json
result.md
gui/actions.jsonl
tui/transcript.txt
tui/keystrokes.jsonl
timeline.jsonl
ipc/request-response-summary.jsonl
daemon/structured-log.jsonl
journal/event-summary.json
journal/stream-heads.json
journal/sqlite-summary.json
projection/summary.json
artifacts/digest-verification.json
processes/preflight.json
processes/postflight.json
processes/cleanup-proof.txt
state/loom.db'
printf '%s\n' "$required_files" | while IFS= read -r relative; do
  [ -f "$root/$relative" ] && [ ! -L "$root/$relative" ] || {
    echo "missing evidence: $relative" >&2
    exit 4
  }
  [ "$(/usr/bin/stat -f '%Lp' "$root/$relative")" = 600 ] || {
    echo "non-private evidence: $relative" >&2
    exit 4
  }
done

manifest="$root/manifest.json"
journey_id=$(/usr/bin/jq -er '.journey_id' "$manifest")
scenario_id=$(/usr/bin/jq -er '.scenario_id' "$manifest")
case "$journey_id" in
  ????????-????-4???-[89ab]???-????????????) ;;
  *) echo "invalid journey identity" >&2; exit 4 ;;
esac
case "$scenario_id" in
  happy-create-evaluate-activate-bind-execute-clean|\
  cancel-reject-retain|\
  stale-view-digest-generation|\
  concurrent-single-winner|\
  crash-before-cas|\
  crash-after-cas-before-response|\
  projection-failure-rebuild-reconnect|\
  slow-client-redelivery-clean-restart) ;;
  *) echo "invalid scenario identity" >&2; exit 4 ;;
esac
test "$(/usr/bin/jq -er '.schema_version' "$manifest")" = 1
test "$(/usr/bin/jq -r '.network_allowed' "$manifest")" = false
test "$(/usr/bin/jq -r '.provider_credentials_present' "$manifest")" = false
test "$(/usr/bin/jq -er '.product_result' "$manifest")" = PASS
test "$(/usr/bin/jq -er '.operational_trace_result' "$manifest")" = PASS

manifest_digest=$(/usr/bin/jq -er '.manifest_digest' "$manifest")
computed_manifest_digest=$(/usr/bin/jq -cS 'del(.manifest_digest)' "$manifest" | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}')
[ "$manifest_digest" = "$computed_manifest_digest" ] || {
  echo "manifest digest mismatch" >&2
  exit 5
}

test "$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'PRAGMA integrity_check;')" = ok
event_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM events;')
unique_events=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(DISTINCT id) FROM events;')
unique_idempotency=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(DISTINCT idempotency_key) FROM events;')
[ "$event_count" = "$unique_events" ] && [ "$event_count" = "$unique_idempotency" ] || {
  echo "duplicate Event or idempotency identity" >&2
  exit 5
}
stream_gap_count=$(/usr/bin/sqlite3 -readonly "$root/state/loom.db" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) AS n,MAX(seq) AS head FROM events GROUP BY stream_id HAVING n != head);')
[ "$stream_gap_count" = 0 ] || {
  echo "Journal stream gap" >&2
  exit 5
}

sqlite_summary="$root/journal/sqlite-summary.json"
test "$(/usr/bin/jq -er '.integrity_check' "$sqlite_summary")" = ok
test "$(/usr/bin/jq -er '.event_count' "$sqlite_summary")" = "$event_count"
test "$(/usr/bin/jq -er '.duplicate_event_id_count' "$sqlite_summary")" = 0
test "$(/usr/bin/jq -er '.duplicate_idempotency_key_count' "$sqlite_summary")" = 0
test "$(/usr/bin/jq -er '.stream_gap_count' "$sqlite_summary")" = 0
test "$(/usr/bin/jq -er '.matches_journal' "$root/projection/summary.json")" = true
test "$(/usr/bin/jq -er '[.artifacts[] | select(.available != true or .match != true)] | length' "$root/artifacts/digest-verification.json")" = 0

for log in "$root/ipc/request-response-summary.jsonl" "$root/daemon/structured-log.jsonl" "$root/gui/actions.jsonl" "$root/tui/keystrokes.jsonl" "$root/timeline.jsonl"; do
  [ -s "$log" ] || {
    echo "empty evidence log: $log" >&2
    exit 5
  }
  /usr/bin/jq -e . "$log" >/dev/null
done
[ "$(/usr/bin/jq -sr --arg journey "$journey_id" '[.[] | select(.journey_id == $journey and .client_kind == "gui")] | length' "$root/ipc/request-response-summary.jsonl")" -gt 0 ]
[ "$(/usr/bin/jq -sr --arg journey "$journey_id" '[.[] | select(.journey_id == $journey and .client_kind == "tui")] | length' "$root/ipc/request-response-summary.jsonl")" -gt 0 ]
[ "$(/usr/bin/jq -sr --arg journey "$journey_id" '[.[] | select(.journey_id != $journey)] | length' "$root/ipc/request-response-summary.jsonl")" = 0 ]
[ "$(/usr/bin/jq -sr --arg journey "$journey_id" '[.[] | select(.journey_id != $journey)] | length' "$root/daemon/structured-log.jsonl")" = 0 ]
/usr/bin/grep -q 'Loom ·' "$root/tui/transcript.txt"
[ "$(find "$root/gui/screenshots" -type f ! -type l | wc -l | tr -d ' ')" -gt 0 ]

for postflight_array in processes sockets locks leases temps; do
  test "$(/usr/bin/jq -er --arg key "$postflight_array" '.[$key] | length' "$root/processes/postflight.json")" = 0
done
[ ! -S "$root/loomd.sock" ] && [ ! -e "$root/loomd.sock.lock" ]

if /usr/bin/grep -E -i 'sk-[A-Za-z0-9_-]{12}|api[_-]?key[[:space:]]*[:=]|authorization:[[:space:]]*bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY' \
  "$root/result.md" \
  "$root/gui/actions.jsonl" \
  "$root/tui/keystrokes.jsonl" \
  "$root/timeline.jsonl" \
  "$root/ipc/request-response-summary.jsonl" \
  "$root/daemon/structured-log.jsonl"; then
  echo "secret-like material in journey evidence" >&2
  exit 6
fi

evidence_rows=$(mktemp -t loom-p3a-evidence.XXXXXX)
trap 'rm -f "$evidence_rows"' EXIT HUP INT TERM
/usr/bin/jq -r '.evidence_files[] | [.relative_path,.sha256,(.size|tostring),(.mode|tostring)] | @tsv' "$manifest" >"$evidence_rows"
while IFS="$(printf '\t')" read -r relative expected_digest expected_size expected_mode; do
  case "$relative" in
    ''|/*|../*|*/../*|*/..|*'//'*) echo "invalid evidence path: $relative" >&2; exit 7 ;;
  esac
  path="$root/$relative"
  [ -f "$path" ] && [ ! -L "$path" ] || exit 7
  actual_digest=$(/usr/bin/shasum -a 256 "$path" | /usr/bin/awk '{print $1}')
  actual_size=$(/usr/bin/stat -f '%z' "$path")
  actual_mode=$(/usr/bin/stat -f '%Lp' "$path")
  [ "$actual_digest" = "$expected_digest" ] &&
    [ "$actual_size" = "$expected_size" ] &&
    [ "$actual_mode" = "$expected_mode" ] || {
      echo "evidence identity mismatch: $relative" >&2
      exit 7
    }
done <"$evidence_rows"

rm -f "$evidence_rows"
trap - EXIT HUP INT TERM
echo "Phase 3A cross-client journey verified: $scenario_id $journey_id"
