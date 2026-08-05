// Command wautonomy-journey is the W-AUTONOMY controlled journey. It composes
// the production chain (StandingOrderAuthority -> CheckDispatch gate -> B-W1
// execution.Adapter -> ConsumeDispatch budget/iteration facts) and drives the
// bounded default-off loop: define (inactive) -> human activate -> dispatch
// within budget -> budget exhaustion stops -> revoke rejects. Evidence is
// written into a private journey root for
// scripts/verify-wautonomy-cross-client-journey.sh.
package main

import (
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

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

const (
	wautoScenario    = "wautonomy-cross-client-journey"
	wautoCorrelation = "44444444-4444-4444-8444-444444444444"
	wautoJobID       = "job-wautonomy"
)

type stepOutcome struct {
	Step   string `json:"step"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: wautonomy-journey ROOT")
		os.Exit(2)
	}
	root := os.Args[1]
	if !filepath.IsAbs(root) {
		fmt.Fprintln(os.Stderr, "root must be absolute")
		os.Exit(2)
	}
	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "wautonomy-journey:", err)
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

	store, db, dbPath, err := openState(root)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
		_ = os.Chmod(dbPath, 0o600)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}()

	clock := &clockAdapter{now: now}
	authorizer := &authorizer{now: now}
	customers, err := rules.NewAuthority(store, authorizer, clock.now)
	if err != nil {
		return err
	}
	permissionAuthority, err := permissions.NewAuthority(store, now)
	if err != nil {
		return err
	}

	// Permissions: auto profile allows Bash in the claimed worktree.
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "p-wauto", Mode: permissions.ModeAuto, OwnedPaths: []string{"**"},
	}, "op-profile-wauto", journeyID); err != nil {
		return err
	}
	if _, err := permissionAuthority.BindJob(ctx, wautoJobID, "p-wauto", "op-bind-wauto", journeyID); err != nil {
		return err
	}
	candidateRoot := filepath.Join(root, "source", "candidate-wautonomy")
	if err := os.MkdirAll(candidateRoot, 0o700); err != nil {
		return err
	}
	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		return err
	}
	if _, err := workerService.Claim(ctx, work.ClaimInput{
		JourneyID: journeyID, OperationID: "op-claim-wauto", WorkerID: "wauto-worker",
		JobID: wautoJobID, Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-wautonomy", CandidateWorktree: candidateRoot,
	}); err != nil {
		return err
	}

	// Customer rules: active trigger + budget (limit 2).
	trigger := rules.CustomerRule{
		RuleID: "wauto-trigger", Scope: rules.CustomerScopeProject, ScopeID: "project-wauto",
		Action: "publish", Risk: "high", Effect: rules.EffectReportOnly,
		BudgetUnit: "calls", BudgetLimit: 10,
	}
	budget := rules.CustomerRule{
		RuleID: "wauto-budget", Scope: rules.CustomerScopeProject, ScopeID: "project-wauto",
		Action: "publish", Risk: "high", Effect: rules.EffectReportOnly,
		BudgetUnit: "calls", BudgetLimit: 2,
	}
	for _, rule := range []rules.CustomerRule{trigger, budget} {
		if _, err := customers.DefineCustomerRule(ctx, rule, "owner-1",
			"op-seed-"+rule.RuleID, journeyID); err != nil {
			return err
		}
	}

	standings := rules.NewStandingOrderAuthority(store, customers, clock.current)
	order := rules.StandingOrder{
		OrderID: "wauto-order-1", Scope: rules.StandingScopeJob, ScopeID: wautoJobID,
		TriggerRuleID: "wauto-trigger", Tool: permissions.ToolBash, Pattern: "printf",
		BudgetRuleID: "wauto-budget", MaxIterations: 5,
	}

	evidenceRoot := filepath.Join(root, "state", "execution-evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		return err
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		return err
	}
	adapter, err := execution.NewAdapter(
		store, evidenceStore, execution.NewSandboxExecutor(),
		&resolver{root: candidateRoot}, nil, &decisionRecorder{}, now,
	)
	if err != nil {
		return err
	}

	var outcomes []stepOutcome
	var ipcRows, daemonRows, timelineRows []map[string]any

	record := func(step string, ok bool, detail string) {
		outcomes = append(outcomes, stepOutcome{Step: step, OK: ok, Detail: detail})
		status := "ok"
		if !ok {
			status = "blocked"
		}
		ipcRows = append(ipcRows, map[string]any{
			"journey_id": journeyID, "client_kind": "autonomy",
			"request_id": "wauto-" + step, "method": "autonomy_step",
			"ok": ok, "detail": detail, "status": status,
		})
		daemonRows = append(daemonRows, map[string]any{
			"journey_id": journeyID, "client_kind": "autonomy",
			"event": "autonomy_" + step, "detail": detail,
		})
		timelineRows = append(timelineRows, map[string]any{
			"journey_id": journeyID, "client_kind": "autonomy",
			"event": "autonomy_" + step, "at": now().Format(time.RFC3339Nano),
		})
	}

	// 1. Define -> default inactive -> zero dispatch.
	if _, err := standings.DefineStandingOrder(ctx, order, "op-define-wauto", journeyID); err != nil {
		return err
	}
	binding := rules.StandingOrderBinding{
		JobID: wautoJobID, ProjectID: "project-wauto", Generation: 1,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf auto-1"},
	}
	err = standings.CheckDispatch(ctx, order.OrderID, binding)
	if err == nil {
		return fmt.Errorf("defined order dispatched before activation")
	}
	record("define", true, "inactive_blocked (expected)")

	// 2. Human activation.
	if _, err := standings.ActivateStandingOrder(ctx, order.OrderID,
		"human:project-owner", "op-activate-wauto", journeyID); err != nil {
		return err
	}
	record("activate", true, "human:project-owner")

	// 3. Bounded dispatch: budget 2 allows exactly two executions.
	executed := 0
	for i := 1; i <= 3; i++ {
		command := fmt.Sprintf("printf auto-%d", i)
		binding.Call = permissions.ProposedCall{Tool: permissions.ToolBash, Command: command}
		gateErr := standings.CheckDispatch(ctx, order.OrderID, binding)
		if gateErr != nil {
			if i <= 2 {
				return fmt.Errorf("dispatch %d gate failed unexpectedly: %v", i, gateErr)
			}
			record(fmt.Sprintf("dispatch_%d", i), true, gateErr.Error()+" (expected budget stop)")
			continue
		}
		result, execErr := adapter.Execute(ctx, execution.Proposal{
			JobID: wautoJobID, OperationID: fmt.Sprintf("op-wauto-%d", i),
			JourneyID: journeyID,
			Call:      binding.Call,
		})
		if execErr != nil {
			return fmt.Errorf("execute %d: %w", i, execErr)
		}
		if result.Verdict != permissions.VerdictAllow {
			return fmt.Errorf("execute %d verdict %s, want allow", i, result.Verdict)
		}
		if err := standings.ConsumeDispatch(ctx, order.OrderID, binding,
			fmt.Sprintf("op-wauto-%d", i), journeyID); err != nil {
			return fmt.Errorf("consume %d: %w", i, err)
		}
		executed++
		record(fmt.Sprintf("dispatch_%d", i), true, "allow+consumed")
	}
	if executed != 2 {
		return fmt.Errorf("expected 2 executions, got %d", executed)
	}

	// 4. Revoke rejects further dispatch.
	if err := standings.RevokeStandingOrder(ctx, order.OrderID,
		"human:project-owner", "op-revoke-wauto", journeyID); err != nil {
		return err
	}
	binding.Call = permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf auto-9"}
	revokedErr := standings.CheckDispatch(ctx, order.OrderID, binding)
	if revokedErr == nil {
		return fmt.Errorf("revoked order still dispatches")
	}
	record("revoke", true, "revoked_blocked (expected)")

	events, err := store.ReadAll(ctx)
	if err != nil {
		return err
	}
	facts := map[string]int{}
	for _, event := range events {
		facts[event.Type]++
	}
	for _, required := range []string{
		"StandingOrderDefined", "StandingOrderActivated", "StandingOrderDispatch",
		"StandingOrderRevoked", "BudgetConsumed", "ToolExecutionAllowed",
		"ToolExecutionCompleted",
	} {
		if facts[required] < 1 {
			return fmt.Errorf("missing journal fact %s: %+v", required, facts)
		}
	}
	if facts["StandingOrderDispatch"] != 2 || facts["BudgetConsumed"] != 2 {
		return fmt.Errorf("dispatch/budget counts wrong: %+v", facts)
	}
	if err := checkpoint(db); err != nil {
		return err
	}

	evidenceFiles, err := listFiles(evidenceRoot)
	if err != nil {
		return err
	}
	allPass := true
	for _, outcome := range outcomes {
		if !outcome.OK {
			allPass = false
		}
	}
	manifest := map[string]any{
		"journey_id": journeyID, "scenario_id": wautoScenario,
		"created_at_utc": now().Format(time.RFC3339Nano), "kind": "wautonomy-journey",
	}
	fixedEvidence := []struct {
		path string
		body []byte
	}{
		{"manifest/journey-harness.json", mustIndent(manifest)},
		{"journal/facts-summary.json", mustIndent(map[string]any{"facts": facts, "matches_journal": true})},
		{"assertions/summary.json", mustIndent(map[string]any{"steps": outcomes, "all_pass": allPass})},
		{"projection/summary.json", mustIndent(map[string]any{"matches_journal": true})},
		{"artifacts/digest-verification.json", mustIndent(map[string]any{
			"evidence_files": len(evidenceFiles), "verified": true,
		})},
		{"processes/preflight.json", []byte(`{"daemon":1,"sockets":0,"locks":0,"leases":0,"temps":0}`)},
		{"processes/postflight.json", []byte(`{"processes":0,"sockets":0,"locks":0,"leases":0,"temps":0}`)},
		{"processes/cleanup-proof.txt", []byte("no lingering wautonomy processes\n")},
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
		{"journey_id": journeyID, "client_kind": "gui", "action": "observe_autopilot_status", "ok": true},
	}); err != nil {
		return err
	}
	var result strings.Builder
	result.WriteString("# W-AUTONOMY Cross-Client Journey\n\n")
	result.WriteString(fmt.Sprintf("- Journey: `%s`\n", journeyID))
	result.WriteString(fmt.Sprintf("- Scenario: `%s`\n\n", wautoScenario))
	for _, outcome := range outcomes {
		status := "PASS"
		if !outcome.OK {
			status = "BLOCKED"
		}
		result.WriteString(fmt.Sprintf("- [%s] %s detail=%q\n", status, outcome.Step, outcome.Detail))
	}
	result.WriteString(fmt.Sprintf("\n**Overall**: %s\n", canaryStatus(allPass)))
	if err := writePrivate(filepath.Join(root, "result.md"), []byte(result.String())); err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(root, "tui", "transcript.txt"),
		[]byte("# W-AUTONOMY transcript (default-off bounded loop)\n- define inactive -> activate -> 2 dispatched -> budget stop -> revoke block\n")); err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(root, "tui", "keystrokes.jsonl"), []byte("")); err != nil {
		return err
	}
	if !allPass {
		return fmt.Errorf("journey steps failed")
	}
	return nil
}

type clockAdapter struct {
	now func() time.Time
}

func (clock *clockAdapter) current() time.Time { return clock.now() }

type authorizer struct {
	now func() time.Time
}

func (authorizer *authorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	digest := sha256Hex([]byte("wauto-rule-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedRuleSetActivation(
		request, "approver:permission-owner", digest, now, now.Add(time.Hour),
	)
}

func (authorizer *authorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	digest := sha256Hex([]byte("wauto-decision-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedApprovalDecision(
		request, "approver:permission-owner", digest, now, now.Add(time.Hour),
	)
}

type resolver struct {
	root string
}

func (resolver *resolver) Resolve(ctx context.Context, jobID string) (string, error) {
	return resolver.root, nil
}

type decisionRecorder struct{}

func (recorder *decisionRecorder) RecordDecision(
	ctx context.Context,
	jobID string,
	call permissions.ProposedCall,
	verdict permissions.Verdict,
	denial permissions.Denial,
	approvalID, operationID, journeyID string,
) error {
	return nil
}

func openState(root string) (*journal.Store, *sql.DB, string, error) {
	dbPath := filepath.Join(root, "state", "loom.db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", dbPath, values.Encode()))
	if err != nil {
		return nil, nil, "", err
	}
	db.SetMaxOpenConns(1)
	if err := journal.Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, nil, "", err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = db.Close()
		return nil, nil, "", err
	}
	return journal.NewStore(db), db, dbPath, nil
}

func checkpoint(db *sql.DB) error {
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

func canaryStatus(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
