package assets_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

func TestP3AAssetAuthorityDeclaresCompleteVerticalLifecycle(t *testing.T) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(), "authority.go", nil, parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse required asset authority: %v", err)
	}
	functions := make(map[string]bool)
	for _, declaration := range parsed.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			functions[function.Name.Name] = true
		}
	}
	for _, name := range []string{
		"CreateSkill",
		"ImportSkill",
		"CreateTemplate",
		"InstantiateTemplate",
		"PromoteRun",
		"RecordEvaluation",
		"SetBinding",
		"ActivateCandidate",
		"RejectCandidate",
		"RetainCandidate",
		"ArchiveRevision",
		"RestoreRevision",
		"RollbackActivation",
	} {
		if !functions[name] {
			t.Fatalf("asset authority function %s is missing", name)
		}
	}
}

const (
	digestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type promotionResolver struct{ source assets.PromotionSource }

func (resolver promotionResolver) ResolvePromotion(
	context.Context,
	assets.PromotionRequest,
) (assets.PromotionSource, error) {
	return resolver.source, nil
}

type subjectResolver struct{ subject assets.SubjectIdentity }

func (resolver subjectResolver) ResolveBindingSubject(
	context.Context,
	assets.SubjectIdentity,
) (assets.SubjectIdentity, error) {
	return resolver.subject, nil
}

type templateSink struct {
	mu      sync.Mutex
	outputs []assets.TemplateInstantiation
}

type templateArtifactResolver struct{}

func (templateArtifactResolver) ResolveTemplateArtifact(
	_ context.Context,
	request assets.TemplateArtifactRequest,
) (assets.TemplateArtifactContract, error) {
	var output assets.TemplateOutput
	switch request.AssetKind {
	case assets.AssetKindAgentTemplate:
		output = assets.TemplateOutputAgentCandidate
	case assets.AssetKindTeamTemplate:
		output = assets.TemplateOutputTeamDraft
	case assets.AssetKindWorkPackageTemplate:
		output = assets.TemplateOutputWorkPackageCandidate
	case assets.AssetKindRecoveryStrategyTemplate:
		output = assets.TemplateOutputRecoveryStrategyCandidate
	}
	return assets.TemplateArtifactContract{
		AssetKind: request.AssetKind, DefinitionID: request.DefinitionID,
		RevisionID: request.RevisionID, TemplateOutput: output,
		ParameterSchemaDigest: digestA, PermissionCeilingDigest: digestA,
		ScopeCeilingDigest: digestA,
	}, nil
}

func (sink *templateSink) CreateTemplateOutput(
	_ context.Context,
	output assets.TemplateInstantiation,
) error {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.outputs = append(sink.outputs, output)
	return nil
}

func TestP3AUnconfirmedImportAndNonUserActorsCannotActivate(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	ctx := context.Background()
	imported, err := authority.ImportSkill(ctx, validCreateCommand("import_skill"))
	if err != nil {
		t.Fatalf("ImportSkill() error = %v, want Candidate proposal", err)
	}
	if imported.Revision.Lifecycle != assets.LifecycleCandidate ||
		imported.Definition.ActiveRevisionID != "" {
		t.Fatalf("import result = %#v, want inactive Candidate", imported)
	}
	for _, actor := range []string{"sidecar", "agent", "runtime", "model"} {
		command := validCreateCommand("activate")
		command.OperationID = "activate-by-" + actor
		command.DecisionSource = actor
		command.CandidateID = imported.Candidate.CandidateID
		if _, err := authority.ActivateCandidate(ctx, command); !errors.Is(err, assets.ErrDenied) {
			t.Fatalf("ActivateCandidate(%s) error = %v, want denied", actor, err)
		}
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "EvolutionAssetCandidateActivated" {
			t.Fatalf("non-user actor committed activation: %#v", event)
		}
	}
}

func TestP3ANonterminalOrUnacceptedRunEvidenceCannotPromote(t *testing.T) {
	cases := []struct {
		name   string
		source assets.PromotionSource
	}{
		{name: "nonterminal", source: assets.PromotionSource{RunID: "run-1", RunGeneration: 1}},
		{name: "run-not-accepted", source: assets.PromotionSource{RunID: "run-1", RunGeneration: 1, Terminal: true}},
		{name: "evidence-not-accepted", source: assets.PromotionSource{RunID: "run-1", RunGeneration: 1, Terminal: true, Accepted: true}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			authority, store := openP3AAuthority(t, assets.AuthorityConfig{
				Promotion: promotionResolver{source: test.source},
			})
			command := validCreateCommand("promote_run")
			command.SourceRunID = "run-1"
			command.SourceRunGeneration = 1
			command.SourceEvidenceIDs = []string{"evidence-1"}
			command.SourceEvidenceDigests = []string{digestA}
			if _, err := authority.PromoteRun(context.Background(), command); !errors.Is(err, assets.ErrDenied) {
				t.Fatalf("PromoteRun() error = %v, want denied", err)
			}
			assertJournalEventCount(t, store, 0)
		})
	}
}

func TestP3AStaleAndIdentityMismatchCommandsHaveZeroWrites(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*assets.Command)
		want   error
	}{
		{name: "stale-view", mutate: func(command *assets.Command) { command.ExpectedViewVersion = digestB }, want: assets.ErrStaleView},
		{name: "stale-head", mutate: func(command *assets.Command) {
			command.ExpectedStreamHeads = []journal.StreamHead{{StreamID: "evolution-asset/skill-1", Sequence: 99, EventID: "event-old"}}
		}, want: assets.ErrConflict},
		{name: "wrong-revision", mutate: func(command *assets.Command) { command.Bindings[0].RevisionID = "revision-wrong" }, want: assets.ErrConflict},
		{name: "stale-generation", mutate: func(command *assets.Command) { command.Subject.GenerationID = "generation-old" }, want: assets.ErrStaleGeneration},
		{name: "wrong-digest", mutate: func(command *assets.Command) { command.Bindings[0].SHA256Digest = digestB }, want: assets.ErrDigestMismatch},
		{name: "wrong-subject", mutate: func(command *assets.Command) { command.Subject.SubjectID = "agent-wrong" }, want: assets.ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authority, store := openP3AAuthority(t, assets.AuthorityConfig{
				Subjects: subjectResolver{subject: validSubject()},
			})
			seedActiveSkill(t, authority, "skill-1", "revision-1", "candidate-1", digestA)
			before := len(readAllEvents(t, store))
			command := validCreateCommand("set_binding")
			command.Subject = validSubject()
			command.Bindings = []assets.ExactAssetRevisionBinding{validBinding()}
			test.mutate(&command)
			if _, err := authority.SetBinding(context.Background(), command); !errors.Is(err, test.want) {
				t.Fatalf("SetBinding() error = %v, want %v", err, test.want)
			}
			assertJournalEventCount(t, store, before)
		})
	}
}

func TestP3AConcurrentActivationHasExactlyOneCASWinner(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	commands := []assets.Command{validCreateCommand("activate"), validCreateCommand("activate")}
	commands[0].OperationID = "activate-a"
	commands[1].OperationID = "activate-b"
	commands[0].DecisionSource = "user_explicit"
	commands[1].DecisionSource = "user_explicit"
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for _, command := range commands {
		command := command
		go func() {
			<-start
			_, err := authority.ActivateCandidate(context.Background(), command)
			errorsFound <- err
		}()
	}
	close(start)
	success, conflicts := 0, 0
	for range commands {
		err := <-errorsFound
		switch {
		case err == nil:
			success++
		case errors.Is(err, assets.ErrConflict):
			conflicts++
		default:
			t.Fatalf("concurrent activation error = %v", err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d, want 1/1", success, conflicts)
	}
	assertEventTypeCount(t, store, "EvolutionAssetCandidateActivated", 1)
}

func TestP3AConcurrentActivateAndRollbackHaveExactlyOneCASWinner(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	seedActiveSkill(t, authority, "skill-1", "revision-previous", "candidate-previous", digestB)
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	activateCurrent := validCreateCommand("activate")
	activateCurrent.OperationID = "activate-current"
	activateCurrent.RevisionID = "revision-1"
	activateCurrent.CandidateID = "candidate-1"
	activateCurrent.ArtifactDigest = digestA
	if _, err := authority.ActivateCandidate(context.Background(), activateCurrent); err != nil {
		t.Fatalf("activate current error = %v", err)
	}
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-next", "candidate-next", digestA)
	beforeFacts := 0
	for _, event := range readAllEvents(t, store) {
		if event.Type == "EvolutionAssetCandidateActivated" || event.Type == "EvolutionAssetActivationRolledBack" {
			beforeFacts++
		}
	}
	activate := validCreateCommand("activate")
	activate.OperationID = "activate-race"
	activate.RevisionID = "revision-next"
	activate.CandidateID = "candidate-next"
	activate.ExpectedPreviousRevisionID = "revision-1"
	rollback := validCreateCommand("rollback")
	rollback.OperationID = "rollback-race"
	rollback.RevisionID = "revision-1"
	rollback.ArtifactDigest = digestA
	rollback.ReasonCode = "rollback_requested"
	rollback.TargetRevisionID = "revision-previous"
	rollback.TargetRevisionDigest = digestB
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := authority.ActivateCandidate(context.Background(), activate)
		results <- err
	}()
	go func() {
		<-start
		_, err := authority.RollbackActivation(context.Background(), rollback)
		results <- err
	}()
	close(start)
	success, conflict := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, assets.ErrConflict) {
			conflict++
		} else {
			t.Fatalf("race error = %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d, want 1/1", success, conflict)
	}
	activationFacts := 0
	for _, event := range readAllEvents(t, store) {
		if event.Type == "EvolutionAssetCandidateActivated" ||
			event.Type == "EvolutionAssetActivationRolledBack" {
			activationFacts++
		}
	}
	if activationFacts != beforeFacts+1 {
		t.Fatalf("activation authority facts = %d, want %d", activationFacts, beforeFacts+1)
	}
}

func TestP3AArchiveRestoreRollbackPreserveExactHistory(t *testing.T) {
	authority, _ := openP3AAuthority(t, assets.AuthorityConfig{})
	ctx := context.Background()
	seedActiveSkill(t, authority, "skill-1", "revision-previous", "candidate-previous", digestB)
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	activate := validCreateCommand("activate")
	activate.OperationID = "activate-current"
	activated, err := authority.ActivateCandidate(ctx, activate)
	if err != nil {
		t.Fatalf("activate current error = %v", err)
	}
	if activated.Revision.Lifecycle != assets.LifecycleActive {
		t.Fatalf("activated revision lifecycle = %q, want active", activated.Revision.Lifecycle)
	}
	archive := validCreateCommand("archive")
	archive.ReasonCode = "archive_requested"
	if _, err := authority.ArchiveRevision(ctx, archive); err != nil {
		t.Fatalf("ArchiveRevision() error = %v", err)
	}
	bind := validCreateCommand("set_binding")
	bind.Bindings = []assets.ExactAssetRevisionBinding{validBinding()}
	if _, err := authority.SetBinding(ctx, bind); !errors.Is(err, assets.ErrDenied) {
		t.Fatalf("archived SetBinding() error = %v, want denied", err)
	}
	restore := validCreateCommand("restore")
	restore.OperationID = "restore-1"
	restore.ReasonCode = "restore_requested"
	if _, err := authority.RestoreRevision(ctx, restore); err != nil {
		t.Fatalf("RestoreRevision() error = %v", err)
	}
	rollback := validCreateCommand("rollback")
	rollback.OperationID = "rollback-1"
	rollback.TargetRevisionID = "revision-previous"
	rollback.TargetRevisionDigest = digestB
	rollback.ReasonCode = "rollback_requested"
	rollback.DecisionSource = "user_explicit"
	result, err := authority.RollbackActivation(ctx, rollback)
	if err != nil {
		t.Fatalf("RollbackActivation() error = %v", err)
	}
	if result.Revision.RevisionID != "revision-previous" || result.Revision.ArtifactDigest != digestB ||
		result.Revision.Lifecycle != assets.LifecycleActive {
		t.Fatalf("rollback revision = %#v, want exact previous identity", result.Revision)
	}
}

func TestP3ARedeliveryIsExactOnceAndOperationDriftConflicts(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	command := validCreateCommand("create_skill")
	first, err := authority.CreateSkill(context.Background(), command)
	if err != nil {
		t.Fatalf("first CreateSkill() error = %v", err)
	}
	second, err := authority.CreateSkill(context.Background(), command)
	if err != nil {
		t.Fatalf("redelivered CreateSkill() error = %v", err)
	}
	if len(first.EventIDs) == 0 || !equalStrings(first.EventIDs, second.EventIDs) {
		t.Fatalf("redelivery event IDs = %v / %v", first.EventIDs, second.EventIDs)
	}
	assertJournalEventCount(t, store, len(first.EventIDs))
	drift := command
	drift.Name = "different"
	if _, err := authority.CreateSkill(context.Background(), drift); !errors.Is(err, assets.ErrConflict) {
		t.Fatalf("operation drift error = %v, want conflict", err)
	}
	assertJournalEventCount(t, store, len(first.EventIDs))
}

func TestP3ARecordEvaluationCommitsImmutableExactEvidenceOnce(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	usage, cost := int64(42), int64(7)
	command := validCreateCommand("record_evaluation")
	command.OperationID = "evaluation-operation-1"
	command.Evaluation = &assets.EvaluationRecord{
		EvaluationID: "evaluation-1", CandidateID: "candidate-1",
		FixtureKind: "synthetic", FixtureDigest: digestB,
		BaselineRevisionID: "revision-baseline", BaselineDigest: digestB,
		CandidateRevisionID: "revision-1", CandidateDigest: digestA,
		QualityResult: "pass", FailureCount: 0, CaseCount: 12,
		UsageObserved: true, UsageValue: &usage,
		CostObserved: true, CostValue: &cost,
		CompatibilityResult: "compatible", ApplicableScope: "project",
		RegressionResult: "equivalent", SecurityResult: "pass",
		EvidenceID: "evidence-1", EvidenceDigest: digestB,
	}
	first, err := authority.RecordEvaluation(context.Background(), command)
	if err != nil {
		t.Fatalf("RecordEvaluation() error = %v", err)
	}
	second, err := authority.RecordEvaluation(context.Background(), command)
	if err != nil {
		t.Fatalf("RecordEvaluation() redelivery error = %v", err)
	}
	if !equalStrings(first.EventIDs, second.EventIDs) || first.Evaluation.EvaluationID != "evaluation-1" {
		t.Fatalf("evaluation redelivery = %#v / %#v", first, second)
	}
	assertEventTypeCount(t, store, "EvolutionAssetEvaluationRecorded", 1)
	drift := command
	driftEvaluation := *drift.Evaluation
	driftEvaluation.QualityResult = "fail"
	drift.Evaluation = &driftEvaluation
	if _, err := authority.RecordEvaluation(context.Background(), drift); !errors.Is(err, assets.ErrConflict) {
		t.Fatalf("evaluation operation drift error = %v, want conflict", err)
	}
}

func TestP3ARecordEvaluationRejectsUnknownMeasurementsAndDigestDriftWithoutWrite(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	base := validCreateCommand("record_evaluation")
	base.OperationID = "evaluation-invalid"
	base.Evaluation = &assets.EvaluationRecord{
		EvaluationID: "evaluation-1", CandidateID: "candidate-1",
		FixtureKind: "historical", FixtureDigest: digestB,
		BaselineRevisionID: "revision-baseline", BaselineDigest: digestB,
		CandidateRevisionID: "revision-1", CandidateDigest: digestA,
		QualityResult: "pass", CaseCount: 1,
		CompatibilityResult: "compatible", ApplicableScope: "project",
		RegressionResult: "equivalent", SecurityResult: "pass",
		EvidenceID: "evidence-1", EvidenceDigest: digestB,
	}
	before := len(readAllEvents(t, store))
	zero := int64(0)
	unknownWithValue := base
	unknownEvaluation := *unknownWithValue.Evaluation
	unknownEvaluation.UsageValue = &zero
	unknownWithValue.Evaluation = &unknownEvaluation
	if _, err := authority.RecordEvaluation(context.Background(), unknownWithValue); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("unknown usage with value error = %v", err)
	}
	drift := base
	drift.OperationID = "evaluation-digest-drift"
	driftEvaluation := *drift.Evaluation
	driftEvaluation.CandidateDigest = digestB
	drift.Evaluation = &driftEvaluation
	if _, err := authority.RecordEvaluation(context.Background(), drift); !errors.Is(err, assets.ErrDigestMismatch) {
		t.Fatalf("candidate digest drift error = %v", err)
	}
	assertJournalEventCount(t, store, before)
}

func TestP3AAllTemplateKindsProduceOnlyTypedDraftOrCandidate(t *testing.T) {
	outputs := &templateSink{}
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{
		TemplateOutputs: outputs, TemplateArtifacts: templateArtifactResolver{},
	})
	tests := []struct {
		kind   assets.AssetKind
		output assets.TemplateOutput
	}{
		{assets.AssetKindAgentTemplate, assets.TemplateOutputAgentCandidate},
		{assets.AssetKindTeamTemplate, assets.TemplateOutputTeamDraft},
		{assets.AssetKindWorkPackageTemplate, assets.TemplateOutputWorkPackageCandidate},
		{assets.AssetKindRecoveryStrategyTemplate, assets.TemplateOutputRecoveryStrategyCandidate},
	}
	for index, test := range tests {
		definitionID := "template-" + string(rune('a'+index))
		candidateID := "template-candidate-" + string(rune('a'+index))
		create := validCreateCommand("create_template")
		create.OperationID = "create-template-" + string(rune('a'+index))
		create.AssetKind = test.kind
		create.DefinitionID = definitionID
		create.CandidateID = candidateID
		create.TemplateOutput = test.output
		if _, err := authority.CreateTemplate(context.Background(), create); err != nil {
			t.Fatalf("CreateTemplate(%s) error = %v", test.kind, err)
		}
		command := validCreateCommand("instantiate_template")
		command.OperationID = "instantiate-" + string(rune('a'+index))
		command.AssetKind = test.kind
		command.DefinitionID = definitionID
		command.CandidateID = "output-" + string(rune('a'+index))
		command.TemplateOutput = test.output
		command.ParametersDigest = digestA
		result, err := authority.InstantiateTemplate(context.Background(), command)
		if err != nil {
			t.Fatalf("InstantiateTemplate(%s) error = %v", test.kind, err)
		}
		if result.TemplateInstantiation.TemplateOutput != test.output {
			t.Fatalf("output = %q, want %q", result.TemplateInstantiation.TemplateOutput, test.output)
		}
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		switch event.Type {
		case "TeamInstanceCreated", "RunCreated", "AgentGrantIssued", "TeamExecutionDispatchCommitted":
			t.Fatalf("template instantiation bypassed authority with %s", event.Type)
		}
	}
}

func TestP3ATemplateInstantiatedEventCarriesExactRevisionDigest(t *testing.T) {
	outputs := &templateSink{}
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{
		TemplateOutputs: outputs, TemplateArtifacts: templateArtifactResolver{},
	})
	ctx := context.Background()
	create := validCreateCommand("create_template")
	create.OperationID = "create-template-digest"
	create.AssetKind = assets.AssetKindAgentTemplate
	create.CandidateID = "template-candidate-digest"
	create.TemplateOutput = assets.TemplateOutputAgentCandidate
	if _, err := authority.CreateTemplate(ctx, create); err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	command := validCreateCommand("instantiate_template")
	command.OperationID = "instantiate-digest"
	command.AssetKind = assets.AssetKindAgentTemplate
	command.DefinitionID = "skill-1"
	command.CandidateID = "output-digest"
	command.TemplateOutput = assets.TemplateOutputAgentCandidate
	command.ParametersDigest = digestA
	if _, err := authority.InstantiateTemplate(ctx, command); err != nil {
		t.Fatalf("InstantiateTemplate() error = %v", err)
	}
	found := false
	for _, event := range readAllEvents(t, store) {
		if event.Type != "EvolutionTemplateInstantiated" {
			continue
		}
		found = true
		var payload struct {
			RevisionDigest string `json:"revision_digest"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal template payload: %v", err)
		}
		if payload.RevisionDigest != digestA {
			t.Fatalf("template revision_digest = %q, want %q", payload.RevisionDigest, digestA)
		}
	}
	if !found {
		t.Fatal("EvolutionTemplateInstantiated event not found")
	}
}

func TestP3AArchiveActiveRevisionRecordsActivePreviousLifecycle(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	ctx := context.Background()
	seedActiveSkill(t, authority, "skill-1", "revision-previous", "candidate-previous", digestB)
	createAssetCandidate(t, authority, assets.AssetKindSkill, "skill-1", "revision-1", "candidate-1", digestA)
	activate := validCreateCommand("activate")
	activate.OperationID = "activate-current"
	if _, err := authority.ActivateCandidate(ctx, activate); err != nil {
		t.Fatalf("ActivateCandidate() error = %v", err)
	}
	archive := validCreateCommand("archive")
	archive.ReasonCode = "archive_requested"
	if _, err := authority.ArchiveRevision(ctx, archive); err != nil {
		t.Fatalf("ArchiveRevision() error = %v", err)
	}
	found := false
	for _, event := range readAllEvents(t, store) {
		if event.Type != "EvolutionAssetRevisionArchived" {
			continue
		}
		found = true
		var payload struct {
			PreviousLifecycle string `json:"previous_lifecycle"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal archive payload: %v", err)
		}
		if payload.PreviousLifecycle != "active" {
			t.Fatalf("archive previous_lifecycle = %q, want active", payload.PreviousLifecycle)
		}
	}
	if !found {
		t.Fatal("EvolutionAssetRevisionArchived event not found")
	}
}

func TestP3ACreateRejectsUnboundedTextAndCapabilityLists(t *testing.T) {
	authority, _ := openP3AAuthority(t, assets.AuthorityConfig{})
	ctx := context.Background()
	command := validCreateCommand("create_skill")
	command.OperationID = "create-bounds-1"
	command.Name = strings.Repeat("n", 129)
	if _, err := authority.CreateSkill(ctx, command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("129-byte name error = %v, want ErrInvalidInput", err)
	}
	command = validCreateCommand("create_skill")
	command.OperationID = "create-bounds-2"
	command.Description = "bad\x1b[31mescape"
	if _, err := authority.CreateSkill(ctx, command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("terminal-escape description error = %v, want ErrInvalidInput", err)
	}
	command = validCreateCommand("create_skill")
	command.OperationID = "create-bounds-3"
	command.Description = strings.Repeat("d", 4097)
	if _, err := authority.CreateSkill(ctx, command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("oversized description error = %v, want ErrInvalidInput", err)
	}
	command = validCreateCommand("create_skill")
	command.OperationID = "create-bounds-4"
	command.Dependencies = make([]string, 33)
	for index := range command.Dependencies {
		command.Dependencies[index] = fmt.Sprintf("dep-%d", index)
	}
	if _, err := authority.CreateSkill(ctx, command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("33-dependency error = %v, want ErrInvalidInput", err)
	}
	command = validCreateCommand("create_skill")
	command.OperationID = "create-bounds-5"
	command.CompatibleCapabilities = make([]string, 33)
	for index := range command.CompatibleCapabilities {
		command.CompatibleCapabilities[index] = fmt.Sprintf("cap-%d", index)
	}
	if _, err := authority.CreateSkill(ctx, command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("33-capability error = %v, want ErrInvalidInput", err)
	}
}

func TestP3ACandidateCreatedPayloadExcludesDecisionFields(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	ctx := context.Background()
	command := validCreateCommand("create_skill")
	command.OperationID = "create-no-decision"
	if _, err := authority.CreateSkill(ctx, command); err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	found := false
	for _, event := range readAllEvents(t, store) {
		if event.Type != "EvolutionAssetCandidateCreated" {
			continue
		}
		found = true
		var payload map[string]any
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal candidate payload: %v", err)
		}
		if _, present := payload["decision"]; present {
			t.Fatalf("candidate-created payload carries decision field")
		}
		if _, present := payload["decision_event_id"]; present {
			t.Fatalf("candidate-created payload carries decision_event_id field")
		}
	}
	if !found {
		t.Fatal("EvolutionAssetCandidateCreated event not found")
	}
}

func TestP3AMaterializationRejectsEmptyBindingSet(t *testing.T) {
	authority, _ := openP3AAuthority(t, assets.AuthorityConfig{})
	command := validCreateCommand("publish_materialization")
	command.OperationID = "materialize-empty"
	command.TeamExecutionID = "team-1"
	command.LogicalNodeID = "main"
	command.RunID = "run-1"
	command.AttemptNumber = 1
	command.Generation = 1
	command.RuntimeInstanceID = "runtime-1"
	command.RuntimeIdentityDigest = digestB
	command.Capability = "loom.skill-materialization.pi.v1"
	command.Bindings = []assets.ExactAssetRevisionBinding{}
	command.AssetRevisionSetDigest = digestA
	command.ManifestArtifactDigest = digestA
	command.MaterializationRootDigest = digestB
	if _, err := authority.PrepareMaterialization(context.Background(), command); !errors.Is(err, assets.ErrInvalidInput) {
		t.Fatalf("empty binding set error = %v, want ErrInvalidInput", err)
	}
}

func TestP3AEventsNeverDiscloseSecretsGrantsPromptsOrTokens(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	command := validCreateCommand("create_skill")
	command.Description = "safe summary"
	if _, err := authority.CreateSkill(context.Background(), command); err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	for _, event := range readAllEvents(t, store) {
		payload := string(event.PayloadJSON)
		for _, forbidden := range []string{
			"sk-secret", "raw_grant", "hidden_reasoning", "sensitive_prompt", "token_delta",
		} {
			if contains(payload, forbidden) {
				t.Fatalf("event %s disclosed %q in %s", event.ID, forbidden, payload)
			}
		}
	}
}

func TestP3APreparedMaterializationCommitsOnlyInsideCompositeCAS(t *testing.T) {
	authority, store := openP3AAuthority(t, assets.AuthorityConfig{})
	seedActiveSkill(t, authority, "skill-1", "revision-1", "candidate-1", digestA)
	bindings := []assets.ExactAssetRevisionBinding{validBinding()}
	setDigest, err := assets.CanonicalAssetRevisionSetDigest(bindings)
	if err != nil {
		t.Fatal(err)
	}
	command := validCreateCommand("publish_materialization")
	command.OperationID = "materialize-run-1"
	command.TeamExecutionID = "team-1"
	command.LogicalNodeID = "main"
	command.RunID = "run-1"
	command.AttemptNumber = 1
	command.Generation = 1
	command.RuntimeInstanceID = "runtime-1"
	command.RuntimeIdentityDigest = digestB
	command.Capability = "loom.skill-materialization.pi.v1"
	command.Bindings = bindings
	command.AssetRevisionSetDigest = setDigest
	command.ManifestArtifactDigest = digestA
	command.MaterializationRootDigest = digestB
	prepared, err := authority.PrepareMaterialization(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	assertEventTypeCount(t, store, "RuntimeSkillMaterializationPublished", 0)
	teamEvent := journal.Event{
		ID: "team-dispatch-1", StreamID: "team-execution/team-1", Seq: 1,
		IdempotencyKey: "team-dispatch-1", Type: "TeamReadySetDispatched",
		SchemaVersion: 1,
		EmittedAt:     time.Date(2026, 8, 3, 10, 0, 1, 0, time.UTC),
		CorrelationID: command.JourneyID, PayloadJSON: []byte(`{"fixture":true}`),
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(prepared.Heads())+1)
	for _, head := range prepared.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
	}
	expectations = append(expectations, journal.StreamHeadExpectation{
		StreamID: teamEvent.StreamID, Sequence: 0,
	})
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(), expectations,
		[]journal.Event{teamEvent, prepared.Event()},
	); err != nil {
		t.Fatal(err)
	}
	teamEvents, err := store.ReadStream(context.Background(), teamEvent.StreamID)
	if err != nil || len(teamEvents) != 1 {
		t.Fatalf("team dispatch facts = %d, %v", len(teamEvents), err)
	}

	loser := command
	loser.OperationID = "materialize-run-2"
	loser.RunID = "run-2"
	loser.ManifestArtifactDigest = digestB
	loser.MaterializationRootDigest = digestA
	loserPrepared, err := authority.PrepareMaterialization(context.Background(), loser)
	if err != nil {
		t.Fatal(err)
	}
	loserEvent := loserPrepared.Event()
	blocker := loserEvent
	blocker.ID = "materialization-blocker"
	blocker.IdempotencyKey = "materialization-blocker"
	blocker.Type = "FixtureBlocker"
	blocker.PayloadJSON = []byte(`{"fixture":"blocker"}`)
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(),
		[]journal.StreamHeadExpectation{{StreamID: blocker.StreamID, Sequence: 0}},
		[]journal.Event{blocker},
	); err != nil {
		t.Fatal(err)
	}
	loserTeam := teamEvent
	loserTeam.ID = "team-dispatch-loser"
	loserTeam.IdempotencyKey = "team-dispatch-loser"
	loserTeam.StreamID = "team-execution/team-loser"
	expectations = expectations[:0]
	for _, head := range loserPrepared.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
	}
	expectations = append(expectations, journal.StreamHeadExpectation{
		StreamID: loserTeam.StreamID, Sequence: 0,
	})
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(), expectations,
		[]journal.Event{loserEvent, loserTeam},
	); !errors.Is(err, journal.ErrSequenceConflict) &&
		!errors.Is(err, journal.ErrStreamHeadConflict) {
		t.Fatalf("losing composite CAS error = %v", err)
	}
	loserTeamEvents, err := store.ReadStream(context.Background(), loserTeam.StreamID)
	if err != nil || len(loserTeamEvents) != 0 {
		t.Fatalf("losing CAS wrote Team fact = %d, %v", len(loserTeamEvents), err)
	}
	assertEventTypeCount(t, store, "RuntimeSkillMaterializationPublished", 1)
}

func openP3AAuthority(t testing.TB, config assets.AuthorityConfig) (*assets.Authority, *journal.Store) {
	t.Helper()
	values := url.Values{}
	values.Set("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "journal.db")+"?"+values.Encode())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	config.Store = store
	if config.ViewVersion == nil {
		config.ViewVersion = func() string { return digestA }
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC) }
	}
	authority, err := assets.NewAuthority(config)
	if err != nil {
		t.Fatal(err)
	}
	return authority, store
}

func createAssetCandidate(
	t testing.TB,
	authority *assets.Authority,
	kind assets.AssetKind,
	definitionID string,
	revisionID string,
	candidateID string,
	digest string,
) {
	t.Helper()
	command := validCreateCommand("create_skill")
	command.OperationID = "create-" + candidateID
	command.AssetKind = kind
	command.DefinitionID = definitionID
	command.RevisionID = revisionID
	command.CandidateID = candidateID
	command.ArtifactDigest = digest
	command.ContentDigest = digest
	if kind == assets.AssetKindSkill {
		if _, err := authority.CreateSkill(context.Background(), command); err != nil {
			t.Fatalf("CreateSkill(%s) error = %v", candidateID, err)
		}
	} else if _, err := authority.CreateTemplate(context.Background(), command); err != nil {
		t.Fatalf("CreateTemplate(%s) error = %v", candidateID, err)
	}
}

func seedActiveSkill(t testing.TB, authority *assets.Authority, definitionID, revisionID, candidateID, digest string) {
	t.Helper()
	createAssetCandidate(t, authority, assets.AssetKindSkill, definitionID, revisionID, candidateID, digest)
	activate := validCreateCommand("activate")
	activate.OperationID = "activate-" + candidateID
	activate.DefinitionID = definitionID
	activate.RevisionID = revisionID
	activate.CandidateID = candidateID
	activate.ArtifactDigest = digest
	if _, err := authority.ActivateCandidate(context.Background(), activate); err != nil {
		t.Fatalf("ActivateCandidate(%s) error = %v", candidateID, err)
	}
}

func validCreateCommand(action string) assets.Command {
	return assets.Command{
		Action: action, OperationID: "operation-1",
		JourneyID:           "123e4567-e89b-42d3-a456-426614174000",
		ExpectedViewVersion: digestA,
		DecisionSource:      "user_explicit", AssetKind: assets.AssetKindSkill,
		DefinitionID: "skill-1", RevisionID: "revision-1", CandidateID: "candidate-1",
		Name: "Review Skill", Description: "safe summary", Scope: "project",
		ArtifactDigest: digestA, ContentDigest: digestA,
		SourceScope: assets.SourceScopeLocal, SourceReferenceDigest: digestA,
		ProvenanceDigest: digestA, Risk: assets.RiskLow,
		CompatibleCapabilities: []string{"loom.skill-materialization.pi.v1"},
		TemplateOutput:         assets.TemplateOutputAgentCandidate,
		ParameterSchemaDigest:  digestA, PermissionCeilingDigest: digestA,
		ScopeCeilingDigest: digestA, Subject: validSubject(),
	}
}

func validBinding() assets.ExactAssetRevisionBinding {
	return assets.ExactAssetRevisionBinding{
		AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
		RevisionID: "revision-1", SHA256Digest: digestA,
		SourceScope: assets.SourceScopeLocal,
	}
}

func validSubject() assets.SubjectIdentity {
	return assets.SubjectIdentity{
		SubjectKind: "agent_definition", SubjectID: "agent-1", SubjectVersion: 1,
		SubjectDigest: digestA, Scope: "transient", ProjectID: "",
		GenerationID: "generation-1",
	}
}

func assertJournalEventCount(t testing.TB, store *journal.Store, want int) {
	t.Helper()
	if got := len(readAllEvents(t, store)); got != want {
		t.Fatalf("Journal event count = %d, want %d", got, want)
	}
}

func assertEventTypeCount(t testing.TB, store *journal.Store, eventType string, want int) {
	t.Helper()
	got := 0
	for _, event := range readAllEvents(t, store) {
		if event.Type == eventType {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", eventType, got, want)
	}
}

func readAllEvents(t testing.TB, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
