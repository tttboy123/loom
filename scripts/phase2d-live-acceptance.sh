#!/bin/sh
#
# Phase 2D installed-live acceptance harness (source library).
#
# Safe by construction: this library never reads, writes or transmits
# credentials, secrets, prompts or Provider bodies. It checks prerequisites and
# exports bounded privacy-safe evidence. The live gates themselves are executed
# by the operator in the App; this harness only validates readiness and
# collects the evidence each gate records.
#
# Source-only mode: set LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1 to load the
# functions without any filesystem side effects (used by the source test).
set -eu

phase2d_acceptance_gates="G1 G2 G3 G4 G5 G6 G7"
phase2d_acceptance_active_gates="G7"

# phase2d_acceptance_installed_app <path-to-Loom.app>
# Prints ok / missing / invalid. Never inspects bundle contents for secrets.
phase2d_acceptance_installed_app() {
  [ "$#" -eq 1 ] || { printf '%s\n' invalid; return 1; }
  app=$1
  canonical_app="${HOME:-}/Applications/Loom.app"
  [ -n "${HOME:-}" ] && [ "$app" = "$canonical_app" ] || {
    printf '%s\n' invalid
    return 1
  }
  [ -d "$app" ] && [ ! -L "$app" ] || { printf '%s\n' missing; return 0; }
  executable="$app/Contents/MacOS/LoomLocalApp"
  [ -f "$executable" ] && [ ! -L "$executable" ] && [ -x "$executable" ] || {
    printf '%s\n' missing; return 0
  }
  helper="$app/Contents/Library/Helpers/loomd"
  [ -f "$helper" ] && [ ! -L "$helper" ] && [ -x "$helper" ] || {
    printf '%s\n' missing_helper; return 0
  }
  printf '%s\n' ok
}

phase2d_acceptance_private_directory() {
  [ "$#" -eq 1 ] || return 1
  directory=$1
  [ -d "$directory" ] && [ ! -L "$directory" ] || return 1
  identity=$(/usr/bin/stat -f '%u %Lp' "$directory" 2>/dev/null) || return 1
  set -- $identity
  [ "$1" = "$(/usr/bin/id -u)" ] && [ "$2" = 700 ]
}

phase2d_acceptance_private_file() {
  [ "$#" -eq 1 ] || return 1
  file=$1
  [ -f "$file" ] && [ ! -L "$file" ] || return 1
  identity=$(/usr/bin/stat -f '%u %Lp %l' "$file" 2>/dev/null) || return 1
  set -- $identity
  [ "$1" = "$(/usr/bin/id -u)" ] && [ "$2" = 600 ] && [ "$3" = 1 ]
}

# phase2d_acceptance_bundle_identity <path-to-Loom.app>
# Prints: "<build> <app-sha256> <daemon-sha256>". The identity contains no
# user data and prevents evidence from a previous bundle satisfying a new gate.
phase2d_acceptance_bundle_identity() {
  [ "$#" -eq 1 ] || return 1
  app=$1
  [ "$(phase2d_acceptance_installed_app "$app")" = ok ] || return 1
  plist="$app/Contents/Info.plist"
  executable="$app/Contents/MacOS/LoomLocalApp"
  helper="$app/Contents/Library/Helpers/loomd"
  [ -f "$plist" ] && [ ! -L "$plist" ] || return 1
  build=$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$plist" 2>/dev/null) || return 1
  case "$build" in
    ''|*[!0-9]*) return 1 ;;
  esac
  app_sha=$(/usr/bin/shasum -a 256 "$executable" | /usr/bin/awk '{print $1}') || return 1
  daemon_sha=$(/usr/bin/shasum -a 256 "$helper" | /usr/bin/awk '{print $1}') || return 1
  [ "${#app_sha}" -eq 64 ] && [ "${#daemon_sha}" -eq 64 ] || return 1
  case "$app_sha$daemon_sha" in
    *[!0-9a-f]*) return 1 ;;
  esac
  printf '%s %s %s\n' "$build" "$app_sha" "$daemon_sha"
}

phase2d_acceptance_valid_hg1_summary() {
  [ "$#" -eq 1 ] || return 1
  printf '%s' "$1" | /usr/bin/jq -e '
    def exact_keys($allowed):
      type == "object" and (keys_unsorted | sort) == ($allowed | sort);
    def identifier:
      type == "string" and test("^[A-Za-z0-9._:/-]{1,255}$");
    def digest:
      type == "string" and test("^[0-9a-f]{64}$");
    def positive_integer:
      type == "number" and floor == . and . > 0;
    . as $summary |
    exact_keys([
      "schema_version", "matrix", "gateway_instance_id", "completed_at",
      "route", "overlap", "cancellation"
    ]) and
    .schema_version == 1 and .matrix == "pass" and
    (.gateway_instance_id | identifier) and
    (.completed_at | type == "string" and length > 0 and length <= 40 and
      test("^[0-9T:.Z+\\-]+$")) and
    (.route | exact_keys(["source", "target"])) and
    (.route.source | exact_keys([
      "conversation_id", "segment_id", "session_id", "workspace_id",
      "workspace_digest", "response_ids", "completed_sequences"
    ])) and
    (.route.target | exact_keys([
      "conversation_id", "segment_id", "session_id", "workspace_id",
      "workspace_digest", "provider_account_id", "credential_revision",
      "model_id", "route_transition_review_digest", "response_ids",
      "opening_sequence"
    ])) and
    all([
      .route.source.conversation_id, .route.source.segment_id,
      .route.source.session_id, .route.source.workspace_id,
      .route.target.conversation_id, .route.target.segment_id,
      .route.target.session_id, .route.target.workspace_id,
      .route.target.provider_account_id, .route.target.model_id
    ][]; identifier) and
    (.route.source.workspace_digest | digest) and
    (.route.target.workspace_digest | digest) and
    (.route.target.route_transition_review_digest | digest) and
    (.route.target.credential_revision | positive_integer) and
    (.route.target.opening_sequence | positive_integer) and
    (.route.source.response_ids | type == "array" and length >= 2 and
      length == (unique | length) and all(.[]; identifier)) and
    (.route.source.completed_sequences | type == "array" and length >= 2 and
      length == (unique | length) and all(.[]; positive_integer)) and
    (.route.source.response_ids | length) ==
      (.route.source.completed_sequences | length) and
    (.route.target.response_ids | type == "array" and length >= 2 and
      length == (unique | length) and all(.[]; identifier)) and
    .route.source.conversation_id == .route.target.conversation_id and
    .route.source.segment_id != .route.target.segment_id and
    .route.source.session_id != .route.target.session_id and
    .route.source.workspace_id == .route.target.workspace_id and
    .route.source.workspace_digest == .route.target.workspace_digest and
    ([ .route.source.completed_sequences[] |
      select(. < $summary.route.target.opening_sequence) ] | length) >= 2 and
    (.overlap | exact_keys([
      "first_response_id", "second_response_id", "first_conversation_id",
      "second_conversation_id"
    ])) and
    all([
      .overlap.first_response_id, .overlap.second_response_id,
      .overlap.first_conversation_id, .overlap.second_conversation_id
    ][]; identifier) and
    .overlap.first_response_id != .overlap.second_response_id and
    .overlap.first_conversation_id != .overlap.second_conversation_id and
    (.cancellation | exact_keys([
      "cancelled_response_id", "cancelled_incident_id", "session_id",
      "peer_response_id", "post_cancel_response_id"
    ])) and
    all([
      .cancellation.cancelled_response_id,
      .cancellation.cancelled_incident_id,
      .cancellation.session_id,
      .cancellation.peer_response_id,
      .cancellation.post_cancel_response_id
    ][]; identifier) and
    .cancellation.cancelled_response_id != .cancellation.peer_response_id and
    .cancellation.cancelled_response_id != .cancellation.post_cancel_response_id and
    .cancellation.peer_response_id != .cancellation.post_cancel_response_id
  ' >/dev/null 2>&1
}

phase2d_acceptance_gate_has_identity() {
  [ "$#" -eq 5 ] || return 1
  gate=$1
  root=$2
  build=$3
  app_sha=$4
  daemon_sha=$5
  for document in "$root/$gate"-*.md; do
    phase2d_acceptance_private_file "$document" || continue
    grep -Fqx -- "- gate: $gate" "$document" &&
      grep -Fqx -- "- bundle_build: $build" "$document" &&
      grep -Fqx -- "- app_sha256: $app_sha" "$document" &&
      grep -Fqx -- "- daemon_sha256: $daemon_sha" "$document" || continue
    if [ "$gate" = G7 ]; then
      grep -Fqx -- "- hg1_matrix: pass" "$document" &&
        grep -Eq '^- hg1_trace_sha256: [0-9a-f]{64}$' "$document" || continue
      hg1_summary=$(/usr/bin/sed -n 's/^- hg1_summary: //p' "$document")
      [ -n "$hg1_summary" ] &&
        [ "$(printf '%s' "$hg1_summary" | wc -l | tr -d ' ')" -eq 0 ] &&
        phase2d_acceptance_valid_hg1_summary "$hg1_summary" || continue
    fi
    return 0
  done
  return 1
}

# phase2d_acceptance_required_gate_documents <app> <evidence-root>
# Prints gates missing evidence for this exact bundle identity, or "ok".
phase2d_acceptance_required_gate_documents() {
  [ "$#" -eq 2 ] || { printf '%s\n' invalid; return 1; }
  app=$1
  root=$2
  phase2d_acceptance_private_directory "$root" || {
    printf '%s\n' "$phase2d_acceptance_active_gates"
    return 0
  }
  identity=$(phase2d_acceptance_bundle_identity "$app" 2>/dev/null) || identity=""
  [ -n "$identity" ] || { printf '%s\n' "$phase2d_acceptance_active_gates"; return 0; }
  set -- $identity
  build=$1
  app_sha=$2
  daemon_sha=$3
  missing=""
  for gate in $phase2d_acceptance_active_gates; do
    phase2d_acceptance_gate_has_identity \
      "$gate" "$root" "$build" "$app_sha" "$daemon_sha" || {
      missing="$missing $gate"
    }
  done
  if [ -z "$missing" ]; then
    printf '%s\n' ok
  else
    printf '%s\n' "${missing# }"
  fi
}

# phase2d_acceptance_prerequisite_summary <app> <evidence-root>
# Prints one line per check: "PASS <check>" or "OPEN <check>".
phase2d_acceptance_prerequisite_summary() {
  [ "$#" -eq 2 ] || { printf '%s\n' "OPEN usage"; return 1; }
  app=$1
  root=$2
  case "$(phase2d_acceptance_installed_app "$app")" in
    ok) printf '%s\n' "PASS installed_app" ;;
    *) printf '%s\n' "OPEN installed_app" ;;
  esac
  missing=$(phase2d_acceptance_required_gate_documents "$app" "$root")
  if [ "$missing" = "ok" ]; then
    printf '%s\n' "PASS gate_evidence_present"
  else
    # shellcheck disable=SC2086
    for gate in $missing; do
      printf '%s\n' "OPEN gate_evidence_$gate"
    done
  fi
  printf '%s\n' "PASS no_credential_required_for_harness"
}

# Internal owner-only atomic evidence writer. The HG1 summary is already
# validated, compact metadata and never contains conversation or Provider body.
phase2d_acceptance_write_evidence() {
  [ "$#" -eq 6 ] || { printf '%s\n' invalid; return 1; }
  app=$1
  root=$2
  gate=$3
  incident=$4
  hg1_summary=$5
  hg1_trace_sha=$6
  case " $phase2d_acceptance_gates " in
    *" $gate "*) ;;
    *) printf '%s\n' invalid_gate; return 1 ;;
  esac
  case "$incident" in
    ''|*[!A-Za-z0-9._-]*) printf '%s\n' invalid_incident; return 1 ;;
  esac
  if [ ! -e "$root" ] && [ ! -L "$root" ]; then
    mkdir -m 700 -p "$root" 2>/dev/null || {
      printf '%s\n' unavailable
      return 0
    }
  fi
  phase2d_acceptance_private_directory "$root" || {
    printf '%s\n' unavailable; return 0
  }
  identity=$(phase2d_acceptance_bundle_identity "$app" 2>/dev/null) || {
    printf '%s\n' unavailable
    return 0
  }
  set -- $identity
  build=$1
  app_sha=$2
  daemon_sha=$3
  document="$root/$gate-$incident.md"
  [ ! -e "$document" ] && [ ! -L "$document" ] || {
    printf '%s\n' unavailable
    return 0
  }
  app_result=$(phase2d_acceptance_installed_app "$app")
  now=$(/bin/date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || printf '%s' unknown)
  umask 077
  temporary=$(/usr/bin/mktemp "$root/.phase2d-evidence.XXXXXX" 2>/dev/null) || {
    printf '%s\n' unavailable
    return 0
  }
  chmod 600 "$temporary" 2>/dev/null || {
    rm -f "$temporary"
    printf '%s\n' unavailable
    return 0
  }
  {
    printf '%s\n' "# Phase 2D gate evidence $gate"
    printf '%s\n' "- incident_id: $incident"
    printf '%s\n' "- exported_at: $now"
    printf '%s\n' "- installed_app: Loom.app"
    printf '%s\n' "- app_check: $app_result"
    printf '%s\n' "- bundle_build: $build"
    printf '%s\n' "- app_sha256: $app_sha"
    printf '%s\n' "- daemon_sha256: $daemon_sha"
    printf '%s\n' "- gate: $gate"
    if [ "$gate" = G7 ]; then
      printf '%s\n' "- hg1_matrix: pass"
      printf '%s\n' "- hg1_trace_sha256: $hg1_trace_sha"
      printf '%s\n' "- hg1_summary: $hg1_summary"
    fi
    printf '%s\n'
    printf '%s\n' "This evidence is bounded and privacy-safe. It contains no"
    printf '%s\n' "credential, secret, Prompt, conversation content or Provider"
    printf '%s\n' "body. Full pass criteria are in"
    printf '%s\n' ".loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md."
  } > "$temporary" || {
    rm -f "$temporary"
    printf '%s\n' unavailable
    return 0
  }
  phase2d_acceptance_private_file "$temporary" || {
    rm -f "$temporary"
    printf '%s\n' unavailable
    return 0
  }
  /bin/ln "$temporary" "$document" 2>/dev/null || {
    rm -f "$temporary"
    printf '%s\n' unavailable
    return 0
  }
  rm -f "$temporary"
  phase2d_acceptance_private_file "$document" || {
    rm -f "$document"
    printf '%s\n' unavailable
    return 0
  }
  printf '%s\n' "$document"
}

# phase2d_acceptance_export_evidence <app> <evidence-root> <gate> <incident-id>
# Writes generic G1-G6 evidence. G7 requires a validated HG1 event trace and
# cannot be satisfied by a generic bundle-identity stamp.
phase2d_acceptance_export_evidence() {
  [ "$#" -eq 4 ] || { printf '%s\n' invalid; return 1; }
  [ "$3" != G7 ] || { printf '%s\n' requires_hg1_trace; return 1; }
  phase2d_acceptance_write_evidence "$1" "$2" "$3" "$4" "" ""
}

# phase2d_acceptance_hg1_trace_summary <diagnostic-bundle-or-operational-jsonl> <jq-verifier>
# Prints one compact privacy-safe JSON summary only when the complete ADR-0022
# matrix is present and every Session preserves exact frozen authority.
phase2d_acceptance_hg1_trace_summary() {
  [ "$#" -eq 2 ] || { printf '%s\n' invalid; return 1; }
  trace=$1
  verifier=$2
  phase2d_acceptance_private_file "$trace" || { printf '%s\n' unavailable; return 1; }
  [ -f "$verifier" ] && [ ! -L "$verifier" ] || {
    printf '%s\n' unavailable
    return 1
  }
  size=$(/usr/bin/stat -f '%z' "$trace" 2>/dev/null) || {
    printf '%s\n' unavailable
    return 1
  }
  [ "$size" -gt 0 ] && [ "$size" -le 4194304 ] || {
    printf '%s\n' unavailable
    return 1
  }
  /usr/bin/jq -c -s -f "$verifier" "$trace" 2>/dev/null || {
    printf '%s\n' unavailable
    return 1
  }
}

# Installed G7 evidence must come from the App's diagnostic export. Raw daemon
# JSONL remains useful for source verification, but it has no bundle identity
# and therefore cannot be combined with an arbitrary installed App stamp.
phase2d_acceptance_hg1_bundle_matches_app() {
  [ "$#" -eq 2 ] || return 1
  bundle=$2
  identity=$(phase2d_acceptance_bundle_identity "$1" 2>/dev/null) || return 1
  set -- $identity
  build=$1
  app_sha=$2
  daemon_sha=$3
  /usr/bin/jq -e \
    --arg build "$build" --arg app_sha "$app_sha" --arg daemon_sha "$daemon_sha" \
    '
     def exact_keys($allowed):
       type == "object" and (keys_unsorted | sort) == ($allowed | sort);
     def safe_token($maximum):
       type == "string" and length > 0 and length <= $maximum and
       test("^[A-Za-z0-9._:/@+\\-]+$");
     def optional_token($maximum):
       type == "string" and (. == "" or safe_token($maximum));
     def nonnegative_integer:
       type == "number" and floor == . and . >= 0;
     def binary_summary:
       exact_keys(["version", "build", "sha256"]) and
       (.version | safe_token(64)) and (.build | safe_token(64)) and
       (.sha256 | type == "string" and test("^[0-9a-f]{64}$"));
     def provider_state:
       exact_keys(["provider_id", "auth_mode", "revision", "status", "reason"]) and
       (.provider_id | safe_token(64)) and (.auth_mode | safe_token(64)) and
       (.revision | nonnegative_integer) and (.status | safe_token(64)) and
       (.reason | optional_token(64));
     def profile_state:
       exact_keys(["profile_id", "provider_id", "model_id", "auth_mode",
                   "credential_revision"]) and
       (.profile_id | safe_token(128)) and (.provider_id | safe_token(64)) and
       (.model_id | safe_token(128)) and (.auth_mode | safe_token(64)) and
       (.credential_revision | nonnegative_integer);
     def agent_state:
       exact_keys(["agent_instance_id", "status", "harness_adapter", "provider_id",
                   "provider_account_id", "model_id", "reasoning_effort",
                   "credential_revision", "terminal_reason"]) and
       (.agent_instance_id | safe_token(128)) and (.status | safe_token(64)) and
       (.harness_adapter | optional_token(64)) and
       (.provider_id | optional_token(64)) and
       (.provider_account_id | optional_token(128)) and
       (.model_id | optional_token(128)) and
       (.reasoning_effort | optional_token(32)) and
       (.credential_revision | nonnegative_integer) and
       (.terminal_reason | optional_token(64));
     exact_keys([
       "schema_version", "generated_at", "app", "daemon", "socket_healthy",
       "console", "providers", "profiles", "agents", "events",
       "diagnostic_files_skipped", "excluded"
     ]) and
     .schema_version == 1 and
     (.generated_at | type == "string" and length > 0 and length <= 40 and
       test("^[0-9T:.Z+\\-]+$")) and
     (.app | binary_summary) and (.daemon | binary_summary) and
     .app.version == .daemon.version and
     .app.build == $build and .app.sha256 == $app_sha and
     .daemon.build == $build and .daemon.sha256 == $daemon_sha and
     (.socket_healthy | type) == "boolean" and
     (.console | exact_keys(["available", "byte_count"]) and
       (.available | type) == "boolean" and (.byte_count | nonnegative_integer)) and
     (.providers | type == "array" and length <= 1024 and all(.[]; provider_state)) and
     (.profiles | type == "array" and length <= 4096 and all(.[]; profile_state)) and
     (.agents | type == "array" and length <= 4096 and all(.[]; agent_state)) and
     (.events | type == "array" and length <= 512 and
       all(.[]; (.source == "app" or .source == "daemon"))) and
     (.diagnostic_files_skipped | nonnegative_integer) and
     (.excluded | type == "array" and sort == ([
       ("api_" + "keys_and_credentials"), ("author" + "ization_headers"),
       "environment_credentials", "prompts_and_conversations",
       "provider_response_bodies", "raw_daemon_console"
     ] | sort))' \
    "$bundle" >/dev/null 2>&1
}

# phase2d_acceptance_export_hg1_evidence <app> <root> <trace> <jq-verifier>
phase2d_acceptance_export_hg1_evidence() {
  [ "$#" -eq 4 ] || { printf '%s\n' invalid; return 1; }
  phase2d_acceptance_hg1_bundle_matches_app "$1" "$3" || {
    printf '%s\n' unavailable
    return 1
  }
  summary=$(phase2d_acceptance_hg1_trace_summary "$3" "$4") || return 1
  [ -n "$summary" ] && [ "$(printf '%s' "$summary" | wc -l | tr -d ' ')" -eq 0 ] || {
    printf '%s\n' unavailable
    return 1
  }
  phase2d_acceptance_valid_hg1_summary "$summary" || {
    printf '%s\n' unavailable
    return 1
  }
  incident=$(printf '%s' "$summary" | /usr/bin/jq -r '.cancellation.cancelled_incident_id') || return 1
  case "$incident" in
    ''|*[!A-Za-z0-9._-]*) printf '%s\n' unavailable; return 1 ;;
  esac
  trace_sha=$(/usr/bin/shasum -a 256 "$3" | /usr/bin/awk '{print $1}') || return 1
  phase2d_acceptance_write_evidence "$1" "$2" G7 "$incident" "$summary" "$trace_sha"
}

# phase2d_acceptance_summary <app> <evidence-root>
# Prints the final PASS/OPEN summary for every gate.
phase2d_acceptance_summary() {
  phase2d_acceptance_prerequisite_summary "$1" "$2"
}

if [ "${LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY:-0}" = 1 ]; then
  # shellcheck disable=SC2034
  phase2d_acceptance_source_only=1
  return 0 2>/dev/null || exit 0
fi
