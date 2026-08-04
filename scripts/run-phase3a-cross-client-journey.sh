#!/bin/sh
set -eu

usage() {
  echo "usage: run-phase3a-cross-client-journey.sh prepare --scenario ID --root ABS --loom ABS --loomd ABS --app ABS --source-lock ABS --runtime-fixture ABS" >&2
  echo "       run-phase3a-cross-client-journey.sh freeze --root ABS --product-result PASS|FAIL|PARTIAL --trace-result PASS|FAIL|PARTIAL" >&2
  echo "       run-phase3a-cross-client-journey.sh verify --root ABS" >&2
  exit 2
}

[ "$#" -gt 0 ] || usage
mode=$1
shift

scenario=
journey_root=
loom=
loomd=
app=
source_lock=
runtime_fixture=
product_result=
trace_result=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --scenario) scenario=${2-}; shift 2 ;;
    --root) journey_root=${2-}; shift 2 ;;
    --loom) loom=${2-}; shift 2 ;;
    --loomd) loomd=${2-}; shift 2 ;;
    --app) app=${2-}; shift 2 ;;
    --source-lock) source_lock=${2-}; shift 2 ;;
    --runtime-fixture) runtime_fixture=${2-}; shift 2 ;;
    --product-result) product_result=${2-}; shift 2 ;;
    --trace-result) trace_result=${2-}; shift 2 ;;
    *) usage ;;
  esac
done

script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ "$mode" = verify ]; then
  [ -n "$journey_root" ] || usage
  exec "$script_root/verify-phase3a-cross-client-journey.sh" "$journey_root"
fi
[ "$mode" = freeze ] || [ "$mode" = prepare ] || usage

sha256_file() {
  /usr/bin/shasum -a 256 "$1" | /usr/bin/awk '{print $1}'
}

path_digest() {
  /usr/bin/printf '%s\000%s' "$1" "$2" | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}'
}

if [ "$mode" = freeze ]; then
  [ -n "$journey_root" ] || usage
  case "$journey_root" in /*) ;; *) usage ;; esac
  case "$product_result" in PASS|FAIL|PARTIAL) ;; *) usage ;; esac
  case "$trace_result" in PASS|FAIL|PARTIAL) ;; *) usage ;; esac
  [ -d "$journey_root" ] && [ ! -L "$journey_root" ] || exit 3
  [ "$(/usr/bin/stat -f '%Lp' "$journey_root")" = 700 ] || exit 3
  context="$journey_root/manifest/journey-context.json"
  source_copy="$journey_root/source/source-lock.json"
  state_path="$journey_root/state/loom.db"
  [ -f "$context" ] && [ ! -L "$context" ] || exit 4
  [ -f "$source_copy" ] && [ ! -L "$source_copy" ] || exit 4
  [ -f "$state_path" ] && [ ! -L "$state_path" ] || exit 4

  journey_id=$(/usr/bin/jq -er '.journey_id' "$context")
  scenario=$(/usr/bin/jq -er '.scenario_id' "$context")
  created_at_utc=$(/usr/bin/jq -er '.created_at_utc' "$context")
  baseline_commit=$(/usr/bin/jq -er '.baseline_commit' "$source_copy")
  entry_amendment_digest=$(/usr/bin/jq -er '.entry_amendment_digest' "$source_copy")
  gate1_contract_set_digest=$(/usr/bin/jq -er '.gate1_contract_set_digest' "$source_copy")
  runtime_fixture_digest=$(/usr/bin/jq -er '.runtime_fixture_digest' "$context")
  source_lock_digest=$(sha256_file "$source_copy")
  expected_source_lock_digest=$(/usr/bin/jq -er '.source_lock_digest' "$context")
  [ "$source_lock_digest" = "$expected_source_lock_digest" ] || {
    echo "source lock changed after prepare" >&2
    exit 4
  }

  loom_path="$journey_root/bin/loom"
  loomd_path="$journey_root/bin/loomd"
  gui_executable=$(/usr/bin/jq -er '.gui_executable' "$context")
  gui_path="$journey_root/app/Loom.app/Contents/MacOS/$gui_executable"
  for component in "$loom_path" "$loomd_path" "$gui_path"; do
    [ -x "$component" ] && [ ! -L "$component" ] || exit 4
  done
  tui_binary_digest=$(sha256_file "$loom_path")
  daemon_binary_digest=$(sha256_file "$loomd_path")
  gui_binary_digest=$(sha256_file "$gui_path")
  [ "$tui_binary_digest" = "$(/usr/bin/jq -er '.tui_binary_digest' "$context")" ] || exit 4
  [ "$daemon_binary_digest" = "$(/usr/bin/jq -er '.daemon_binary_digest' "$context")" ] || exit 4
  [ "$gui_binary_digest" = "$(/usr/bin/jq -er '.gui_binary_digest' "$context")" ] || exit 4

  for required in \
    result.md gui/actions.jsonl tui/transcript.txt tui/keystrokes.jsonl timeline.jsonl \
    ipc/request-response-summary.jsonl daemon/structured-log.jsonl \
    projection/summary.json artifacts/digest-verification.json \
    processes/preflight.json processes/postflight.json processes/cleanup-proof.txt; do
    [ -f "$journey_root/$required" ] && [ ! -L "$journey_root/$required" ] || {
      echo "cannot freeze without evidence: $required" >&2
      exit 4
    }
    /bin/chmod 600 "$journey_root/$required"
  done
  [ "$(find "$journey_root/gui/screenshots" -type f ! -type l | wc -l | tr -d ' ')" -gt 0 ] || {
    echo "cannot freeze without native-window screenshots" >&2
    exit 4
  }
  find "$journey_root/gui/screenshots" -type f ! -type l -exec /bin/chmod 600 {} +

  event_rows=$(mktemp -t loom-p3a-events.XXXXXX)
  raw_event_rows=$(mktemp -t loom-p3a-events-raw.XXXXXX)
  head_rows=$(mktemp -t loom-p3a-heads.XXXXXX)
  evidence_rows=$(mktemp -t loom-p3a-manifest.XXXXXX)
  trap 'rm -f "$event_rows" "$raw_event_rows" "$head_rows" "$evidence_rows"' EXIT HUP INT TERM

  tab=$(printf '\t')
  /usr/bin/sqlite3 -readonly -noheader -separator "$tab" "$state_path" \
    "SELECT id,event_type,stream_id,seq,COALESCE(correlation_id,''),COALESCE(causation_id,''),printf('%s.%09dZ',strftime('%Y-%m-%dT%H:%M:%S',CAST(emitted_at/1000000000 AS INTEGER),'unixepoch'),emitted_at%1000000000),hex(payload_json) FROM events ORDER BY rowid;" \
    >"$raw_event_rows"
  : >"$event_rows"
  while IFS="$tab" read -r event_id event_type stream_id sequence correlation_id causation_id emitted_at_utc payload_hex; do
    [ -n "$event_id" ] || continue
    payload_digest=$(printf '%s' "$payload_hex" | /usr/bin/xxd -r -p | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}')
    /usr/bin/jq -cn \
      --arg event_id "$event_id" --arg event_type "$event_type" --arg stream_id "$stream_id" \
      --argjson sequence "$sequence" --arg correlation_id "$correlation_id" --arg causation_id "$causation_id" \
      --arg emitted_at_utc "$emitted_at_utc" --arg payload_digest "$payload_digest" \
      '{event_id:$event_id,event_type:$event_type,stream_id:$stream_id,sequence:$sequence,correlation_id:$correlation_id,causation_id:$causation_id,emitted_at_utc:$emitted_at_utc,payload_digest:$payload_digest}' \
      >>"$event_rows"
  done <"$raw_event_rows"
  /usr/bin/jq -cs --arg journey "$journey_id" \
    '{schema_version:1,journey_id:$journey,event_count:length,events:.}' \
    "$event_rows" >"$journey_root/journal/event-summary.json"

  /usr/bin/sqlite3 -readonly -json "$state_path" \
    "SELECT e.stream_id,e.seq AS sequence,e.id AS event_id FROM events e JOIN (SELECT stream_id,MAX(seq) AS seq FROM events GROUP BY stream_id) h ON h.stream_id=e.stream_id AND h.seq=e.seq ORDER BY e.stream_id;" \
    >"$head_rows"
  /usr/bin/jq -c --arg journey "$journey_id" \
    '{schema_version:1,journey_id:$journey,streams:.}' \
    "$head_rows" >"$journey_root/journal/stream-heads.json"

  integrity_check=$(/usr/bin/sqlite3 -readonly "$state_path" 'PRAGMA integrity_check;')
  foreign_key_violation_count=$(/usr/bin/sqlite3 -readonly "$state_path" 'SELECT COUNT(*) FROM pragma_foreign_key_check;')
  event_count=$(/usr/bin/sqlite3 -readonly "$state_path" 'SELECT COUNT(*) FROM events;')
  duplicate_event_id_count=$(/usr/bin/sqlite3 -readonly "$state_path" 'SELECT COUNT(*)-COUNT(DISTINCT id) FROM events;')
  duplicate_idempotency_key_count=$(/usr/bin/sqlite3 -readonly "$state_path" 'SELECT COUNT(*)-COUNT(DISTINCT idempotency_key) FROM events;')
  stream_gap_count=$(/usr/bin/sqlite3 -readonly "$state_path" 'SELECT COUNT(*) FROM (SELECT stream_id,COUNT(*) AS n,MAX(seq) AS head FROM events GROUP BY stream_id HAVING n != head);')
  journey_event_count=$(/usr/bin/sqlite3 -readonly "$state_path" "SELECT COUNT(*) FROM events WHERE correlation_id='$journey_id';")
  /usr/bin/jq -cn \
    --arg journey "$journey_id" \
    --arg integrity "$integrity_check" \
    --argjson foreign_keys "$foreign_key_violation_count" \
    --argjson events "$event_count" \
    --argjson duplicate_events "$duplicate_event_id_count" \
    --argjson duplicate_operations "$duplicate_idempotency_key_count" \
    --argjson gaps "$stream_gap_count" \
    --argjson journey_events "$journey_event_count" \
    '{schema_version:1,journey_id:$journey,integrity_check:$integrity,foreign_key_violation_count:$foreign_keys,event_count:$events,duplicate_event_id_count:$duplicate_events,duplicate_idempotency_key_count:$duplicate_operations,stream_gap_count:$gaps,journey_event_count:$journey_events}' \
    >"$journey_root/journal/sqlite-summary.json"
  /bin/chmod 600 "$journey_root/journal/event-summary.json" "$journey_root/journal/stream-heads.json" "$journey_root/journal/sqlite-summary.json"

  for json_file in \
    projection/summary.json artifacts/digest-verification.json \
    processes/preflight.json processes/postflight.json; do
    test "$(/usr/bin/jq -er '.schema_version' "$journey_root/$json_file")" = 1
    test "$(/usr/bin/jq -er '.journey_id' "$journey_root/$json_file")" = "$journey_id"
  done

  evidence_files='result.md
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
state/loom.db
source/source-lock.json
source/runtime-fixture.json
manifest/journey-context.json
manifest/journey-harness.json
manifest/preflight-source-lock.json'
  {
    printf '%s\n' "$evidence_files"
    find "$journey_root/gui/screenshots" -type f ! -type l | sed "s#^$journey_root/##"
    find "$journey_root/assertions" -type f ! -type l | sed "s#^$journey_root/##"
  } | LC_ALL=C sort -u | while IFS= read -r relative; do
    [ -n "$relative" ] || continue
    path="$journey_root/$relative"
    [ -f "$path" ] && [ ! -L "$path" ] || exit 5
    digest=$(sha256_file "$path")
    size=$(/usr/bin/stat -f '%z' "$path")
    mode_value=$(/usr/bin/stat -f '%Lp' "$path")
    /usr/bin/jq -cn --arg path "$relative" --arg digest "$digest" --argjson size "$size" --argjson mode "$mode_value" \
      '{relative_path:$path,sha256:$digest,size:$size,mode:$mode}'
  done | /usr/bin/jq -cs 'sort_by(.relative_path)' >"$evidence_rows"

  user_actions_digest=$(sha256_file "$journey_root/gui/actions.jsonl")
  ipc_summary_digest=$(sha256_file "$journey_root/ipc/request-response-summary.jsonl")
  daemon_log_digest=$(sha256_file "$journey_root/daemon/structured-log.jsonl")
  journal_summary_digest=$(sha256_file "$journey_root/journal/event-summary.json")
  projection_summary_digest=$(sha256_file "$journey_root/projection/summary.json")
  sqlite_summary_digest=$(sha256_file "$journey_root/journal/sqlite-summary.json")
  artifact_verification_digest=$(sha256_file "$journey_root/artifacts/digest-verification.json")
  preflight_process_digest=$(sha256_file "$journey_root/processes/preflight.json")
  postflight_process_digest=$(sha256_file "$journey_root/processes/postflight.json")
  cleanup_proof_digest=$(sha256_file "$journey_root/processes/cleanup-proof.txt")
  state_root_path_digest=$(path_digest "$journey_id" "$journey_root/state")
  socket_path_digest=$(path_digest "$journey_id" "$journey_root/loomd.sock")

  production_components=$(/usr/bin/jq -cn \
    --arg daemon_path "$(path_digest "$journey_id" "$loomd_path")" \
    --arg daemon_sha "$daemon_binary_digest" \
    --arg gui_path "$(path_digest "$journey_id" "$gui_path")" \
    --arg gui_sha "$gui_binary_digest" \
    --arg tui_path "$(path_digest "$journey_id" "$loom_path")" \
    --arg tui_sha "$tui_binary_digest" \
    '[{kind:"daemon",path_digest:$daemon_path,sha256:$daemon_sha,build_id:("sha256:"+$daemon_sha)},
      {kind:"gui",path_digest:$gui_path,sha256:$gui_sha,build_id:("sha256:"+$gui_sha)},
      {kind:"tui",path_digest:$tui_path,sha256:$tui_sha,build_id:("sha256:"+$tui_sha)}]')

  /usr/bin/jq -cn \
    --arg journey_id "$journey_id" --arg scenario_id "$scenario" \
    --arg created_at_utc "$created_at_utc" --arg baseline_commit "$baseline_commit" \
    --arg entry_amendment_digest "$entry_amendment_digest" \
    --arg gate1_contract_set_digest "$gate1_contract_set_digest" \
    --arg source_lock_digest "$source_lock_digest" \
    --arg daemon_binary_digest "$daemon_binary_digest" --arg gui_binary_digest "$gui_binary_digest" \
    --arg tui_binary_digest "$tui_binary_digest" --arg runtime_fixture_digest "$runtime_fixture_digest" \
    --arg state_root_path_digest "$state_root_path_digest" --arg socket_path_digest "$socket_path_digest" \
    --argjson production_components "$production_components" \
    --arg user_actions_digest "$user_actions_digest" --arg ipc_summary_digest "$ipc_summary_digest" \
    --arg daemon_log_digest "$daemon_log_digest" --arg journal_summary_digest "$journal_summary_digest" \
    --arg projection_summary_digest "$projection_summary_digest" --arg sqlite_summary_digest "$sqlite_summary_digest" \
    --arg artifact_verification_digest "$artifact_verification_digest" \
    --arg preflight_process_digest "$preflight_process_digest" --arg postflight_process_digest "$postflight_process_digest" \
    --arg cleanup_proof_digest "$cleanup_proof_digest" --arg product_result "$product_result" \
    --arg operational_trace_result "$trace_result" --slurpfile evidence_files "$evidence_rows" \
    '{schema_version:1,journey_id:$journey_id,scenario_id:$scenario_id,created_at_utc:$created_at_utc,baseline_commit:$baseline_commit,entry_amendment_digest:$entry_amendment_digest,gate1_contract_set_digest:$gate1_contract_set_digest,source_lock_digest:$source_lock_digest,daemon_binary_digest:$daemon_binary_digest,gui_binary_digest:$gui_binary_digest,tui_binary_digest:$tui_binary_digest,runtime_fixture_digest:$runtime_fixture_digest,state_root_path_digest:$state_root_path_digest,socket_path_digest:$socket_path_digest,network_allowed:false,provider_credentials_present:false,production_components:$production_components,user_actions_digest:$user_actions_digest,ipc_summary_digest:$ipc_summary_digest,daemon_log_digest:$daemon_log_digest,journal_summary_digest:$journal_summary_digest,projection_summary_digest:$projection_summary_digest,sqlite_summary_digest:$sqlite_summary_digest,artifact_verification_digest:$artifact_verification_digest,preflight_process_digest:$preflight_process_digest,postflight_process_digest:$postflight_process_digest,cleanup_proof_digest:$cleanup_proof_digest,product_result:$product_result,operational_trace_result:$operational_trace_result,evidence_files:$evidence_files[0]}' \
    >"$journey_root/manifest.json.unsigned"
  manifest_digest=$(/usr/bin/jq -cS . "$journey_root/manifest.json.unsigned" | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}')
  /usr/bin/jq -c --arg digest "$manifest_digest" '. + {manifest_digest:$digest}' \
    "$journey_root/manifest.json.unsigned" >"$journey_root/manifest.json"
  /bin/rm "$journey_root/manifest.json.unsigned"
  /bin/chmod 600 "$journey_root/manifest.json"
  rm -f "$event_rows" "$raw_event_rows" "$head_rows" "$evidence_rows"
  trap - EXIT HUP INT TERM
  exec "$script_root/verify-phase3a-cross-client-journey.sh" "$journey_root"
fi

case "$scenario" in
  happy-create-evaluate-activate-bind-execute-clean|\
  cancel-reject-retain|\
  stale-view-digest-generation|\
  concurrent-single-winner|\
  crash-before-cas|\
  crash-after-cas-before-response|\
  projection-failure-rebuild-reconnect|\
  slow-client-redelivery-clean-restart) ;;
  *) usage ;;
esac

for path in "$journey_root" "$loom" "$loomd" "$app" "$source_lock" "$runtime_fixture"; do
  case "$path" in /*) ;; *) usage ;; esac
done
[ -x "$loom" ] && [ ! -L "$loom" ] || usage
[ -x "$loomd" ] && [ ! -L "$loomd" ] || usage
[ -d "$app" ] && [ ! -L "$app" ] || usage
[ -f "$source_lock" ] && [ ! -L "$source_lock" ] || usage
[ -f "$runtime_fixture" ] && [ ! -L "$runtime_fixture" ] || usage
[ -z "$(find "$app" -type l -print -quit)" ] || usage
[ "$(/usr/bin/jq -er '.schema_version' "$source_lock")" = 1 ] || usage
baseline_commit=$(/usr/bin/jq -er '.baseline_commit' "$source_lock")
entry_amendment_digest=$(/usr/bin/jq -er '.entry_amendment_digest' "$source_lock")
gate1_contract_set_digest=$(/usr/bin/jq -er '.gate1_contract_set_digest' "$source_lock")
case "$baseline_commit$entry_amendment_digest$gate1_contract_set_digest" in
  *[!0-9a-f]*) usage ;;
esac
[ "${#baseline_commit}" -eq 40 ] && [ "${#entry_amendment_digest}" -eq 64 ] && [ "${#gate1_contract_set_digest}" -eq 64 ] || usage
[ ! -e "$journey_root" ] && [ ! -L "$journey_root" ] || {
  echo "journey root already exists" >&2
  exit 3
}

journey_id=$(/usr/bin/uuidgen | /usr/bin/tr '[:upper:]' '[:lower:]')
/bin/mkdir -m 700 "$journey_root"
for relative in \
  manifest source state isolation ipc daemon gui gui/screenshots tui journal \
  bin app \
  projection artifacts processes assertions; do
  /bin/mkdir -m 700 "$journey_root/$relative"
done
/usr/bin/install -m 600 /dev/null "$journey_root/state/loom.db"
/usr/bin/install -m 600 "$source_lock" "$journey_root/source/source-lock.json"
/usr/bin/install -m 600 "$runtime_fixture" "$journey_root/source/runtime-fixture.json"
/usr/bin/install -m 700 "$loom" "$journey_root/bin/loom"
/usr/bin/install -m 700 "$loomd" "$journey_root/bin/loomd"
/bin/cp -R "$app" "$journey_root/app/Loom.app"
/bin/chmod 700 "$journey_root/app/Loom.app"

state_path="$journey_root/state/loom.db"
socket_path="$journey_root/loomd.sock"
evidence_root="$journey_root"
fault_kind=none
fault_action=none
delay_millis=0
case "$scenario" in
  crash-before-cas)
    fault_kind=crash_before_cas
    fault_action=activate
    ;;
  crash-after-cas-before-response)
    fault_kind=crash_after_cas_before_response
    fault_action=activate
    ;;
  projection-failure-rebuild-reconnect)
    fault_kind=projection_failure
    fault_action=activate
    ;;
  slow-client-redelivery-clean-restart)
    fault_kind=slow_response
    fault_action=activate
    delay_millis=7000
    ;;
esac

/usr/bin/jq -cn \
  --arg journey_id "$journey_id" \
  --arg state_path "$state_path" \
  --arg socket_path "$socket_path" \
  --arg evidence_root "$evidence_root" \
  --arg fault_kind "$fault_kind" \
  --arg fault_action "$fault_action" \
  --argjson delay_millis "$delay_millis" \
  '{schema_version:1,purpose:"phase3a-cross-client-e2e",journey_id:$journey_id,state_path:$state_path,socket_path:$socket_path,evidence_root:$evidence_root,fault_kind:$fault_kind,fault_action:$fault_action,delay_millis:$delay_millis}' \
  >"$journey_root/manifest/journey-harness.json"
/bin/chmod 600 "$journey_root/manifest/journey-harness.json"

loom_digest=$(sha256_file "$journey_root/bin/loom")
loomd_digest=$(sha256_file "$journey_root/bin/loomd")
gui_executable=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$journey_root/app/Loom.app/Contents/Info.plist")
gui_digest=$(sha256_file "$journey_root/app/Loom.app/Contents/MacOS/$gui_executable")
source_digest=$(sha256_file "$journey_root/source/source-lock.json")
runtime_fixture_digest=$(sha256_file "$journey_root/source/runtime-fixture.json")
created_at_utc=$(/bin/date -u '+%Y-%m-%dT%H:%M:%SZ')
/usr/bin/jq -cn \
  --arg journey_id "$journey_id" --arg scenario_id "$scenario" --arg created_at_utc "$created_at_utc" \
  --arg baseline_commit "$baseline_commit" --arg entry_amendment_digest "$entry_amendment_digest" \
  --arg gate1_contract_set_digest "$gate1_contract_set_digest" --arg source_lock_digest "$source_digest" \
  --arg runtime_fixture_digest "$runtime_fixture_digest" --arg gui_executable "$gui_executable" \
  --arg tui_binary_digest "$loom_digest" --arg daemon_binary_digest "$loomd_digest" --arg gui_binary_digest "$gui_digest" \
  '{schema_version:1,journey_id:$journey_id,scenario_id:$scenario_id,created_at_utc:$created_at_utc,baseline_commit:$baseline_commit,entry_amendment_digest:$entry_amendment_digest,gate1_contract_set_digest:$gate1_contract_set_digest,source_lock_digest:$source_lock_digest,runtime_fixture_digest:$runtime_fixture_digest,gui_executable:$gui_executable,tui_binary_digest:$tui_binary_digest,daemon_binary_digest:$daemon_binary_digest,gui_binary_digest:$gui_binary_digest}' \
  >"$journey_root/manifest/journey-context.json"
/bin/chmod 600 "$journey_root/manifest/journey-context.json"
/usr/bin/jq -cn \
  --arg journey_id "$journey_id" \
  --arg scenario_id "$scenario" \
  --arg loom_sha256 "$loom_digest" \
  --arg loomd_sha256 "$loomd_digest" \
  --arg gui_sha256 "$gui_digest" \
  --arg source_lock_sha256 "$source_digest" \
  --arg runtime_fixture_sha256 "$runtime_fixture_digest" \
  '{schema_version:1,journey_id:$journey_id,scenario_id:$scenario_id,loom_sha256:$loom_sha256,loomd_sha256:$loomd_sha256,gui_sha256:$gui_sha256,source_lock_sha256:$source_lock_sha256,runtime_fixture_sha256:$runtime_fixture_sha256}' \
  >"$journey_root/manifest/preflight-source-lock.json"
/bin/chmod 600 "$journey_root/manifest/preflight-source-lock.json"

/usr/bin/jq -cn \
  --arg journey_id "$journey_id" \
  --arg scenario_id "$scenario" \
  --arg root "$journey_root" \
  --arg state_path "$state_path" \
  --arg socket_path "$socket_path" \
  --arg harness_manifest "$journey_root/manifest/journey-harness.json" \
  --arg loom "$journey_root/bin/loom" --arg loomd "$journey_root/bin/loomd" --arg app "$journey_root/app/Loom.app" \
  '{journey_id:$journey_id,scenario_id:$scenario_id,root:$root,state_path:$state_path,socket_path:$socket_path,harness_manifest:$harness_manifest,loom:$loom,loomd:$loomd,app:$app,next:"Alternative verification (P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT): launch the production native app with --journey-id, drive mutations/reads through the production Swift client over the real socket, capture real window screenshots, and drive the production TUI through a real PTY; never substitute a service call, AppleScript or direct state write. Freeze only after shutdown and evidence capture."}'
