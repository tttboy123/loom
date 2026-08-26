#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
harness="$repo_root/scripts/phase2d-live-acceptance.sh"
runbook="$repo_root/.loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md"
source_plist="$repo_root/apps/macos/Resources/Info.plist"

if [ ! -f "$harness" ]; then
  echo "RED: Phase 2D acceptance harness is missing" >&2
  exit 1
fi
sh -n "$harness"

grep -Fq -- '--destination $HOME/Applications/Loom.app' "$runbook" || {
  echo "RED: installed G7 runbook does not use the canonical user App path" >&2
  exit 1
}
if grep -Fq -- '--destination /Applications/Loom.app' "$runbook"; then
  echo "RED: installed G7 runbook still uses the rejected system App path" >&2
  exit 1
fi
source_build=$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$source_plist")
case "$source_build" in
  ''|*[!0-9]*) echo "RED: source App build is invalid" >&2; exit 1 ;;
esac
[ "$source_build" -gt 127 ] || {
  echo "RED: HG1 source App build did not advance beyond installed Build 127" >&2
  exit 1
}

# Source-only mode must load without side effects.
LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1
# shellcheck disable=SC1090
. "$harness"
test "${phase2d_acceptance_source_only:-0}" = 1

# No secret-shaped markers may appear in the harness source.
if grep -Eiq 'api[_-]?key|authorization|bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE|DEEPSEEK-|sk-[A-Za-z0-9]' "$harness"; then
  echo "RED: harness contains a secret-shaped marker" >&2
  exit 1
fi

# Required functions must exist.
for fn in \
  phase2d_acceptance_installed_app \
  phase2d_acceptance_bundle_identity \
  phase2d_acceptance_valid_hg1_summary \
  phase2d_acceptance_gate_has_identity \
  phase2d_acceptance_required_gate_documents \
  phase2d_acceptance_prerequisite_summary \
  phase2d_acceptance_export_evidence \
  phase2d_acceptance_hg1_trace_summary \
  phase2d_acceptance_hg1_bundle_matches_app \
  phase2d_acceptance_export_hg1_evidence \
  phase2d_acceptance_summary
do
  grep -q "^$fn()" "$harness" || {
    echo "RED: harness missing function $fn" >&2
    exit 1
  }
done

# Behavior checks in a private temp dir (no real App, no credentials).
private_root=$(mktemp -d "${TMPDIR:-/tmp}/loom-phase2d-acceptance.XXXXXX")
trap 'chmod -R u+rwX "$private_root" 2>/dev/null || true; rm -rf "$private_root"' EXIT HUP INT TERM
chmod 700 "$private_root"
HOME=$private_root
export HOME

test "$(phase2d_acceptance_installed_app "$private_root/Applications/Loom.app")" = "missing"
test "$(phase2d_acceptance_installed_app "relative/Loom.app")" = "invalid"
test "$(phase2d_acceptance_installed_app "$private_root/Applications/Other.app")" = "invalid"

fake_app="$private_root/Applications/Loom.app"
mkdir -p "$fake_app/Contents/MacOS" "$fake_app/Contents/Library/Helpers"
cat > "$fake_app/Contents/Info.plist" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleVersion</key><string>124</string>
</dict></plist>
EOF
printf '%s\n' app-build-124 > "$fake_app/Contents/MacOS/LoomLocalApp"
printf '%s\n' daemon-build-124 > "$fake_app/Contents/Library/Helpers/loomd"
chmod 700 "$fake_app/Contents/MacOS/LoomLocalApp" \
  "$fake_app/Contents/Library/Helpers/loomd"
test "$(phase2d_acceptance_installed_app "$fake_app")" = "ok"

external_binary="$private_root/external-binary"
printf '%s\n' external > "$external_binary"
chmod 700 "$external_binary"
mv "$fake_app/Contents/MacOS/LoomLocalApp" \
  "$fake_app/Contents/MacOS/LoomLocalApp.real"
ln -s "$external_binary" "$fake_app/Contents/MacOS/LoomLocalApp"
test "$(phase2d_acceptance_installed_app "$fake_app")" = "missing"
rm "$fake_app/Contents/MacOS/LoomLocalApp"
mv "$fake_app/Contents/MacOS/LoomLocalApp.real" \
  "$fake_app/Contents/MacOS/LoomLocalApp"
mv "$fake_app/Contents/Library/Helpers/loomd" \
  "$fake_app/Contents/Library/Helpers/loomd.real"
ln -s "$external_binary" "$fake_app/Contents/Library/Helpers/loomd"
test "$(phase2d_acceptance_installed_app "$fake_app")" = "missing_helper"
rm "$fake_app/Contents/Library/Helpers/loomd"
mv "$fake_app/Contents/Library/Helpers/loomd.real" \
  "$fake_app/Contents/Library/Helpers/loomd"

missing=$(phase2d_acceptance_required_gate_documents \
  "$fake_app" "$private_root/evidence")
test "$missing" = G7

test "$(phase2d_acceptance_export_evidence \
  "$fake_app" "$private_root/evidence" G7 incident-generic)" = "requires_hg1_trace"

hg1_trace="$private_root/hg1-operational.jsonl"
hg1_verifier="$repo_root/scripts/phase2d-hg1-trace.jq"
touch "$hg1_trace"
chmod 600 "$hg1_trace"
hg1_digest=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
hg1_segment_digest=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
hg1_policy_digest=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
hg1_workspace_digest=dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
hg1_cancel_digest=eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
hg1_route_review_digest=ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
hg1_instance=gateway-fixture-main

emit_hg1_event() {
  seq=$1 type=$2 session=$3 harness_id=$4 backend_id=$5 thread=$6 segment=$7
  provider=$8 account=$9 response=${10} incident=${11} capsule=${12}
  credential_revision=3
  governance_policy_digest=$hg1_policy_digest
  route_transition_review_digest=
  model_id=model-main
  profile_id=conversation-codex-default
  if [ "$harness_id" = codex ] && [ "$provider" = openai ] && [ -z "$account" ]; then
    credential_revision=0
    governance_policy_digest=
  elif [ "$harness_id" = loom-native ] && [ "$provider" = deepseek ]; then
    route_transition_review_digest=$hg1_route_review_digest
    model_id=deepseek-chat
    profile_id=conversation-deepseek-chat
  fi
  /usr/bin/jq -cn \
    --argjson seq "$seq" --arg type "$type" --arg session "$session" \
    --arg harness "$harness_id" --arg backend "$backend_id" \
    --arg thread "$thread" --arg segment "$segment" \
    --arg provider "$provider" --arg account "$account" \
    --arg model "$model_id" --arg profile "$profile_id" \
    --arg response "$response" --arg incident "$incident" --arg capsule "$capsule" \
    --argjson credential_revision "$credential_revision" \
    --arg binding "$hg1_digest" --arg segment_capsule "$hg1_segment_digest" \
    --arg policy "$governance_policy_digest" --arg workspace "$hg1_workspace_digest" \
    --arg route_review "$route_transition_review_digest" \
    --arg gateway_instance "$hg1_instance" \
    '{
      schema_version:1, occurred_at:"2026-08-24T12:00:00Z",
      operation:"chat_message", stage:"conversation_dispatch",
      result:(if $type == "response_cancelled" then "failed" else "succeeded" end),
      error_code:(if $type == "response_cancelled" then "cancelled" else "" end),
      retryable:false, incident_id:(if $incident == "" then "loom-session-fixture" else $incident end),
      gateway_instance_id:$gateway_instance,
      gateway_event_schema_version:3, gateway_configured_harness_version:1,
      gateway_backend_version:1, gateway_event_sequence:$seq,
      gateway_event_type:$type, session_id:$session, harness_id:$harness,
      backend_id:$backend, thread_id:$thread, segment_id:$segment,
      workspace_id:"workspace-main", workspace_digest:$workspace,
      execution_binding_digest:$binding, provider_id:$provider,
      provider_account_id:$account, credential_revision:$credential_revision,
      model_id:$model, profile_id:$profile,
      segment_context_capsule_digest:$segment_capsule,
      governance_policy_digest:$policy
    } + (if $route_review == "" then {} else {
      route_transition_review_digest:$route_review
    } end) + (if $response == "" then {} else {
      response_id:$response, context_capsule_digest:$capsule
    } end)' >> "$hg1_trace"
}

emit_hg1_event 1 session_opening session-codex codex backend.codex.app-server conversation-main segment-codex openai "" "" "" ""
emit_hg1_event 2 session_ready session-codex codex backend.codex.app-server conversation-main segment-codex openai "" "" "" ""
emit_hg1_event 3 response_started session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-codex-1 incident-codex-1 "$hg1_digest"
emit_hg1_event 4 response_completed session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-codex-1 incident-codex-1 "$hg1_digest"
emit_hg1_event 5 response_started session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-codex-2 incident-codex-2 "$hg1_segment_digest"
emit_hg1_event 6 response_completed session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-codex-2 incident-codex-2 "$hg1_segment_digest"
emit_hg1_event 7 session_opening session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary "" "" ""
emit_hg1_event 8 session_ready session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary "" "" ""
emit_hg1_event 9 response_started session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary response-deepseek-1 incident-deepseek-1 "$hg1_digest"
emit_hg1_event 10 response_completed session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary response-deepseek-1 incident-deepseek-1 "$hg1_digest"
emit_hg1_event 11 session_opening session-overlap-a codex backend.codex.app-server conversation-a segment-a openai "" "" "" ""
emit_hg1_event 12 session_ready session-overlap-a codex backend.codex.app-server conversation-a segment-a openai "" "" "" ""
emit_hg1_event 13 session_opening session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" "" "" ""
emit_hg1_event 14 session_ready session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" "" "" ""
emit_hg1_event 15 response_started session-overlap-a codex backend.codex.app-server conversation-a segment-a openai "" response-overlap-a incident-overlap-a "$hg1_digest"
emit_hg1_event 16 response_started session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" response-overlap-b incident-overlap-b "$hg1_digest"
emit_hg1_event 17 response_completed session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" response-overlap-b incident-overlap-b "$hg1_digest"
emit_hg1_event 18 response_completed session-overlap-a codex backend.codex.app-server conversation-a segment-a openai "" response-overlap-a incident-overlap-a "$hg1_digest"
emit_hg1_event 19 response_started session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-cancel incident-cancel "$hg1_cancel_digest"
emit_hg1_event 20 response_started session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" response-peer incident-peer "$hg1_segment_digest"
emit_hg1_event 21 response_cancelled session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-cancel incident-cancel "$hg1_cancel_digest"
emit_hg1_event 22 response_completed session-overlap-b codex backend.codex.app-server conversation-b segment-b openai "" response-peer incident-peer "$hg1_segment_digest"
emit_hg1_event 23 response_started session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-after-cancel incident-after-cancel "$hg1_policy_digest"
emit_hg1_event 24 response_completed session-codex codex backend.codex.app-server conversation-main segment-codex openai "" response-after-cancel incident-after-cancel "$hg1_policy_digest"
emit_hg1_event 25 response_started session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary response-deepseek-2 incident-deepseek-2 "$hg1_segment_digest"
emit_hg1_event 26 response_completed session-deepseek loom-native backend.segment.loom-native conversation-main segment-deepseek deepseek deepseek.primary response-deepseek-2 incident-deepseek-2 "$hg1_segment_digest"

summary=$(phase2d_acceptance_hg1_trace_summary "$hg1_trace" "$hg1_verifier")
phase2d_acceptance_valid_hg1_summary "$summary"
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.matrix')" = pass
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.gateway_instance_id')" = "$hg1_instance"
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.source.session_id')" = session-codex
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.session_id')" = session-deepseek
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.provider_account_id')" = deepseek.primary
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.credential_revision')" = 3
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.model_id')" = deepseek-chat
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.route_transition_review_digest')" = "$hg1_route_review_digest"
test "$(printf '%s' "$summary" | /usr/bin/jq -r '.route.target.response_ids | length')" = 2

# The user-facing App export wraps allowlisted events in one diagnostic bundle.
hg1_bundle="$private_root/loom-diagnostics.json"
set -- $(phase2d_acceptance_bundle_identity "$fake_app")
/usr/bin/jq -s --arg build "$1" --arg app_sha "$2" --arg daemon_sha "$3" \
  '{schema_version:1,
    generated_at:"2026-08-25T12:00:00.000Z",
    app:{version:"0.5.3",build:$build,sha256:$app_sha},
    daemon:{version:"0.5.3",build:$build,sha256:$daemon_sha},
    socket_healthy:true,
    console:{available:true,byte_count:128},
    providers:[{provider_id:"deepseek",auth_mode:"api_key",revision:3,
      status:"verified",reason:""}],
    profiles:[{profile_id:"conversation.deepseek",provider_id:"deepseek",
      model_id:"deepseek-chat",auth_mode:"api_key",credential_revision:3}],
    agents:[{agent_instance_id:"agent.deepseek",status:"succeeded",
      harness_adapter:"loom-native",provider_id:"deepseek",
      provider_account_id:"deepseek.primary",model_id:"deepseek-chat",
      reasoning_effort:"",credential_revision:3,terminal_reason:""}],
    events:map(. + {source:"daemon"}),
    diagnostic_files_skipped:0,
    excluded:["api_keys_and_credentials","authorization_headers",
      "environment_credentials","prompts_and_conversations",
      "provider_response_bodies","raw_daemon_console"]}' \
  "$hg1_trace" > "$hg1_bundle"
chmod 600 "$hg1_bundle"
bundle_summary=$(phase2d_acceptance_hg1_trace_summary "$hg1_bundle" "$hg1_verifier")
test "$(printf '%s' "$bundle_summary" | /usr/bin/jq -r '.matrix')" = pass
bundle_document=$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/bundle-evidence" "$hg1_bundle" "$hg1_verifier")
phase2d_acceptance_private_file "$bundle_document"
grep -Fqx -- "- hg1_matrix: pass" "$bundle_document"

test "$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/evidence" "$hg1_trace" "$hg1_verifier")" = unavailable
wrapper_document=$("$repo_root/scripts/export-phase2d-hg1-evidence.sh" \
  "$fake_app" "$private_root/wrapper-evidence" "$hg1_bundle")
phase2d_acceptance_private_file "$wrapper_document"
grep -Fqx -- "- hg1_matrix: pass" "$wrapper_document"
test "$(phase2d_acceptance_required_gate_documents \
  "$fake_app" "$private_root/wrapper-evidence")" = ok

weak_summary=$(printf '%s' "$summary" | /usr/bin/jq -c \
  'del(.route.source.completed_sequences)')
if phase2d_acceptance_valid_hg1_summary "$weak_summary"; then
  echo "RED: incomplete G7 summary satisfied the closed summary contract" >&2
  exit 1
fi
mkdir -m 700 "$private_root/weak-summary-evidence"
weak_document="$private_root/weak-summary-evidence/G7-weak.md"
/usr/bin/awk -v summary="$weak_summary" '
  /^- hg1_summary: / { print "- hg1_summary: " summary; next }
  { print }
' "$wrapper_document" > "$weak_document"
chmod 600 "$weak_document"
set -- $(phase2d_acceptance_bundle_identity "$fake_app")
if phase2d_acceptance_gate_has_identity \
  G7 "$private_root/weak-summary-evidence" "$1" "$2" "$3"
then
  echo "RED: incomplete G7 summary satisfied persisted gate evidence" >&2
  exit 1
fi

wrong_identity_bundle="$private_root/hg1-wrong-bundle-identity.json"
/usr/bin/jq '.app.sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' \
  "$hg1_bundle" > "$wrong_identity_bundle"
chmod 600 "$wrong_identity_bundle"
test "$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/evidence" "$wrong_identity_bundle" "$hg1_verifier")" = unavailable

unknown_binary_metadata_bundle="$private_root/hg1-unknown-binary-metadata.json"
/usr/bin/jq '.app.opaque_future_field = "not allowlisted"' \
  "$hg1_bundle" > "$unknown_binary_metadata_bundle"
chmod 600 "$unknown_binary_metadata_bundle"
test "$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/unknown-binary-evidence" \
  "$unknown_binary_metadata_bundle" "$hg1_verifier")" = unavailable

missing_export_shape_bundle="$private_root/hg1-missing-export-shape.json"
/usr/bin/jq 'del(.console)' "$hg1_bundle" > "$missing_export_shape_bundle"
chmod 600 "$missing_export_shape_bundle"
test "$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/missing-shape-evidence" \
  "$missing_export_shape_bundle" "$hg1_verifier")" = unavailable

unknown_provider_metadata_bundle="$private_root/hg1-unknown-provider-metadata.json"
/usr/bin/jq '.providers[0].opaque_future_field = "not allowlisted"' \
  "$hg1_bundle" > "$unknown_provider_metadata_bundle"
chmod 600 "$unknown_provider_metadata_bundle"
test "$(phase2d_acceptance_export_hg1_evidence \
  "$fake_app" "$private_root/unknown-provider-evidence" \
  "$unknown_provider_metadata_bundle" "$hg1_verifier")" = unavailable

tampered_trace="$private_root/hg1-tampered.jsonl"
/usr/bin/jq -c \
  'if .session_id == "session-codex" and .gateway_event_sequence == 4 then .credential_revision = 8 else . end' \
  "$hg1_trace" > "$tampered_trace"
chmod 600 "$tampered_trace"
test "$(phase2d_acceptance_hg1_trace_summary "$tampered_trace" "$hg1_verifier")" = unavailable

content_trace="$private_root/hg1-content-bearing.jsonl"
/usr/bin/jq -c \
  'if .gateway_event_sequence == 4 then .private_message = "must not be accepted" else . end' \
  "$hg1_trace" > "$content_trace"
chmod 600 "$content_trace"
test "$(phase2d_acceptance_hg1_trace_summary "$content_trace" "$hg1_verifier")" = unavailable

top_level_content_bundle="$private_root/hg1-top-level-content.json"
/usr/bin/jq '.content = "must not be accepted"' \
  "$hg1_bundle" > "$top_level_content_bundle"
chmod 600 "$top_level_content_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$top_level_content_bundle" "$hg1_verifier")" = unavailable

unknown_top_level_bundle="$private_root/hg1-unknown-bundle-field.json"
/usr/bin/jq '.opaque_future_bundle_field = "not allowlisted"' \
  "$hg1_bundle" > "$unknown_top_level_bundle"
chmod 600 "$unknown_top_level_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$unknown_top_level_bundle" "$hg1_verifier")" = unavailable

unknown_event_bundle="$private_root/hg1-unknown-event-field.json"
/usr/bin/jq '.events[3].opaque_future_field = "not allowlisted"' \
  "$hg1_bundle" > "$unknown_event_bundle"
chmod 600 "$unknown_event_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$unknown_event_bundle" "$hg1_verifier")" = unavailable

missing_profile_bundle="$private_root/hg1-missing-profile.json"
/usr/bin/jq 'del(.events[3].profile_id)' \
  "$hg1_bundle" > "$missing_profile_bundle"
chmod 600 "$missing_profile_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_profile_bundle" "$hg1_verifier")" = unavailable

drifted_profile_bundle="$private_root/hg1-drifted-profile.json"
/usr/bin/jq '.events[3].profile_id = "conversation-substituted"' \
  "$hg1_bundle" > "$drifted_profile_bundle"
chmod 600 "$drifted_profile_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$drifted_profile_bundle" "$hg1_verifier")" = unavailable

synthetic_native_authority_bundle="$private_root/hg1-synthetic-native-authority.json"
/usr/bin/jq --arg policy "$hg1_policy_digest" \
  '.events |= map(if .harness_id == "codex" then
    .provider_account_id = "openai.native" |
    .credential_revision = 1 |
    .governance_policy_digest = $policy
  else . end)' \
  "$hg1_bundle" > "$synthetic_native_authority_bundle"
chmod 600 "$synthetic_native_authority_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$synthetic_native_authority_bundle" "$hg1_verifier")" = unavailable

missing_brokered_authority_bundle="$private_root/hg1-missing-brokered-authority.json"
/usr/bin/jq \
  '.events |= map(if .session_id == "session-deepseek" then
    .provider_account_id = "" |
    .credential_revision = 0 |
    .governance_policy_digest = ""
  else . end)' \
  "$hg1_bundle" > "$missing_brokered_authority_bundle"
chmod 600 "$missing_brokered_authority_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_brokered_authority_bundle" "$hg1_verifier")" = unavailable

absent_brokered_policy_bundle="$private_root/hg1-absent-brokered-policy.json"
/usr/bin/jq \
  '.events |= map(if .session_id == "session-deepseek" then
    .governance_policy_digest = ""
  else . end)' \
  "$hg1_bundle" > "$absent_brokered_policy_bundle"
chmod 600 "$absent_brokered_policy_bundle"
test "$(printf '%s' "$(phase2d_acceptance_hg1_trace_summary \
  "$absent_brokered_policy_bundle" "$hg1_verifier")" |
  /usr/bin/jq -r '.matrix')" = pass

missing_route_review_bundle="$private_root/hg1-missing-route-review.json"
/usr/bin/jq \
  '.events |= map(if .session_id == "session-deepseek" then
    del(.route_transition_review_digest)
  else . end)' \
  "$hg1_bundle" > "$missing_route_review_bundle"
chmod 600 "$missing_route_review_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_route_review_bundle" "$hg1_verifier")" = unavailable

invalid_route_review_bundle="$private_root/hg1-invalid-route-review.json"
/usr/bin/jq \
  '.events |= map(if .session_id == "session-deepseek" then
    .route_transition_review_digest = "invalid"
  else . end)' \
  "$hg1_bundle" > "$invalid_route_review_bundle"
chmod 600 "$invalid_route_review_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$invalid_route_review_bundle" "$hg1_verifier")" = unavailable

target_opened_before_source_completion_bundle="$private_root/hg1-target-before-source-completion.json"
/usr/bin/jq \
  '.events |= map(
    if .gateway_event_sequence == 3 then .gateway_event_sequence = 5
    elif .gateway_event_sequence == 4 then .gateway_event_sequence = 6
    elif .gateway_event_sequence == 5 then .gateway_event_sequence = 7
    elif .gateway_event_sequence == 6 then .gateway_event_sequence = 8
    elif .gateway_event_sequence == 7 then .gateway_event_sequence = 3
    elif .gateway_event_sequence == 8 then .gateway_event_sequence = 4
    else . end)' \
  "$hg1_bundle" > "$target_opened_before_source_completion_bundle"
chmod 600 "$target_opened_before_source_completion_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$target_opened_before_source_completion_bundle" "$hg1_verifier")" = unavailable

drifted_route_review_bundle="$private_root/hg1-drifted-route-review.json"
/usr/bin/jq --arg review "$hg1_digest" \
  '.events |= map(if .gateway_event_sequence == 10 then
    .route_transition_review_digest = $review
  else . end)' \
  "$hg1_bundle" > "$drifted_route_review_bundle"
chmod 600 "$drifted_route_review_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$drifted_route_review_bundle" "$hg1_verifier")" = unavailable

missing_second_deepseek_turn_bundle="$private_root/hg1-missing-second-deepseek-turn.json"
/usr/bin/jq \
  '.events |= map(select(.gateway_event_sequence != 25 and .gateway_event_sequence != 26))' \
  "$hg1_bundle" > "$missing_second_deepseek_turn_bundle"
chmod 600 "$missing_second_deepseek_turn_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_second_deepseek_turn_bundle" "$hg1_verifier")" = unavailable

for field_and_value in \
  'provider_account_id=deepseek.secondary' \
  'credential_revision=4' \
  'model_id=deepseek-reasoner'
do
  field=${field_and_value%%=*}
  value=${field_and_value#*=}
  drifted_target_authority_bundle="$private_root/hg1-drifted-target-$field.json"
  /usr/bin/jq --arg field "$field" --arg value "$value" \
    '.events |= map(if .gateway_event_sequence == 26 then
      .[$field] = (if $field == "credential_revision" then ($value | tonumber) else $value end)
    else . end)' \
    "$hg1_bundle" > "$drifted_target_authority_bundle"
  chmod 600 "$drifted_target_authority_bundle"
  test "$(phase2d_acceptance_hg1_trace_summary \
    "$drifted_target_authority_bundle" "$hg1_verifier")" = unavailable
done

non_codex_cancellation_bundle="$private_root/hg1-non-codex-cancellation.json"
/usr/bin/jq \
  '.events[6] as $session |
  .events |= map(if
    (.gateway_event_sequence == 19 or .gateway_event_sequence == 21 or
      .gateway_event_sequence == 23 or .gateway_event_sequence == 24)
    then . + ($session | {
      session_id, harness_id, backend_id, gateway_configured_harness_version,
      gateway_backend_version, thread_id, segment_id, workspace_id,
      workspace_digest, execution_binding_digest, provider_id,
      provider_account_id, credential_revision, model_id,
      segment_context_capsule_digest, governance_policy_digest,
      route_transition_review_digest
    }) else . end)' \
  "$hg1_bundle" > "$non_codex_cancellation_bundle"
chmod 600 "$non_codex_cancellation_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$non_codex_cancellation_bundle" "$hg1_verifier")" = unavailable

failed_peer_bundle="$private_root/hg1-failed-cancellation-peer.json"
/usr/bin/jq \
  '.events |= map(if .gateway_event_sequence == 22 then
    .gateway_event_type = "response_failed" |
    .result = "failed" |
    .error_code = "provider_unavailable" |
    .retryable = true
  else . end)' \
  "$hg1_bundle" > "$failed_peer_bundle"
chmod 600 "$failed_peer_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$failed_peer_bundle" "$hg1_verifier")" = unavailable

pre_cancel_recovery_bundle="$private_root/hg1-pre-cancel-recovery-start.json"
/usr/bin/jq \
  '.events |= map(if .gateway_event_sequence == 18 then
    .gateway_event_sequence = 23
  elif .gateway_event_sequence == 23 then
    .gateway_event_sequence = 18
  else . end)' \
  "$hg1_bundle" > "$pre_cancel_recovery_bundle"
chmod 600 "$pre_cancel_recovery_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$pre_cancel_recovery_bundle" "$hg1_verifier")" = unavailable

overlapping_session_bundle="$private_root/hg1-overlapping-session-responses.json"
/usr/bin/jq \
  '.events |= map(if .gateway_event_sequence == 4 then
    .gateway_event_sequence = 5
  elif .gateway_event_sequence == 5 then
    .gateway_event_sequence = 4
  else . end)' \
  "$hg1_bundle" > "$overlapping_session_bundle"
chmod 600 "$overlapping_session_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$overlapping_session_bundle" "$hg1_verifier")" = unavailable

reused_capsule_bundle="$private_root/hg1-reused-attempt-capsule.json"
/usr/bin/jq --arg capsule "$hg1_digest" \
  '.events |= map(if
    (.gateway_event_sequence == 5 or .gateway_event_sequence == 6)
    then .context_capsule_digest = $capsule else . end)' \
  "$hg1_bundle" > "$reused_capsule_bundle"
chmod 600 "$reused_capsule_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$reused_capsule_bundle" "$hg1_verifier")" = unavailable

missing_opening_bundle="$private_root/hg1-missing-peer-session-opening.json"
/usr/bin/jq \
  '.events |= map(select(.gateway_event_sequence != 13))' \
  "$hg1_bundle" > "$missing_opening_bundle"
chmod 600 "$missing_opening_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_opening_bundle" "$hg1_verifier")" = unavailable

missing_ready_bundle="$private_root/hg1-missing-session-ready.json"
/usr/bin/jq \
  '.events |= map(select(.gateway_event_sequence != 2))' \
  "$hg1_bundle" > "$missing_ready_bundle"
chmod 600 "$missing_ready_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$missing_ready_bundle" "$hg1_verifier")" = unavailable

misordered_ready_bundle="$private_root/hg1-misordered-session-ready.json"
/usr/bin/jq \
  '.events |= map(if .gateway_event_sequence == 2 then
    .gateway_event_sequence = 3
  elif .gateway_event_sequence == 3 then
    .gateway_event_sequence = 2
  else . end)' \
  "$hg1_bundle" > "$misordered_ready_bundle"
chmod 600 "$misordered_ready_bundle"
test "$(phase2d_acceptance_hg1_trace_summary \
  "$misordered_ready_bundle" "$hg1_verifier")" = unavailable

semantic_bundle="$private_root/hg1-invalid-semantics.json"
/usr/bin/jq '.events[20].gateway_event_type = "response_completed"' \
  "$hg1_bundle" > "$semantic_bundle"
chmod 600 "$semantic_bundle"
test "$(phase2d_acceptance_hg1_trace_summary "$semantic_bundle" "$hg1_verifier")" = unavailable

split_instance_bundle="$private_root/hg1-split-instance.json"
/usr/bin/jq '.events |= map(if .gateway_event_sequence > 6 then
    .gateway_instance_id = "gateway-fixture-split" else . end)' \
  "$hg1_bundle" > "$split_instance_bundle"
chmod 600 "$split_instance_bundle"
test "$(phase2d_acceptance_hg1_trace_summary "$split_instance_bundle" "$hg1_verifier")" = unavailable

duplicate_sequence_bundle="$private_root/hg1-duplicate-sequence.json"
/usr/bin/jq '.events[3].gateway_event_sequence = 3' \
  "$hg1_bundle" > "$duplicate_sequence_bundle"
chmod 600 "$duplicate_sequence_bundle"
test "$(phase2d_acceptance_hg1_trace_summary "$duplicate_sequence_bundle" "$hg1_verifier")" = unavailable

incomplete_response_bundle="$private_root/hg1-incomplete-response.json"
/usr/bin/jq '.events |= map(select(.gateway_event_sequence != 4))' \
  "$hg1_bundle" > "$incomplete_response_bundle"
chmod 600 "$incomplete_response_bundle"
test "$(phase2d_acceptance_hg1_trace_summary "$incomplete_response_bundle" "$hg1_verifier")" = unavailable

multi_instance_bundle="$private_root/hg1-multi-instance.json"
/usr/bin/jq '.events += [(.events[0] |
    .gateway_instance_id = "gateway-fixture-incomplete" |
    .gateway_event_sequence = 1 |
    .occurred_at = "2026-08-25T12:00:00Z" |
    .session_id = "session-incomplete" |
    .thread_id = "conversation-incomplete" |
    .segment_id = "segment-incomplete")]' \
  "$hg1_bundle" > "$multi_instance_bundle"
chmod 600 "$multi_instance_bundle"
multi_summary=$(phase2d_acceptance_hg1_trace_summary \
  "$multi_instance_bundle" "$hg1_verifier")
test "$(printf '%s' "$multi_summary" | /usr/bin/jq -r '.gateway_instance_id')" = "$hg1_instance"

mixed_version_bundle="$private_root/hg1-mixed-version.json"
/usr/bin/jq '.events += [(.events[0] |
    .gateway_event_schema_version = 2 |
    del(.gateway_instance_id))]' \
  "$hg1_bundle" > "$mixed_version_bundle"
chmod 600 "$mixed_version_bundle"
mixed_summary=$(phase2d_acceptance_hg1_trace_summary \
  "$mixed_version_bundle" "$hg1_verifier")
test "$(printf '%s' "$mixed_summary" | /usr/bin/jq -r '.gateway_instance_id')" = "$hg1_instance"

document=$(phase2d_acceptance_export_evidence \
  "$fake_app" \
  "$private_root/evidence" \
  G1 \
  "incident-0001")
[ -f "$document" ] || {
  echo "RED: evidence file not written" >&2
  exit 1
}
permissions=$(/usr/bin/stat -f '%Lp' "$document")
[ "$permissions" = "600" ] || {
  echo "RED: evidence file not owner-only: $permissions" >&2
  exit 1
}
grep -q "incident_id: incident-0001" "$document" || {
  echo "RED: evidence incident not recorded" >&2
  exit 1
}
grep -q "bundle_build: 124" "$document" || {
  echo "RED: evidence build identity not recorded" >&2
  exit 1
}
grep -Eq 'app_sha256: [0-9a-f]{64}' "$document" || {
  echo "RED: evidence App hash not recorded" >&2
  exit 1
}
grep -Eq 'daemon_sha256: [0-9a-f]{64}' "$document" || {
  echo "RED: evidence daemon hash not recorded" >&2
  exit 1
}
if grep -Fq -- "$fake_app" "$document"; then
  echo "RED: evidence exposes the full local App path" >&2
  exit 1
fi

# A copied gate document must not satisfy a different gate.
cp "$document" "$private_root/evidence/G2-renamed-wrong-gate.md"
set -- $(phase2d_acceptance_bundle_identity "$fake_app")
phase2d_acceptance_gate_has_identity G1 "$private_root/evidence" "$1" "$2" "$3"
if phase2d_acceptance_gate_has_identity G2 "$private_root/evidence" "$1" "$2" "$3"; then
  echo "RED: renamed G1 evidence satisfied G2" >&2
  exit 1
fi

# Export must reject symlink roots and destinations without modifying targets.
mkdir -m 700 "$private_root/real-evidence"
ln -s "$private_root/real-evidence" "$private_root/linked-evidence"
test "$(phase2d_acceptance_export_evidence \
  "$fake_app" "$private_root/linked-evidence" G2 incident-root-link)" = "unavailable"
sentinel="$private_root/sentinel"
printf '%s\n' unchanged > "$sentinel"
ln -s "$sentinel" "$private_root/evidence/G2-incident-file-link.md"
test "$(phase2d_acceptance_export_evidence \
  "$fake_app" "$private_root/evidence" G2 incident-file-link)" = "unavailable"
test "$(cat "$sentinel")" = "unchanged"
chmod 755 "$private_root/evidence"
test "$(phase2d_acceptance_export_evidence \
  "$fake_app" "$private_root/evidence" G2 incident-public-root)" = "unavailable"
chmod 700 "$private_root/evidence"

# Changing either executable must make the old evidence stale.
printf '%s\n' app-build-125 > "$fake_app/Contents/MacOS/LoomLocalApp"
set -- $(phase2d_acceptance_bundle_identity "$fake_app")
if phase2d_acceptance_gate_has_identity G1 "$private_root/evidence" "$1" "$2" "$3"; then
  echo "RED: stale gate evidence satisfied a different bundle" >&2
  exit 1
fi

test "$(phase2d_acceptance_export_evidence \
  "$private_root" "$private_root/evidence" G9 "incident-0002")" = "invalid_gate"
test "$(phase2d_acceptance_export_evidence \
  "$private_root" "$private_root/evidence" G1 "bad incident!")" = "invalid_incident"

echo "phase2d acceptance harness fixture PASS"
