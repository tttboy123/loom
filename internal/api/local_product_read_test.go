package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

type controlledLocalProductViewSource struct {
	view projection.GlobalReadView
	err  error
}

type controlledRuntimeObservationHealthSource struct {
	reason  string
	partial bool
}

func (source *controlledRuntimeObservationHealthSource) RuntimeObservationHealth() (
	string,
	bool,
) {
	return source.reason, source.partial
}

type sequencedMissionDecisionCommandSource struct {
	commands [][]app.MissionDecisionCommand
	calls    int
	queries  []app.MissionDecisionCommandQuery
}

func (source *sequencedMissionDecisionCommandSource) ListMissionDecisionCommands(
	_ context.Context,
	query app.MissionDecisionCommandQuery,
) ([]app.MissionDecisionCommand, error) {
	source.queries = append(source.queries, query)
	index := source.calls
	if index >= len(source.commands) {
		index = len(source.commands) - 1
	}
	source.calls++
	return append([]app.MissionDecisionCommand(nil), source.commands[index]...), nil
}

func (source *controlledLocalProductViewSource) Rebuild(context.Context) error {
	return source.err
}

func (source *controlledLocalProductViewSource) GlobalReadView() projection.GlobalReadView {
	return source.view
}

func TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	definitionPayload := mustMarshalLocalProductTest(t, map[string]any{
		"definition": map[string]any{
			"id": "team.delivery", "version": 1, "scope": "project",
			"scope_identity": map[string]any{
				"project_id": "project.one", "generation_id": "generation.one",
			},
			"name": "Release Crew", "status": "active",
			"roles": []map[string]any{
				{
					"kind": "main", "agent_definition_id": "agent.main",
					"runtime_profile_id": "profile.main",
					"responsibility":     "Coordinate bounded work",
				},
				{
					"kind": "subagent", "agent_definition_id": "agent.reviewer",
					"runtime_profile_id": "profile.reviewer",
					"responsibility":     "Review bounded work",
				},
			},
			"digest": strings.Repeat("a", 64),
		},
		"draft_id": "draft-release", "draft_revision": 1,
		"catalog_digest": strings.Repeat("b", 64),
		"content_digest": strings.Repeat("c", 64),
		"binding_digest": strings.Repeat("d", 64),
		"configuration": map[string]any{
			"requested_concurrency": 1, "maximum_budget_credits": 100,
			"role_bindings": []map[string]any{
				{
					"kind": "main", "agent_definition_id": "agent.main",
					"runtime_profile_id":  "profile.main",
					"runtime_instance_id": "runtime.shared",
					"model_id":            "deepseek-chat",
					"execution_profile": map[string]any{
						"version": 1, "id": "profile.main",
						"harness_adapter":       "loom-native",
						"provider_id":           "deepseek",
						"provider_account_id":   "deepseek.primary",
						"model_id":              "deepseek-chat",
						"auth_mode":             "brokered",
						"endpoint_fingerprint":  strings.Repeat("e", 64),
						"credential_reference":  "credential-ref-private",
						"credential_revision":   int64(7),
						"timeout_nanoseconds":   int64(time.Minute),
						"required_capabilities": []any{},
					},
					"skill_revisions": []any{}, "permission_ids": []any{},
					"resource_ids": []any{},
				},
				{
					"kind": "subagent", "agent_definition_id": "agent.reviewer",
					"runtime_profile_id":  "profile.reviewer",
					"runtime_instance_id": "runtime.shared",
					"model_id":            "model.pending",
					"skill_revisions":     []any{}, "permission_ids": []any{},
					"resource_ids": []any{},
				},
			},
		},
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event.team.definition.saved", StreamID: "team-definition/team.delivery",
		Seq: 1, IdempotencyKey: "key.team.definition.saved",
		Type: "TeamDefinitionSaved", SchemaVersion: 1,
		EmittedAt:   time.Date(2026, 7, 25, 2, 29, 0, 0, time.UTC),
		PayloadJSON: definitionPayload,
	}); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	source := &controlledLocalProductViewSource{view: readModel.GlobalReadView()}
	health := &controlledRuntimeObservationHealthSource{}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: source,
		Now: func() time.Time {
			return time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
		},
		RuntimeHealth: health,
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 3 ||
		snapshot.Health != (LocalProductHealth{
			Daemon: "serving_request", Journal: "available", Projection: "current",
		}) ||
		snapshot.ViewVersion == "" ||
		snapshot.Stale ||
		len(snapshot.Teams) != 1 ||
		snapshot.Teams[0].TeamInstanceID != "team-instance.one" ||
		snapshot.Teams[0].TeamDefinitionID != "team.delivery" ||
		snapshot.Teams[0].TeamDefinitionVersion != 1 ||
		snapshot.Teams[0].DisplayName != "Release Crew" ||
		snapshot.Teams[0].SourceKind != "saved_team" ||
		len(snapshot.Teams[0].Agents) != 2 ||
		len(snapshot.Missions) != 1 ||
		snapshot.Missions[0].Title != "Release Crew" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if got := snapshot.Teams[0].Agents[0]; got != (LocalProductTeamAgentSummary{
		RoleKind:           "main",
		AgentDefinitionID:  "agent.main",
		RuntimeProfileID:   "profile.main",
		BindingStatus:      "configured",
		HarnessAdapter:     "loom-native",
		ProviderID:         "deepseek",
		ProviderAccountID:  "deepseek.primary",
		ModelID:            "deepseek-chat",
		CredentialRevision: 7,
	}) {
		t.Fatalf("configured Agent summary = %#v", got)
	}
	if got := snapshot.Teams[0].Agents[1]; got != (LocalProductTeamAgentSummary{
		RoleKind:          "subagent",
		AgentDefinitionID: "agent.reviewer",
		RuntimeProfileID:  "profile.reviewer",
		BindingStatus:     "unavailable",
		ModelID:           "model.pending",
	}) {
		t.Fatalf("unavailable Agent summary = %#v", got)
	}
	encodedSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encodedSnapshot, []byte(`"side_tasks":[]`)) {
		t.Fatalf("snapshot missing required empty side_tasks array: %s", encodedSnapshot)
	}
	for _, privateValue := range [][]byte{
		[]byte("credential-ref-private"),
		[]byte(strings.Repeat("e", 64)),
		[]byte("credential_reference"),
		[]byte("endpoint_fingerprint"),
	} {
		if bytes.Contains(encodedSnapshot, privateValue) {
			t.Fatalf("snapshot leaked private binding data %q: %s", privateValue, encodedSnapshot)
		}
	}
	health.reason = "observer_models_timeout"
	health.partial = true
	partial, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !partial.Partial || partial.Reason != "observer_models_timeout" ||
		partial.Health != (LocalProductHealth{
			Daemon: "serving_request", Journal: "available", Projection: "current",
		}) {
		t.Fatalf("Runtime-partial snapshot = %#v", partial)
	}
	health.reason = "unknown_observer_failure"
	if _, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	); !errors.Is(err, ErrLocalProductStateUnavailable) {
		t.Fatalf("unknown Runtime health error = %v", err)
	}
	health.reason = ""
	health.partial = false

	source.err = errors.New("private sqlite path must not escape")
	stale, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale ||
		stale.Reason != "projection_refresh_failed" ||
		stale.Health != (LocalProductHealth{
			Daemon: "serving_request", Journal: "available", Projection: "stale",
		}) ||
		stale.ViewVersion != snapshot.ViewVersion ||
		!equalLocalProductSnapshotsIgnoringStale(snapshot, stale) {
		t.Fatalf("stale snapshot = %#v, previous = %#v", stale, snapshot)
	}
	if strings.Contains(stale.Reason, "sqlite") ||
		strings.Contains(stale.Reason, "path") {
		t.Fatalf("stale reason leaked private error: %q", stale.Reason)
	}
}

func TestLocalProductReadServicePreservesPreparedCommandsWithStaleView(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	source := &controlledLocalProductViewSource{view: view}
	initial := app.MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       "read",
		Kind:            "authorization",
		Action:          "read",
		MissionID:       "mission/team-one",
		TeamInstanceID:  "team-one",
		ViewVersion:     view.Version(),
		DecisionID:      "decision-one",
		DecisionDigest:  strings.Repeat("a", 64),
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		ClaimGeneration: 0,
		CorrelationID:   "77777777-7777-4777-8777-777777777777",
	}
	decisions := &sequencedMissionDecisionCommandSource{
		commands: [][]app.MissionDecisionCommand{{initial}},
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: source,
		Now: func() time.Time {
			return time.Date(2026, 7, 30, 22, 0, 0, 0, time.UTC)
		},
		Decisions: decisions,
	})
	if err != nil {
		t.Fatal(err)
	}

	current, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(current.PreparedDecisions, []app.MissionDecisionCommand{initial}) {
		t.Fatalf("current prepared decisions = %#v", current.PreparedDecisions)
	}

	source.err = errors.New("private projection failure")
	stale, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale || stale.ViewVersion != current.ViewVersion {
		t.Fatalf("stale snapshot = %#v", stale)
	}
	if !reflect.DeepEqual(stale.PreparedDecisions, current.PreparedDecisions) {
		t.Fatalf(
			"stale commands changed: current=%#v stale=%#v",
			current.PreparedDecisions,
			stale.PreparedDecisions,
		)
	}
	if len(decisions.queries) != 2 ||
		decisions.queries[0].Mode !=
			app.MissionDecisionCommandRefreshCurrent ||
		decisions.queries[0].ViewVersion != current.ViewVersion ||
		decisions.queries[1].Mode !=
			app.MissionDecisionCommandPreserveStale ||
		decisions.queries[1].ViewVersion != current.ViewVersion {
		t.Fatalf("decision queries = %#v", decisions.queries)
	}
}

func TestLocalProductReadServiceRejectsMixedStaleViewAndCommands(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	source := &controlledLocalProductViewSource{view: view}
	command := app.MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       "read",
		Kind:            "authorization",
		Action:          "read",
		MissionID:       "mission/team-mixed",
		TeamInstanceID:  "team-mixed",
		ViewVersion:     view.Version(),
		DecisionID:      "decision-mixed",
		DecisionDigest:  strings.Repeat("c", 64),
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		ClaimGeneration: 0,
		CorrelationID:   "88888888-8888-4888-8888-888888888888",
	}
	decisions := &sequencedMissionDecisionCommandSource{
		commands: [][]app.MissionDecisionCommand{{command}},
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: source,
		Now: func() time.Time {
			return time.Date(2026, 7, 30, 22, 5, 0, 0, time.UTC)
		},
		Decisions: decisions,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	); err != nil {
		t.Fatal(err)
	}

	decisions.commands[0][0].ViewVersion = strings.Repeat("d", 64)
	source.err = errors.New("private projection failure")
	if _, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	); !errors.Is(err, ErrLocalProductStateUnavailable) {
		t.Fatalf("mixed stale state error = %v", err)
	}
}

func TestLocalProductReadServiceCanonicalizesRuntimeCollectionsForWire(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name             string
		modelIDs         any
		capabilities     any
		wantModelIDs     []string
		wantCapabilities []string
	}{
		{
			name:             "historical null model ids",
			modelIDs:         nil,
			capabilities:     []string{"models", "streaming"},
			wantModelIDs:     []string{},
			wantCapabilities: []string{"models", "streaming"},
		},
		{
			name:             "nil observed capabilities",
			modelIDs:         []string{"model-a", "model-b"},
			capabilities:     nil,
			wantModelIDs:     []string{"model-a", "model-b"},
			wantCapabilities: []string{},
		},
		{
			name:             "nonempty order is preserved",
			modelIDs:         []string{"model-a", "model-b"},
			capabilities:     []string{"models", "streaming"},
			wantModelIDs:     []string{"model-a", "model-b"},
			wantCapabilities: []string{"models", "streaming"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, view := localProductRuntimeCollectionView(
				t,
				test.modelIDs,
				test.capabilities,
			)
			source := &controlledLocalProductViewSource{view: view}
			service, err := NewLocalProductReadService(
				LocalProductReadConfig{
					Journal:    store,
					Projection: source,
					Now: func() time.Time {
						return time.Date(
							2026, 7, 29, 3, 0, 0, 0, time.UTC,
						)
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			snapshot, err := service.ReadLocalProductSnapshot(
				context.Background(),
				LocalProductSnapshotRequest{Limit: 64},
			)
			if err != nil {
				t.Fatal(err)
			}
			assertCanonicalLocalProductRuntimeCollections(
				t,
				snapshot,
				test.wantModelIDs,
				test.wantCapabilities,
			)

			encoded := mustMarshalLocalProductTest(t, snapshot)
			if len(test.wantModelIDs) == 0 &&
				!bytes.Contains(encoded, []byte(`"model_ids":[]`)) {
				t.Fatalf("model_ids wire value is not []: %s", encoded)
			}
			if len(test.wantCapabilities) == 0 &&
				!bytes.Contains(
					encoded,
					[]byte(`"observed_capabilities":[]`),
				) {
				t.Fatalf(
					"observed_capabilities wire value is not []: %s",
					encoded,
				)
			}

			snapshot.Runtimes[0].ModelIDs = append(
				snapshot.Runtimes[0].ModelIDs,
				"caller-mutation",
			)
			snapshot.Runtimes[0].ObservedCapabilities = append(
				snapshot.Runtimes[0].ObservedCapabilities,
				"caller-mutation",
			)
			source.err = errors.New("projection refresh failed")
			stale, err := service.ReadLocalProductSnapshot(
				context.Background(),
				LocalProductSnapshotRequest{Limit: 64},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !stale.Stale ||
				stale.Reason != "projection_refresh_failed" {
				t.Fatalf("stale snapshot state = %#v", stale)
			}
			assertCanonicalLocalProductRuntimeCollections(
				t,
				stale,
				test.wantModelIDs,
				test.wantCapabilities,
			)
		})
	}
}

func TestLocalProductSnapshotCloneCanonicalizesAllRequiredCollectionsForWire(
	t *testing.T,
) {
	t.Parallel()

	snapshot := cloneLocalProductSnapshot(LocalProductSnapshot{
		SchemaVersion: localProductSchemaVersion,
		ViewVersion:   "view-empty",
		Runtimes: []LocalProductRuntimeSummary{{
			RuntimeInstanceID: "runtime-empty",
		}},
	})
	encoded := mustMarshalLocalProductTest(t, snapshot)
	for _, requiredArray := range [][]byte{
		[]byte(`"runtimes":[`),
		[]byte(`"model_ids":[]`),
		[]byte(`"observed_capabilities":[]`),
		[]byte(`"teams":[]`),
		[]byte(`"runs":[]`),
		[]byte(`"evidence":[]`),
		[]byte(`"attention":[]`),
	} {
		if !bytes.Contains(encoded, requiredArray) {
			t.Fatalf(
				"authoritative snapshot collection %s is not an array: %s",
				requiredArray,
				encoded,
			)
		}
	}
}

func TestLocalProductReadServiceMapsTeamTimelineWithoutExposingEventPayload(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 28, 8, 5, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	page, err := service.ReadLocalProductTimeline(
		context.Background(),
		LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one",
			Limit:          128,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if page.SchemaVersion != 1 ||
		page.TeamInstanceID != "team-instance.one" ||
		page.ViewVersion == "" ||
		page.NextCursor == "" ||
		page.Records == nil ||
		page.Attention == nil {
		t.Fatalf("timeline page = %#v", page)
	}
	encoded := mustMarshalLocalProductTest(t, page)
	for _, requiredArray := range [][]byte{
		[]byte(`"records":[`),
		[]byte(`"nodes":[`),
		[]byte(`"attention":[`),
	} {
		if !bytes.Contains(encoded, requiredArray) {
			t.Fatalf(
				"authoritative timeline collection %s is not an array: %s",
				requiredArray,
				encoded,
			)
		}
	}
	for _, forbidden := range []string{
		"payload_json",
		"token_hash",
		"allowed_operations",
		"sqlite",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("timeline disclosed %q: %s", forbidden, encoded)
		}
	}
}

func TestLocalProductReadServiceReplaysBoundedTentativeOutputWithoutDispatch(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal: store, Projection: readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := LocalProductTimelineRecord{
		SchemaVersion:  1,
		DeliveryID:     strings.Repeat("d", 64),
		Kind:           "node_output_delta",
		Authority:      "tentative",
		TeamInstanceID: "team-instance.one",
		LogicalNodeID:  "main",
		AttemptNumber:  1,
		SourceSequence: 3,
		SourceEventID:  "frame-3",
		OccurredAt:     "2026-07-28T08:00:00Z",
		Payload: LocalProductTimelinePayload{
			TextDelta: "authorized tentative output",
		},
	}
	if err := service.cacheMissionExecutionDelivery(record); err != nil {
		t.Fatal(err)
	}
	first, err := service.ReadLocalProductTimeline(
		context.Background(),
		LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one", Limit: 128,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ReadLocalProductTimeline(
		context.Background(),
		LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one", Limit: 128,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []LocalProductTimelinePage{first, second} {
		last := page.Records[len(page.Records)-1]
		if last.DeliveryID != record.DeliveryID ||
			last.Authority != "tentative" ||
			last.Payload.TextDelta != "authorized tentative output" ||
			last.Cursor != page.NextCursor {
			t.Fatalf("tentative timeline = %#v", page)
		}
	}
	if len(first.Records) != len(second.Records) {
		t.Fatalf("reconnect record counts = %d, %d", len(first.Records), len(second.Records))
	}
	observerOne, err := service.MissionExecutionObserver(
		context.Background(),
		"team-instance.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	observerTwo, err := service.MissionExecutionObserver(
		context.Background(),
		"team-instance.one",
	)
	if err != nil || observerOne != observerTwo {
		t.Fatalf("observer reuse = %T %T, %v", observerOne, observerTwo, err)
	}
	if err := service.CloseMissionExecutionObservers(); err != nil {
		t.Fatal(err)
	}
}

func TestLocalProductReadServiceTentativeCacheIsBoundedAndEmitsGap(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal: store, Projection: readModel,
		Now: func() time.Time {
			return time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 65; index++ {
		record := LocalProductTimelineRecord{
			SchemaVersion: 1,
			DeliveryID: fmt.Sprintf(
				"%064x", index+1,
			),
			Kind: "node_output_delta", Authority: "tentative",
			TeamInstanceID: "team-instance.one",
			LogicalNodeID:  fmt.Sprintf("node-%02d", index),
			AttemptNumber:  1,
			SourceSequence: int64(index + 1),
			SourceEventID:  fmt.Sprintf("frame-%d", index+1),
			OccurredAt:     "2026-07-28T08:00:00Z",
			Payload: LocalProductTimelinePayload{
				TextDelta: "x",
			},
		}
		if err := service.cacheMissionExecutionDelivery(record); err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.ReadLocalProductTimeline(
		context.Background(),
		LocalProductTimelineRequest{
			TeamInstanceID: "team-instance.one", Limit: 128,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if page.Gap == nil || page.Gap.Reason != "tentative_overflow" ||
		len(page.Records) > 65 {
		t.Fatalf("bounded tentative page = %#v", page)
	}
}

func TestLocalProductTeamPageAdvancesPastNonHistoricalExecutions(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	digest := strings.Repeat("a", 64)
	for index := 0; index < 65; index++ {
		teamID := fmt.Sprintf("team-active-%03d", index)
		payload := mustMarshalLocalProductTest(t, map[string]any{
			"team_instance_id": teamID,
			"plan_digest":      digest,
			"view_version":     digest,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        1,
			}},
		})
		if _, err := store.Append(context.Background(), journal.Event{
			ID:             "event-plan-" + teamID,
			StreamID:       "team-execution/" + teamID,
			Seq:            1,
			IdempotencyKey: "key-plan-" + teamID,
			Type:           "TeamExecutionPlanned",
			SchemaVersion:  1,
			EmittedAt: time.Date(
				2026,
				7,
				28,
				9,
				0,
				index,
				0,
				time.UTC,
			),
			PayloadJSON: payload,
		}); err != nil {
			t.Fatal(err)
		}
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	source := &controlledLocalProductViewSource{
		view: readModel.GlobalReadView(),
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: source,
		Now: func() time.Time {
			return time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Teams) != 0 ||
		!first.TeamPage.HasMore ||
		first.TeamPage.NextCursor != "team-active-063" {
		t.Fatalf("first nonhistorical page = %#v", first.TeamPage)
	}
	source.err = errors.New("projection rebuild unavailable")
	second, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{
			AfterTeamID: first.TeamPage.NextCursor,
			Limit:       64,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Teams) != 0 ||
		!second.Stale ||
		second.TeamPage.HasMore ||
		second.TeamPage.NextCursor != "team-active-064" {
		t.Fatalf("second stale nonhistorical page = %#v", second)
	}
	if _, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{AfterTeamID: "team-missing", Limit: 64},
	); !errors.Is(err, ErrInvalidLocalProductRequest) {
		t.Fatalf("unknown Team cursor error = %v", err)
	}
}

func TestLocalProductMissionPageAdvancesPastSavedOnlyTeamRows(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	savedTeamID := "team-mission-saved-only"
	savedTeamPayload := mustMarshalLocalProductTest(t, map[string]any{
		"team": map[string]any{
			"id": savedTeamID, "work_request_id": "request.mission.saved-only",
			"source_kind": "saved_team", "team_definition_id": "team.saved-only",
			"team_definition_version": 1, "team_definition_scope": "project",
			"scope_identity": map[string]any{
				"project_id": "project.one", "generation_id": "",
			},
			"team_definition_digest": digestA, "source_plan_digest": digestB,
			"state": "created", "created_at": int64(1_722_000_000),
		},
		"dormant_sub_agents": []any{}, "source_plan_digest": digestB,
		"source_record_set_digest": digestA, "team_instance_count": 1,
		"agent_instance_count": 1, "active_sub_agent_count": 0,
		"work_item_count": 0,
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-team-mission-saved-only", StreamID: "team_instance:" + savedTeamID,
		Seq: 1, IdempotencyKey: "key-team-mission-saved-only",
		Type: "TeamInstanceCreated", SchemaVersion: 1,
		EmittedAt:     time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		CorrelationID: "request.mission.saved-only", PayloadJSON: savedTeamPayload,
	}); err != nil {
		t.Fatal(err)
	}
	savedAgentPayload := mustMarshalLocalProductTest(t, map[string]any{
		"main_agent": map[string]any{
			"id": "agent-mission-saved-only", "team_instance_id": savedTeamID,
			"agent_definition_id": "agent.main", "agent_definition_version": 1,
			"agent_definition_scope": "project",
			"scope_identity": map[string]any{
				"project_id": "project.one", "generation_id": "",
			},
			"runtime_profile_id": "profile.main", "runtime_instance_id": "runtime.main",
			"is_main": true, "state": "created",
		},
		"runtime_binding": map[string]any{
			"accepted": true, "profile_id": "profile.main", "instance_id": "runtime.main",
		},
		"source_plan_digest": digestB, "source_record_set_digest": digestA,
		"team_created_at": int64(1_722_000_000), "binding_digest": digestB,
		"runtime_discovery_digest": digestA,
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID:       "event-agent-mission-saved-only",
		StreamID: "agent_instance:agent-mission-saved-only",
		Seq:      1, IdempotencyKey: "key-agent-mission-saved-only",
		Type: "AgentInstanceCreated", SchemaVersion: 1,
		EmittedAt:     time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		CorrelationID: "request.mission.saved-only",
		CausationID:   "event-team-mission-saved-only", PayloadJSON: savedAgentPayload,
	}); err != nil {
		t.Fatal(err)
	}
	missionTeamID := "team-mission-after-saved-only"
	appendLocalProductMissionPlan(t, store, missionTeamID, 1)
	service := newLocalProductMissionReadService(t, store, projection.New(db))

	first, err := service.ReadLocalProductSnapshot(
		context.Background(), LocalProductSnapshotRequest{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Missions) != 0 || !first.MissionPage.HasMore ||
		first.MissionPage.NextCursor != savedTeamID {
		t.Fatalf("saved-only Mission page = %#v", first)
	}
	second, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{
			AfterTeamID: first.MissionPage.NextCursor,
			Limit:       1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Missions) != 1 || second.MissionPage.HasMore ||
		second.Missions[0].TeamInstanceID != missionTeamID {
		t.Fatalf("Mission page after saved-only row = %#v", second)
	}
}

func TestLocalProductMissionPageAdvancesPastFilteredSideTaskRows(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	filteredTeamID := "team-mission-filtered-000"
	missionTeamID := "team-mission-filtered-001"
	appendLocalProductMissionPlan(t, store, filteredTeamID, 0)
	appendLocalProductMissionPlan(t, store, missionTeamID, 1)
	sideTaskPayload := mustMarshalLocalProductTest(t, map[string]any{
		"side_task_id":            "side-mission-cursor",
		"parent_mission_id":       "mission/team-parent",
		"parent_team_instance_id": "team-parent", "parent_task_id": "task-parent",
		"parent_run_id": "run-parent", "parent_claim_generation": int64(1),
		"parent_execution_digest":         strings.Repeat("1", 64),
		"side_execution_team_instance_id": filteredTeamID,
		"purpose":                         "verification", "mode": "decision_required", "title": "Verify result",
		"admission_kind": "explicit_confirmation", "proposal_digest": strings.Repeat("2", 64),
		"input_artifact_digest": strings.Repeat("3", 64),
		"expected_view_version": strings.Repeat("4", 64),
		"permission_scopes":     []any{}, "policy_stream_id": "", "policy_version": 0,
		"policy_digest": "", "budget_microunits": int64(0), "budget_currency": "",
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-side-mission-cursor", StreamID: "side-task/side-mission-cursor",
		Seq: 1, IdempotencyKey: "key-side-mission-cursor", Type: "SideTaskAdmitted",
		SchemaVersion: 1, EmittedAt: time.Date(2026, 8, 24, 10, 0, 2, 0, time.UTC),
		CorrelationID: "request.side-mission-cursor", PayloadJSON: sideTaskPayload,
	}); err != nil {
		t.Fatal(err)
	}
	service := newLocalProductMissionReadService(t, store, projection.New(db))

	first, err := service.ReadLocalProductSnapshot(
		context.Background(), LocalProductSnapshotRequest{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Missions) != 0 || !first.MissionPage.HasMore ||
		first.MissionPage.NextCursor != filteredTeamID {
		t.Fatalf("filtered side-task Mission page = %#v", first)
	}
	second, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{
			AfterTeamID: first.MissionPage.NextCursor,
			Limit:       1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Missions) != 1 || second.MissionPage.HasMore ||
		second.Missions[0].TeamInstanceID != missionTeamID {
		t.Fatalf("Mission page after filtered side-task row = %#v", second)
	}
}

func TestLocalProductMissionPageCursorFeedsAfterTeamID(t *testing.T) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	digest := strings.Repeat("a", 64)
	for index := 0; index < 2; index++ {
		teamID := fmt.Sprintf("team-mission-cursor-%03d", index)
		payload := mustMarshalLocalProductTest(t, map[string]any{
			"team_instance_id": teamID,
			"plan_digest":      digest,
			"view_version":     digest,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        1,
			}},
		})
		if _, err := store.Append(context.Background(), journal.Event{
			ID:             "event-plan-" + teamID,
			StreamID:       "team-execution/" + teamID,
			Seq:            1,
			IdempotencyKey: "key-plan-" + teamID,
			Type:           "TeamExecutionPlanned",
			SchemaVersion:  1,
			EmittedAt: time.Date(
				2026, 8, 24, 10, 0, index, 0, time.UTC,
			),
			PayloadJSON: payload,
		}); err != nil {
			t.Fatal(err)
		}
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal: store, Projection: readModel,
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 10, 1, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ReadLocalProductSnapshot(
		context.Background(), LocalProductSnapshotRequest{Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Missions) != 1 || !first.MissionPage.HasMore ||
		first.MissionPage.NextCursor != "team-mission-cursor-000" {
		t.Fatalf("first Mission page = %#v", first)
	}
	second, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{
			AfterTeamID: first.MissionPage.NextCursor,
			Limit:       1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Missions) != 1 || second.MissionPage.HasMore ||
		second.Missions[0].TeamInstanceID != "team-mission-cursor-001" {
		t.Fatalf("second Mission page = %#v", second)
	}
}

func appendLocalProductMissionPlan(
	t *testing.T,
	store *journal.Store,
	teamID string,
	second int,
) {
	t.Helper()
	payload := mustMarshalLocalProductTest(t, map[string]any{
		"team_instance_id": teamID,
		"plan_digest":      strings.Repeat("a", 64),
		"view_version":     strings.Repeat("b", 64),
		"nodes": []map[string]any{{
			"logical_node_id": "main", "title": "Main",
			"agent_instance_id": "agent-main", "runtime_instance_id": "runtime-main",
			"role": "main", "depends_on": []string{}, "max_attempts": 1,
		}},
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID: "event-plan-" + teamID, StreamID: "team-execution/" + teamID,
		Seq: 1, IdempotencyKey: "key-plan-" + teamID,
		Type: "TeamExecutionPlanned", SchemaVersion: 1,
		EmittedAt:   time.Date(2026, 8, 24, 10, 0, second, 0, time.UTC),
		PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func newLocalProductMissionReadService(
	t *testing.T,
	store *journal.Store,
	readModel *projection.Projection,
) *LocalProductReadService {
	t.Helper()
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal: store, Projection: readModel,
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 10, 1, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func localProductRuntimeCollectionView(
	t *testing.T,
	modelIDs any,
	capabilities any,
) (*journal.Store, projection.GlobalReadView) {
	t.Helper()
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	payload := mustMarshalLocalProductTest(t, map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    "runtime-1",
			"device_id":             "device-1",
			"adapter_type":          "pi",
			"display_name":          "Pi Runtime",
			"executable_version":    "0.82.1",
			"status":                "online",
			"observed_capabilities": capabilities,
			"capacity":              1,
		},
		"model_ids": modelIDs,
	})
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-discovery",
		StreamID:       "runtime_instance:runtime-1",
		Seq:            1,
		IdempotencyKey: "runtime-discovery",
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt: time.Date(
			2026, 7, 29, 2, 55, 0, 0, time.UTC,
		),
		CorrelationID: "22222222-2222-4222-8222-222222222222",
		PayloadJSON:   payload,
	}); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store, readModel.GlobalReadView()
}

func assertCanonicalLocalProductRuntimeCollections(
	t *testing.T,
	snapshot LocalProductSnapshot,
	wantModelIDs []string,
	wantCapabilities []string,
) {
	t.Helper()
	if len(snapshot.Runtimes) != 1 {
		t.Fatalf("runtime count = %d", len(snapshot.Runtimes))
	}
	runtime := snapshot.Runtimes[0]
	if runtime.ModelIDs == nil ||
		runtime.ObservedCapabilities == nil ||
		!reflect.DeepEqual(runtime.ModelIDs, wantModelIDs) ||
		!reflect.DeepEqual(
			runtime.ObservedCapabilities,
			wantCapabilities,
		) {
		t.Fatalf("runtime collections = %#v", runtime)
	}
}

func equalLocalProductSnapshotsIgnoringStale(
	left LocalProductSnapshot,
	right LocalProductSnapshot,
) bool {
	left.Stale = false
	left.Reason = ""
	left.Health.Projection = ""
	right.Stale = false
	right.Reason = ""
	right.Health.Projection = ""
	return reflect.DeepEqual(left, right)
}

func mustMarshalLocalProductTest(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
