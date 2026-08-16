package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/mode"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"

	_ "modernc.org/sqlite"
)

func TestCommitSavedTeamInstanceRecordSetPersistsCanonicalEvents(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		roleCount             int
		reusableTeam          bool
		reusableAgentFallback bool
	}{
		{name: "project main only", roleCount: 1},
		{name: "project one dormant", roleCount: 2},
		{name: "project two dormant", roleCount: 3},
		{name: "reusable same id shadow", roleCount: 3, reusableTeam: true},
		{name: "project team reusable agent fallback", roleCount: 3, reusableAgentFallback: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newSavedTeamCommitFixture(
				t, tt.roleCount, tt.reusableTeam, tt.reusableAgentFallback,
			)
			if tt.reusableTeam {
				if len(fixture.catalog.TeamDefinitions) != 2 ||
					fixture.records.Team().TeamDefinitionScope != teams.TeamDefinitionScopeReusable ||
					fixture.records.Team().ScopeIdentity != (agents.ScopeIdentity{}) ||
					fixture.records.MainAgent().AgentDefinitionScope != agents.ScopeReusable {
					t.Fatalf("same-ID project/reusable shadow fixture = %#v", fixture.records)
				}
			}
			store, db := openSavedTeamWriterStore(t)

			got, err := fixture.commitTo(context.Background(), store)
			if err != nil {
				t.Fatalf("CommitSavedTeamInstanceRecordSet() error = %v", err)
			}
			if !got.Committed() || got.EventCount() != 2 ||
				got.SourceRecordSetDigest() != fixture.records.RecordSetDigest() ||
				len(got.CommitDigest()) != 64 {
				t.Fatalf("commit Candidate = %#v", got)
			}

			teamEvent := got.TeamEvent()
			mainEvent := got.MainAgentEvent()
			assertSavedTeamWriterEventEnvelope(t, fixture, teamEvent, mainEvent)
			assertSavedTeamWriterPayloads(t, fixture, teamEvent, mainEvent)
			assertSavedTeamWriterStoredEvents(t, store, fixture, teamEvent, mainEvent)
			if count := savedTeamWriterRowCount(t, db); count != 2 {
				t.Fatalf("row count = %d, want 2", count)
			}
		})
	}
}

func TestCommitSavedTeamInstanceRecordSetExactRetryIsIdempotent(t *testing.T) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	store, db := openSavedTeamWriterStore(t)

	first, err := fixture.commitTo(context.Background(), store)
	if err != nil {
		t.Fatalf("first commit error = %v", err)
	}
	second, err := fixture.commitTo(context.Background(), store)
	if err != nil {
		t.Fatalf("retry commit error = %v", err)
	}
	if !reflect.DeepEqual(second, first) {
		t.Fatalf("retry Candidate = %#v, want %#v", second, first)
	}
	if count := savedTeamWriterRowCount(t, db); count != 2 {
		t.Fatalf("row count after retry = %d, want 2", count)
	}
}

func TestCommitSavedTeamInstanceRecordSetConflictsAreAtomic(t *testing.T) {
	t.Run("idempotency", func(t *testing.T) {
		fixture := newSavedTeamCommitFixture(t, 3, false, false)
		store, db := openSavedTeamWriterStore(t)
		if _, err := fixture.commitTo(context.Background(), store); err != nil {
			t.Fatalf("seed commit error = %v", err)
		}
		changed := fixture
		changed.commit.TeamEventID = "event.team.changed"
		got, err := changed.commitTo(context.Background(), store)
		if !errors.Is(err, journal.ErrIdempotencyConflict) {
			t.Fatalf("changed identity error = %v", err)
		}
		assertZeroSavedTeamCommit(t, got)
		if count := savedTeamWriterRowCount(t, db); count != 2 {
			t.Fatalf("row count = %d, want 2", count)
		}
	})

	t.Run("partial exact batch", func(t *testing.T) {
		fixture := newSavedTeamCommitFixture(t, 3, false, false)
		recorder := &recordingEventBatchAppender{}
		if _, err := fixture.commitTo(context.Background(), recorder); err != nil {
			t.Fatalf("capture error = %v", err)
		}
		store, db := openSavedTeamWriterStore(t)
		if _, err := store.Append(context.Background(), recorder.batches[0][0]); err != nil {
			t.Fatalf("seed Team event error = %v", err)
		}
		got, err := fixture.commitTo(context.Background(), store)
		if !errors.Is(err, journal.ErrPartialEventBatchConflict) {
			t.Fatalf("partial error = %v", err)
		}
		assertZeroSavedTeamCommit(t, got)
		if count := savedTeamWriterRowCount(t, db); count != 1 {
			t.Fatalf("row count = %d, want 1", count)
		}
	})

	t.Run("occupied stream", func(t *testing.T) {
		fixture := newSavedTeamCommitFixture(t, 3, false, false)
		store, db := openSavedTeamWriterStore(t)
		occupied := journal.Event{
			ID:             "event.occupied",
			StreamID:       "team_instance:" + fixture.identity.TeamInstanceID,
			Seq:            1,
			IdempotencyKey: "key.occupied",
			Type:           "Other",
			SchemaVersion:  1,
			EmittedAt:      fixture.commit.EmittedAt,
			PayloadJSON:    []byte(`{"occupied":true}`),
		}
		if _, err := store.Append(context.Background(), occupied); err != nil {
			t.Fatalf("seed occupied stream error = %v", err)
		}
		got, err := fixture.commitTo(context.Background(), store)
		if !errors.Is(err, journal.ErrSequenceConflict) {
			t.Fatalf("occupied stream error = %v", err)
		}
		assertZeroSavedTeamCommit(t, got)
		if count := savedTeamWriterRowCount(t, db); count != 1 {
			t.Fatalf("row count = %d, want 1", count)
		}
	})
}

func TestCommitSavedTeamInstanceRecordSetRejectsInvalidSourcesBeforeAppend(t *testing.T) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	tests := []struct {
		name   string
		change func(*savedTeamCommitFixture)
	}{
		{name: "zero records", change: func(f *savedTeamCommitFixture) {
			f.records = teams.SavedTeamInstanceRecordSetCandidate{}
		}},
		{name: "zero plan", change: func(f *savedTeamCommitFixture) {
			f.plan = teams.SavedTeamInstantiationPlanCandidate{}
		}},
		{name: "zero binding", change: func(f *savedTeamCommitFixture) {
			f.binding = teams.SavedTeamRuntimeBindingCandidate{}
		}},
		{name: "zero discovery", change: func(f *savedTeamCommitFixture) {
			f.discovery = loomruntime.RuntimeDiscoverySnapshot{}
		}},
		{name: "stale intent", change: func(f *savedTeamCommitFixture) {
			f.intent = mode.Intent{Trigger: mode.TriggerAssign, TargetID: "team.delivery"}
		}},
		{name: "stale scope", change: func(f *savedTeamCommitFixture) {
			f.scope = agents.ScopeIdentity{ProjectID: "project.other"}
		}},
		{name: "stale catalog", change: func(f *savedTeamCommitFixture) {
			f.catalog.ProjectDefaultTeamID = "team.other"
		}},
		{name: "stale selection", change: func(f *savedTeamCommitFixture) {
			f.selections[0].RuntimeInstanceID = "runtime.other"
		}},
		{name: "stale identity", change: func(f *savedTeamCommitFixture) {
			f.identity.TeamInstanceID = "team-instance.other"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := fixture.clone()
			tt.change(&current)
			appender := &recordingEventBatchAppender{}
			got, err := current.commitTo(context.Background(), appender)
			if err == nil {
				t.Fatal("error = nil")
			}
			assertZeroSavedTeamCommit(t, got)
			if appender.calls != 0 {
				t.Fatalf("AppendBatch calls = %d, want 0", appender.calls)
			}
		})
	}
}

func TestCommitSavedTeamInstanceRecordSetRejectsInvalidCommitAndAppenderFailures(t *testing.T) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	invalid := []struct {
		name   string
		change func(*SavedTeamCommitInput)
	}{
		{name: "empty Team event id", change: func(c *SavedTeamCommitInput) { c.TeamEventID = "" }},
		{name: "empty Team key", change: func(c *SavedTeamCommitInput) { c.TeamIdempotencyKey = "" }},
		{name: "empty Main event id", change: func(c *SavedTeamCommitInput) { c.MainAgentEventID = "" }},
		{name: "empty Main key", change: func(c *SavedTeamCommitInput) { c.MainAgentIdempotencyKey = "" }},
		{name: "duplicate event ids", change: func(c *SavedTeamCommitInput) { c.MainAgentEventID = c.TeamEventID }},
		{name: "duplicate keys", change: func(c *SavedTeamCommitInput) { c.MainAgentIdempotencyKey = c.TeamIdempotencyKey }},
		{name: "zero time", change: func(c *SavedTeamCommitInput) { c.EmittedAt = time.Time{} }},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			current := fixture.clone()
			tt.change(&current.commit)
			appender := &recordingEventBatchAppender{}
			got, err := current.commitTo(context.Background(), appender)
			if !errors.Is(err, ErrInvalidSavedTeamCommitInput) {
				t.Fatalf("error = %v", err)
			}
			assertZeroSavedTeamCommit(t, got)
			if appender.calls != 0 {
				t.Fatalf("AppendBatch calls = %d, want 0", appender.calls)
			}
		})
	}

	t.Run("nil and typed nil appender", func(t *testing.T) {
		for _, appender := range []EventBatchAppender{
			nil,
			(*recordingEventBatchAppender)(nil),
		} {
			got, err := fixture.commitTo(context.Background(), appender)
			if !errors.Is(err, ErrInvalidSavedTeamCommitInput) {
				t.Fatalf("error = %v", err)
			}
			assertZeroSavedTeamCommit(t, got)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		appender := &recordingEventBatchAppender{}
		got, err := fixture.commitTo(ctx, appender)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
		assertZeroSavedTeamCommit(t, got)
		if appender.calls != 0 {
			t.Fatalf("AppendBatch calls = %d, want 0", appender.calls)
		}
	})

	t.Run("append failure", func(t *testing.T) {
		appendErr := errors.New("append failed")
		appender := &recordingEventBatchAppender{err: appendErr}
		got, err := fixture.commitTo(context.Background(), appender)
		if !errors.Is(err, appendErr) {
			t.Fatalf("error = %v", err)
		}
		assertZeroSavedTeamCommit(t, got)
		if appender.calls != 1 {
			t.Fatalf("AppendBatch calls = %d, want 1", appender.calls)
		}
	})

	t.Run("closed database", func(t *testing.T) {
		store, db := openSavedTeamWriterStore(t)
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		got, err := fixture.commitTo(context.Background(), store)
		if err == nil {
			t.Fatal("error = nil")
		}
		assertZeroSavedTeamCommit(t, got)
	})
}

func TestCommitSavedTeamInstanceRecordSetRejectsMismatchedAppenderResult(t *testing.T) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	for _, tt := range []struct {
		name   string
		result func([]journal.Event) []journal.Event
	}{
		{name: "nil", result: func([]journal.Event) []journal.Event { return nil }},
		{name: "short", result: func(input []journal.Event) []journal.Event { return cloneStateEvents(input[:1]) }},
		{name: "long", result: func(input []journal.Event) []journal.Event {
			return append(cloneStateEvents(input), cloneStateEvent(input[0]))
		}},
		{name: "reordered", result: func(input []journal.Event) []journal.Event {
			return []journal.Event{cloneStateEvent(input[1]), cloneStateEvent(input[0])}
		}},
		{name: "mutated envelope", result: func(input []journal.Event) []journal.Event {
			result := cloneStateEvents(input)
			result[0].ID = "mutated"
			return result
		}},
		{name: "mutated payload", result: func(input []journal.Event) []journal.Event {
			result := cloneStateEvents(input)
			result[1].PayloadJSON[0] = '['
			return result
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			appender := &recordingEventBatchAppender{result: tt.result}
			got, err := fixture.commitTo(context.Background(), appender)
			if !errors.Is(err, ErrSavedTeamCommitResultMismatch) {
				t.Fatalf("error = %v", err)
			}
			assertZeroSavedTeamCommit(t, got)
			if appender.calls != 1 {
				t.Fatalf("AppendBatch calls = %d, want 1", appender.calls)
			}
		})
	}
}

func TestCommitSavedTeamInstanceRecordSetDigestAndMutationIsolation(t *testing.T) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	appender := &recordingEventBatchAppender{}
	first, err := fixture.commitTo(context.Background(), appender)
	if err != nil {
		t.Fatalf("commit error = %v", err)
	}

	originalTeamPayload := append([]byte(nil), first.TeamEvent().PayloadJSON...)
	originalMainPayload := append([]byte(nil), first.MainAgentEvent().PayloadJSON...)
	appender.batches[0][0].PayloadJSON[0] = '['
	appender.batches[0][1].PayloadJSON[0] = '['
	returnedTeam := first.TeamEvent()
	returnedMain := first.MainAgentEvent()
	returnedTeam.PayloadJSON[0] = '['
	returnedMain.PayloadJSON[0] = '['
	if !reflect.DeepEqual(first.TeamEvent().PayloadJSON, originalTeamPayload) ||
		!reflect.DeepEqual(first.MainAgentEvent().PayloadJSON, originalMainPayload) {
		t.Fatal("input or accessor mutation changed commit Candidate")
	}

	equalAppender := &recordingEventBatchAppender{}
	equal, err := fixture.commitTo(context.Background(), equalAppender)
	if err != nil {
		t.Fatalf("equal commit error = %v", err)
	}
	if equal.CommitDigest() != first.CommitDigest() ||
		!reflect.DeepEqual(equal.TeamEvent().PayloadJSON, originalTeamPayload) ||
		!reflect.DeepEqual(equal.MainAgentEvent().PayloadJSON, originalMainPayload) {
		t.Fatal("equal semantics did not produce equal payloads and digest")
	}

	eventMutations := []struct {
		name   string
		change func(*SavedTeamCommitCandidate)
	}{
		{name: "Team id", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.ID += ".changed" }},
		{name: "Team stream", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.StreamID += ".changed" }},
		{name: "Team sequence", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.Seq++ }},
		{name: "Team idempotency", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.IdempotencyKey += ".changed" }},
		{name: "Team type", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.Type += ".changed" }},
		{name: "Team schema", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.SchemaVersion++ }},
		{name: "Team timestamp", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.EmittedAt = c.teamEvent.EmittedAt.Add(time.Nanosecond) }},
		{name: "Team correlation", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.CorrelationID += ".changed" }},
		{name: "Team causation", change: func(c *SavedTeamCommitCandidate) { c.teamEvent.CausationID = "changed" }},
		{name: "Team payload", change: func(c *SavedTeamCommitCandidate) {
			c.teamEvent.PayloadJSON = append(c.teamEvent.PayloadJSON, ' ')
		}},
		{name: "Main id", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.ID += ".changed" }},
		{name: "Main stream", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.StreamID += ".changed" }},
		{name: "Main sequence", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.Seq++ }},
		{name: "Main idempotency", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.IdempotencyKey += ".changed" }},
		{name: "Main type", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.Type += ".changed" }},
		{name: "Main schema", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.SchemaVersion++ }},
		{name: "Main timestamp", change: func(c *SavedTeamCommitCandidate) {
			c.mainAgentEvent.EmittedAt = c.mainAgentEvent.EmittedAt.Add(time.Nanosecond)
		}},
		{name: "Main correlation", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.CorrelationID += ".changed" }},
		{name: "Main causation", change: func(c *SavedTeamCommitCandidate) { c.mainAgentEvent.CausationID += ".changed" }},
		{name: "Main payload", change: func(c *SavedTeamCommitCandidate) {
			c.mainAgentEvent.PayloadJSON = append(c.mainAgentEvent.PayloadJSON, ' ')
		}},
		{name: "source record-set digest", change: func(c *SavedTeamCommitCandidate) {
			c.sourceRecordSetDigest = "changed"
		}},
		{name: "event count", change: func(c *SavedTeamCommitCandidate) { c.eventCount++ }},
	}
	for _, tt := range eventMutations {
		t.Run("digest field/"+tt.name, func(t *testing.T) {
			changed := cloneSavedTeamCommitCandidate(first)
			tt.change(&changed)
			digest, digestErr := digestSavedTeamCommitCandidate(changed)
			if digestErr != nil {
				t.Fatalf("digest error = %v", digestErr)
			}
			if digest == first.CommitDigest() {
				t.Fatalf("%s preserved commit digest", tt.name)
			}
		})
	}

	semanticVariants := []struct {
		name    string
		fixture savedTeamCommitFixture
	}{
		{
			name: "timestamp",
			fixture: mutateSavedTeamWriterFixture(fixture, func(f *savedTeamCommitFixture) {
				f.commit.EmittedAt = f.commit.EmittedAt.Add(time.Second)
			}),
		},
		{
			name:    "dormant member and source record digest",
			fixture: newSavedTeamCommitFixture(t, 2, false, false),
		},
		{
			name:    "Main definition scope and same-ID Team shadow",
			fixture: newSavedTeamCommitFixture(t, 3, true, false),
		},
		{
			name: "Main definition version",
			fixture: newSavedTeamCommitFixtureWithOptions(t, savedTeamWriterFixtureOptions{
				roleCount:             3,
				mainDefinitionVersion: 2,
			}),
		},
		{
			name: "Runtime binding",
			fixture: newSavedTeamCommitFixtureWithOptions(t, savedTeamWriterFixtureOptions{
				roleCount:         3,
				runtimeInstanceID: "runtime.other",
			}),
		},
		{
			name: "identity",
			fixture: savedTeamWriterFixtureWithIdentity(t, fixture, teams.SavedTeamInstanceIdentityInput{
				WorkRequestID:       "request.other",
				TeamInstanceID:      "team-instance.other",
				MainAgentInstanceID: "agent-instance.other",
				CreatedAt:           fixture.identity.CreatedAt + 1,
			}),
		},
	}
	for _, tt := range semanticVariants {
		t.Run("semantic/"+tt.name, func(t *testing.T) {
			different, commitErr := tt.fixture.commitTo(
				context.Background(), &recordingEventBatchAppender{},
			)
			if commitErr != nil {
				t.Fatalf("changed commit error = %v", commitErr)
			}
			if different.CommitDigest() == first.CommitDigest() {
				t.Fatalf("%s preserved commit digest", tt.name)
			}
			if reflect.DeepEqual(different.TeamEvent().PayloadJSON, originalTeamPayload) &&
				reflect.DeepEqual(different.MainAgentEvent().PayloadJSON, originalMainPayload) &&
				different.TeamEvent().EmittedAt.Equal(first.TeamEvent().EmittedAt) {
				t.Fatalf("%s changed no Event payload or timestamp", tt.name)
			}
		})
	}

	tampered := first
	tampered.commitDigest = "tampered"
	if err := validateSavedTeamCommitCandidate(tampered); !errors.Is(err, ErrSavedTeamCommitDigestMismatch) {
		t.Fatalf("tampered digest error = %v", err)
	}
}

func TestCommitSavedTeamInstanceRecordSetProductionBoundary(t *testing.T) {
	content, err := os.ReadFile("saved_team_writer.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "saved_team_writer.go", content, parser.ImportsOnly,
	)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	allowed := map[string]bool{
		`"bytes"`:                            true,
		`"context"`:                          true,
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"reflect"`:                          true,
		`"time"`:                             true,
		`"loom-pi-rebuild/internal/agents"`:  true,
		`"loom-pi-rebuild/internal/journal"`: true,
		`"loom-pi-rebuild/internal/mode"`:    true,
		`"loom-pi-rebuild/internal/runtime"`: true,
		`"loom-pi-rebuild/internal/teams"`:   true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("saved_team_writer.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

type savedTeamCommitFixture struct {
	intent     mode.Intent
	scope      agents.ScopeIdentity
	catalog    teams.TeamResolutionCatalogInput
	binding    teams.SavedTeamRuntimeBindingCandidate
	discovery  loomruntime.RuntimeDiscoverySnapshot
	selections []teams.SavedTeamRuntimeSelection
	plan       teams.SavedTeamInstantiationPlanCandidate
	identity   teams.SavedTeamInstanceIdentityInput
	records    teams.SavedTeamInstanceRecordSetCandidate
	commit     SavedTeamCommitInput
}

type savedTeamWriterFixtureOptions struct {
	roleCount             int
	reusableTeam          bool
	reusableAgentFallback bool
	mainDefinitionVersion int
	runtimeInstanceID     string
}

func newSavedTeamCommitFixture(
	t *testing.T,
	roleCount int,
	reusableTeam bool,
	reusableAgentFallback bool,
) savedTeamCommitFixture {
	t.Helper()
	return newSavedTeamCommitFixtureWithOptions(t, savedTeamWriterFixtureOptions{
		roleCount:             roleCount,
		reusableTeam:          reusableTeam,
		reusableAgentFallback: reusableAgentFallback,
	})
}

func newSavedTeamCommitFixtureWithOptions(
	t *testing.T,
	options savedTeamWriterFixtureOptions,
) savedTeamCommitFixture {
	t.Helper()
	if options.mainDefinitionVersion == 0 {
		options.mainDefinitionVersion = 1
	}
	if options.runtimeInstanceID == "" {
		options.runtimeInstanceID = "runtime.shared"
	}
	scope := agents.ScopeIdentity{ProjectID: "project.one"}
	projectDefinitions := savedTeamWriterDefinitions(agents.ScopeProject, scope)
	reusableDefinitions := savedTeamWriterDefinitions(agents.ScopeReusable, agents.ScopeIdentity{})
	projectDefinitions[0].Version = options.mainDefinitionVersion
	reusableDefinitions[0].Version = options.mainDefinitionVersion
	definitions := append(append([]agents.AgentDefinition(nil), projectDefinitions...), reusableDefinitions...)
	if options.reusableAgentFallback {
		definitions = reusableDefinitions
	}
	profiles := savedTeamWriterProfiles()
	teamScope := teams.TeamDefinitionScopeProject
	teamScopeIdentity := scope
	if options.reusableTeam {
		teamScope = teams.TeamDefinitionScopeReusable
		teamScopeIdentity = agents.ScopeIdentity{}
	}
	input := teams.TeamDefinitionInput{
		ID:            "team.delivery",
		Version:       1,
		Scope:         teamScope,
		ScopeIdentity: teamScopeIdentity,
		Name:          "Delivery Team",
		Status:        teams.TeamDefinitionActive,
		Roles: []teams.TeamDefinitionRole{
			{Kind: teams.TeamDefinitionRoleMain, AgentDefinitionID: "agent.main", RuntimeProfileID: "profile.main", Responsibility: "coordinate"},
			{Kind: teams.TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent.sub.one", RuntimeProfileID: "profile.sub.one", Responsibility: "implement"},
			{Kind: teams.TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent.sub.two", RuntimeProfileID: "profile.sub.two", Responsibility: "review"},
		}[:options.roleCount],
	}
	team, err := teams.BuildTeamDefinition(input, definitions, profiles)
	if err != nil {
		t.Fatalf("BuildTeamDefinition() error = %v", err)
	}
	teamDefinitions := []teams.TeamDefinition{team}
	if options.reusableTeam {
		projectInput := input
		projectInput.Scope = teams.TeamDefinitionScopeProject
		projectInput.ScopeIdentity = scope
		projectShadow, buildErr := teams.BuildTeamDefinition(
			projectInput, definitions, profiles,
		)
		if buildErr != nil {
			t.Fatalf("BuildTeamDefinition(project shadow) error = %v", buildErr)
		}
		teamDefinitions = []teams.TeamDefinition{projectShadow, team}
	}
	catalog := teams.TeamResolutionCatalogInput{
		AgentDefinitions:       definitions,
		RuntimeProfiles:        profiles,
		TeamDefinitions:        teamDefinitions,
		MainAgentDefinitionIDs: []string{"agent.main"},
		DefaultMainAgentID:     "agent.main",
	}
	if options.reusableTeam {
		catalog.ReusableDefaultTeamID = input.ID
	} else {
		catalog.ProjectDefaultTeamID = input.ID
	}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{savedTeamWriterProbe{runtimeInstanceID: options.runtimeInstanceID}},
	)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	allSelections := []teams.SavedTeamRuntimeSelection{
		{AgentDefinitionID: "agent.main", RuntimeInstanceID: options.runtimeInstanceID},
		{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: options.runtimeInstanceID},
		{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: options.runtimeInstanceID},
	}
	selections := append(
		[]teams.SavedTeamRuntimeSelection(nil), allSelections[:options.roleCount]...,
	)
	binding, err := teams.BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions,
		input.ID,
		input.ScopeIdentity,
		catalog.AgentDefinitions,
		catalog.RuntimeProfiles,
		discovery,
		selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	intent := mode.Intent{
		Text:     "PRIVATE_PROMPT_MUST_NOT_ENTER_JOURNAL",
		Trigger:  mode.TriggerSelectTeam,
		TargetID: input.ID,
	}
	if options.reusableTeam {
		intent.Trigger = mode.TriggerUseAgent
		intent.TargetID = ""
	}
	plan, err := teams.BuildSavedTeamInstantiationPlan(
		intent, scope, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	identity := teams.SavedTeamInstanceIdentityInput{
		WorkRequestID:       "request.one",
		TeamInstanceID:      "team-instance.one",
		MainAgentInstanceID: "agent-instance.main",
		CreatedAt:           1_721_865_600,
	}
	records, err := teams.BuildSavedTeamInstanceRecordSet(
		plan, intent, scope, catalog, binding, discovery, selections, identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
	}
	return savedTeamCommitFixture{
		intent:     intent,
		scope:      scope,
		catalog:    catalog,
		binding:    binding,
		discovery:  discovery,
		selections: selections,
		plan:       plan,
		identity:   identity,
		records:    records,
		commit: SavedTeamCommitInput{
			TeamEventID:             "event.team.created",
			TeamIdempotencyKey:      "key.team.created",
			MainAgentEventID:        "event.agent.created",
			MainAgentIdempotencyKey: "key.agent.created",
			EmittedAt:               time.Date(2026, 7, 25, 10, 30, 0, 123, time.FixedZone("fixture", 8*60*60)),
		},
	}
}

func (f savedTeamCommitFixture) clone() savedTeamCommitFixture {
	f.catalog.AgentDefinitions = append([]agents.AgentDefinition(nil), f.catalog.AgentDefinitions...)
	f.catalog.RuntimeProfiles = append([]loomruntime.RuntimeProfile(nil), f.catalog.RuntimeProfiles...)
	f.catalog.TeamDefinitions = append([]teams.TeamDefinition(nil), f.catalog.TeamDefinitions...)
	f.catalog.MainAgentDefinitionIDs = append([]string(nil), f.catalog.MainAgentDefinitionIDs...)
	f.selections = append([]teams.SavedTeamRuntimeSelection(nil), f.selections...)
	return f
}

func mutateSavedTeamWriterFixture(
	input savedTeamCommitFixture,
	change func(*savedTeamCommitFixture),
) savedTeamCommitFixture {
	result := input.clone()
	change(&result)
	return result
}

func savedTeamWriterFixtureWithIdentity(
	t *testing.T,
	input savedTeamCommitFixture,
	identity teams.SavedTeamInstanceIdentityInput,
) savedTeamCommitFixture {
	t.Helper()
	result := input.clone()
	result.identity = identity
	records, err := teams.BuildSavedTeamInstanceRecordSet(
		result.plan,
		result.intent,
		result.scope,
		result.catalog,
		result.binding,
		result.discovery,
		result.selections,
		result.identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet(changed identity) error = %v", err)
	}
	result.records = records
	return result
}

func (f savedTeamCommitFixture) commitTo(
	ctx context.Context,
	appender EventBatchAppender,
) (SavedTeamCommitCandidate, error) {
	return CommitSavedTeamInstanceRecordSet(
		ctx,
		appender,
		f.records,
		f.plan,
		f.intent,
		f.scope,
		f.catalog,
		f.binding,
		f.discovery,
		f.selections,
		f.identity,
		f.commit,
	)
}

func savedTeamWriterDefinitions(
	scope agents.Scope,
	identity agents.ScopeIdentity,
) []agents.AgentDefinition {
	result := make([]agents.AgentDefinition, 0, 3)
	for _, id := range []string{"agent.main", "agent.sub.one", "agent.sub.two"} {
		result = append(result, agents.AgentDefinition{
			ID:            id,
			Version:       1,
			Scope:         scope,
			ScopeIdentity: identity,
			Name:          id,
			RoleSpec:      "bounded role",
			Status:        agents.DefinitionActive,
		})
	}
	return result
}

func savedTeamWriterProfiles() []loomruntime.RuntimeProfile {
	budget := int64(100)
	result := make([]loomruntime.RuntimeProfile, 0, 3)
	for _, suffix := range []string{"main", "sub.one", "sub.two"} {
		currentBudget := budget
		result = append(result, loomruntime.RuntimeProfile{
			ID:                   "profile." + suffix,
			AdapterType:          "test",
			ProviderID:           "provider.test",
			ProviderAccountID:    "provider-account.test." + suffix,
			ModelID:              "model.test",
			AuthMode:             loomruntime.AuthBrokered,
			EndpointFingerprint:  strings.Repeat("a", 64),
			CredentialReference:  "credential-ref-test-" + strings.ReplaceAll(suffix, ".", "-"),
			CredentialRevision:   1,
			RequiredCapabilities: []string{"text"},
			Timeout:              time.Minute,
			Budget:               &currentBudget,
		})
	}
	return result
}

type savedTeamWriterProbe struct {
	runtimeInstanceID string
}

func (savedTeamWriterProbe) ID() string {
	return "probe.saved-team-writer"
}

func (p savedTeamWriterProbe) ObserveRuntime(context.Context) ([]loomruntime.RuntimeObservation, error) {
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID:                   p.runtimeInstanceID,
			DeviceID:             "device.local",
			AdapterType:          "test",
			DisplayName:          "Local Runtime",
			ExecutableVersion:    "1.0.0",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"text"},
			Capacity:             3,
		},
		ModelIDs: []string{"model.test"},
	}}, nil
}

type recordingEventBatchAppender struct {
	calls   int
	batches [][]journal.Event
	result  func([]journal.Event) []journal.Event
	err     error
}

func (a *recordingEventBatchAppender) AppendBatch(
	_ context.Context,
	events []journal.Event,
) ([]journal.Event, error) {
	a.calls++
	a.batches = append(a.batches, cloneStateEvents(events))
	if a.err != nil {
		return nil, a.err
	}
	if a.result != nil {
		return a.result(events), nil
	}
	return cloneStateEvents(events), nil
}

func openSavedTeamWriterStore(t *testing.T) (*journal.Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/journal.db")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return journal.NewStore(db), db
}

func assertSavedTeamWriterEventEnvelope(
	t *testing.T,
	fixture savedTeamCommitFixture,
	teamEvent journal.Event,
	mainEvent journal.Event,
) {
	t.Helper()
	wantTime := fixture.commit.EmittedAt.UTC()
	if teamEvent.ID != fixture.commit.TeamEventID ||
		teamEvent.StreamID != "team_instance:"+fixture.identity.TeamInstanceID ||
		teamEvent.Seq != 1 ||
		teamEvent.IdempotencyKey != fixture.commit.TeamIdempotencyKey ||
		teamEvent.Type != "TeamInstanceCreated" ||
		teamEvent.SchemaVersion != 1 ||
		!teamEvent.EmittedAt.Equal(wantTime) ||
		teamEvent.EmittedAt.Location() != time.UTC ||
		teamEvent.CorrelationID != fixture.identity.WorkRequestID ||
		teamEvent.CausationID != "" {
		t.Fatalf("Team event = %#v", teamEvent)
	}
	if mainEvent.ID != fixture.commit.MainAgentEventID ||
		mainEvent.StreamID != "agent_instance:"+fixture.identity.MainAgentInstanceID ||
		mainEvent.Seq != 1 ||
		mainEvent.IdempotencyKey != fixture.commit.MainAgentIdempotencyKey ||
		mainEvent.Type != "AgentInstanceCreated" ||
		mainEvent.SchemaVersion != 1 ||
		!mainEvent.EmittedAt.Equal(wantTime) ||
		mainEvent.EmittedAt.Location() != time.UTC ||
		mainEvent.CorrelationID != fixture.identity.WorkRequestID ||
		mainEvent.CausationID != fixture.commit.TeamEventID {
		t.Fatalf("Main Agent event = %#v", mainEvent)
	}
}

func assertSavedTeamWriterPayloads(
	t *testing.T,
	fixture savedTeamCommitFixture,
	teamEvent journal.Event,
	mainEvent journal.Event,
) {
	t.Helper()
	var teamPayload map[string]any
	if err := json.Unmarshal(teamEvent.PayloadJSON, &teamPayload); err != nil {
		t.Fatalf("Team payload decode error = %v", err)
	}
	var mainPayload map[string]any
	if err := json.Unmarshal(mainEvent.PayloadJSON, &mainPayload); err != nil {
		t.Fatalf("Main payload decode error = %v", err)
	}
	for _, key := range []string{
		"team", "dormant_sub_agents", "source_plan_digest",
		"source_record_set_digest", "team_instance_count",
		"agent_instance_count", "active_sub_agent_count", "work_item_count",
	} {
		if _, ok := teamPayload[key]; !ok {
			t.Fatalf("Team payload missing %q: %s", key, teamEvent.PayloadJSON)
		}
	}
	for _, key := range []string{
		"main_agent", "runtime_binding", "source_plan_digest",
		"source_record_set_digest", "team_created_at",
	} {
		if _, ok := mainPayload[key]; !ok {
			t.Fatalf("Main payload missing %q: %s", key, mainEvent.PayloadJSON)
		}
	}
	if got := int(teamPayload["team_instance_count"].(float64)); got != 1 {
		t.Fatalf("team_instance_count = %d", got)
	}
	if got := int(teamPayload["agent_instance_count"].(float64)); got != 1 {
		t.Fatalf("agent_instance_count = %d", got)
	}
	if got := int(teamPayload["active_sub_agent_count"].(float64)); got != 0 {
		t.Fatalf("active_sub_agent_count = %d", got)
	}
	if got := int(teamPayload["work_item_count"].(float64)); got != 0 {
		t.Fatalf("work_item_count = %d", got)
	}
	if got := len(teamPayload["dormant_sub_agents"].([]any)); got != len(fixture.records.DormantSubAgents()) {
		t.Fatalf("dormant_sub_agents length = %d", got)
	}
	if teamPayload["source_plan_digest"] != fixture.plan.PlanDigest() ||
		teamPayload["source_record_set_digest"] != fixture.records.RecordSetDigest() ||
		mainPayload["source_plan_digest"] != fixture.plan.PlanDigest() ||
		mainPayload["source_record_set_digest"] != fixture.records.RecordSetDigest() {
		t.Fatal("payload source digests do not match accepted sources")
	}
	teamRecord := fixture.records.Team()
	teamPayloadRecord := teamPayload["team"].(map[string]any)
	teamScope := teamPayloadRecord["scope_identity"].(map[string]any)
	if teamPayloadRecord["id"] != teamRecord.ID ||
		teamPayloadRecord["team_definition_scope"] != string(teamRecord.TeamDefinitionScope) ||
		teamPayloadRecord["team_definition_digest"] != teamRecord.TeamDefinitionDigest ||
		teamScope["project_id"] != teamRecord.ScopeIdentity.ProjectID ||
		teamScope["generation_id"] != teamRecord.ScopeIdentity.GenerationID {
		t.Fatalf("Team payload record = %#v, want %#v", teamPayloadRecord, teamRecord)
	}
	mainRecord := fixture.records.MainAgent()
	mainPayloadRecord := mainPayload["main_agent"].(map[string]any)
	mainScope := mainPayloadRecord["scope_identity"].(map[string]any)
	runtimeBinding := mainPayload["runtime_binding"].(map[string]any)
	if mainPayloadRecord["id"] != mainRecord.ID ||
		int(mainPayloadRecord["agent_definition_version"].(float64)) != mainRecord.AgentDefinitionVersion ||
		mainPayloadRecord["agent_definition_scope"] != string(mainRecord.AgentDefinitionScope) ||
		mainScope["project_id"] != mainRecord.ScopeIdentity.ProjectID ||
		mainScope["generation_id"] != mainRecord.ScopeIdentity.GenerationID ||
		runtimeBinding["profile_id"] != mainRecord.Binding.ProfileID ||
		runtimeBinding["instance_id"] != mainRecord.Binding.InstanceID ||
		runtimeBinding["accepted"] != mainRecord.Binding.Accepted {
		t.Fatalf("Main payload record = %#v binding=%#v, want %#v", mainPayloadRecord, runtimeBinding, mainRecord)
	}
	encoded := string(teamEvent.PayloadJSON) + string(mainEvent.PayloadJSON)
	if containsAnyStateString(encoded, fixture.intent.Text, "credential", "secret", "process_env") {
		t.Fatalf("payload contains forbidden content: %s", encoded)
	}
}

func assertSavedTeamWriterStoredEvents(
	t *testing.T,
	store *journal.Store,
	fixture savedTeamCommitFixture,
	teamEvent journal.Event,
	mainEvent journal.Event,
) {
	t.Helper()
	teamStored, err := store.ReadStream(
		context.Background(), "team_instance:"+fixture.identity.TeamInstanceID,
	)
	if err != nil {
		t.Fatalf("ReadStream(Team) error = %v", err)
	}
	mainStored, err := store.ReadStream(
		context.Background(), "agent_instance:"+fixture.identity.MainAgentInstanceID,
	)
	if err != nil {
		t.Fatalf("ReadStream(Main Agent) error = %v", err)
	}
	if !reflect.DeepEqual(teamStored, []journal.Event{teamEvent}) ||
		!reflect.DeepEqual(mainStored, []journal.Event{mainEvent}) {
		t.Fatalf("stored Events = (%#v,%#v)", teamStored, mainStored)
	}
}

func savedTeamWriterRowCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatalf("row count error = %v", err)
	}
	return count
}

func assertZeroSavedTeamCommit(t *testing.T, got SavedTeamCommitCandidate) {
	t.Helper()
	if !reflect.DeepEqual(got, SavedTeamCommitCandidate{}) {
		t.Fatalf("failed commit returned Candidate %#v", got)
	}
}

func cloneStateEvents(input []journal.Event) []journal.Event {
	result := make([]journal.Event, len(input))
	for index, event := range input {
		result[index] = cloneStateEvent(event)
	}
	return result
}

func cloneStateEvent(event journal.Event) journal.Event {
	event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	return event
}

func cloneSavedTeamCommitCandidate(
	input SavedTeamCommitCandidate,
) SavedTeamCommitCandidate {
	input.teamEvent = cloneStateEvent(input.teamEvent)
	input.mainAgentEvent = cloneStateEvent(input.mainAgentEvent)
	return input
}

func containsAnyStateString(input string, values ...string) bool {
	for _, value := range values {
		if value != "" && containsStateString(input, value) {
			return true
		}
	}
	return false
}

func containsStateString(input, search string) bool {
	if len(search) == 0 || len(search) > len(input) {
		return false
	}
	for index := 0; index+len(search) <= len(input); index++ {
		if input[index:index+len(search)] == search {
			return true
		}
	}
	return false
}

func TestSavedTeamCommitMainAgentRecordsPerInstanceRuntimeDiscoveryDigest(
	t *testing.T,
) {
	fixture := newSavedTeamCommitFixture(t, 3, false, false)
	const perInstanceDigest = "1111111111111111111111111111111111111111111111111111111111111111"
	binding, err := fixture.binding.WithMainRuntimeDiscoveryDigest(perInstanceDigest)
	if err != nil {
		t.Fatalf("WithMainRuntimeDiscoveryDigest() error = %v", err)
	}
	if binding.MainRuntimeDiscoveryDigest() != perInstanceDigest {
		t.Fatalf(
			"MainRuntimeDiscoveryDigest() = %q, want %q",
			binding.MainRuntimeDiscoveryDigest(), perInstanceDigest,
		)
	}
	if binding.RuntimeDiscoveryDigest() == perInstanceDigest {
		t.Fatal("composite RuntimeDiscoveryDigest must differ from the per-instance digest")
	}
	fixture.binding = binding
	store, _ := openSavedTeamWriterStore(t)
	got, err := fixture.commitTo(context.Background(), store)
	if err != nil {
		t.Fatalf("CommitSavedTeamInstanceRecordSet() error = %v", err)
	}
	var mainPayload map[string]any
	if err := json.Unmarshal(
		got.MainAgentEvent().PayloadJSON, &mainPayload,
	); err != nil {
		t.Fatalf("main payload decode error = %v", err)
	}
	recorded, ok := mainPayload["runtime_discovery_digest"].(string)
	if !ok || recorded != perInstanceDigest {
		t.Fatalf(
			"main agent runtime_discovery_digest = %q (%v), want %q",
			recorded, ok, perInstanceDigest,
		)
	}
}
