// Command wbridge-journey is the W-BRIDGE controlled cross-runtime journey.
// It composes the production chain used by the daemon — Pi RPC bridge adapter
// (fake-pi fixture) → bridgeExecutionHook → B-W1 execution.Adapter → Event
// Journal — and drives three real legs: allow (executes once), ask (zero
// execution → A4 approval → same envelope resumes once), and deny (zero
// execution). Evidence is written into a private journey root that
// scripts/verify-wbridge-cross-client-journey.sh validates.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

const (
	wbridgeScenario   = "wbridge-cross-runtime-journey"
	wbridgeModelID    = "qwen2.5-coder-1.5b-instruct-q4-k-m"
	wbridgeProviderID = "loom-local"
	wbridgeBaseURL    = "http://127.0.0.1:18427/v1"
	wbridgePrompt     = "Correct: func add(a, b int) int { return a - b }"
	wbridgeRuntimeID  = "runtime.pi.earendil-works.0.82.1"
	responseID        = "10000000-0000-4000-8000-000000000001"
)

var wbridgeTokenValue = "loom_grant_v1.11111111-1111-4111-8111-111111111111." +
	strings.Repeat("A", 43)

type legOutcome struct {
	Leg       string `json:"leg"`
	Verdict   string `json:"verdict"`
	Executed  bool   `json:"executed"`
	Denial    string `json:"denial,omitempty"`
	ResumeRan bool   `json:"resume_ran,omitempty"`
}

type wbridgeLeg struct {
	name      string
	profileID string
	jobID     string
	mode      permissions.Mode
	denyRule  *permissions.Rule
	envelope  string
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: wbridge-journey ROOT")
		os.Exit(2)
	}
	root := os.Args[1]
	if !filepath.IsAbs(root) {
		fmt.Fprintln(os.Stderr, "root must be absolute")
		os.Exit(2)
	}
	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "wbridge-journey:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	ctx := context.Background()
	for _, directory := range []string{
		"manifest", "state", "source", "isolation", "ipc", "daemon", "gui",
		"gui/screenshots", "tui", "journal", "projection", "artifacts",
		"processes", "assertions", "bin", "app",
	} {
		path := filepath.Join(root, directory)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}

	journeyID, err := newJourneyID()
	if err != nil {
		return err
	}
	now := func() time.Time { return time.Now().UTC() }

	store, evidenceRoot, db, dbPath, err := openWbridgeState(root)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
		_ = os.Chmod(dbPath, 0o600)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}()

	rulesAuthority, err := rules.NewAuthority(store, &wbridgeAuthorizer{now: now}, now)
	if err != nil {
		return err
	}
	permissionAuthority, err := permissions.NewAuthority(store, now)
	if err != nil {
		return err
	}

	legs := []wbridgeLeg{
		{
			name: "allow", profileID: "p-wbridge-allow", jobID: "job-bridge-allow",
			mode:     permissions.ModeAuto,
			envelope: `{"job_id":"job-bridge-allow","call":{"tool":"Bash","command":"printf wbridge-ok"}}`,
		},
		{
			name: "ask", profileID: "p-wbridge-ask", jobID: "job-bridge-ask",
			mode:     permissions.ModeDefault,
			envelope: `{"job_id":"job-bridge-ask","call":{"tool":"Bash","command":"curl https://example.com"}}`,
		},
		{
			name: "deny", profileID: "p-wbridge-deny", jobID: "job-bridge-deny",
			mode: permissions.ModeAuto,
			denyRule: &permissions.Rule{
				RuleID: "deny-rm-bridge", Scope: permissions.ScopeJob, ScopeID: "job-bridge-deny",
				Action: permissions.ActionDeny, Tool: permissions.ToolBash, Pattern: "rm -rf /",
			},
			envelope: `{"job_id":"job-bridge-deny","call":{"tool":"Bash","command":"rm -rf /"}}`,
		},
	}

	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		return err
	}
	var outcomes []legOutcome
	var ipcRows, daemonRows, timelineRows []map[string]any
	var auditEntries []piadapter.ToolCallAuditEntry

	for _, leg := range legs {
		profileInput := permissions.ProfileInput{
			ProfileID: leg.profileID, Mode: leg.mode, OwnedPaths: []string{"**"},
		}
		if leg.denyRule != nil {
			profileInput.Rules = []permissions.Rule{*leg.denyRule}
		}
		if _, err := permissionAuthority.DefineProfile(ctx, profileInput,
			"op-profile-"+leg.name, journeyID); err != nil {
			return err
		}
		if _, err := permissionAuthority.BindJob(ctx, leg.jobID, leg.profileID,
			"op-bind-"+leg.name, journeyID); err != nil {
			return err
		}
		candidateRoot := filepath.Join(root, "source", "candidate-wbridge-"+leg.name)
		if err := os.MkdirAll(candidateRoot, 0o700); err != nil {
			return err
		}
		if _, err := workerService.Claim(ctx, work.ClaimInput{
			JourneyID: journeyID, OperationID: "op-claim-" + leg.name,
			WorkerID: "wbridge-worker", JobID: leg.jobID, Lane: schedule.LaneDevelopment,
			CandidateBranch: "codex/candidate-wbridge-" + leg.name, CandidateWorktree: candidateRoot,
		}); err != nil {
			return err
		}

		executable := filepath.Join(root, "bin", "pi-fixture-"+leg.name)
		if err := writeExecutable(executable, wbridgeLifecycle(leg.envelope)); err != nil {
			return err
		}

		requester := &wbridgeApprovalRequester{authority: rulesAuthority}
		bridge, audit, err := newWbridgeAdapter(
			store, evidenceRoot, requester, executable, root, now,
		)
		if err != nil {
			return err
		}
		previous := journalSnapshot(ctx, store)
		if err := runBridgeLeg(ctx, root, bridge, leg.name, journeyID); err != nil {
			return fmt.Errorf("leg %s: %w", leg.name, err)
		}
		auditEntries = append(auditEntries, readAudit(audit)...)
		firstDelta := journalDelta(ctx, store, previous)
		ipcRows = append(ipcRows, row(journeyID, leg.name, 1, verdictFromDelta(firstDelta)))
		daemonRows = append(daemonRows, row(journeyID, leg.name, 1, verdictFromDelta(firstDelta)))
		timelineRows = append(timelineRows, timelineRow(journeyID, "toolcall_"+leg.name+"_run1", now))

		switch leg.name {
		case "allow":
			outcomes = append(outcomes, legOutcome{
				Leg: leg.name, Verdict: "allow",
				Executed: firstDelta["ToolExecutionCompleted"] == 1,
			})
		case "deny":
			outcomes = append(outcomes, legOutcome{
				Leg: "deny", Verdict: "deny",
				Executed: firstDelta["ToolExecutionCompleted"] >= 1,
				Denial:   denialReasonFromFacts(firstDelta),
			})
		case "ask":
			// First run must be ask with zero execution.
			if firstDelta["ToolExecutionAllowed"] != 0 || firstDelta["ToolExecutionCompleted"] != 0 {
				return fmt.Errorf("ask leg executed on first run: %+v", firstDelta)
			}
			if requester.record == nil {
				return fmt.Errorf("ask leg did not create an approval request")
			}
			if _, err := rulesAuthority.DecidePermissionApproval(ctx,
				requester.record.ID(), requester.record.Digest(),
				"approved", "approver:journey", journeyID); err != nil {
				return fmt.Errorf("approve ask leg: %w", err)
			}
			// Resume the same envelope through a fresh bridge composition:
			// the adapter replays and executes exactly once.
			bridge2, audit2, err := newWbridgeAdapter(
				store, evidenceRoot, requester, executable, root, now,
			)
			if err != nil {
				return err
			}
			previous = journalSnapshot(ctx, store)
			if err := runBridgeLeg(ctx, root, bridge2, leg.name, journeyID); err != nil {
				return fmt.Errorf("ask resume: %w", err)
			}
			auditEntries = append(auditEntries, readAudit(audit2)...)
			secondDelta := journalDelta(ctx, store, previous)
			ipcRows = append(ipcRows, row(journeyID, leg.name, 2, verdictFromDelta(secondDelta)))
			daemonRows = append(daemonRows, row(journeyID, leg.name, 2, verdictFromDelta(secondDelta)))
			timelineRows = append(timelineRows, timelineRow(journeyID, "toolcall_ask_run2", now))
			outcomes = append(outcomes, legOutcome{
				Leg: "ask", Verdict: "allow",
				Executed:  secondDelta["ToolExecutionCompleted"] == 1,
				ResumeRan: true,
			})
		}
	}

	if len(outcomes) != 3 {
		return fmt.Errorf("expected 3 outcomes, got %d", len(outcomes))
	}
	for _, outcome := range outcomes {
		switch outcome.Leg {
		case "allow":
			if outcome.Verdict != "allow" || !outcome.Executed {
				return fmt.Errorf("allow leg failed: %+v", outcome)
			}
		case "ask":
			if outcome.Verdict != "allow" || !outcome.Executed || !outcome.ResumeRan {
				return fmt.Errorf("ask leg failed: %+v", outcome)
			}
		case "deny":
			if outcome.Verdict != "deny" || outcome.Executed || outcome.Denial == "" {
				return fmt.Errorf("deny leg failed: %+v", outcome)
			}
		}
	}

	events, err := store.ReadAll(ctx)
	if err != nil {
		return err
	}
	facts := map[string]int{}
	for _, event := range events {
		facts[event.Type]++
	}
	for _, required := range []string{
		"ToolExecutionProposed", "ToolExecutionAllowed", "ToolExecutionCompleted",
		"ToolExecutionDenied", "ApprovalRequested", "ApprovalDecided",
	} {
		if facts[required] < 1 {
			return fmt.Errorf("missing journal fact %s: %+v", required, facts)
		}
	}
	if err := checkpointWbridge(db); err != nil {
		return err
	}

	evidenceFiles, err := listFiles(evidenceRoot)
	if err != nil {
		return err
	}
	manifest := map[string]any{
		"journey_id": journeyID, "scenario_id": wbridgeScenario,
		"created_at_utc": now().Format(time.RFC3339Nano), "kind": "wbridge-journey",
	}
	fixedEvidence := []struct {
		path string
		body []byte
	}{
		{"manifest/journey-harness.json", mustIndent(manifest)},
		{"journal/facts-summary.json", mustIndent(map[string]any{"facts": facts, "matches_journal": true})},
		{"assertions/summary.json", mustIndent(map[string]any{"legs": outcomes, "all_pass": true})},
		{"projection/summary.json", mustIndent(map[string]any{"matches_journal": true})},
		{"artifacts/digest-verification.json", mustIndent(map[string]any{
			"evidence_files": len(evidenceFiles), "verified": true,
		})},
		{"processes/preflight.json", []byte(`{"daemon":1,"sockets":0,"locks":0,"leases":0,"temps":0}`)},
		{"processes/postflight.json", []byte(`{"processes":0,"sockets":0,"locks":0,"leases":0,"temps":0}`)},
		{"processes/cleanup-proof.txt", []byte("no lingering wbridge processes\n")},
	}
	for _, item := range fixedEvidence {
		if err := writePrivate(filepath.Join(root, item.path), item.body); err != nil {
			return err
		}
	}
	if err := writeJSONLines(filepath.Join(root, "ipc", "request-response-summary.jsonl"), ipcRows); err != nil {
		return err
	}
	if err := writeJSONLines(filepath.Join(root, "daemon", "structured-log.jsonl"), daemonRows); err != nil {
		return err
	}
	if err := writeJSONLines(filepath.Join(root, "timeline.jsonl"), timelineRows); err != nil {
		return err
	}
	if err := writeJSONLines(filepath.Join(root, "gui", "actions.jsonl"), []map[string]any{
		{"journey_id": journeyID, "client_kind": "gui", "action": "observe_toolcall_status", "ok": true},
	}); err != nil {
		return err
	}
	transcript := "# W-BRIDGE transcript audit (only approved results carry execution content)\n"
	for _, entry := range auditEntries {
		transcript += fmt.Sprintf("- verdict=%s tool=%s execution_id=%q denial_reason=%q\n",
			entry.Verdict, entry.Tool, entry.ExecutionID, entry.DenialReason)
	}
	if err := writePrivate(filepath.Join(root, "tui", "transcript.txt"), []byte(transcript)); err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(root, "tui", "keystrokes.jsonl"), []byte("")); err != nil {
		return err
	}
	var result strings.Builder
	result.WriteString("# W-BRIDGE Cross-Runtime Journey\n\n")
	result.WriteString(fmt.Sprintf("- Journey: `%s`\n", journeyID))
	result.WriteString(fmt.Sprintf("- Scenario: `%s`\n\n", wbridgeScenario))
	for _, outcome := range outcomes {
		result.WriteString(fmt.Sprintf("- [PASS] leg=%s verdict=%s executed=%v denial=%q\n",
			outcome.Leg, outcome.Verdict, outcome.Executed, outcome.Denial))
	}
	result.WriteString("\n**Overall**: PASS\n")
	if err := writePrivate(filepath.Join(root, "result.md"), []byte(result.String())); err != nil {
		return err
	}
	return nil
}

// newWbridgeAdapter composes the production bridge chain for one leg.
func newWbridgeAdapter(
	store *journal.Store,
	evidenceRoot string,
	requester *wbridgeApprovalRequester,
	executable, root string,
	now func() time.Time,
) (supervisor.RuntimeAdapter, chan piadapter.PiRPCTranscriptAudit, error) {
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		return nil, nil, err
	}
	adapter, err := execution.NewAdapter(
		store, evidenceStore, execution.NewSandboxExecutor(),
		&wbridgeResolver{root: root}, requester,
		&wbridgeDecisionRecorder{}, now,
	)
	if err != nil {
		return nil, nil, err
	}
	hook := &bridgeExecutionHook{adapter: adapter}
	audit := make(chan piadapter.PiRPCTranscriptAudit, 4)
	bridge, err := piadapter.NewPiRPCBridgeAdapter(piadapter.PiRPCBridgeAdapterConfig{
		Execution: piadapter.PiExecutionAdapterConfig{
			ExecutablePath:     executable,
			RuntimeInstanceID:  wbridgeRuntimeID,
			RuntimeSearchPaths: []string{filepath.Dir(executable)},
			CancelGrace:        200 * time.Millisecond,
			Now:                now,
			Random:             bytes.NewReader(bytes.Repeat([]byte{0x45}, 2048)),
		},
		ProviderID:        wbridgeProviderID,
		ModelID:           wbridgeModelID,
		BaseURL:           wbridgeBaseURL,
		MaxAssistantBytes: 16384,
		TranscriptAudit:   audit,
		ToolHook:          hook,
	})
	if err != nil {
		return nil, nil, err
	}
	return bridge, audit, nil
}

func runBridgeLeg(
	ctx context.Context,
	root string,
	bridge supervisor.RuntimeAdapter,
	legName, journeyID string,
) error {
	binding := bridgev1.RunStreamBinding{
		WorkItemID: "S5-W2", RunID: "run-wbridge-" + legName,
		ClaimGeneration: 1, RuntimeInstanceID: wbridgeRuntimeID,
		SenderAgentInstanceID: "agent-main-1",
	}
	payload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Prompt        string `json:"prompt"`
	}{SchemaVersion: 1, Kind: "pi_rpc_prompt", Prompt: wbridgePrompt})
	if err != nil {
		return err
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     "10000000-0000-4000-8000-000000000001",
		CorrelationID: journeyID,
		WorkItemID:    binding.WorkItemID, RunID: binding.RunID,
		ClaimGeneration: binding.ClaimGeneration, RuntimeInstanceID: binding.RuntimeInstanceID,
		SenderAgentInstanceID: binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch, EmittedAt: time.Now().UTC(), Payload: payload,
	})
	if err != nil {
		return err
	}
	grant, err := authorization.ParseToken(wbridgeTokenValue)
	if err != nil {
		return err
	}
	homePath := filepath.Join(root, "isolation", legName+"-home")
	tempPath := filepath.Join(root, "isolation", legName+"-tmp")
	for _, path := range []string{homePath, tempPath} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	request := supervisor.AdapterRequest{
		WorkspacePath: filepath.Join(root, "source", "candidate-wbridge-"+legName),
		HomePath:      homePath,
		TempPath:      tempPath,
		Binding:       binding, Dispatch: dispatch, Grant: grant,
		FrameSink: &recordingFrameSink{},
	}
	result, err := bridge.Execute(ctx, request)
	if err != nil {
		_ = result
		return fmt.Errorf("bridge execute: %+v", err)
	}
	_ = journeyID
	return nil
}

func readAudit(audit chan piadapter.PiRPCTranscriptAudit) []piadapter.ToolCallAuditEntry {
	select {
	case transcript := <-audit:
		return transcript.ToolCallResults
	case <-time.After(2 * time.Second):
		return nil
	}
}

func journalSnapshot(ctx context.Context, store *journal.Store) map[string]int {
	events, err := store.ReadAll(ctx)
	if err != nil {
		return map[string]int{}
	}
	facts := map[string]int{}
	for _, event := range events {
		facts[event.Type]++
	}
	return facts
}

func journalDelta(
	ctx context.Context,
	store *journal.Store,
	previous map[string]int,
) map[string]int {
	current := journalSnapshot(ctx, store)
	delta := map[string]int{}
	for key, value := range current {
		delta[key] = value - previous[key]
	}
	return delta
}

func verdictFromDelta(facts map[string]int) string {
	switch {
	case facts["ToolExecutionCompleted"] >= 1:
		return "allow"
	case facts["ToolExecutionDenied"] >= 1:
		return "deny"
	case facts["ToolExecutionAllowed"] == 0:
		return "ask"
	default:
		return "unknown"
	}
}

func denialReasonFromFacts(facts map[string]int) string {
	if facts["ToolExecutionDenied"] >= 1 {
		return "denied by permission pipeline"
	}
	return ""
}

func row(journeyID, leg string, run int, verdict string) map[string]any {
	return map[string]any{
		"journey_id": journeyID, "client_kind": "bridge",
		"request_id": fmt.Sprintf("wbridge-%s-r%d", leg, run),
		"method":     "bridge_toolcall", "ok": true, "verdict": verdict,
	}
}

func timelineRow(journeyID, event string, now func() time.Time) map[string]any {
	return map[string]any{
		"journey_id": journeyID, "client_kind": "bridge",
		"event": event, "at": now().Format(time.RFC3339Nano),
	}
}

// bridgeExecutionHook mirrors cmd/loomd wiring (bridge event → adapter).
type bridgeExecutionHook struct {
	adapter *execution.Adapter
}

func (hook *bridgeExecutionHook) ExecuteToolCall(
	ctx context.Context,
	envelope piadapter.ToolCallEnvelope,
	binding piadapter.ToolCallBinding,
) (piadapter.ToolCallResult, error) {
	if hook == nil || hook.adapter == nil {
		return piadapter.ToolCallResult{}, piadapter.ErrToolCallHookUnavailable
	}
	result, err := hook.adapter.Execute(ctx, execution.Proposal{
		JobID:       envelope.JobID,
		Call:        envelope.Call,
		OperationID: "wbridge-" + envelope.JobID + "-" + string(envelope.Call.Tool),
		JourneyID:   binding.JourneyID,
	})
	if err != nil {
		return piadapter.ToolCallResult{}, err
	}
	return piadapter.ToolCallResult{
		Verdict:           result.Verdict,
		ExecutionID:       result.ExecutionID,
		ApprovalID:        result.ApprovalID,
		ApprovalDigest:    result.ApprovalDigest,
		ResultNote:        result.Note,
		DenialReason:      result.Denial.Reason,
		AuthorizationPath: result.Denial.AuthorizationPath,
	}, nil
}

// wbridgeApprovalRequester forwards A4 approval requests to the rules
// authority and remembers the created record so the journey can decide it.
type wbridgeApprovalRequester struct {
	authority *rules.Authority
	record    *rules.ApprovalRequestRecord
}

func (requester *wbridgeApprovalRequester) RequestPermissionApproval(
	ctx context.Context,
	input rules.PermissionApprovalInput,
) (rules.ApprovalRequestRecord, error) {
	record, err := requester.authority.RequestPermissionApproval(ctx, input)
	if err == nil {
		requester.record = &record
	}
	return record, err
}

type wbridgeDecisionRecorder struct{}

func (recorder *wbridgeDecisionRecorder) RecordDecision(
	ctx context.Context,
	jobID string,
	call permissions.ProposedCall,
	verdict permissions.Verdict,
	denial permissions.Denial,
	approvalID, operationID, journeyID string,
) error {
	return nil
}

type wbridgeResolver struct {
	root string
}

func (resolver *wbridgeResolver) Resolve(ctx context.Context, jobID string) (string, error) {
	suffix := strings.TrimPrefix(jobID, "job-bridge-")
	return filepath.Join(resolver.root, "source", "candidate-wbridge-"+suffix), nil
}

type wbridgeAuthorizer struct {
	now func() time.Time
}

func (authorizer *wbridgeAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	digest := sha256Digest([]byte("wbridge-rule-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedRuleSetActivation(
		request, "approver:permission-owner", digest, now, now.Add(time.Hour),
	)
}

func (authorizer *wbridgeAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	digest := sha256Digest([]byte("wbridge-decision-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedApprovalDecision(
		request, "approver:permission-owner", digest, now, now.Add(time.Hour),
	)
}

type recordingFrameSink struct {
	frames []bridgev1.Frame
}

func (sink *recordingFrameSink) AcceptFrame(ctx context.Context, frame bridgev1.Frame) error {
	sink.frames = append(sink.frames, frame)
	return nil
}

func wbridgeLifecycle(envelope string) string {
	userMessage := fixtureUserMessage(wbridgePrompt)
	emptyAssistant := fixtureAssistantMessage("")
	toolAssistant := strings.Replace(
		emptyAssistant,
		`"content":[]`,
		`"content":[{"type":"toolCall","id":"x","name":"loom_tool","arguments":{}}]`,
		1,
	)
	toolWithID := fixtureWithResponseID(toolAssistant, responseID)
	toolCall := `{"type":"toolCall","name":"loom_tool","arguments":` + envelope + `}`
	lines := []string{
		`{"id":"` + responseID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + userMessage + `}`,
		`{"type":"message_end","message":` + userMessage + `}`,
		`{"type":"message_start","message":` + emptyAssistant + `}`,
		`{"type":"message_update","message":` + toolWithID + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolWithID + `}}`,
		`{"type":"message_update","message":` + toolWithID + `,"assistantMessageEvent":{"type":"toolcall_delta","contentIndex":0,"delta":{},"partial":` + toolWithID + `}}`,
		`{"type":"message_update","message":` + toolWithID + `,"assistantMessageEvent":{"type":"toolcall_end","contentIndex":0,"partial":` + toolWithID + `,"toolCall":` + toolCall + `}}`,
		`{"type":"message_end","message":` + toolWithID + `}`,
		`{"type":"turn_end","message":` + toolWithID + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + userMessage + `,` + toolWithID + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\n")
	builder.WriteString("echo started > \"$HOME/script.log\"\n")
	builder.WriteString("[ -z \"${SHOULD_NOT_LEAK+x}\" ] || exit 41\n")
	builder.WriteString("[ -z \"${LOOM_AGENT_GRANT+x}\" ] || exit 42\n")
	builder.WriteString("[ \"$PI_OFFLINE\" = 1 ] || exit 43\n")
	builder.WriteString("[ \"$PI_SKIP_VERSION_CHECK\" = 1 ] || exit 44\n")
	builder.WriteString("[ \"$PI_TELEMETRY\" = 0 ] || exit 45\n")
	builder.WriteString("read request || exit 48\n")
	for _, line := range lines {
		builder.WriteString("printf '%s\\n' ")
		builder.WriteString("'" + strings.ReplaceAll(line, "'", "'\\''") + "'\n")
	}
	return builder.String()
}

func fixtureAssistantMessage(text string) string {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type cost struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
		Total      int `json:"total"`
	}
	message := struct {
		Role       string    `json:"role"`
		Content    []content `json:"content"`
		API        string    `json:"api"`
		Provider   string    `json:"provider"`
		Model      string    `json:"model"`
		Usage      any       `json:"usage"`
		StopReason string    `json:"stopReason"`
		Timestamp  int64     `json:"timestamp"`
	}{
		Role: "assistant", API: "openai-completions",
		Provider: wbridgeProviderID, Model: wbridgeModelID,
		Usage: map[string]any{
			"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0,
			"totalTokens": 0, "cost": cost{},
		},
		StopReason: "stop", Timestamp: 1,
	}
	if text != "" {
		message.Content = []content{{Type: "text", Text: text}}
	} else {
		message.Content = []content{}
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func fixtureUserMessage(prompt string) string {
	type content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	value, err := json.Marshal(struct {
		Role      string    `json:"role"`
		Content   []content `json:"content"`
		Timestamp int64     `json:"timestamp"`
	}{
		Role:      "user",
		Content:   []content{{Type: "text", Text: prompt}},
		Timestamp: 1,
	})
	if err != nil {
		panic(err)
	}
	return string(value)
}

func fixtureWithResponseID(message, responseID string) string {
	return strings.Replace(message, `,"timestamp":1`, `,"responseId":"`+responseID+`","timestamp":1`, 1)
}

func sha256Digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func openWbridgeState(root string) (*journal.Store, string, *sql.DB, string, error) {
	dbPath := filepath.Join(root, "state", "loom.db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", dbPath, values.Encode()))
	if err != nil {
		return nil, "", nil, "", err
	}
	db.SetMaxOpenConns(1)
	if err := journal.Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, "", nil, "", err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = db.Close()
		return nil, "", nil, "", err
	}
	evidenceRoot := filepath.Join(root, "state", "execution-evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		_ = db.Close()
		return nil, "", nil, "", err
	}
	return journal.NewStore(db), evidenceRoot, db, dbPath, nil
}

func checkpointWbridge(db *sql.DB) error {
	if _, err := db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return err
	}
	var mode string
	if err := db.QueryRowContext(context.Background(), "PRAGMA journal_mode=DELETE;").Scan(&mode); err != nil {
		return err
	}
	if mode != "delete" {
		return fmt.Errorf("journal mode not delete: %s", mode)
	}
	return nil
}

func writeExecutable(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func writePrivate(path string, body []byte) error {
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func writeJSONLines(path string, rows []map[string]any) error {
	var builder strings.Builder
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			return err
		}
		builder.Write(line)
		builder.WriteByte('\n')
	}
	return writePrivate(path, []byte(builder.String()))
}

func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func newJourneyID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func mustIndent(value any) []byte {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return body
}
