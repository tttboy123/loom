package projection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type errorEventSource struct {
	err error
}

func TestP3AGlobalReadViewExposesTypedEvolutionAssetCopies(t *testing.T) {
	viewType := reflect.TypeOf(GlobalReadView{})
	for _, method := range []string{
		"EvolutionAssetDefinition",
		"EvolutionAssetDefinitions",
		"EvolutionAssetRevision",
		"EvolutionAssetCandidate",
		"EvolutionAssetEvaluation",
		"EvolutionAssetBinding",
		"RuntimeSkillMaterialization",
	} {
		if _, found := viewType.MethodByName(method); !found {
			t.Fatalf("GlobalReadView.%s is missing", method)
		}
	}
}

func mustBuildGlobalReadView(
	t *testing.T,
	snapshot Snapshot,
	events []journal.Event,
	teamExecutions map[string]TeamExecution,
) GlobalReadView {
	t.Helper()
	view, err := buildGlobalReadView(snapshot, events, teamExecutions)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func (source errorEventSource) Events(context.Context) ([]journal.Event, error) {
	return nil, source.err
}

func TestGlobalReadViewPublishesAtomicallyWithCanonicalVersionAndLocalCopies(t *testing.T) {
	create := projectionEvent(
		"evt-work-create",
		"view-stream",
		1,
		"idem-work-create",
		"WorkItemCreated",
		map[string]string{
			"work_item_id": "work-1",
			"title":        "View boundary",
			"status":       "open",
		},
	)
	projection := newForTestSource(eventSliceSource{events: []journal.Event{create}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	view := projection.GlobalReadView()
	sum := sha256.Sum256([]byte("view-stream\x001\x00evt-work-create\n"))
	if got, want := view.Version(), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
	head, ok := view.Head("view-stream")
	if !ok || head.Sequence != 1 || head.EventID != create.ID {
		t.Fatalf("Head() = %#v, %v", head, ok)
	}
	workItem, ok := view.WorkItem("work-1")
	if !ok || workItem.Title != "View boundary" {
		t.Fatalf("WorkItem() = %#v, %v", workItem, ok)
	}
	workItem.Title = "mutated"
	if again, _ := view.WorkItem("work-1"); again.Title != "View boundary" {
		t.Fatalf("view record mutation escaped: %#v", again)
	}

	failing := newForTestSource(errorEventSource{err: errors.New("source failed")})
	failing.snapshot = projection.Snapshot()
	failing.view = view
	if err := failing.Rebuild(context.Background()); err == nil {
		t.Fatal("failing Rebuild() error = nil")
	}
	if got := failing.GlobalReadView().Version(); got != view.Version() {
		t.Fatalf("failed rebuild replaced old view: %q != %q", got, view.Version())
	}
}

func TestEmptyGlobalReadViewUsesEmptySHA256(t *testing.T) {
	projection := newForTestSource(eventSliceSource{})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(nil)
	if got, want := projection.GlobalReadView().Version(), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("empty version = %q, want %q", got, want)
	}
}

func TestGlobalReadViewReturnsLatestAgentGrantForRunBySequence(t *testing.T) {
	issuedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	snapshot := emptySnapshot()
	snapshot.AgentGrants["grant-old"] = AgentGrant{
		ID:                "grant-old",
		RunID:             "run-1",
		AllowedOperations: []string{"bridge.event"},
		IssuedAt:          issuedAt,
	}
	snapshot.AgentGrants["grant-new"] = AgentGrant{
		ID:                "grant-new",
		RunID:             "run-1",
		AllowedOperations: []string{"bridge.event", "bridge.result"},
		IssuedAt:          issuedAt.Add(time.Second),
		RevokedAt:         issuedAt.Add(2 * time.Second),
		RevocationReason:  "operator",
	}
	issuePayload := func(grantID string, issued time.Time) map[string]any {
		return map[string]any{
			"grant_id":            grantID,
			"work_item_id":        "work-1",
			"run_id":              "run-1",
			"claim_id":            "11111111-1111-4111-8111-111111111111",
			"claim_generation":    1,
			"runtime_instance_id": "runtime-1",
			"agent_instance_id":   "agent-1",
			"allowed_operations":  []string{"bridge.event"},
			"token_hash":          strings.Repeat("a", 64),
			"issued_at":           issued.Format(time.RFC3339Nano),
			"expires_at":          issued.Add(time.Minute).Format(time.RFC3339Nano),
			"run_stream":          "run/run-1",
			"run_sequence":        1,
			"run_event_id":        "run-claimed",
		}
	}
	events := []journal.Event{
		teamProjectionEvent(
			t,
			"grant-old-issued",
			"agent-grant/run-1",
			1,
			"grant-old-issued",
			"AgentGrantIssued",
			issuePayload("grant-old", issuedAt),
		),
		teamProjectionEvent(
			t,
			"grant-new-issued",
			"agent-grant/run-1",
			2,
			"grant-new-issued",
			"AgentGrantIssued",
			issuePayload("grant-new", issuedAt.Add(time.Second)),
		),
	}
	view := mustBuildGlobalReadView(t, snapshot, events, nil)
	latest, ok := view.LatestAgentGrantForRun("run-1")
	if !ok || latest.ID != "grant-new" ||
		latest.RevocationReason != "operator" {
		t.Fatalf("LatestAgentGrantForRun() = %#v, %v", latest, ok)
	}
	latest.AllowedOperations[0] = "mutated"
	again, _ := view.LatestAgentGrantForRun("run-1")
	if again.AllowedOperations[0] != "bridge.event" {
		t.Fatal("latest AgentGrant aliases caller mutation")
	}
}

func TestGlobalReadViewReturnsOnlyRequestedTeamRecordsAsStableCopies(t *testing.T) {
	issuedAt := time.Date(2026, 7, 26, 13, 0, 0, 0, time.UTC)
	snapshot := emptySnapshot()
	snapshot.WorkItems["work-b"] = WorkItem{
		ID:             "work-b",
		TeamInstanceID: "team-1",
		LogicalNodeID:  "node-b",
	}
	snapshot.WorkItems["work-a"] = WorkItem{
		ID:             "work-a",
		TeamInstanceID: "team-1",
		LogicalNodeID:  "node-a",
	}
	snapshot.WorkItems["work-other"] = WorkItem{
		ID:             "work-other",
		TeamInstanceID: "team-2",
	}
	snapshot.ApprovalRequests["approval-b"] = ProjectedApprovalRequest{
		ID:             "approval-b",
		TeamInstanceID: "team-1",
		ApproverRefs:   []string{"operator-b"},
	}
	snapshot.ApprovalRequests["approval-a"] = ProjectedApprovalRequest{
		ID:                "approval-a",
		TeamInstanceID:    "team-1",
		WarningMarkers:    []string{"warning-a"},
		RuleSetReferences: []ProjectedRuleSetReference{{StreamID: "rule-a"}},
	}
	snapshot.ApprovalRequests["approval-other"] = ProjectedApprovalRequest{
		ID:             "approval-other",
		TeamInstanceID: "team-2",
	}
	snapshot.AgentGrants["grant-b"] = AgentGrant{
		ID:                "grant-b",
		RunID:             "run-1",
		IssuedAt:          issuedAt,
		AllowedOperations: []string{"bridge.result"},
	}
	snapshot.AgentGrants["grant-a"] = AgentGrant{
		ID:                "grant-a",
		RunID:             "run-1",
		IssuedAt:          issuedAt,
		AllowedOperations: []string{"bridge.event"},
	}
	snapshot.AgentGrants["grant-other"] = AgentGrant{
		ID:       "grant-other",
		RunID:    "run-2",
		IssuedAt: issuedAt.Add(-time.Second),
	}

	view := mustBuildGlobalReadView(t, snapshot, nil, nil)
	workItems := view.WorkItemsForTeam("team-1")
	if got, want := []string{workItems[0].ID, workItems[1].ID},
		[]string{"work-a", "work-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WorkItemsForTeam() IDs = %v, want %v", got, want)
	}
	approvals := view.ApprovalRequestsForTeam("team-1")
	if got, want := []string{approvals[0].ID, approvals[1].ID},
		[]string{"approval-a", "approval-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ApprovalRequestsForTeam() IDs = %v, want %v", got, want)
	}
	grants := view.AgentGrantsForRun("run-1")
	if got, want := []string{grants[0].ID, grants[1].ID},
		[]string{"grant-a", "grant-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AgentGrantsForRun() IDs = %v, want %v", got, want)
	}

	workItems[0].Status = "mutated"
	approvals[0].WarningMarkers[0] = "mutated"
	approvals[0].RuleSetReferences[0].StreamID = "mutated"
	grants[0].AllowedOperations[0] = "mutated"
	if again := view.WorkItemsForTeam("team-1"); again[0].Status == "mutated" {
		t.Fatal("WorkItemsForTeam aliases caller mutation")
	}
	againApprovals := view.ApprovalRequestsForTeam("team-1")
	if againApprovals[0].WarningMarkers[0] != "warning-a" ||
		againApprovals[0].RuleSetReferences[0].StreamID != "rule-a" {
		t.Fatal("ApprovalRequestsForTeam aliases nested caller mutation")
	}
	if again := view.AgentGrantsForRun("run-1"); again[0].AllowedOperations[0] != "bridge.event" {
		t.Fatal("AgentGrantsForRun aliases caller mutation")
	}

	if got := view.WorkItemsForTeam(""); got == nil || len(got) != 0 {
		t.Fatalf("empty WorkItemsForTeam = %#v, want non-nil empty", got)
	}
	if got := view.ApprovalRequestsForTeam("missing"); got == nil || len(got) != 0 {
		t.Fatalf("missing ApprovalRequestsForTeam = %#v, want non-nil empty", got)
	}
	if got := view.AgentGrantsForRun("missing"); got == nil || len(got) != 0 {
		t.Fatalf("missing AgentGrantsForRun = %#v, want non-nil empty", got)
	}
}

func TestGlobalReadViewReturnsBoundedStableProductPages(t *testing.T) {
	snapshot := emptySnapshot()
	snapshot.Teams["team-c"] = TeamInstance{
		ID:               "team-c",
		DormantSubAgents: []DormantSubAgent{{AgentDefinitionID: "agent-c"}},
	}
	snapshot.Teams["team-a"] = TeamInstance{ID: "team-a"}
	snapshot.Teams["team-b"] = TeamInstance{ID: "team-b"}
	snapshot.Runs["run-b"] = Run{ID: "run-b", Phase: "running"}
	snapshot.Runs["run-a"] = Run{ID: "run-a", Phase: "terminal"}
	snapshot.Evidence["evidence-b"] = Evidence{ID: "evidence-b"}
	snapshot.Evidence["evidence-a"] = Evidence{ID: "evidence-a"}
	snapshot.RuntimeInstances["runtime-b"] = RuntimeInstance{
		ID:                   "runtime-b",
		ModelIDs:             []string{"provider/model-b"},
		ObservedCapabilities: []string{"streaming"},
	}
	snapshot.RuntimeInstances["runtime-a"] = RuntimeInstance{
		ID: "runtime-a",
	}
	executions := map[string]TeamExecution{
		"team-c": {
			TeamInstanceID: "team-c",
			Status:         "succeeded",
			Nodes: []TeamExecutionNode{{
				LogicalNodeID: "main",
				Attempts: []TeamExecutionAttempt{{
					AttemptNumber: 1,
				}},
			}},
		},
		"team-a": {TeamInstanceID: "team-a", Status: "planned"},
	}
	view := mustBuildGlobalReadView(t, snapshot, nil, executions)

	teams, more := view.Teams("", 2)
	if got, want := []string{teams[0].ID, teams[1].ID},
		[]string{"team-a", "team-b"}; !reflect.DeepEqual(got, want) || !more {
		t.Fatalf("Teams() IDs=%v more=%v, want %v true", got, more, want)
	}
	teams, more = view.Teams("team-b", 2)
	if len(teams) != 1 || teams[0].ID != "team-c" || more {
		t.Fatalf("Teams(after) = %#v, %v", teams, more)
	}
	teams[0].DormantSubAgents[0].AgentDefinitionID = "mutated"
	again, _ := view.Teams("team-b", 2)
	if again[0].DormantSubAgents[0].AgentDefinitionID != "agent-c" {
		t.Fatal("Teams() aliases nested caller mutation")
	}

	runs, _ := view.Runs("", 64)
	evidence, _ := view.EvidenceRecords("", 64)
	runtimes, _ := view.RuntimeInstances("", 64)
	teamExecutions, _ := view.TeamExecutions("", 64)
	if got := []string{runs[0].ID, runs[1].ID}; !reflect.DeepEqual(got, []string{"run-a", "run-b"}) {
		t.Fatalf("Runs() IDs = %v", got)
	}
	if got := []string{evidence[0].ID, evidence[1].ID}; !reflect.DeepEqual(got, []string{"evidence-a", "evidence-b"}) {
		t.Fatalf("EvidenceRecords() IDs = %v", got)
	}
	if got := []string{runtimes[0].ID, runtimes[1].ID}; !reflect.DeepEqual(got, []string{"runtime-a", "runtime-b"}) {
		t.Fatalf("RuntimeInstances() IDs = %v", got)
	}
	if got := []string{teamExecutions[0].TeamInstanceID, teamExecutions[1].TeamInstanceID}; !reflect.DeepEqual(got, []string{"team-a", "team-c"}) {
		t.Fatalf("TeamExecutions() IDs = %v", got)
	}
	runtimes[1].ModelIDs[0] = "mutated"
	againRuntimes, _ := view.RuntimeInstances("", 64)
	if againRuntimes[1].ModelIDs[0] != "provider/model-b" {
		t.Fatal("RuntimeInstances() aliases caller mutation")
	}
	teamExecutions[1].Nodes[0].Attempts[0].AttemptNumber = 99
	againExecutions, _ := view.TeamExecutions("", 64)
	if againExecutions[1].Nodes[0].Attempts[0].AttemptNumber != 1 {
		t.Fatal("TeamExecutions() aliases caller mutation")
	}

	for _, invalid := range []struct {
		name string
		call func() int
	}{
		{name: "teams-zero", call: func() int { got, _ := view.Teams("", 0); return len(got) }},
		{name: "runs-high", call: func() int { got, _ := view.Runs("", 65); return len(got) }},
		{name: "evidence-invalid-after", call: func() int { got, _ := view.EvidenceRecords("\n", 1); return len(got) }},
		{name: "runtimes-invalid-after", call: func() int { got, _ := view.RuntimeInstances(" ", 1); return len(got) }},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			if got := invalid.call(); got != 0 {
				t.Fatalf("invalid page length = %d, want 0", got)
			}
		})
	}
}

func TestGlobalReadViewArchivedTeamDefinitionIsNotExecutable(t *testing.T) {
	snapshot := emptySnapshot()
	snapshot.TeamDefinitions = make(map[string]TeamDefinitionRecord)
	snapshot.TeamDefinitions["team-def-active"] = TeamDefinitionRecord{
		ID: "team-def-active", Status: "active",
	}
	snapshot.TeamDefinitions["team-def-archived"] = TeamDefinitionRecord{
		ID: "team-def-archived", Status: "archived",
	}
	snapshot.Teams["team-active"] = TeamInstance{
		ID: "team-active", TeamDefinitionID: "team-def-active",
	}
	snapshot.Teams["team-archived"] = TeamInstance{
		ID: "team-archived", TeamDefinitionID: "team-def-archived",
	}
	view := mustBuildGlobalReadView(t, snapshot, nil, nil)

	active, ok := view.TeamTimelineAnchor("team-active")
	if !ok || !active.Confirmed || !active.Executable || active.ReadOnly {
		t.Fatalf("active anchor = %#v, %v", active, ok)
	}
	archived, ok := view.TeamTimelineAnchor("team-archived")
	if !ok || !archived.Confirmed || archived.Executable || archived.ReadOnly {
		t.Fatalf("archived anchor must stay visible but be non-executable: %#v, %v", archived, ok)
	}
}

func TestGlobalReadViewTeamTimelineAnchorIsExactReadOnlyAndTerminal(t *testing.T) {
	snapshot := emptySnapshot()
	snapshot.Teams["team-saved"] = TeamInstance{ID: "team-saved"}
	terminal := TeamExecution{
		TeamInstanceID: "team-legacy",
		Status:         "succeeded",
		Nodes: []TeamExecutionNode{{
			LogicalNodeID: "main",
			Attempts: []TeamExecutionAttempt{{
				AttemptNumber: 1,
				WorkItemID:    "work-1",
				RunID:         "run-1",
			}},
		}},
	}
	events := []journal.Event{{
		ID:       "event-team-legacy-terminal",
		StreamID: "team-execution/team-legacy",
		Seq:      1,
	}}
	view := mustBuildGlobalReadView(t,
		snapshot,
		events,
		map[string]TeamExecution{
			"team-legacy": terminal,
			"team-saved": {
				TeamInstanceID: "team-saved",
				Status:         "planned",
			},
		},
	)

	saved, ok := view.TeamTimelineAnchor("team-saved")
	if !ok || saved.Kind != "saved_team" || !saved.Confirmed ||
		!saved.Executable || saved.ReadOnly {
		t.Fatalf("saved anchor = %#v, %v", saved, ok)
	}
	legacy, ok := view.TeamTimelineAnchor("team-legacy")
	if !ok ||
		legacy.TeamInstanceID != "team-legacy" ||
		legacy.Kind != "historical_execution_only" ||
		legacy.Confirmed ||
		legacy.Executable ||
		!legacy.ReadOnly {
		t.Fatalf("legacy anchor = %#v, %v", legacy, ok)
	}

	for _, test := range []struct {
		name      string
		execution TeamExecution
		events    []journal.Event
	}{
		{name: "missing-head", execution: terminal},
		{
			name: "nonterminal",
			execution: TeamExecution{
				TeamInstanceID: "team-legacy",
				Status:         "running",
				Nodes:          terminal.Nodes,
			},
			events: events,
		},
		{
			name: "no-attempt",
			execution: TeamExecution{
				TeamInstanceID: "team-legacy",
				Status:         "failed",
				Nodes:          []TeamExecutionNode{{LogicalNodeID: "main"}},
			},
			events: events,
		},
		{
			name: "mismatched-id",
			execution: TeamExecution{
				TeamInstanceID: "other-team",
				Status:         "failed",
				Nodes:          terminal.Nodes,
			},
			events: events,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := mustBuildGlobalReadView(t,
				emptySnapshot(),
				test.events,
				map[string]TeamExecution{"team-legacy": test.execution},
			)
			if got, ok := candidate.TeamTimelineAnchor("team-legacy"); ok {
				t.Fatalf("invalid anchor = %#v, want rejected", got)
			}
		})
	}
}
