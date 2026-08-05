package permissions

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

const (
	permTestCorrelation = "11111111-1111-4111-8111-111111111111"
	permTestJobA        = "job-aaaa"
	permTestJobB        = "job-bbbb"
	permTestProject     = "project-1"
	permTestProfileA    = "profile-a"
	permTestProfileB    = "profile-b"
)

func openPermStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/perm.db?%s",
		t.TempDir(),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return journal.NewStore(db)
}

func mustPermAuthority(t testing.TB, store *journal.Store) *Authority {
	t.Helper()
	auth, err := NewAuthority(store, func() time.Time {
		return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("NewAuthority() error = %v", err)
	}
	return auth
}

func mustProfile(t testing.TB, auth *Authority, input ProfileInput) PermissionProfile {
	t.Helper()
	events, err := auth.DefineProfile(context.Background(), input, "op-"+input.ProfileID, permTestCorrelation)
	if err != nil {
		t.Fatalf("DefineProfile() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("DefineProfile() events = %d, want 1", len(events))
	}
	profile, err := CompileProfile(input)
	if err != nil {
		t.Fatalf("CompileProfile() error = %v", err)
	}
	return profile
}

func mustBind(t testing.TB, auth *Authority, jobID, profileID string) {
	t.Helper()
	if _, err := auth.BindJob(context.Background(), jobID, profileID, "op-bind-"+jobID, permTestCorrelation); err != nil {
		t.Fatalf("BindJob() error = %v", err)
	}
}

func allEvents(t testing.TB, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return events
}

func replayOf(t testing.TB, events []journal.Event) *Projection {
	t.Helper()
	projection, err := Replay(events)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	return projection
}

func effectiveFor(t testing.TB, projection *Projection, jobID string) EffectiveProfile {
	t.Helper()
	effective, err := ResolveEffectiveProfile(projection, jobID)
	if err != nil {
		t.Fatalf("EffectiveProfile() error = %v", err)
	}
	return effective
}

func verdictFor(t testing.TB, effective EffectiveProfile, call ProposedCall) Verdict {
	t.Helper()
	verdict, denial, err := Evaluate(effective, call)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if verdict == VerdictAsk && denial.Reason == "" {
		t.Fatal("ask verdict must carry a denial reason")
	}
	if verdict == VerdictDeny && denial.Reason == "" {
		t.Fatal("deny verdict must carry a denial reason")
	}
	return verdict
}

func TestRed01_DenyBeatsAllowAcrossScopes(t *testing.T) {
	profile := PermissionProfile{
		ProfileID: permTestProfileA, Generation: 1, Digest: "digest-a",
		Mode: ModeDefault, OwnedPaths: []string{"src/**"},
	}
	effective := EffectiveProfile{
		Profile: profile, Mode: ModeDefault,
		MergedRules: []Rule{
			{RuleID: "r-project-allow", Scope: ScopeProject, ScopeID: permTestProject, Action: ActionAllow, Tool: ToolBash, Pattern: "git push *"},
			{RuleID: "r-root-deny", Scope: ScopeRoot, ScopeID: "global", Action: ActionDeny, Tool: ToolBash, Pattern: "git push *"},
		},
	}
	verdict := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "git push origin main"})
	if verdict != VerdictDeny {
		t.Fatalf("verdict = %s, want deny (root deny must beat project allow)", verdict)
	}
}

func TestRed02_DenyBeatsBypass(t *testing.T) {
	effective := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeBypassPermissions},
		Mode:    ModeBypassPermissions,
		MergedRules: []Rule{
			{RuleID: "r-deny-rm", Scope: ScopeRoot, ScopeID: "global", Action: ActionDeny, Tool: ToolBash, Pattern: "rm -rf *"},
		},
	}
	verdict := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "rm -rf /tmp/x"})
	if verdict != VerdictDeny {
		t.Fatalf("verdict = %s, want deny (deny must beat bypass)", verdict)
	}
}

func TestRed03_AskEmitsDecisionFactAndNoApprovalEvent(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	input := ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault, OwnedPaths: []string{"src/**"}}
	mustProfile(t, auth, input)
	mustBind(t, auth, permTestJobA, permTestProfileA)
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-ask-curl", Scope: ScopeJob, ScopeID: permTestJobA,
		Action: ActionAsk, Tool: ToolBash, Pattern: "curl *",
	}, "user-1", "op-add-rule", permTestCorrelation); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	projection := replayOf(t, allEvents(t, store))
	effective := effectiveFor(t, projection, permTestJobA)
	verdict := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "curl https://example.com"})
	if verdict != VerdictAsk {
		t.Fatalf("verdict = %s, want ask", verdict)
	}
	if _, err := auth.RecordDecision(context.Background(), permTestJobA,
		ProposedCall{Tool: ToolBash, Command: "curl https://example.com"},
		VerdictAsk, Denial{Reason: "needs approval", AuthorizationPath: "resolve approval"},
		"", "op-decision-1", permTestCorrelation); err != nil {
		t.Fatalf("RecordDecision() error = %v", err)
	}

	for _, event := range allEvents(t, store) {
		if strings.Contains(event.Type, "Approval") || strings.HasPrefix(event.StreamID, "approval") {
			t.Fatalf("permission layer must not emit approval lifecycle events, got %s/%s", event.Type, event.StreamID)
		}
	}
	if _, err := auth.ResolveApproval(context.Background(), "approval-1", "allow", "user-1", "op-resolve", permTestCorrelation); err == nil {
		t.Fatal("ResolveApproval() must be a typed forward to rules authority, not a local write")
	}
}

func TestRed05_DangerousReasksEvenWithGrantAndBypass(t *testing.T) {
	effective := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeBypassPermissions},
		Mode:    ModeBypassPermissions,
		Grants: []Grant{
			{GrantID: "g-rm", Scope: ScopeProject, ScopeID: permTestProject, Tool: ToolBash, Pattern: "rm *"},
		},
	}
	verdict := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "rm -rf vendor"})
	if verdict != VerdictAsk {
		t.Fatalf("verdict = %s, want ask (dangerous must re-ask despite grant+bypass)", verdict)
	}
}

func TestRed07_StaleGenerationAndRetiredProfileError(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)

	projection := replayOf(t, allEvents(t, store))
	// stale generation: tamper binding generation.
	binding := projection.Bindings[permTestJobA]
	binding.ProfileGeneration = binding.ProfileGeneration + 1
	projection.Bindings[permTestJobA] = binding
	if _, err := ResolveEffectiveProfile(projection, permTestJobA); err == nil {
		t.Fatal("stale generation binding must error (fail-closed)")
	}

	// retired profile: binding to retired profile must error.
	if _, err := auth.RetireProfile(context.Background(), permTestProfileA, "op-retire", permTestCorrelation); err != nil {
		t.Fatalf("RetireProfile() error = %v", err)
	}
	projection2 := replayOf(t, allEvents(t, store))
	if _, err := ResolveEffectiveProfile(projection2, permTestJobA); err == nil {
		t.Fatal("binding to retired profile must error (fail-closed)")
	}
}

func TestRed08_AdminLockBlocksBypass(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	if _, err := auth.SetAdminLock(context.Background(), true, "", "op-lock", permTestCorrelation); err == nil {
		t.Fatal("SetAdminLock(true) with empty authorized_by must error")
	}
	if _, err := auth.SetAdminLock(context.Background(), true, "owner-1", "op-lock", permTestCorrelation); err != nil {
		t.Fatalf("SetAdminLock(true) error = %v", err)
	}
	if _, err := auth.ActivateMode(context.Background(), ModeBypassPermissions, ScopeRoot, "global", "user-1", "op-bypass", permTestCorrelation); err == nil {
		t.Fatal("ActivateMode(bypass) with admin lock must error")
	}

	projection := replayOf(t, allEvents(t, store))
	effective, err := ResolveEffectiveProfile(projection, "unbound-job")
	if err != nil {
		t.Fatalf("EffectiveProfile() error = %v", err)
	}
	effective.Mode = ModeBypassPermissions
	verdict := verdictFor(t, effective, ProposedCall{Tool: ToolRead, Path: "src/main.go"})
	if verdict != VerdictDeny {
		t.Fatalf("verdict = %s, want deny (admin lock blocks bypass)", verdict)
	}
}

func TestRed11_GrantRevocationRestartIdempotency(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)

	grant := Grant{
		GrantID: "g-go-test", Scope: ScopeJob, ScopeID: permTestJobA,
		Tool: ToolBash, Pattern: "go test *", IssuedAt: "2026-08-05T12:00:00Z",
	}
	if _, err := auth.IssueGrant(context.Background(), grant, "op-grant", permTestCorrelation); err != nil {
		t.Fatalf("IssueGrant() error = %v", err)
	}
	projection := replayOf(t, allEvents(t, store))
	effective := effectiveFor(t, projection, permTestJobA)
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "go test ./..."}); got != VerdictAllow {
		t.Fatalf("verdict = %s, want allow with grant", got)
	}
	if _, err := auth.RevokeGrant(context.Background(), "g-go-test", "op-revoke-grant", permTestCorrelation); err != nil {
		t.Fatalf("RevokeGrant() error = %v", err)
	}
	projection2 := replayOf(t, allEvents(t, store))
	effective2 := effectiveFor(t, projection2, permTestJobA)
	if got := verdictFor(t, effective2, ProposedCall{Tool: ToolBash, Command: "go test ./..."}); got != VerdictAsk {
		t.Fatalf("verdict = %s, want ask after grant revocation", got)
	}

	// restart/idempotency: same operationID replayed twice must not duplicate facts.
	before := len(allEvents(t, store))
	if _, err := auth.RecordDecision(context.Background(), permTestJobA,
		ProposedCall{Tool: ToolBash, Command: "curl https://example.com"},
		VerdictAsk, Denial{Reason: "x"}, "", "op-decision-dup", permTestCorrelation); err != nil {
		t.Fatalf("RecordDecision() error = %v", err)
	}
	afterOnce := len(allEvents(t, store))
	if _, err := auth.RecordDecision(context.Background(), permTestJobA,
		ProposedCall{Tool: ToolBash, Command: "curl https://example.com"},
		VerdictAsk, Denial{Reason: "x"}, "", "op-decision-dup", permTestCorrelation); err != nil {
		t.Fatalf("RecordDecision() second call error = %v", err)
	}
	if afterOnce != before+1 {
		t.Fatalf("events after first RecordDecision = %d, want %d", afterOnce, before+1)
	}
	if got := len(allEvents(t, store)); got != afterOnce {
		t.Fatalf("events after idempotent replay = %d, want %d (no duplicate)", got, afterOnce)
	}
}

func TestRed12_ParallelJobIsolation(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileB, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	mustBind(t, auth, permTestJobB, permTestProfileB)
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-a-allow-test", Scope: ScopeJob, ScopeID: permTestJobA,
		Action: ActionAllow, Tool: ToolBash, Pattern: "go test *",
	}, "user-1", "op-rule-a", permTestCorrelation); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-b-deny-test", Scope: ScopeJob, ScopeID: permTestJobB,
		Action: ActionDeny, Tool: ToolBash, Pattern: "go test *",
	}, "user-1", "op-rule-b", permTestCorrelation); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	projection := replayOf(t, allEvents(t, store))
	call := ProposedCall{Tool: ToolBash, Command: "go test ./..."}
	if got := verdictFor(t, effectiveFor(t, projection, permTestJobA), call); got != VerdictAllow {
		t.Fatalf("job A verdict = %s, want allow", got)
	}
	if got := verdictFor(t, effectiveFor(t, projection, permTestJobB), call); got != VerdictDeny {
		t.Fatalf("job B verdict = %s, want deny (job rules must be isolated)", got)
	}
}

func TestRed19_PersonalAndProjectScopeApplyToJob(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-personal-allow", Scope: ScopePersonal, ScopeID: "user-1",
		Action: ActionAllow, Tool: ToolBash, Pattern: "go run *",
	}, "user-1", "op-rule-personal", permTestCorrelation); err != nil {
		t.Fatalf("AddRule(personal) error = %v", err)
	}
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-project-deny", Scope: ScopeProject, ScopeID: "project-1",
		Action: ActionDeny, Tool: ToolBash, Pattern: "rm *",
	}, "user-1", "op-rule-project", permTestCorrelation); err != nil {
		t.Fatalf("AddRule(project) error = %v", err)
	}
	projection := replayOf(t, allEvents(t, store))
	effective := effectiveFor(t, projection, permTestJobA)
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "go run cmd/main.go"}); got != VerdictAllow {
		t.Fatalf("personal allow verdict = %s, want allow", got)
	}
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "rm -rf vendor"}); got != VerdictDeny {
		t.Fatalf("project deny verdict = %s, want deny", got)
	}
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "chmod +x run.sh"}); got != VerdictAsk {
		t.Fatalf("dangerous without deny verdict = %s, want ask", got)
	}
}

func TestRed13_BypassActivationRequiresExplicitUser(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	if _, err := auth.ActivateMode(context.Background(), ModeBypassPermissions, ScopeRoot, "global", "", "op-bypass", permTestCorrelation); err == nil {
		t.Fatal("ActivateMode(bypass) with empty authorized_by must error")
	}
	if _, err := auth.ActivateMode(context.Background(), ModeBypassPermissions, ScopeRoot, "global", "user-1", "op-bypass", permTestCorrelation); err != nil {
		t.Fatalf("ActivateMode(bypass) error = %v", err)
	}
	projection := replayOf(t, allEvents(t, store))
	activation, ok := projection.Activations["root/global"]
	if !ok {
		t.Fatal("activation fact missing after ActivateMode")
	}
	if activation.Mode != ModeBypassPermissions {
		t.Fatalf("activation mode = %s, want bypass", activation.Mode)
	}
}

func TestRed16_ConfigFileImportOnly(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeDefault})
	mustBind(t, auth, permTestJobA, permTestProfileA)

	// No rule was added to the Journal: a hypothetical project config file
	// (never imported) must have zero effect.
	projection := replayOf(t, allEvents(t, store))
	effective := effectiveFor(t, projection, permTestJobA)
	if len(effective.MergedRules) != 0 {
		t.Fatalf("unimported config must have zero effect, got rules %+v", effective.MergedRules)
	}
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "curl https://example.com"}); got != VerdictAsk {
		t.Fatalf("verdict = %s, want ask (no implicit allow)", got)
	}
}

func TestRed17_RootHumanGateAndDangerousGrantRejected(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-root", Scope: ScopeRoot, ScopeID: "global",
		Action: ActionDeny, Tool: ToolBash, Pattern: "git push *",
	}, "", "op-root-rule", permTestCorrelation); err == nil {
		t.Fatal("root-scope rule without authorized_by must error")
	}
	if _, err := auth.AddRule(context.Background(), Rule{
		RuleID: "r-root-ok", Scope: ScopeRoot, ScopeID: "global",
		Action: ActionDeny, Tool: ToolBash, Pattern: "git push *",
	}, "owner-1", "op-root-rule-ok", permTestCorrelation); err != nil {
		t.Fatalf("root-scope rule with authorized_by error = %v", err)
	}
	grant := Grant{
		GrantID: "g-danger", Scope: ScopeProject, ScopeID: permTestProject,
		Tool: ToolBash, Pattern: "rm -rf *", IssuedAt: "2026-08-05T12:00:00Z",
	}
	if _, err := auth.IssueGrant(context.Background(), grant, "op-danger-grant", permTestCorrelation); err == nil {
		t.Fatal("IssueGrant with dangerous pattern must be rejected")
	}
}

func TestCompileProfileCanonicalDigest(t *testing.T) {
	input := ProfileInput{
		ProfileID: permTestProfileA, Mode: ModeAuto,
		Rules: []Rule{{RuleID: "r1", Scope: ScopeJob, ScopeID: permTestJobA, Action: ActionAllow, Tool: ToolBash, Pattern: "go test *"}},
	}
	first, err := CompileProfile(input)
	if err != nil {
		t.Fatalf("CompileProfile() error = %v", err)
	}
	second, err := CompileProfile(input)
	if err != nil {
		t.Fatalf("CompileProfile() second error = %v", err)
	}
	if first.Digest != second.Digest || first.Digest == "" {
		t.Fatalf("digest not canonical: %q vs %q", first.Digest, second.Digest)
	}
	if first.Generation != 1 {
		t.Fatalf("generation = %d, want 1", first.Generation)
	}
}

func TestProjectionDeepEqualityAfterReplay(t *testing.T) {
	store := openPermStore(t)
	auth := mustPermAuthority(t, store)
	mustProfile(t, auth, ProfileInput{ProfileID: permTestProfileA, Mode: ModeAuto, OwnedPaths: []string{"src/**"}})
	mustBind(t, auth, permTestJobA, permTestProfileA)
	events := allEvents(t, store)
	first := replayOf(t, events)
	second := replayOf(t, events)
	if !reflect.DeepEqual(first.Profiles, second.Profiles) {
		t.Fatal("profile rebuild differs across replays")
	}
	if !reflect.DeepEqual(first.Bindings, second.Bindings) {
		t.Fatal("binding rebuild differs across replays")
	}
}
