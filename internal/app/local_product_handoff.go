package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidSideTaskProduct         = errors.New("invalid Side-task product request")
	ErrSideTaskProductConflict        = errors.New("Side-task product conflict")
	ErrSideTaskProductCapabilityGap   = errors.New("Side-task product capability gap")
	ErrSideTaskProductUnavailable     = errors.New("Side-task product unavailable")
	ErrSideTaskProductStaleView       = errors.New("Side-task product stale view")
	ErrSideTaskProductStaleGeneration = errors.New("Side-task product stale generation")
	ErrSideTaskProductDigestMismatch  = errors.New("Side-task product digest mismatch")
	ErrSideTaskProductHumanRequired   = errors.New("Side-task product human required")
)

const (
	SideTaskProductSchemaVersion    = 1
	maxSideTaskInputArtifactBytes   = 16 << 10
	maxSideTaskSummaryArtifactBytes = 64 << 10
	maxSideTaskContextPacketBytes   = 16 << 10
	maxSideTaskProductInFlight      = 64
)

type SideTaskEvidenceReference struct {
	EvidenceID string `json:"evidence_id"`
	Digest     string `json:"digest"`
	Kind       string `json:"kind"`
}

type SideTaskArtifactReference struct {
	Digest string `json:"digest"`
	Kind   string `json:"kind"`
}

type SideTaskInputArtifact struct {
	SchemaVersion          int      `json:"schema_version"`
	SideTaskID             string   `json:"side_task_id"`
	ParentMissionID        string   `json:"parent_mission_id"`
	ParentTaskID           string   `json:"parent_task_id"`
	ParentRunID            string   `json:"parent_run_id"`
	ParentClaimGeneration  int64    `json:"parent_claim_generation"`
	ParentExecutionDigest  string   `json:"parent_execution_digest"`
	Purpose                string   `json:"purpose"`
	Mode                   string   `json:"mode"`
	Title                  string   `json:"title"`
	AuthorizedRequest      string   `json:"authorized_request"`
	PermissionScopes       []string `json:"permission_scopes"`
	ProposalDigest         string   `json:"proposal_digest"`
	DecisionTimeoutSeconds int64    `json:"decision_timeout_seconds"`
	CreatedAt              string   `json:"created_at"`
}

type SideTaskSummaryArtifact struct {
	SchemaVersion           int                         `json:"schema_version"`
	SideTaskID              string                      `json:"side_task_id"`
	ParentTaskID            string                      `json:"parent_task_id"`
	Purpose                 string                      `json:"purpose"`
	Status                  string                      `json:"status"`
	SourceGeneration        int64                       `json:"source_generation"`
	SummaryVersion          int                         `json:"summary_version"`
	WhatHappened            string                      `json:"what_happened"`
	AuthorizedFindings      []string                    `json:"authorized_findings"`
	EvidenceReferences      []SideTaskEvidenceReference `json:"evidence_references"`
	ArtifactReferences      []SideTaskArtifactReference `json:"artifact_references"`
	Risk                    string                      `json:"risk"`
	Uncertainties           []string                    `json:"uncertainties"`
	ScopeDelta              []string                    `json:"scope_delta"`
	DecisionOptions         []string                    `json:"decision_options"`
	RecommendedOption       string                      `json:"recommended_option"`
	RecommendationAuthority string                      `json:"recommendation_authority"`
	UsageObserved           bool                        `json:"usage_observed"`
	UsageMicrounits         int64                       `json:"usage_microunits"`
	UsageCurrency           string                      `json:"usage_currency"`
	CreatedAt               string                      `json:"created_at"`
}

type SideTaskContextPacket struct {
	SchemaVersion         int      `json:"schema_version"`
	ContextPacketID       string   `json:"context_packet_id"`
	SideTaskID            string   `json:"side_task_id"`
	ParentTaskID          string   `json:"parent_task_id"`
	ParentRunID           string   `json:"parent_run_id"`
	ParentClaimGeneration int64    `json:"parent_claim_generation"`
	SideTaskGeneration    int64    `json:"side_task_generation"`
	HandoffVersion        int      `json:"handoff_version"`
	HandoffDigest         string   `json:"handoff_digest"`
	SummaryArtifactDigest string   `json:"summary_artifact_digest"`
	AuthorizedFindings    []string `json:"authorized_findings"`
	Risk                  string   `json:"risk"`
	Uncertainties         []string `json:"uncertainties"`
	ScopeDelta            []string `json:"scope_delta"`
	CreatedAt             string   `json:"created_at"`
}

type SideTaskProposalRequest struct {
	SchemaVersion          int      `json:"schema_version"`
	Operation              string   `json:"operation"`
	ParentMissionID        string   `json:"parent_mission_id"`
	ParentTeamInstanceID   string   `json:"parent_team_instance_id"`
	ParentTaskID           string   `json:"parent_task_id"`
	ParentRunID            string   `json:"parent_run_id"`
	ParentClaimGeneration  int64    `json:"parent_claim_generation"`
	ParentExecutionDigest  string   `json:"parent_execution_digest"`
	Purpose                string   `json:"purpose"`
	Mode                   string   `json:"mode"`
	Title                  string   `json:"title"`
	AuthorizedRequest      string   `json:"authorized_request"`
	PermissionScopes       []string `json:"permission_scopes"`
	DecisionTimeoutSeconds int64    `json:"decision_timeout_seconds"`
	ExpectedViewVersion    string   `json:"expected_view_version"`
	CorrelationID          string   `json:"correlation_id"`
}

type SideTaskProposalResult struct {
	SchemaVersion          int      `json:"schema_version"`
	Status                 string   `json:"status"`
	ProposalDigest         string   `json:"proposal_digest"`
	ViewVersion            string   `json:"view_version"`
	Purpose                string   `json:"purpose"`
	Mode                   string   `json:"mode"`
	Title                  string   `json:"title"`
	PermissionScopes       []string `json:"permission_scopes"`
	DecisionTimeoutSeconds int64    `json:"decision_timeout_seconds"`
	RequiresConfirmation   bool     `json:"requires_confirmation"`
	PolicyAvailable        bool     `json:"policy_available"`
}

type SideTaskCreateRequest struct {
	SideTaskProposalRequest
	ProposalDigest string `json:"proposal_digest"`
	Confirmed      bool   `json:"confirmed"`
	PolicyStreamID string `json:"policy_stream_id"`
	PolicyVersion  int    `json:"policy_version"`
	PolicyDigest   string `json:"policy_digest"`
}

type SideTaskCreateResult struct {
	SchemaVersion               int    `json:"schema_version"`
	SideTaskID                  string `json:"side_task_id"`
	Status                      string `json:"status"`
	ViewVersion                 string `json:"view_version"`
	SideExecutionTeamInstanceID string `json:"side_execution_team_instance_id"`
	ProposalDigest              string `json:"proposal_digest"`
	HandoffVersion              int    `json:"handoff_version"`
	HandoffDigest               string `json:"handoff_digest"`
}

type SideTaskReadRequest struct {
	SchemaVersion       int    `json:"schema_version"`
	Operation           string `json:"operation"`
	SideTaskID          string `json:"side_task_id"`
	ExpectedViewVersion string `json:"expected_view_version"`
	CorrelationID       string `json:"correlation_id"`
}

type SideTaskReadResult struct {
	SchemaVersion               int                         `json:"schema_version"`
	SideTaskID                  string                      `json:"side_task_id"`
	ParentMissionID             string                      `json:"parent_mission_id"`
	ParentTeamInstanceID        string                      `json:"parent_team_instance_id"`
	ParentTaskID                string                      `json:"parent_task_id"`
	ParentRunID                 string                      `json:"parent_run_id"`
	ParentClaimGeneration       int64                       `json:"parent_claim_generation"`
	ParentExecutionDigest       string                      `json:"parent_execution_digest"`
	SideExecutionTeamInstanceID string                      `json:"side_execution_team_instance_id"`
	Purpose                     string                      `json:"purpose"`
	Mode                        string                      `json:"mode"`
	Title                       string                      `json:"title"`
	Status                      string                      `json:"status"`
	SourceGeneration            int64                       `json:"source_generation"`
	HandoffVersion              int                         `json:"handoff_version"`
	HandoffDigest               string                      `json:"handoff_digest"`
	SummaryArtifactDigest       string                      `json:"summary_artifact_digest"`
	WhatHappened                string                      `json:"what_happened"`
	AuthorizedFindings          []string                    `json:"authorized_findings"`
	EvidenceReferences          []SideTaskEvidenceReference `json:"evidence_references"`
	ArtifactReferences          []SideTaskArtifactReference `json:"artifact_references"`
	Risk                        string                      `json:"risk"`
	Uncertainties               []string                    `json:"uncertainties"`
	ScopeDelta                  []string                    `json:"scope_delta"`
	DecisionOptions             []string                    `json:"decision_options"`
	RecommendedOption           string                      `json:"recommended_option"`
	RecommendationAuthority     string                      `json:"recommendation_authority"`
	UsageObserved               bool                        `json:"usage_observed"`
	UsageMicrounits             int64                       `json:"usage_microunits"`
	UsageCurrency               string                      `json:"usage_currency"`
	DecisionDeadline            string                      `json:"decision_deadline"`
	AvailableDecisions          []string                    `json:"available_decisions"`
	EffectStatus                string                      `json:"effect_status"`
	ViewVersion                 string                      `json:"view_version"`
}

type SideTaskDecisionRequest struct {
	SchemaVersion         int    `json:"schema_version"`
	Operation             string `json:"operation"`
	SideTaskID            string `json:"side_task_id"`
	ParentMissionID       string `json:"parent_mission_id"`
	ParentTeamInstanceID  string `json:"parent_team_instance_id"`
	ParentTaskID          string `json:"parent_task_id"`
	ParentRunID           string `json:"parent_run_id"`
	ParentLogicalNodeID   string `json:"parent_logical_node_id"`
	ParentAttemptNumber   int    `json:"parent_attempt_number"`
	ParentClaimGeneration int64  `json:"parent_claim_generation"`
	ParentExecutionDigest string `json:"parent_execution_digest"`
	SideTaskGeneration    int64  `json:"side_task_generation"`
	HandoffVersion        int    `json:"handoff_version"`
	HandoffDigest         string `json:"handoff_digest"`
	Decision              string `json:"decision"`
	EffectDigest          string `json:"effect_digest"`
	ExpectedViewVersion   string `json:"expected_view_version"`
	CorrelationID         string `json:"correlation_id"`
}

type SideTaskDecisionResult struct {
	SchemaVersion                       int    `json:"schema_version"`
	SideTaskID                          string `json:"side_task_id"`
	Decision                            string `json:"decision"`
	Status                              string `json:"status"`
	EffectStatus                        string `json:"effect_status"`
	ContextPacketDigest                 string `json:"context_packet_digest"`
	ContinuationExecutionTeamInstanceID string `json:"continuation_execution_team_instance_id"`
	ViewVersion                         string `json:"view_version"`
}

type SideTaskArtifactStore interface {
	Publish(context.Context, io.Reader, string) (evidence.Artifact, error)
	ReadArtifact(context.Context, string, int64) ([]byte, error)
}

type SideTaskExecutionBindingSource struct {
	parent MissionExecutionBindingSource
}

func NewProjectionSideTaskExecutionBindingSource(
	parent MissionExecutionBindingSource,
) (*SideTaskExecutionBindingSource, error) {
	if nilMissionExecutionInterface(parent) {
		return nil, ErrInvalidSideTaskProduct
	}
	return &SideTaskExecutionBindingSource{parent: parent}, nil
}

func (source *SideTaskExecutionBindingSource) ResolveSideTaskExecutionBinding(
	ctx context.Context,
	parentTeamInstanceID string,
	childTeamInstanceID string,
) (MissionExecutionBinding, error) {
	if source == nil || ctx == nil || parentTeamInstanceID == "" ||
		childTeamInstanceID == "" || parentTeamInstanceID == childTeamInstanceID {
		return MissionExecutionBinding{}, ErrInvalidSideTaskProduct
	}
	binding, err := source.parent.ResolveMissionExecutionBinding(ctx, parentTeamInstanceID)
	if err != nil {
		return MissionExecutionBinding{}, err
	}
	binding.TeamInstanceID = childTeamInstanceID
	return binding, nil
}

type SideTaskExecutionCompileInput struct {
	SideTaskID           string
	ParentTeamInstanceID string
	ChildTeamInstanceID  string
	Objective            string
	PermissionScopes     []string
	ExpectedViewVersion  string
	CorrelationID        string
}

type SideTaskExecutionCompilation struct {
	MissionExecutionCompilation
	Collector *sideTaskOutputCollector
}

type BuiltInSideTaskExecutionCompiler struct {
	bindings   *SideTaskExecutionBindingSource
	sourcePath string
	now        func() time.Time
}

func NewBuiltInSideTaskExecutionCompiler(
	bindings *SideTaskExecutionBindingSource,
	sourcePath string,
	now func() time.Time,
) (*BuiltInSideTaskExecutionCompiler, error) {
	if bindings == nil || now == nil || !validMissionExecutionSourcePath(sourcePath) {
		return nil, ErrInvalidSideTaskProduct
	}
	return &BuiltInSideTaskExecutionCompiler{bindings: bindings, sourcePath: sourcePath, now: now}, nil
}

func (compiler *BuiltInSideTaskExecutionCompiler) CompileSideTaskExecution(
	ctx context.Context,
	input SideTaskExecutionCompileInput,
) (SideTaskExecutionCompilation, error) {
	if compiler == nil || ctx == nil || input.SideTaskID == "" ||
		input.ParentTeamInstanceID == "" || input.ChildTeamInstanceID == "" ||
		input.Objective == "" || len(input.Objective) > 4096 ||
		!validSHA256(input.ExpectedViewVersion) {
		return SideTaskExecutionCompilation{}, ErrInvalidSideTaskProduct
	}
	binding, err := compiler.bindings.ResolveSideTaskExecutionBinding(
		ctx, input.ParentTeamInstanceID, input.ChildTeamInstanceID,
	)
	if err != nil || binding.ViewVersion != input.ExpectedViewVersion {
		return SideTaskExecutionCompilation{}, errors.Join(ErrSideTaskProductConflict, err)
	}
	collector := &sideTaskOutputCollector{}
	fallbackApprovals, _ := compiler.bindings.parent.(MissionFallbackApprovalSource)
	missionCompiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings:          compiler.bindings.parent,
			FallbackApprovals: fallbackApprovals,
			SourcePath:        compiler.sourcePath,
			OutputObserver:    collector, Now: compiler.now,
		},
	)
	if err != nil {
		return SideTaskExecutionCompilation{}, err
	}
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		return SideTaskExecutionCompilation{}, err
	}
	command := MissionExecutionCommand{
		SchemaVersion:  MissionExecutionSchemaVersion,
		Operation:      missionExecutionStart,
		MissionID:      "mission/" + input.ChildTeamInstanceID,
		TeamInstanceID: input.ChildTeamInstanceID,
		WorkPackageID:  workPackage.ID(), WorkPackageDigest: workPackage.Digest(),
		Objective: input.Objective, ExpectedViewVersion: input.ExpectedViewVersion,
		PreflightDigest: strings.Repeat("0", 64), CorrelationID: input.CorrelationID,
	}
	compilation, err := missionCompiler.compileMissionExecution(ctx, command, binding, true)
	if err != nil {
		return SideTaskExecutionCompilation{}, err
	}
	compilation.Preflight.PermissionScopes = append([]string{}, input.PermissionScopes...)
	compilation.Preflight.SideEffects = []string{}
	return SideTaskExecutionCompilation{MissionExecutionCompilation: compilation, Collector: collector}, nil
}

type sideTaskOutputCollector struct {
	mu      sync.Mutex
	results [][]byte
}

func (collector *sideTaskOutputCollector) ObserveNodeOutput(
	ctx context.Context,
	output NodeOutput,
) error {
	if collector == nil || ctx == nil {
		return ErrInvalidSideTaskProduct
	}
	frame := output.AuthorizedFrame().Frame()
	if frame.Type() != bridgev1.MessageResult {
		return nil
	}
	payload := frame.Payload()
	if len(payload) == 0 || len(payload) > 4096 || !utf8.Valid(payload) {
		return ErrInvalidSideTaskProduct
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.results = append(collector.results, append([]byte{}, payload...))
	return nil
}

func (collector *sideTaskOutputCollector) AuthorizedResult() (string, error) {
	if collector == nil {
		return "", ErrInvalidSideTaskProduct
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if len(collector.results) != 1 {
		return "", ErrInvalidSideTaskProduct
	}
	return string(collector.results[0]), nil
}

type LocalProductHandoffConfig struct {
	Authority       *work.Authority
	Projection      *projection.Projection
	Artifacts       SideTaskArtifactStore
	Compiler        *BuiltInSideTaskExecutionCompiler
	Runner          MissionExecutionRunner
	ParentCanceller ParentMissionCancellationPort
	ParentExecution ParentMissionExecutionDigestPort
	Now             func() time.Time
}

type ParentMissionCancellation struct {
	MissionID           string
	TeamInstanceID      string
	ExpectedViewVersion string
	ExecutionDigest     string
	LogicalNodeID       string
	AttemptNumber       int
	ClaimGeneration     int64
	CorrelationID       string
}

type ParentMissionCancellationPort interface {
	CancelRecoveredOrActiveMissionExecution(
		context.Context,
		ParentMissionCancellation,
	) (MissionExecutionResult, error)
}

type ParentMissionExecutionDigestPort interface {
	ValidateParentExecutionDigest(context.Context, string, string) error
}

type ProjectionParentContinuationGate struct{ projection *projection.Projection }

func NewProjectionParentContinuationGate(source *projection.Projection) (*ProjectionParentContinuationGate, error) {
	if source == nil {
		return nil, ErrInvalidSideTaskProduct
	}
	return &ProjectionParentContinuationGate{projection: source}, nil
}

func (gate *ProjectionParentContinuationGate) AllowParentContinuation(ctx context.Context, teamID string, generation int64) error {
	if gate == nil || gate.projection == nil || ctx == nil || teamID == "" || generation < 0 {
		return ErrInvalidSideTaskProduct
	}
	records, more := gate.projection.GlobalReadView().SideTaskHandoffs("", maxSideTaskProductInFlight)
	if more {
		return ErrSideTaskProductUnavailable
	}
	for _, record := range records {
		if record.SideExecutionTeamInstanceID == teamID &&
			record.HandoffVersion == 0 && record.Status == "admitted" {
			return ErrSideTaskProductConflict
		}
		if record.ContinuationExecutionTeamInstanceID == teamID &&
			record.EffectStatus == "pending" &&
			(record.Decision == "absorb" || record.Decision == "continue") {
			return ErrSideTaskProductConflict
		}
		if record.ParentTeamInstanceID == teamID && record.Status == "decision_required" && record.DecisionID == "" &&
			(generation == 0 || record.ParentClaimGeneration == generation) {
			return ErrSideTaskProductConflict
		}
	}
	return nil
}

type LocalProductHandoffService struct {
	authority       *work.Authority
	projection      *projection.Projection
	artifacts       SideTaskArtifactStore
	compiler        *BuiltInSideTaskExecutionCompiler
	runner          MissionExecutionRunner
	parentCanceller ParentMissionCancellationPort
	parentExecution ParentMissionExecutionDigestPort
	now             func() time.Time
	mu              sync.Mutex
	inFlight        map[string]struct{}
}

func NewLocalProductHandoffService(
	config LocalProductHandoffConfig,
) (*LocalProductHandoffService, error) {
	if config.Authority == nil || config.Projection == nil ||
		nilMissionExecutionInterface(config.Artifacts) || config.Compiler == nil ||
		nilMissionExecutionInterface(config.Runner) ||
		nilMissionExecutionInterface(config.ParentCanceller) ||
		nilMissionExecutionInterface(config.ParentExecution) || config.Now == nil {
		return nil, ErrInvalidSideTaskProduct
	}
	now := config.Now()
	if now.IsZero() || now.Location() != time.UTC {
		return nil, ErrInvalidSideTaskProduct
	}
	return &LocalProductHandoffService{
		authority: config.Authority, projection: config.Projection,
		artifacts: config.Artifacts, compiler: config.Compiler,
		runner: config.Runner, parentCanceller: config.ParentCanceller,
		parentExecution: config.ParentExecution,
		now:             config.Now, inFlight: make(map[string]struct{}),
	}, nil
}

func (service *LocalProductHandoffService) ProposeSideTask(
	ctx context.Context,
	request SideTaskProposalRequest,
) (SideTaskProposalResult, error) {
	if service == nil || ctx == nil || !validSideTaskProposalRequest(request, "propose") {
		return SideTaskProposalResult{}, ErrInvalidSideTaskProduct
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskProposalResult{}, ErrSideTaskProductUnavailable
	}
	view := service.projection.GlobalReadView()
	if err := validateParentSideTaskBinding(view, request); err != nil {
		return SideTaskProposalResult{}, err
	}
	if err := service.parentExecution.ValidateParentExecutionDigest(
		ctx, request.ParentTeamInstanceID, request.ParentExecutionDigest,
	); err != nil {
		return SideTaskProposalResult{}, mapSideTaskProductError(err)
	}
	digest, err := sideTaskProposalDigest(request)
	if err != nil {
		return SideTaskProposalResult{}, err
	}
	return SideTaskProposalResult{
		SchemaVersion: 1, Status: "proposal", ProposalDigest: digest,
		ViewVersion: view.Version(), Purpose: request.Purpose, Mode: request.Mode,
		Title: request.Title, PermissionScopes: append([]string{}, request.PermissionScopes...),
		DecisionTimeoutSeconds: request.DecisionTimeoutSeconds,
		RequiresConfirmation:   true, PolicyAvailable: false,
	}, nil
}

func (service *LocalProductHandoffService) CreateSideTask(
	ctx context.Context,
	request SideTaskCreateRequest,
) (SideTaskCreateResult, error) {
	if service == nil || ctx == nil || !validSideTaskCreateRequest(request) {
		return SideTaskCreateResult{}, ErrInvalidSideTaskProduct
	}
	if request.PolicyStreamID != "" || request.PolicyVersion != 0 || request.PolicyDigest != "" {
		return SideTaskCreateResult{}, ErrSideTaskProductCapabilityGap
	}
	if !request.Confirmed {
		return SideTaskCreateResult{}, ErrSideTaskProductConflict
	}
	proposalRequest := request.SideTaskProposalRequest
	proposalRequest.Operation = "propose"
	wantProposal, err := sideTaskProposalDigest(proposalRequest)
	if err != nil || wantProposal != request.ProposalDigest {
		return SideTaskCreateResult{}, ErrSideTaskProductDigestMismatch
	}
	sideTaskID := deterministicSideTaskUUID("side-task", request.ParentMissionID, request.ProposalDigest)
	service.mu.Lock()
	if _, exists := service.inFlight[sideTaskID]; exists || len(service.inFlight) >= maxSideTaskProductInFlight {
		service.mu.Unlock()
		return SideTaskCreateResult{}, ErrSideTaskProductConflict
	}
	service.inFlight[sideTaskID] = struct{}{}
	service.mu.Unlock()
	defer func() {
		service.mu.Lock()
		delete(service.inFlight, sideTaskID)
		service.mu.Unlock()
	}()
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskCreateResult{}, ErrSideTaskProductUnavailable
	}
	view := service.projection.GlobalReadView()
	if existing, ok := view.SideTaskHandoff(sideTaskID); ok {
		return sideTaskCreateResult(existing, view.Version()), nil
	}
	if err := validateParentSideTaskBinding(view, proposalRequest); err != nil {
		return SideTaskCreateResult{}, err
	}
	if err := service.parentExecution.ValidateParentExecutionDigest(
		ctx, request.ParentTeamInstanceID, request.ParentExecutionDigest,
	); err != nil {
		return SideTaskCreateResult{}, mapSideTaskProductError(err)
	}
	now := service.now()
	inputArtifact := SideTaskInputArtifact{
		SchemaVersion: 2, SideTaskID: sideTaskID,
		ParentMissionID: request.ParentMissionID, ParentTaskID: request.ParentTaskID,
		ParentRunID: request.ParentRunID, ParentClaimGeneration: request.ParentClaimGeneration,
		ParentExecutionDigest: request.ParentExecutionDigest,
		Purpose:               request.Purpose, Mode: request.Mode, Title: request.Title,
		AuthorizedRequest:      request.AuthorizedRequest,
		PermissionScopes:       append([]string{}, request.PermissionScopes...),
		ProposalDigest:         request.ProposalDigest,
		DecisionTimeoutSeconds: request.DecisionTimeoutSeconds,
		CreatedAt:              now.Format(time.RFC3339Nano),
	}
	inputBytes, inputDigest, err := canonicalSideTaskArtifact(inputArtifact, maxSideTaskInputArtifactBytes)
	if err != nil {
		return SideTaskCreateResult{}, err
	}
	if err := publishAndVerifySideTaskArtifact(ctx, service.artifacts, inputBytes,
		inputDigest, maxSideTaskInputArtifactBytes); err != nil {
		return SideTaskCreateResult{}, err
	}
	childID := deterministicSideTaskUUID("side-execution", sideTaskID,
		request.ProposalDigest, inputDigest)
	record, err := service.authority.AdmitSideTask(ctx, work.SideTaskAdmissionInput{
		SideTaskID: sideTaskID, ParentMissionID: request.ParentMissionID,
		ParentTeamInstanceID: request.ParentTeamInstanceID,
		ParentTaskID:         request.ParentTaskID, ParentRunID: request.ParentRunID,
		ParentClaimGeneration:       request.ParentClaimGeneration,
		ParentExecutionDigest:       request.ParentExecutionDigest,
		SideExecutionTeamInstanceID: childID,
		Purpose:                     request.Purpose, Mode: request.Mode, Title: request.Title,
		ProposalDigest: request.ProposalDigest, InputArtifactDigest: inputDigest,
		ExpectedViewVersion: request.ExpectedViewVersion,
		PermissionScopes:    append([]string{}, request.PermissionScopes...),
		Confirmed:           true, CorrelationID: request.CorrelationID,
	})
	if err != nil {
		return SideTaskCreateResult{}, mapSideTaskProductError(err)
	}
	if err := service.executeAdmittedSideTask(ctx, record, inputArtifact,
		request.CorrelationID); err != nil {
		return sideTaskCreateResult(record, view.Version()), err
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskCreateResult{}, ErrSideTaskProductUnavailable
	}
	view = service.projection.GlobalReadView()
	committed, ok := view.SideTaskHandoff(sideTaskID)
	if !ok {
		return SideTaskCreateResult{}, ErrSideTaskProductUnavailable
	}
	return sideTaskCreateResult(committed, view.Version()), nil
}

func (service *LocalProductHandoffService) ReadSideTask(
	ctx context.Context,
	request SideTaskReadRequest,
) (SideTaskReadResult, error) {
	if service == nil || ctx == nil || request.SchemaVersion != 1 ||
		request.Operation != "read" || request.SideTaskID == "" ||
		!validSHA256(request.ExpectedViewVersion) || request.CorrelationID == "" {
		return SideTaskReadResult{}, ErrInvalidSideTaskProduct
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskReadResult{}, ErrSideTaskProductUnavailable
	}
	view := service.projection.GlobalReadView()
	if view.Version() != request.ExpectedViewVersion {
		return SideTaskReadResult{}, ErrSideTaskProductStaleView
	}
	record, ok := view.SideTaskHandoff(request.SideTaskID)
	if !ok {
		return SideTaskReadResult{}, work.ErrSideTaskNotFound
	}
	return service.sideTaskReadResult(ctx, record, view.Version())
}

func (service *LocalProductHandoffService) ListSideTasks(
	ctx context.Context,
	view projection.GlobalReadView,
	limit int,
) ([]SideTaskReadResult, bool, error) {
	if service == nil || ctx == nil || limit <= 0 || limit > maxSideTaskProductInFlight {
		return nil, false, ErrInvalidSideTaskProduct
	}
	records, hasMore := view.SideTaskHandoffs("", limit)
	results := make([]SideTaskReadResult, 0, len(records))
	for _, record := range records {
		result, err := service.sideTaskReadResult(ctx, record, view.Version())
		if err != nil {
			return nil, false, err
		}
		results = append(results, result)
	}
	return results, hasMore, nil
}

func (service *LocalProductHandoffService) DecideSideTask(
	ctx context.Context,
	request SideTaskDecisionRequest,
) (SideTaskDecisionResult, error) {
	if service == nil || ctx == nil || !validSideTaskDecisionRequest(request) {
		return SideTaskDecisionResult{}, ErrInvalidSideTaskProduct
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskDecisionResult{}, ErrSideTaskProductUnavailable
	}
	view := service.projection.GlobalReadView()
	if view.Version() != request.ExpectedViewVersion {
		return SideTaskDecisionResult{}, ErrSideTaskProductStaleView
	}
	record, ok := view.SideTaskHandoff(request.SideTaskID)
	if !ok || record.ParentMissionID != request.ParentMissionID ||
		record.ParentTeamInstanceID != request.ParentTeamInstanceID ||
		record.ParentTaskID != request.ParentTaskID ||
		record.ParentRunID != request.ParentRunID || record.DecisionID != "" {
		return SideTaskDecisionResult{}, ErrSideTaskProductConflict
	}
	if record.ParentClaimGeneration != request.ParentClaimGeneration ||
		record.SourceGeneration != request.SideTaskGeneration {
		return SideTaskDecisionResult{}, ErrSideTaskProductStaleGeneration
	}
	if record.HandoffVersion != request.HandoffVersion || record.HandoffDigest != request.HandoffDigest {
		return SideTaskDecisionResult{}, ErrSideTaskProductDigestMismatch
	}
	wantEffect := canonicalSideTaskDigest(
		"parent-effect-v1", request.SideTaskID, request.ParentTeamInstanceID,
		request.ParentTaskID, request.ParentRunID,
		fmt.Sprint(request.ParentClaimGeneration), request.ParentExecutionDigest,
		request.HandoffDigest, request.Decision,
	)
	if request.EffectDigest != wantEffect {
		return SideTaskDecisionResult{}, ErrSideTaskProductDigestMismatch
	}
	summaryBytes, err := service.artifacts.ReadArtifact(
		ctx, record.SummaryArtifactDigest, maxSideTaskSummaryArtifactBytes,
	)
	if err != nil {
		return SideTaskDecisionResult{}, ErrSideTaskProductUnavailable
	}
	var summary SideTaskSummaryArtifact
	if decodeCanonicalSideTaskArtifact(summaryBytes, &summary) != nil ||
		!validBoundSideTaskSummary(record, summary) {
		return SideTaskDecisionResult{}, ErrSideTaskProductUnavailable
	}
	decisionID := deterministicSideTaskUUID(
		"side-decision", request.SideTaskID, request.HandoffDigest,
		request.Decision,
	)
	authorityInput := work.SideTaskDecisionInput{
		DecisionID: decisionID, SideTaskID: request.SideTaskID,
		ParentMissionID:      request.ParentMissionID,
		ParentTeamInstanceID: request.ParentTeamInstanceID,
		ParentTaskID:         request.ParentTaskID, ParentRunID: request.ParentRunID,
		ParentLogicalNodeID:   request.ParentLogicalNodeID,
		ParentAttemptNumber:   request.ParentAttemptNumber,
		ParentClaimGeneration: request.ParentClaimGeneration,
		ParentExecutionDigest: request.ParentExecutionDigest,
		SideTaskGeneration:    request.SideTaskGeneration,
		HandoffVersion:        request.HandoffVersion, HandoffDigest: request.HandoffDigest,
		Decision: request.Decision, EffectDigest: request.EffectDigest,
		ExpectedViewVersion: request.ExpectedViewVersion,
		CorrelationID:       request.CorrelationID,
	}
	if err := service.prepareSideTaskDecisionEffect(
		ctx, record, summary, request, &authorityInput,
	); err != nil {
		return SideTaskDecisionResult{}, err
	}
	decided, err := service.authority.DecideSideTaskHandoff(ctx, authorityInput)
	if err != nil {
		return SideTaskDecisionResult{}, mapSideTaskProductError(err)
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SideTaskDecisionResult{}, ErrSideTaskProductUnavailable
	}
	updatedView := service.projection.GlobalReadView()
	updated, ok := updatedView.SideTaskHandoff(request.SideTaskID)
	if !ok {
		return SideTaskDecisionResult{}, ErrSideTaskProductUnavailable
	}
	result := SideTaskDecisionResult{
		SchemaVersion: 1, SideTaskID: updated.SideTaskID,
		Decision: request.Decision, Status: updated.Status,
		EffectStatus:                        updated.EffectStatus,
		ContextPacketDigest:                 decided.ContextPacketDigest,
		ContinuationExecutionTeamInstanceID: authorityInput.ContinuationExecutionTeamInstanceID,
		ViewVersion:                         updatedView.Version(),
	}
	if updated.EffectStatus == "pending" {
		if err := service.ReconcileParentHandoffEffects(ctx); err != nil {
			return result, err
		}
		if err := service.projection.Rebuild(ctx); err == nil {
			updatedView = service.projection.GlobalReadView()
			if reconciled, found := updatedView.SideTaskHandoff(request.SideTaskID); found {
				result.Status = reconciled.Status
				result.EffectStatus = reconciled.EffectStatus
				result.ViewVersion = updatedView.Version()
			}
		}
	}
	return result, nil
}

func (service *LocalProductHandoffService) prepareSideTaskDecisionEffect(
	ctx context.Context,
	record projection.SideTaskHandoff,
	summary SideTaskSummaryArtifact,
	request SideTaskDecisionRequest,
	input *work.SideTaskDecisionInput,
) error {
	switch request.Decision {
	case "absorb":
		packetID := deterministicSideTaskUUID(
			"context-packet", request.SideTaskID, request.HandoffDigest,
		)
		packet := SideTaskContextPacket{
			SchemaVersion: 1, ContextPacketID: packetID,
			SideTaskID: record.SideTaskID, ParentTaskID: record.ParentTaskID,
			ParentRunID:           record.ParentRunID,
			ParentClaimGeneration: record.ParentClaimGeneration,
			SideTaskGeneration:    record.SourceGeneration,
			HandoffVersion:        record.HandoffVersion, HandoffDigest: record.HandoffDigest,
			SummaryArtifactDigest: record.SummaryArtifactDigest,
			AuthorizedFindings:    append([]string{}, summary.AuthorizedFindings...),
			Risk:                  summary.Risk,
			Uncertainties:         append([]string{}, summary.Uncertainties...),
			ScopeDelta:            append([]string{}, summary.ScopeDelta...),
			CreatedAt:             service.now().Format(time.RFC3339Nano),
		}
		packetBytes, packetDigest, err := canonicalSideTaskArtifact(
			packet, maxSideTaskContextPacketBytes,
		)
		if err != nil {
			return err
		}
		if err := publishAndVerifySideTaskArtifact(
			ctx, service.artifacts, packetBytes, packetDigest,
			maxSideTaskContextPacketBytes,
		); err != nil {
			return err
		}
		input.ContextPacketID = packetID
		input.ContextPacketVersion = 1
		input.ContextPacketDigest = packetDigest
		objective := boundedSideTaskContinuationObjective(summary.AuthorizedFindings)
		return service.prepareSideTaskContinuation(ctx, record, objective, request, input)
	case "continue":
		objective := "Continue the parent Mission after the reviewed Side-task decision."
		return service.prepareSideTaskContinuation(ctx, record, objective, request, input)
	case "request_followup":
		input.FollowupProposalDigest = canonicalSideTaskDigest(
			"followup-proposal-v1", record.SideTaskID, record.HandoffDigest,
		)
	case "pivot":
		input.ScopeDeltaDigest = canonicalSideTaskDigest(
			"scope-delta-v1", record.SideTaskID, record.HandoffDigest,
			strings.Join(summary.ScopeDelta, "\x00"),
		)
	case "cancel_parent":
		input.CancellationEffectDigest = canonicalSideTaskDigest(
			"parent-cancel-v1", request.ParentExecutionDigest,
			fmt.Sprint(request.ParentClaimGeneration),
		)
	case "discard", "archive":
	default:
		return ErrInvalidSideTaskProduct
	}
	return nil
}

func (service *LocalProductHandoffService) prepareSideTaskContinuation(
	ctx context.Context,
	record projection.SideTaskHandoff,
	objective string,
	request SideTaskDecisionRequest,
	input *work.SideTaskDecisionInput,
) error {
	childID := deterministicSideTaskUUID(
		"parent-continuation", record.ParentTaskID, record.SideTaskID,
		record.HandoffDigest, request.Decision,
	)
	compilation, err := service.compiler.CompileSideTaskExecution(
		ctx,
		SideTaskExecutionCompileInput{
			SideTaskID:           record.SideTaskID,
			ParentTeamInstanceID: record.ParentTeamInstanceID,
			ChildTeamInstanceID:  childID, Objective: objective,
			PermissionScopes:    []string{"read:context_packet"},
			ExpectedViewVersion: service.projection.GlobalReadView().Version(),
			CorrelationID:       request.CorrelationID,
		},
	)
	if err != nil {
		return err
	}
	input.ContinuationExecutionTeamInstanceID = childID
	input.ContinuationPlanDigest = compilation.Plan.Digest()
	return nil
}

func (service *LocalProductHandoffService) ReconcileParentHandoffEffects(
	ctx context.Context,
) error {
	if service == nil || ctx == nil {
		return ErrInvalidSideTaskProduct
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return err
	}
	view := service.projection.GlobalReadView()
	records, hasMore := view.SideTaskHandoffs("", maxSideTaskProductInFlight)
	if hasMore {
		return ErrSideTaskProductUnavailable
	}
	for _, record := range records {
		if record.DecisionID == "" && record.Status == "decision_required" &&
			!record.DecisionDeadline.IsZero() && !service.now().Before(record.DecisionDeadline) {
			if _, err := service.authority.ExpireSideTaskDecision(ctx, work.SideTaskDecisionExpiryInput{
				SideTaskID: record.SideTaskID, HandoffVersion: record.HandoffVersion,
				HandoffDigest: record.HandoffDigest, Outcome: "human_required",
				ExpectedViewVersion: view.Version(), CorrelationID: deterministicSideTaskUUID("timeout-correlation", record.SideTaskID, record.HandoffDigest),
			}); err != nil && !errors.Is(err, work.ErrSideTaskConflict) {
				return mapSideTaskProductError(err)
			}
			if err := service.projection.Rebuild(ctx); err != nil {
				return err
			}
			view = service.projection.GlobalReadView()
			continue
		}
		if record.EffectStatus != "pending" {
			continue
		}
		switch record.Decision {
		case "absorb", "continue":
			summaryBytes, err := service.artifacts.ReadArtifact(
				ctx, record.SummaryArtifactDigest, maxSideTaskSummaryArtifactBytes,
			)
			if err != nil {
				return ErrSideTaskProductUnavailable
			}
			var summary SideTaskSummaryArtifact
			if decodeCanonicalSideTaskArtifact(summaryBytes, &summary) != nil ||
				!validBoundSideTaskSummary(record, summary) {
				return ErrSideTaskProductUnavailable
			}
			objective := "Continue the parent Mission after the reviewed Side-task decision."
			if record.Decision == "absorb" {
				objective = boundedSideTaskContinuationObjective(summary.AuthorizedFindings)
			}
			compilation, err := service.compiler.CompileSideTaskExecution(
				ctx,
				SideTaskExecutionCompileInput{
					SideTaskID:           record.SideTaskID,
					ParentTeamInstanceID: record.ParentTeamInstanceID,
					ChildTeamInstanceID:  record.ContinuationExecutionTeamInstanceID,
					Objective:            objective,
					PermissionScopes:     []string{"read:context_packet"},
					ExpectedViewVersion:  service.projection.GlobalReadView().Version(),
					CorrelationID: deterministicSideTaskUUID(
						"continuation-correlation", record.DecisionID,
					),
				},
			)
			if err != nil || compilation.Plan.Digest() != record.ContinuationPlanDigest {
				return errors.Join(ErrSideTaskProductConflict, err)
			}
			result, err := service.runner.Run(ctx, compilation.Request)
			if err != nil || result.Team().Status() != "succeeded" {
				return errors.Join(ErrSideTaskProductUnavailable, err)
			}
			if err := service.completeSideTaskContinuation(ctx, record, compilation.Plan); err != nil {
				return err
			}
		case "cancel_parent":
			result, err := service.parentCanceller.CancelRecoveredOrActiveMissionExecution(
				ctx,
				ParentMissionCancellation{
					MissionID:           record.ParentMissionID,
					TeamInstanceID:      record.ParentTeamInstanceID,
					ExpectedViewVersion: view.Version(),
					ExecutionDigest:     record.ParentExecutionDigest,
					LogicalNodeID:       record.ParentLogicalNodeID,
					AttemptNumber:       record.ParentAttemptNumber,
					ClaimGeneration:     record.ParentClaimGeneration,
					CorrelationID: deterministicSideTaskUUID(
						"cancel-correlation", record.DecisionID,
					),
				},
			)
			if err != nil || result.Status != "cancelled" {
				return errors.Join(ErrSideTaskProductUnavailable, err)
			}
			if err := service.completeSideTaskCancellation(ctx, record); err != nil {
				return err
			}
		}
		if err := service.projection.Rebuild(ctx); err != nil {
			return err
		}
		view = service.projection.GlobalReadView()
	}
	return nil
}

func (service *LocalProductHandoffService) ReconcileSideTasks(ctx context.Context) error {
	if service == nil || ctx == nil {
		return ErrInvalidSideTaskProduct
	}
	if err := service.reconcileAdmittedSideTasks(ctx); err != nil {
		return err
	}
	return service.ReconcileParentHandoffEffects(ctx)
}

func (service *LocalProductHandoffService) reconcileAdmittedSideTasks(ctx context.Context) error {
	if err := service.projection.Rebuild(ctx); err != nil {
		return err
	}
	view := service.projection.GlobalReadView()
	records, hasMore := view.SideTaskHandoffs("", maxSideTaskProductInFlight)
	if hasMore {
		return ErrSideTaskProductUnavailable
	}
	for _, projected := range records {
		if projected.Status != "admitted" || projected.HandoffVersion != 0 {
			continue
		}
		artifactBytes, err := service.artifacts.ReadArtifact(
			ctx, projected.InputArtifactDigest, maxSideTaskInputArtifactBytes,
		)
		if err != nil {
			return errors.Join(ErrSideTaskProductUnavailable, err)
		}
		var input SideTaskInputArtifact
		if decodeCanonicalSideTaskArtifact(artifactBytes, &input) != nil ||
			!validRecoveredSideTaskInput(projected, input) {
			return ErrSideTaskProductUnavailable
		}
		record := work.SideTaskHandoffRecord{
			SideTaskID:                  projected.SideTaskID,
			ParentMissionID:             projected.ParentMissionID,
			ParentTeamInstanceID:        projected.ParentTeamInstanceID,
			ParentTaskID:                projected.ParentTaskID,
			ParentRunID:                 projected.ParentRunID,
			ParentClaimGeneration:       projected.ParentClaimGeneration,
			ParentExecutionDigest:       projected.ParentExecutionDigest,
			SideExecutionTeamInstanceID: projected.SideExecutionTeamInstanceID,
			Purpose:                     projected.Purpose, Mode: projected.Mode, Title: projected.Title,
			ProposalDigest:      projected.ProposalDigest,
			InputArtifactDigest: projected.InputArtifactDigest,
			PermissionScopes:    append([]string{}, projected.PermissionScopes...),
			Status:              projected.Status, LastEventID: projected.LastEventID,
			StreamSequence: projected.StreamSequence,
		}
		if err := service.executeAdmittedSideTask(
			ctx, record, input,
			deterministicSideTaskUUID("restart-correlation", projected.SideTaskID, projected.InputArtifactDigest),
		); err != nil {
			return err
		}
		if err := service.projection.Rebuild(ctx); err != nil {
			return err
		}
		view = service.projection.GlobalReadView()
	}
	return nil
}

func (service *LocalProductHandoffService) completeSideTaskCancellation(ctx context.Context, record projection.SideTaskHandoff) error {
	if err := service.projection.Rebuild(ctx); err != nil {
		return err
	}
	view := service.projection.GlobalReadView()
	team, ok := view.TeamExecution(record.ParentTeamInstanceID)
	if !ok || team.Status != "cancelled" {
		return ErrSideTaskProductUnavailable
	}
	var workItemID string
	for _, node := range team.Nodes {
		if node.LogicalNodeID != record.ParentLogicalNodeID {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber == record.ParentAttemptNumber && attempt.ClaimGeneration == record.ParentClaimGeneration {
				workItemID = attempt.WorkItemID
			}
		}
	}
	workItem, ok := view.WorkItem(workItemID)
	if !ok || workItem.SourceEvidenceID == "" || !validSHA256(workItem.SourceEvidenceDigest) {
		return ErrSideTaskProductUnavailable
	}
	_, err := service.authority.CompleteParentHandoffEffect(ctx, work.ParentHandoffEffectCompletionInput{
		DecisionID: record.DecisionID, SideTaskID: record.SideTaskID, EffectKind: "cancellation", EffectDigest: record.EffectDigest,
		ExecutionTeamInstanceID: record.ParentTeamInstanceID, ExecutionPlanDigest: team.PlanDigest, TerminalStatus: "cancelled",
		TerminalEvidenceID: workItem.SourceEvidenceID, TerminalEvidenceDigest: workItem.SourceEvidenceDigest,
		ExpectedViewVersion: view.Version(), CorrelationID: deterministicSideTaskUUID("effect-complete-correlation", record.DecisionID),
	})
	return mapSideTaskProductError(err)
}

func (service *LocalProductHandoffService) completeSideTaskContinuation(
	ctx context.Context,
	record projection.SideTaskHandoff,
	plan teams.ExecutionPlan,
) error {
	if err := service.projection.Rebuild(ctx); err != nil {
		return err
	}
	view := service.projection.GlobalReadView()
	team, ok := view.TeamExecution(plan.TeamInstanceID())
	if !ok || team.Status != "succeeded" || len(team.Nodes) != 1 ||
		len(team.Nodes[0].Attempts) == 0 {
		return ErrSideTaskProductUnavailable
	}
	attempt := team.Nodes[0].Attempts[len(team.Nodes[0].Attempts)-1]
	workItem, ok := view.WorkItem(attempt.WorkItemID)
	if !ok || workItem.SourceEvidenceID == "" ||
		!validSHA256(workItem.SourceEvidenceDigest) {
		return ErrSideTaskProductUnavailable
	}
	_, err := service.authority.CompleteParentHandoffEffect(
		ctx,
		work.ParentHandoffEffectCompletionInput{
			DecisionID: record.DecisionID, SideTaskID: record.SideTaskID,
			EffectKind: "continuation", EffectDigest: record.EffectDigest,
			ExecutionTeamInstanceID: plan.TeamInstanceID(),
			ExecutionPlanDigest:     plan.Digest(), TerminalStatus: "succeeded",
			TerminalEvidenceID:     workItem.SourceEvidenceID,
			TerminalEvidenceDigest: workItem.SourceEvidenceDigest,
			ExpectedViewVersion:    view.Version(),
			CorrelationID: deterministicSideTaskUUID(
				"effect-complete-correlation", record.DecisionID,
			),
		},
	)
	return mapSideTaskProductError(err)
}

func (service *LocalProductHandoffService) executeAdmittedSideTask(
	ctx context.Context,
	record work.SideTaskHandoffRecord,
	input SideTaskInputArtifact,
	correlationID string,
) error {
	compilation, err := service.compiler.CompileSideTaskExecution(ctx,
		SideTaskExecutionCompileInput{
			SideTaskID:           record.SideTaskID,
			ParentTeamInstanceID: record.ParentTeamInstanceID,
			ChildTeamInstanceID:  record.SideExecutionTeamInstanceID,
			Objective:            input.AuthorizedRequest,
			PermissionScopes:     append([]string{}, input.PermissionScopes...),
			ExpectedViewVersion:  service.projection.GlobalReadView().Version(),
			CorrelationID:        correlationID,
		})
	if err != nil {
		return err
	}
	result, err := service.runner.Run(ctx, compilation.Request)
	if err != nil || result.Team().Status() != "succeeded" {
		return errors.Join(ErrSideTaskProductUnavailable, err)
	}
	whatHappened, err := compilation.Collector.AuthorizedResult()
	if err != nil {
		return err
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return err
	}
	view := service.projection.GlobalReadView()
	team, ok := view.TeamExecution(record.SideExecutionTeamInstanceID)
	if !ok || team.Status != "succeeded" || len(team.Nodes) != 1 || len(team.Nodes[0].Attempts) == 0 {
		return ErrSideTaskProductUnavailable
	}
	attempt := team.Nodes[0].Attempts[len(team.Nodes[0].Attempts)-1]
	workItem, ok := view.WorkItem(attempt.WorkItemID)
	if !ok || workItem.SourceEvidenceID == "" || workItem.SourceEvidenceDigest == "" ||
		workItem.AcceptanceDecisionKind != "accepted" {
		return ErrSideTaskProductUnavailable
	}
	evidenceReferences := []SideTaskEvidenceReference{{
		EvidenceID: workItem.SourceEvidenceID, Digest: workItem.SourceEvidenceDigest, Kind: "source",
	}}
	if workItem.VerifierEvidenceID != "" {
		evidenceReferences = append(evidenceReferences, SideTaskEvidenceReference{
			EvidenceID: workItem.VerifierEvidenceID,
			Digest:     workItem.VerifierEvidenceDigest, Kind: "verifier",
		})
	}
	if record.Mode == "merge_candidate" && workItem.VerifierEvidenceID == "" {
		return ErrSideTaskProductUnavailable
	}
	status := "report_delivered"
	if record.Mode == "decision_required" {
		status = "decision_required"
	} else if record.Mode == "merge_candidate" {
		status = "merge_candidate_ready"
	}
	summary := SideTaskSummaryArtifact{
		SchemaVersion: 1, SideTaskID: record.SideTaskID,
		ParentTaskID: record.ParentTaskID, Purpose: record.Purpose, Status: status,
		SourceGeneration: attempt.ClaimGeneration, SummaryVersion: 1,
		WhatHappened: whatHappened, AuthorizedFindings: []string{whatHappened},
		EvidenceReferences: evidenceReferences,
		ArtifactReferences: []SideTaskArtifactReference{{Digest: record.InputArtifactDigest, Kind: "input"}},
		Risk:               "medium", Uncertainties: []string{}, ScopeDelta: []string{},
		DecisionOptions:   sideTaskDecisionOptions(record.Mode),
		RecommendedOption: "", RecommendationAuthority: "proposal_only",
		UsageObserved: false, UsageMicrounits: 0, UsageCurrency: "",
		CreatedAt: service.now().Format(time.RFC3339Nano),
	}
	summaryBytes, summaryDigest, err := canonicalSideTaskArtifact(summary, maxSideTaskSummaryArtifactBytes)
	if err != nil {
		return err
	}
	if err := publishAndVerifySideTaskArtifact(ctx, service.artifacts, summaryBytes,
		summaryDigest, maxSideTaskSummaryArtifactBytes); err != nil {
		return err
	}
	handoffDigest := canonicalSideTaskDigest("handoff-v1", record.SideTaskID,
		record.ParentTaskID, fmt.Sprint(attempt.ClaimGeneration), summaryDigest,
		workItem.SourceEvidenceDigest, workItem.VerifierEvidenceDigest)
	_, err = service.authority.CommitSideTaskHandoff(ctx,
		work.SideTaskHandoffCommitInput{
			SideTaskID:                  record.SideTaskID,
			SideExecutionTeamInstanceID: record.SideExecutionTeamInstanceID,
			SourceWorkItemID:            attempt.WorkItemID, SourceRunID: attempt.RunID,
			SourceGeneration:       attempt.ClaimGeneration,
			SourceEvidenceID:       workItem.SourceEvidenceID,
			SourceEvidenceDigest:   workItem.SourceEvidenceDigest,
			VerifierEvidenceID:     workItem.VerifierEvidenceID,
			VerifierEvidenceDigest: workItem.VerifierEvidenceDigest,
			HandoffVersion:         1, HandoffDigest: handoffDigest,
			SummaryArtifactDigest: summaryDigest,
			DecisionTimeout:       time.Duration(input.DecisionTimeoutSeconds) * time.Second,
			ExpectedViewVersion:   view.Version(), CorrelationID: correlationID,
		})
	return mapSideTaskProductError(err)
}

func validRecoveredSideTaskInput(record projection.SideTaskHandoff, input SideTaskInputArtifact) bool {
	if !validSideTaskInputArtifact(input) || input.SideTaskID != record.SideTaskID ||
		input.ParentMissionID != record.ParentMissionID ||
		input.ParentTaskID != record.ParentTaskID || input.ParentRunID != record.ParentRunID ||
		input.ParentClaimGeneration != record.ParentClaimGeneration ||
		input.ParentExecutionDigest != record.ParentExecutionDigest ||
		input.Purpose != record.Purpose || input.Mode != record.Mode || input.Title != record.Title ||
		input.ProposalDigest != record.ProposalDigest || input.AuthorizedRequest == "" ||
		!validSHA256(input.ProposalDigest) ||
		!equalSideTaskStrings(input.PermissionScopes, record.PermissionScopes) {
		return false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, input.CreatedAt)
	if err != nil || createdAt.Location() != time.UTC {
		return false
	}
	if input.Mode == "report_only" {
		return input.DecisionTimeoutSeconds == 0
	}
	return input.DecisionTimeoutSeconds >= 60 && input.DecisionTimeoutSeconds <= 2592000
}

func equalSideTaskStrings(left, right []string) bool {
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

func (service *LocalProductHandoffService) sideTaskReadResult(
	ctx context.Context,
	record projection.SideTaskHandoff,
	viewVersion string,
) (SideTaskReadResult, error) {
	summary := SideTaskSummaryArtifact{
		AuthorizedFindings: []string{}, EvidenceReferences: []SideTaskEvidenceReference{},
		ArtifactReferences: []SideTaskArtifactReference{}, Uncertainties: []string{},
		ScopeDelta: []string{}, DecisionOptions: []string{},
	}
	if record.SummaryArtifactDigest != "" {
		bytes, err := service.artifacts.ReadArtifact(ctx, record.SummaryArtifactDigest,
			maxSideTaskSummaryArtifactBytes)
		if err != nil || decodeCanonicalSideTaskArtifact(bytes, &summary) != nil ||
			!validBoundSideTaskSummary(record, summary) {
			return SideTaskReadResult{}, ErrSideTaskProductUnavailable
		}
	}
	return SideTaskReadResult{
		SchemaVersion: 1, SideTaskID: record.SideTaskID,
		ParentMissionID:      record.ParentMissionID,
		ParentTeamInstanceID: record.ParentTeamInstanceID,
		ParentTaskID:         record.ParentTaskID, ParentRunID: record.ParentRunID,
		ParentClaimGeneration:       record.ParentClaimGeneration,
		ParentExecutionDigest:       record.ParentExecutionDigest,
		SideExecutionTeamInstanceID: record.SideExecutionTeamInstanceID,
		Purpose:                     record.Purpose, Mode: record.Mode, Title: record.Title,
		Status: record.Status, SourceGeneration: record.SourceGeneration,
		HandoffVersion: record.HandoffVersion, HandoffDigest: record.HandoffDigest,
		SummaryArtifactDigest: record.SummaryArtifactDigest,
		WhatHappened:          summary.WhatHappened,
		AuthorizedFindings:    append([]string{}, summary.AuthorizedFindings...),
		EvidenceReferences:    append([]SideTaskEvidenceReference{}, summary.EvidenceReferences...),
		ArtifactReferences:    append([]SideTaskArtifactReference{}, summary.ArtifactReferences...),
		Risk:                  summary.Risk, Uncertainties: append([]string{}, summary.Uncertainties...),
		ScopeDelta:              append([]string{}, summary.ScopeDelta...),
		DecisionOptions:         append([]string{}, summary.DecisionOptions...),
		RecommendedOption:       summary.RecommendedOption,
		RecommendationAuthority: summary.RecommendationAuthority,
		UsageObserved:           summary.UsageObserved, UsageMicrounits: summary.UsageMicrounits,
		UsageCurrency:      summary.UsageCurrency,
		DecisionDeadline:   formatSideTaskUTC(record.DecisionDeadline),
		AvailableDecisions: availableSideTaskDecisions(record),
		EffectStatus:       record.EffectStatus, ViewVersion: viewVersion,
	}, nil
}

func validateParentSideTaskBinding(view projection.GlobalReadView,
	request SideTaskProposalRequest) error {
	if view.Version() != request.ExpectedViewVersion {
		return ErrSideTaskProductStaleView
	}
	team, ok := view.Team(request.ParentTeamInstanceID)
	if !ok || team.ID != request.ParentTeamInstanceID ||
		request.ParentMissionID != "mission/"+team.ID {
		return ErrSideTaskProductConflict
	}
	workItem, ok := view.WorkItem(request.ParentTaskID)
	if !ok || workItem.RunID != request.ParentRunID ||
		workItem.TeamInstanceID != request.ParentTeamInstanceID {
		return ErrSideTaskProductConflict
	}
	run, ok := view.Run(request.ParentRunID)
	if !ok || run.WorkItemID != request.ParentTaskID {
		return ErrSideTaskProductConflict
	}
	if run.ClaimGeneration != request.ParentClaimGeneration {
		return ErrSideTaskProductStaleGeneration
	}
	if request.Mode == "decision_required" {
		teamExecution, ok := view.TeamExecution(request.ParentTeamInstanceID)
		if !ok || (teamExecution.Status != "succeeded" &&
			teamExecution.Status != "failed" && teamExecution.Status != "cancelled") ||
			run.Phase != "terminal" {
			return ErrSideTaskProductConflict
		}
	}
	return nil
}

func validSideTaskProposalRequest(request SideTaskProposalRequest, operation string) bool {
	if request.SchemaVersion != 1 || request.Operation != operation ||
		request.ParentMissionID == "" || request.ParentTeamInstanceID == "" ||
		request.ParentTaskID == "" || request.ParentRunID == "" ||
		request.ParentClaimGeneration < 0 || !validSHA256(request.ParentExecutionDigest) ||
		request.Title == "" || len(request.Title) > 128 ||
		request.AuthorizedRequest == "" || len(request.AuthorizedRequest) > 4096 ||
		!validSideTaskProductPurpose(request.Purpose) || !validSideTaskProductMode(request.Mode) ||
		request.PermissionScopes == nil || len(request.PermissionScopes) > 32 ||
		!sort.StringsAreSorted(request.PermissionScopes) ||
		!validSHA256(request.ExpectedViewVersion) || request.CorrelationID == "" {
		return false
	}
	for index, scope := range request.PermissionScopes {
		if scope == "" || index > 0 && request.PermissionScopes[index-1] == scope {
			return false
		}
	}
	if request.Mode == "report_only" {
		return request.DecisionTimeoutSeconds == 0
	}
	return request.DecisionTimeoutSeconds >= 60 && request.DecisionTimeoutSeconds <= 2592000
}

func validSideTaskCreateRequest(request SideTaskCreateRequest) bool {
	request.SideTaskProposalRequest.Operation = "create"
	return validSideTaskProposalRequest(request.SideTaskProposalRequest, "create") &&
		validSHA256(request.ProposalDigest)
}

func validSideTaskDecisionRequest(request SideTaskDecisionRequest) bool {
	if request.SchemaVersion != 1 || request.Operation != "decide" ||
		request.SideTaskID == "" || request.ParentMissionID == "" ||
		request.ParentTeamInstanceID == "" || request.ParentTaskID == "" ||
		request.ParentRunID == "" || request.ParentLogicalNodeID == "" ||
		request.ParentAttemptNumber <= 0 || request.ParentClaimGeneration < 0 ||
		request.SideTaskGeneration < 0 || request.HandoffVersion <= 0 ||
		!validSHA256(request.ParentExecutionDigest) ||
		!validSHA256(request.HandoffDigest) || !validSHA256(request.EffectDigest) ||
		!validSHA256(request.ExpectedViewVersion) || request.CorrelationID == "" {
		return false
	}
	switch request.Decision {
	case "absorb", "continue", "request_followup", "pivot", "discard", "archive", "cancel_parent":
		return true
	default:
		return false
	}
}

func boundedSideTaskContinuationObjective(findings []string) string {
	const prefix = "Continue the parent Mission using only this authorized Side-task context: "
	const maximum = 4096
	joined := strings.Join(findings, "\n")
	if len(joined) > maximum-len(prefix) {
		joined = joined[:maximum-len(prefix)]
		for !utf8.ValidString(joined) {
			joined = joined[:len(joined)-1]
		}
	}
	return prefix + joined
}

func sideTaskProposalDigest(request SideTaskProposalRequest) (string, error) {
	request.Operation = "propose"
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalSideTaskArtifact(value any, maximum int) ([]byte, string, error) {
	if !validSideTaskArtifactValue(value) {
		return nil, "", ErrInvalidSideTaskProduct
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || len(encoded) > maximum {
		return nil, "", ErrInvalidSideTaskProduct
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func decodeCanonicalSideTaskArtifact(data []byte, target any) error {
	if len(data) == 0 || !json.Valid(data) || hasDuplicateSideTaskJSONKeys(data) {
		return ErrInvalidSideTaskProduct
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidSideTaskProduct
	}
	reencoded, err := json.Marshal(target)
	if err != nil || !bytes.Equal(data, reencoded) || !validSideTaskArtifactValue(target) {
		return ErrInvalidSideTaskProduct
	}
	return nil
}

func validSideTaskArtifactValue(value any) bool {
	switch typed := value.(type) {
	case SideTaskInputArtifact:
		return validSideTaskInputArtifact(typed)
	case *SideTaskInputArtifact:
		return typed != nil && validSideTaskInputArtifact(*typed)
	case SideTaskSummaryArtifact:
		return validSideTaskSummaryArtifact(typed)
	case *SideTaskSummaryArtifact:
		return typed != nil && validSideTaskSummaryArtifact(*typed)
	case SideTaskContextPacket:
		return validSideTaskContextPacket(typed)
	case *SideTaskContextPacket:
		return typed != nil && validSideTaskContextPacket(*typed)
	default:
		return false
	}
}

func validSideTaskInputArtifact(input SideTaskInputArtifact) bool {
	return input.SchemaVersion == 2 && validSideTaskID(input.SideTaskID) &&
		validSideTaskID(input.ParentMissionID) && validSideTaskID(input.ParentTaskID) &&
		validSideTaskID(input.ParentRunID) && input.ParentClaimGeneration >= 0 &&
		validSHA256(input.ParentExecutionDigest) && validSideTaskProductPurpose(input.Purpose) &&
		validSideTaskProductMode(input.Mode) && validSideTaskText(input.Title, 128, false) &&
		validSideTaskText(input.AuthorizedRequest, 4096, false) &&
		validSideTaskStringList(input.PermissionScopes, 32, 512) &&
		validSHA256(input.ProposalDigest) && validSideTaskTimeout(input.Mode, input.DecisionTimeoutSeconds) &&
		validSideTaskUTC(input.CreatedAt)
}

func validSideTaskSummaryArtifact(summary SideTaskSummaryArtifact) bool {
	if summary.SchemaVersion != 1 || !validSideTaskID(summary.SideTaskID) ||
		!validSideTaskID(summary.ParentTaskID) || !validSideTaskProductPurpose(summary.Purpose) ||
		summary.SourceGeneration < 0 || summary.SummaryVersion != 1 ||
		!validSideTaskText(summary.WhatHappened, 4096, false) ||
		!validSideTaskStringList(summary.AuthorizedFindings, 32, 512) ||
		!validSideTaskEvidenceReferences(summary.EvidenceReferences) ||
		!validSideTaskArtifactReferences(summary.ArtifactReferences) ||
		!validSideTaskRisk(summary.Risk) ||
		!validSideTaskStringList(summary.Uncertainties, 16, 512) ||
		!validSideTaskStringList(summary.ScopeDelta, 16, 512) ||
		!validSideTaskStringList(summary.DecisionOptions, 32, 512) ||
		summary.RecommendationAuthority != "proposal_only" ||
		!validSideTaskUsage(summary.UsageObserved, summary.UsageMicrounits, summary.UsageCurrency) ||
		!validSideTaskUTC(summary.CreatedAt) {
		return false
	}
	expectedStatus := "report_delivered"
	if len(summary.DecisionOptions) > 0 {
		expectedStatus = "decision_required"
	}
	if summary.Status == "merge_candidate_ready" {
		expectedStatus = "merge_candidate_ready"
	}
	if summary.Status != expectedStatus ||
		!equalSideTaskStrings(summary.DecisionOptions, sideTaskDecisionOptionsForStatus(summary.Status)) {
		return false
	}
	if summary.RecommendedOption != "" &&
		!containsSideTaskString(summary.DecisionOptions, summary.RecommendedOption) {
		return false
	}
	return true
}

func validBoundSideTaskSummary(
	record projection.SideTaskHandoff,
	summary SideTaskSummaryArtifact,
) bool {
	if summary.SideTaskID != record.SideTaskID ||
		summary.ParentTaskID != record.ParentTaskID ||
		summary.Purpose != record.Purpose ||
		summary.Status != sideTaskSummaryStatus(record.Mode) ||
		summary.SourceGeneration != record.SourceGeneration {
		return false
	}
	wantEvidence := []SideTaskEvidenceReference{{
		EvidenceID: record.SourceEvidenceID,
		Digest:     record.SourceEvidenceDigest,
		Kind:       "source",
	}}
	if record.VerifierEvidenceID != "" {
		wantEvidence = append(wantEvidence, SideTaskEvidenceReference{
			EvidenceID: record.VerifierEvidenceID,
			Digest:     record.VerifierEvidenceDigest,
			Kind:       "verifier",
		})
	}
	wantArtifacts := []SideTaskArtifactReference{{
		Digest: record.InputArtifactDigest,
		Kind:   "input",
	}}
	return equalSideTaskEvidenceReferences(summary.EvidenceReferences, wantEvidence) &&
		equalSideTaskArtifactReferences(summary.ArtifactReferences, wantArtifacts)
}

func sideTaskSummaryStatus(mode string) string {
	switch mode {
	case "report_only":
		return "report_delivered"
	case "decision_required":
		return "decision_required"
	case "merge_candidate":
		return "merge_candidate_ready"
	default:
		return ""
	}
}

func equalSideTaskEvidenceReferences(left, right []SideTaskEvidenceReference) bool {
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

func equalSideTaskArtifactReferences(left, right []SideTaskArtifactReference) bool {
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

func validSideTaskContextPacket(packet SideTaskContextPacket) bool {
	return packet.SchemaVersion == 1 && validSideTaskID(packet.ContextPacketID) &&
		validSideTaskID(packet.SideTaskID) && validSideTaskID(packet.ParentTaskID) &&
		validSideTaskID(packet.ParentRunID) && packet.ParentClaimGeneration >= 0 &&
		packet.SideTaskGeneration >= 0 && packet.HandoffVersion > 0 &&
		validSHA256(packet.HandoffDigest) && validSHA256(packet.SummaryArtifactDigest) &&
		validSideTaskStringList(packet.AuthorizedFindings, 32, 512) &&
		validSideTaskRisk(packet.Risk) &&
		validSideTaskStringList(packet.Uncertainties, 16, 512) &&
		validSideTaskStringList(packet.ScopeDelta, 16, 512) && validSideTaskUTC(packet.CreatedAt)
}

func validSideTaskID(value string) bool { return validSideTaskText(value, 128, false) }

func validSideTaskText(value string, maximum int, allowEmpty bool) bool {
	return utf8.ValidString(value) && len(value) <= maximum && (allowEmpty || value != "")
}

func validSideTaskStringList(values []string, maximumItems, maximumBytes int) bool {
	if values == nil || len(values) > maximumItems || !sort.StringsAreSorted(values) {
		return false
	}
	for index, value := range values {
		if !validSideTaskText(value, maximumBytes, false) ||
			index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func validSideTaskEvidenceReferences(values []SideTaskEvidenceReference) bool {
	if values == nil || len(values) > 32 {
		return false
	}
	previous := ""
	for _, value := range values {
		key := value.Kind + "\x00" + value.EvidenceID + "\x00" + value.Digest
		if !validSideTaskID(value.EvidenceID) || !validSHA256(value.Digest) ||
			(value.Kind != "source" && value.Kind != "verifier") ||
			previous != "" && key <= previous {
			return false
		}
		previous = key
	}
	return true
}

func validSideTaskArtifactReferences(values []SideTaskArtifactReference) bool {
	if values == nil || len(values) > 32 {
		return false
	}
	previous := ""
	for _, value := range values {
		key := value.Kind + "\x00" + value.Digest
		if !validSHA256(value.Digest) ||
			(value.Kind != "input" && value.Kind != "summary" && value.Kind != "context_packet") ||
			previous != "" && key <= previous {
			return false
		}
		previous = key
	}
	return true
}

func validSideTaskRisk(value string) bool {
	return value == "low" || value == "medium" || value == "high"
}

func validSideTaskTimeout(mode string, seconds int64) bool {
	if mode == "report_only" {
		return seconds == 0
	}
	return seconds >= 60 && seconds <= 2592000
}

func validSideTaskUsage(observed bool, microunits int64, currency string) bool {
	if !observed {
		return microunits == 0 && currency == ""
	}
	if microunits < 0 || len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validSideTaskUTC(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && !parsed.IsZero() && parsed.Location() == time.UTC
}

func sideTaskDecisionOptionsForStatus(status string) []string {
	if status == "report_delivered" {
		return []string{}
	}
	return sideTaskDecisionOptions("decision_required")
}

func containsSideTaskString(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func hasDuplicateSideTaskJSONKeys(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return scanDuplicateSideTaskJSONValue(decoder)
}

func scanDuplicateSideTaskJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return true
			}
			key, ok := keyToken.(string)
			if !ok {
				return true
			}
			if _, exists := seen[key]; exists {
				return true
			}
			seen[key] = struct{}{}
			if scanDuplicateSideTaskJSONValue(decoder) {
				return true
			}
		}
	case '[':
		for decoder.More() {
			if scanDuplicateSideTaskJSONValue(decoder) {
				return true
			}
		}
	default:
		return true
	}
	_, err = decoder.Token()
	return err != nil
}

func publishAndVerifySideTaskArtifact(ctx context.Context,
	store SideTaskArtifactStore, data []byte, digest string, maximum int64) error {
	artifact, err := store.Publish(ctx, bytes.NewReader(data), digest)
	if err != nil || artifact.Digest != digest {
		return errors.Join(ErrSideTaskProductUnavailable, err)
	}
	readBack, err := store.ReadArtifact(ctx, digest, maximum)
	if err != nil || !bytes.Equal(readBack, data) {
		return errors.Join(ErrSideTaskProductUnavailable, err)
	}
	return nil
}

func deterministicSideTaskUUID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	hexValue := hex.EncodeToString(sum[:16])
	return hexValue[:8] + "-" + hexValue[8:12] + "-4" + hexValue[13:16] +
		"-8" + hexValue[17:20] + "-" + hexValue[20:32]
}

func canonicalSideTaskDigest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func sideTaskCreateResult(record interface{}, viewVersion string) SideTaskCreateResult {
	switch current := record.(type) {
	case work.SideTaskHandoffRecord:
		return SideTaskCreateResult{1, current.SideTaskID, current.Status, viewVersion,
			current.SideExecutionTeamInstanceID, current.ProposalDigest,
			current.HandoffVersion, current.HandoffDigest}
	case projection.SideTaskHandoff:
		return SideTaskCreateResult{1, current.SideTaskID, current.Status, viewVersion,
			current.SideExecutionTeamInstanceID, current.ProposalDigest,
			current.HandoffVersion, current.HandoffDigest}
	default:
		return SideTaskCreateResult{}
	}
}

func sideTaskDecisionOptions(mode string) []string {
	if mode == "report_only" {
		return []string{}
	}
	return []string{"absorb", "archive", "cancel_parent", "continue", "discard", "pivot", "request_followup"}
}

func availableSideTaskDecisions(record projection.SideTaskHandoff) []string {
	if record.HandoffVersion == 0 || record.DecisionID != "" ||
		record.Status == "paused" || record.Status == "human_required" ||
		record.Mode == "report_only" {
		return []string{}
	}
	return sideTaskDecisionOptions(record.Mode)
}

func validSideTaskProductPurpose(value string) bool {
	switch value {
	case "research", "comparison", "diagnosis", "verification", "read_only_review":
		return true
	default:
		return false
	}
}

func validSideTaskProductMode(value string) bool {
	return value == "report_only" || value == "decision_required" || value == "merge_candidate"
}

func formatSideTaskUTC(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func mapSideTaskProductError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, work.ErrSideTaskCapabilityGap):
		return ErrSideTaskProductCapabilityGap
	case errors.Is(err, work.ErrSideTaskConflict):
		return ErrSideTaskProductConflict
	case errors.Is(err, work.ErrSideTaskDecisionExpired):
		return ErrSideTaskProductHumanRequired
	default:
		return err
	}
}

func buildSideTaskContinuationPlan(childID, title, agentID, runtimeID string) (
	teams.ExecutionPlan,
	error,
) {
	return teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: childID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: title, AgentInstanceID: agentID,
			RuntimeInstanceID: runtimeID, Role: teams.ExecutionRoleMain,
			DependsOn: []string{}, MaxAttempts: 2,
		}},
	})
}
