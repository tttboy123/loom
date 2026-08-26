def digest:
  type == "string" and test("^[0-9a-f]{64}$");

def identifier:
  type == "string" and test("^[A-Za-z0-9._-]{1,255}$");

def response_event:
  . == "response_started" or . == "response_completed" or
  . == "response_cancelled" or . == "response_failed";

def terminal_event:
  . == "response_completed" or . == "response_cancelled" or
  . == "response_failed";

def session_event:
  . == "session_opening" or . == "session_ready" or
  . == "session_closing" or . == "session_closed" or
  . == "session_failed";

def valid_outcome:
  if .gateway_event_type == "response_cancelled" then
    .result == "failed" and .error_code == "cancelled" and .retryable == false
  elif .gateway_event_type == "response_failed" then
    .result == "failed" and .error_code == "provider_unavailable" and .retryable == true
  elif .gateway_event_type == "session_failed" then
    .result == "failed" and .error_code == "conversation_unavailable" and .retryable == true
  else
    .result == "succeeded" and (.error_code // "") == "" and .retryable == false
  end;

def built_in_native_route:
  (.harness_id == "codex" and
    .backend_id == "backend.codex.app-server" and .provider_id == "openai") or
  (.harness_id == "claude-code" and
    .backend_id == "backend.segment.claude-code" and .provider_id == "anthropic") or
  (.harness_id == "opencode" and
    .backend_id == "backend.segment.opencode" and .provider_id == "opencode") or
  (.harness_id == "pi" and
    .backend_id == "backend.segment.pi" and .provider_id == "loom-local");

def valid_route_authority:
  if built_in_native_route then
    .provider_account_id == "" and .credential_revision == 0 and
    .governance_policy_digest == ""
  else
    (.provider_account_id | type) == "string" and .provider_account_id != "" and
    (.credential_revision | type) == "number" and .credential_revision > 0 and
    (.governance_policy_digest == "" or
      (.governance_policy_digest | digest))
  end;

def frozen_authority:
  [
    .gateway_instance_id,
    .harness_id,
    .backend_id,
    .gateway_configured_harness_version,
    .gateway_backend_version,
    .thread_id,
    .segment_id,
    .workspace_id,
    .workspace_digest,
    .execution_binding_digest,
    .profile_id,
    .provider_id,
    .provider_account_id,
    .credential_revision,
    .model_id,
    (.reasoning_effort // ""),
    .segment_context_capsule_digest,
    .governance_policy_digest,
    (.route_transition_review_digest // "")
  ];

def response_authority:
  [
    .gateway_instance_id,
    .session_id,
    .response_id,
    .incident_id,
    .context_capsule_digest,
    (.route_transition_review_digest // "")
  ];

def valid_accepted_session:
  . as $session
  | ([ $session[] | select(.gateway_event_type == "session_opening") |
      .gateway_event_sequence ]) as $openings
  | ([ $session[] | select(.gateway_event_type == "session_ready") |
      .gateway_event_sequence ]) as $ready
  | ([ $session[] | select(.gateway_event_type == "session_closing") |
      .gateway_event_sequence ]) as $closing
  | ([ $session[] | select(.gateway_event_type == "session_closed") |
      .gateway_event_sequence ]) as $closed
  | ([ $session[] | select(.gateway_event_type == "session_failed") |
      .gateway_event_sequence ]) as $failed
  | ([ $session[] | select(.gateway_event_type | response_event) |
      .gateway_event_sequence ]) as $response_sequences
  | ($openings | length) == 1 and ($ready | length) == 1 and
    $openings[0] < $ready[0] and
    all($session[]; .gateway_event_sequence >= $openings[0]) and
    all($response_sequences[]; . > $ready[0]) and
    ($closing | length) <= 1 and ($closed | length) <= 1 and
    ($failed | length) <= 1 and
    (($closed | length) + ($failed | length)) <= 1 and
    (if ($closing | length) == 1 then
       (($closed | length) + ($failed | length)) == 1 and
       all($response_sequences[]; . < $closing[0]) and
       $closing[0] < (($closed + $failed)[0])
     else
       ($closed | length) == 0 and
       (if ($failed | length) == 1 then
          all($response_sequences[]; . < $failed[0])
        else true end)
     end);

def response_start_sequence:
  [ .[] | select(.gateway_event_type == "response_started") |
    .gateway_event_sequence ] | first;

def response_terminal_sequence:
  [ .[] | select(.gateway_event_type | terminal_event) |
    .gateway_event_sequence ] | first;

def instance_matrix($events):
  if ([ $events[].gateway_event_sequence ] | unique | length) != ($events | length)
  then error("duplicate Gateway sequence") else . end
  | ($events | sort_by(.session_id) | group_by(.session_id)) as $sessions
  | if all($sessions[]; ([.[] | frozen_authority] | unique | length) == 1)
    then . else error("Session authority drift") end
  | (
      $events
      | map(select(.gateway_event_type | response_event))
      | sort_by(.session_id, .response_id)
      | group_by([.session_id, .response_id])
    ) as $responses
  | if all($responses[];
      length == 2 and
      ([.[] | select(.gateway_event_type == "response_started")] | length) == 1 and
      ([.[] | select(.gateway_event_type | terminal_event)] | length) == 1 and
      ([.[] | response_authority] | unique | length) == 1 and
      ([.[] | select(.gateway_event_type == "response_started") |
          .gateway_event_sequence] | first) <
        ([.[] | select(.gateway_event_type | terminal_event) |
          .gateway_event_sequence] | first))
    then . else error("invalid Response event lifecycle") end
  | [
      $responses[] as $first
      | $responses[] as $second
      | select($first[0].session_id == $second[0].session_id and
          $first[0].response_id != $second[0].response_id)
      | ($first | response_start_sequence) as $first_start
      | ($first | response_terminal_sequence) as $first_terminal
      | ($second | response_start_sequence) as $second_start
      | select($first_start < $second_start and $second_start < $first_terminal)
    ] as $same_session_overlap
  | if ($same_session_overlap | length) == 0
    then . else error("overlapping Responses in one Session") end
  | [
      $sessions[]
      | select(valid_accepted_session)
      | .[0].session_id
    ] as $accepted_session_ids
  | [
      $sessions[] as $session
      | select($session[0].harness_id == "codex")
      | select($session | valid_accepted_session)
      | ([ $session[] | select(.gateway_event_type == "response_completed") ]) as $completed
      | ([ $session[] | select(.gateway_event_type == "response_started") ]) as $attempts
      | select(($completed | length) >= 2 and
          ([ $attempts[].context_capsule_digest ] | unique | length) ==
            ($attempts | length))
      | {
          conversation_id: $session[0].thread_id,
          segment_id: $session[0].segment_id,
          session_id: $session[0].session_id,
          workspace_id: $session[0].workspace_id,
          workspace_digest: $session[0].workspace_digest,
          response_ids: [ $completed[].response_id ],
          completed_sequences: [ $completed[].gateway_event_sequence ]
        }
    ] as $codex_reuse
  | [
      $sessions[] as $session
      | select($session[0].harness_id == "loom-native" and
          $session[0].provider_id == "deepseek")
      | select($session | valid_accepted_session)
      | select(($session[0].route_transition_review_digest // "") | digest)
      | ([ $session[] | select(.gateway_event_type == "response_completed") ]) as $completed
      | ([ $session[] | select(.gateway_event_type == "response_started") ]) as $attempts
      | select(($completed | length) >= 2 and
          ([ $attempts[].context_capsule_digest ] | unique | length) ==
            ($attempts | length))
      | {
          conversation_id: $session[0].thread_id,
          segment_id: $session[0].segment_id,
          session_id: $session[0].session_id,
          workspace_id: $session[0].workspace_id,
          workspace_digest: $session[0].workspace_digest,
          provider_account_id: $session[0].provider_account_id,
          credential_revision: $session[0].credential_revision,
          model_id: $session[0].model_id,
          route_transition_review_digest: $session[0].route_transition_review_digest,
          response_ids: [ $completed[].response_id ],
          opening_sequence: ([ $session[] |
            select(.gateway_event_type == "session_opening") |
            .gateway_event_sequence ] | first)
        }
    ] as $deepseek_sessions
  | [
      $codex_reuse[] as $source
      | $deepseek_sessions[] as $target
      | select($source.conversation_id == $target.conversation_id and
          $source.segment_id != $target.segment_id and
          $source.session_id != $target.session_id and
          $source.workspace_id == $target.workspace_id and
          $source.workspace_digest == $target.workspace_digest and
          ([ $source.completed_sequences[] |
            select(. < $target.opening_sequence) ] | length) >= 2)
      | {source: $source, target: $target}
    ] as $routes
  | [
      $events[]
      | select(.gateway_event_type == "response_started")
      | .session_id as $session_id
      | select(($accepted_session_ids | index($session_id)) != null)
    ] as $starts
  | [ $events[] | select(.gateway_event_type | terminal_event) ] as $terminals
  | [
      $starts[] as $first
      | $starts[] as $second
      | select($first.gateway_event_sequence < $second.gateway_event_sequence and
          $first.thread_id != $second.thread_id)
      | ([ $terminals[] | select(.session_id == $first.session_id and
          .response_id == $first.response_id) ] | first) as $first_terminal
      | ([ $terminals[] | select(.session_id == $second.session_id and
          .response_id == $second.response_id) ] | first) as $second_terminal
      | select($first_terminal != null and $second_terminal != null and
          $first_terminal.gateway_event_sequence > $second.gateway_event_sequence)
      | {
          first_response_id: $first.response_id,
          second_response_id: $second.response_id,
          first_conversation_id: $first.thread_id,
          second_conversation_id: $second.thread_id
        }
    ] as $overlap
  | [
      $events[] as $cancelled
      | select($cancelled.gateway_event_type == "response_cancelled")
      | select($cancelled.harness_id == "codex")
      | select(($accepted_session_ids | index($cancelled.session_id)) != null)
      | select(([ $events[] | select(.session_id == $cancelled.session_id and
          .response_id == $cancelled.response_id and
          .gateway_event_type == "response_completed") ] | length) == 0)
      | ([ $events[] | select(.session_id == $cancelled.session_id and
          .response_id == $cancelled.response_id and
          .gateway_event_type == "response_started" and
          .gateway_event_sequence < $cancelled.gateway_event_sequence) ] | last) as $cancelled_start
      | ([ $events[] | select(.gateway_event_type == "response_started" and
          .thread_id != $cancelled.thread_id and
          .gateway_event_sequence < $cancelled.gateway_event_sequence) ] | last) as $peer_start
      | ([ $events[] | select($peer_start != null and
          .session_id == $peer_start.session_id and
          .response_id == $peer_start.response_id and
          .gateway_event_type == "response_completed" and
          .gateway_event_sequence > $cancelled.gateway_event_sequence) ] | first) as $peer_terminal
      | ([ $events[] | select(.session_id == $cancelled.session_id and
          .response_id != $cancelled.response_id and
          .gateway_event_type == "response_completed" and
          .gateway_event_sequence > $cancelled.gateway_event_sequence) ] | first) as $post_cancel
      | ([ $events[] | select($post_cancel != null and
          .session_id == $post_cancel.session_id and
          .response_id == $post_cancel.response_id and
          .gateway_event_type == "response_started" and
          .gateway_event_sequence > $cancelled.gateway_event_sequence and
          .gateway_event_sequence < $post_cancel.gateway_event_sequence) ] | first) as $post_cancel_start
      | select($cancelled_start != null and $peer_start != null and
          ($accepted_session_ids | index($peer_start.session_id)) != null and
          $peer_terminal != null and $post_cancel != null and
          $post_cancel_start != null)
      | {
          cancelled_response_id: $cancelled.response_id,
          cancelled_incident_id: $cancelled.incident_id,
          session_id: $cancelled.session_id,
          peer_response_id: $peer_start.response_id,
          post_cancel_response_id: $post_cancel.response_id
        }
    ] as $cancellation
  | if ($routes | length) == 0 then error("missing Codex to DeepSeek Segment transition")
    elif ($overlap | length) == 0 then error("missing cross-Conversation overlap")
    elif ($cancellation | length) == 0 then error("missing exact cancellation and reuse")
    else {
      schema_version: 1,
      matrix: "pass",
      gateway_instance_id: $events[0].gateway_instance_id,
      completed_at: ([ $events[].occurred_at ] | max),
      route: $routes[0],
      overlap: $overlap[0],
      cancellation: $cancellation[0]
    } end;

def allowed_gateway_keys:
  [
    "schema_version", "source", "occurred_at", "incident_id", "operation",
    "credential_runtime", "credential_helper_spawn_attempts",
    "provider_id", "provider_account_id", "credential_revision", "model_id",
    "reasoning_effort", "thread_id", "profile_id", "gateway_event_schema_version",
    "gateway_instance_id",
    "gateway_configured_harness_version", "gateway_backend_version",
    "gateway_event_sequence", "gateway_event_type", "session_id", "harness_id",
    "backend_id", "segment_id", "workspace_id", "workspace_digest",
    "segment_context_capsule_digest", "governance_policy_digest", "response_id",
    "execution_binding_digest", "route_transition_review_digest",
    "context_capsule_digest", "stage", "elapsed_ms",
    "result", "error_code", "retryable"
  ];

def allowed_bundle_keys:
  [
    "schema_version", "generated_at", "app", "daemon", "socket_healthy",
    "console", "providers", "profiles", "agents", "events",
    "diagnostic_files_skipped", "excluded"
  ];

def forbidden_trace_key:
  . == "content" or . == "prompt" or . == "provider_body" or
  . == "authorization" or . == "credential_reference" or
  . == "workspace_path";

. as $documents
| if all($documents[];
    type == "object" and
    ((has("events") and (.events | type) == "array") or
      (has("operation") and (has("events") | not))))
  then . else error("invalid HG1 trace container") end
| if all($documents[];
    if has("events") then
      ((keys_unsorted - allowed_bundle_keys) | length) == 0
    else
      true
    end)
  then . else error("unknown HG1 bundle field") end
| [
    $documents[]
    | if type == "object" and (.events | type?) == "array" then
        .events[]
      elif type == "object" then
        .
      else
        empty
      end
  ] as $records
| ([
    $documents[]
    | .. | objects | keys_unsorted[]
    | select(forbidden_trace_key)
  ] | length) as $forbidden
| if $forbidden != 0 then error("content-bearing HG1 trace") else . end
| [
    $records[]
    | select(.operation == "chat_message" and
        (.gateway_event_type // "") != "" and
        .gateway_event_schema_version == 3)
  ] | sort_by(.gateway_event_sequence) as $events
| if ($events | length) == 0 then error("missing HG1 events") else . end
| if all($events[]; ((keys_unsorted - allowed_gateway_keys) | length) == 0)
  then . else error("unknown HG1 event field") end
| if all($events[];
    .gateway_event_schema_version == 3 and
    ((.source // "daemon") == "daemon") and
    (.occurred_at | type) == "string" and .occurred_at != "" and
    (.gateway_instance_id | identifier) and
    (.gateway_configured_harness_version | type) == "number" and
    .gateway_configured_harness_version > 0 and
    (.gateway_backend_version | type) == "number" and
    .gateway_backend_version > 0 and
    (.gateway_event_sequence | type) == "number" and
    .gateway_event_sequence > 0 and
    ((.gateway_event_type | session_event) or
      (.gateway_event_type | response_event)) and
    (.harness_id | type) == "string" and .harness_id != "" and
    (.backend_id | type) == "string" and .backend_id != "" and
    (.session_id | type) == "string" and .session_id != "" and
    (.thread_id | type) == "string" and .thread_id != "" and
    (.segment_id | type) == "string" and .segment_id != "" and
    (.workspace_id | type) == "string" and .workspace_id != "" and
    (.workspace_digest | digest) and
    (.execution_binding_digest | digest) and
    (.profile_id | type) == "string" and
    (.profile_id == "" or (.profile_id | identifier)) and
    (.provider_id | type) == "string" and .provider_id != "" and
    valid_route_authority and
    (.model_id | type) == "string" and .model_id != "" and
    (.segment_context_capsule_digest | digest) and
    (if has("route_transition_review_digest") then
       .route_transition_review_digest == "" or
       (.route_transition_review_digest | digest)
     else true end) and
    valid_outcome and
    (if (.gateway_event_type | response_event) then
       (.response_id | type) == "string" and .response_id != "" and
       (.incident_id | type) == "string" and .incident_id != "" and
       (.context_capsule_digest | digest)
     else
       (.response_id // "") == "" and (.context_capsule_digest // "") == ""
     end)
  ) then . else error("invalid HG1 authority event") end
| ($events | sort_by(.gateway_instance_id) | group_by(.gateway_instance_id)) as $instances
| [ $instances[] | try instance_matrix(.) catch empty ] as $matrices
| if ($matrices | length) == 0 then error("no complete single-instance HG1 matrix")
  else ($matrices | sort_by(.completed_at) | last) end
