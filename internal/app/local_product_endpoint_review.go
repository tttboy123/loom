package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/endpointapproval"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/providerendpoint"
)

const endpointReviewTTL = 15 * time.Minute

const (
	endpointReviewApprovedEvent   = "ProviderEndpointReviewApproved"
	endpointReviewRequestedEvent  = "ProviderEndpointReviewRequested"
	endpointReviewSupersededEvent = "ProviderEndpointReviewSuperseded"
)

var ErrEndpointReviewUnavailable = errors.New("endpoint review unavailable")

type EndpointReviewCommand struct {
	CandidateID         string `json:"candidate_id"`
	CandidateDigest     string `json:"candidate_digest"`
	ProviderID          string `json:"provider_id"`
	ProviderAccountID   string `json:"provider_account_id"`
	EndpointFingerprint string `json:"endpoint_fingerprint"`
	ReviewPolicyVersion uint64 `json:"review_policy_version"`
	ReviewPolicyDigest  string `json:"review_policy_digest"`
	Confirm             bool   `json:"confirm"`
}

// EndpointReviewResult contains only non-secret authority metadata.
type EndpointReviewResult struct {
	CandidateID              string `json:"candidate_id"`
	CandidateDigest          string `json:"candidate_digest"`
	AuthorityCandidateDigest string `json:"authority_candidate_digest"`
	ProviderID               string `json:"provider_id"`
	ProviderAccountID        string `json:"provider_account_id"`
	Protocol                 string `json:"protocol"`
	Endpoint                 string `json:"endpoint"`
	EndpointFingerprint      string `json:"endpoint_fingerprint"`
	ModelDigest              string `json:"model_digest"`
	ReviewPolicyVersion      uint64 `json:"review_policy_version"`
	ReviewPolicyDigest       string `json:"review_policy_digest"`
	Status                   string `json:"status"`
	Revision                 uint64 `json:"revision"`
	ApprovalDigest           string `json:"approval_digest"`
	ApprovedAt               string `json:"approved_at"`
	ExpiresAt                string `json:"expires_at"`
}

func (service *LocalProductSetupService) ApproveEndpointCandidate(
	ctx context.Context,
	command EndpointReviewCommand,
) (EndpointReviewResult, error) {
	if service == nil || ctx == nil || !command.Confirm || service.journal == nil ||
		service.credentialImports == nil || !validSetupDigest(command.CandidateID) ||
		!validSetupDigest(command.CandidateDigest) ||
		!validSetupDigest(command.EndpointFingerprint) ||
		!validSetupDigest(command.ReviewPolicyDigest) || command.ReviewPolicyVersion == 0 ||
		!credentials.ValidProviderAccountIdentifier(
			command.ProviderID, command.ProviderAccountID,
		) {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}

	candidate, err := service.boundImportCandidate(ctx, command.CandidateID)
	if err != nil || candidate.ImportMode != credentials.ImportModeCustomEndpointReview ||
		candidate.TargetProviderID != command.ProviderID ||
		candidate.CandidateDigest != command.CandidateDigest ||
		candidate.EndpointFingerprint != command.EndpointFingerprint ||
		candidate.ReviewPolicyVersion != command.ReviewPolicyVersion ||
		candidate.ReviewPolicyDigest != command.ReviewPolicyDigest {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}

	policy, err := providerendpoint.NewEndpointPolicy(candidate.Endpoint)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	resolver := service.endpointResolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if _, err := policy.Resolve(ctx, resolver); err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	modelDigest, err := credentials.ImportCandidateModelDigest(candidate)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	authorityCandidate, err := endpointapproval.NewReviewCandidate(
		endpointapproval.ReviewCandidateInput{
			EndpointFingerprint: candidate.EndpointFingerprint,
			ProviderID:          candidate.TargetProviderID,
			AccountID:           command.ProviderAccountID,
			Protocol:            candidate.Protocol,
			ModelDigest:         modelDigest,
			ReviewPolicyVersion: candidate.ReviewPolicyVersion,
			ReviewPolicyDigest:  candidate.ReviewPolicyDigest,
		},
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	now := service.now().UTC()
	baseStreamID := endpointReviewStreamID(
		candidate.CandidateDigest, command.ProviderAccountID,
	)
	lineage, err := service.readEndpointReviewLineage(ctx, baseStreamID)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	generation := uint64(1)
	if len(lineage) > 0 {
		latest := lineage[len(lineage)-1]
		if !endpointReviewMatches(
			latest.result, candidate, command.ProviderAccountID,
			authorityCandidate.Digest(),
		) {
			return EndpointReviewResult{}, ErrEndpointReviewUnavailable
		}
		if !latest.superseded && now.Before(parseEndpointReviewTime(latest.result.ExpiresAt)) {
			active, activeErr := service.activeEndpointReviewForAccount(
				ctx, candidate.TargetProviderID, command.ProviderAccountID, now,
			)
			if activeErr != nil || active.result.ApprovalDigest != latest.result.ApprovalDigest {
				return EndpointReviewResult{}, ErrEndpointReviewUnavailable
			}
			return latest.result, nil
		}
		if latest.generation == ^uint64(0) {
			return EndpointReviewResult{}, ErrEndpointReviewUnavailable
		}
		generation = latest.generation + 1
	}
	streamID := endpointReviewGenerationStreamID(
		candidate.CandidateDigest, command.ProviderAccountID, generation,
	)
	superseded, err := service.activeEndpointReviewForAccount(
		ctx, candidate.TargetProviderID, command.ProviderAccountID, now,
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	expiresAt := now.Add(endpointReviewTTL)
	requestOperationID, err := service.identity.NextSetupID("endpoint-review-request")
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	requestCommand, err := endpointapproval.NewApprovalCommand(
		authorityCandidate,
		endpointapproval.ApprovalCommandInput{
			Kind: endpointapproval.CommandRequest, Actor: "local-user",
			OperationID: requestOperationID, OccurredAt: now, ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	requested, err := endpointapproval.Apply(authorityCandidate, nil, requestCommand)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	approveOperationID, err := service.identity.NextSetupID("endpoint-review-approve")
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	approveCommand, err := endpointapproval.NewApprovalCommand(
		authorityCandidate,
		endpointapproval.ApprovalCommandInput{
			Kind: endpointapproval.CommandApprove, Actor: "local-user",
			OperationID: approveOperationID, ExpectedRevision: requested.Revision(),
			ExpectedRecordDigest: requested.Digest(), OccurredAt: now,
		},
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	approved, err := endpointapproval.Apply(authorityCandidate, &requested, approveCommand)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	requestedResult := endpointReviewResult(
		candidate, command.ProviderAccountID, authorityCandidate.Digest(), requested,
	)
	approvedResult := endpointReviewResult(
		candidate, command.ProviderAccountID, authorityCandidate.Digest(), approved,
	)
	requestEvent, err := endpointReviewEvent(
		requestOperationID, streamID, 1, endpointReviewRequestedEvent,
		now, requestOperationID, requestedResult,
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	approveEvent, err := endpointReviewEvent(
		approveOperationID, streamID, 2, endpointReviewApprovedEvent,
		now, requestOperationID, approvedResult,
	)
	if err != nil {
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	expectations := []journal.StreamHeadExpectation{{StreamID: streamID, Sequence: 0}}
	events := []journal.Event{requestEvent, approveEvent}
	if superseded.result.ApprovalDigest != "" {
		supersedeEvent, eventErr := endpointReviewEvent(
			approveOperationID, superseded.streamID, superseded.head+1,
			endpointReviewSupersededEvent, now, requestOperationID, superseded.result,
		)
		if eventErr != nil {
			return EndpointReviewResult{}, ErrEndpointReviewUnavailable
		}
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: superseded.streamID, Sequence: superseded.head,
		})
		events = append([]journal.Event{supersedeEvent}, events...)
	}
	if _, err := service.journal.AppendBatchIfStreamHeads(ctx, expectations, events); err != nil {
		active, activeErr := service.activeEndpointReviewForAccount(
			ctx, candidate.TargetProviderID, command.ProviderAccountID, now,
		)
		if activeErr == nil && endpointReviewMatches(
			active.result, candidate, command.ProviderAccountID,
			authorityCandidate.Digest(),
		) {
			return active.result, nil
		}
		return EndpointReviewResult{}, ErrEndpointReviewUnavailable
	}
	return approvedResult, nil
}

func (service *LocalProductSetupService) boundImportCandidate(
	ctx context.Context,
	candidateID string,
) (credentials.ImportCandidate, error) {
	candidates, err := service.credentialImports.Discover(ctx)
	if err != nil {
		return credentials.ImportCandidate{}, ErrCredentialImportUnavailable
	}
	var selected *credentials.ImportCandidate
	for index := range candidates {
		if candidates[index].CandidateID != candidateID {
			continue
		}
		if selected != nil {
			return credentials.ImportCandidate{}, ErrCredentialImportUnavailable
		}
		selected = &candidates[index]
	}
	if selected == nil {
		return credentials.ImportCandidate{}, ErrCredentialImportUnavailable
	}
	bound, err := credentials.BindImportCandidate(*selected)
	if err != nil {
		return credentials.ImportCandidate{}, ErrCredentialImportUnavailable
	}
	return bound, nil
}

func endpointReviewResult(
	candidate credentials.ImportCandidate,
	accountID string,
	authorityCandidateDigest string,
	record endpointapproval.ApprovalRecord,
) EndpointReviewResult {
	return EndpointReviewResult{
		CandidateID: candidate.CandidateID, CandidateDigest: candidate.CandidateDigest,
		AuthorityCandidateDigest: authorityCandidateDigest,
		ProviderID:               candidate.TargetProviderID, ProviderAccountID: accountID,
		Protocol: candidate.Protocol, Endpoint: candidate.Endpoint,
		EndpointFingerprint: candidate.EndpointFingerprint,
		ModelDigest:         record.ModelDigest(), ReviewPolicyVersion: candidate.ReviewPolicyVersion,
		ReviewPolicyDigest: candidate.ReviewPolicyDigest, Status: string(record.Status()),
		Revision: record.Revision(), ApprovalDigest: record.Digest(),
		ApprovedAt: record.DecidedAt().Format(time.RFC3339Nano),
		ExpiresAt:  record.ExpiresAt().Format(time.RFC3339Nano),
	}
}

func endpointReviewStreamID(candidateDigest, accountID string) string {
	sum := sha256.Sum256([]byte(candidateDigest + "\x00" + accountID))
	return "provider-endpoint-review/" + hex.EncodeToString(sum[:])
}

func endpointReviewGenerationStreamID(
	candidateDigest, accountID string,
	generation uint64,
) string {
	base := endpointReviewStreamID(candidateDigest, accountID)
	if generation <= 1 {
		return base
	}
	return base + "/generation/" + strconv.FormatUint(generation, 10)
}

func endpointReviewEvent(
	operationID, streamID string,
	sequence int64,
	eventType string,
	emittedAt time.Time,
	causationID string,
	payload EndpointReviewResult,
) (journal.Event, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, err
	}
	sum := sha256.Sum256([]byte(operationID + "\x00" + streamID + "\x00" + eventType))
	return journal.Event{
		ID: "endpoint-review-" + hex.EncodeToString(sum[:16]), StreamID: streamID,
		Seq: sequence, IdempotencyKey: "endpoint-review/" + hex.EncodeToString(sum[:]),
		Type: eventType, SchemaVersion: 1, EmittedAt: emittedAt,
		CorrelationID: operationID, CausationID: causationID, PayloadJSON: encoded,
	}, nil
}

type endpointReviewGenerationState struct {
	result       EndpointReviewResult
	generation   uint64
	streamID     string
	head         int64
	superseded   bool
	supersededAt time.Time
}

func (service *LocalProductSetupService) readApprovedEndpointReview(
	ctx context.Context,
	streamID string,
) (EndpointReviewResult, bool) {
	lineage, err := service.readEndpointReviewLineage(ctx, streamID)
	if err != nil || len(lineage) == 0 || lineage[len(lineage)-1].superseded {
		return EndpointReviewResult{}, false
	}
	return lineage[len(lineage)-1].result, true
}

func (service *LocalProductSetupService) readEndpointReviewLineage(
	ctx context.Context,
	baseStreamID string,
) ([]endpointReviewGenerationState, error) {
	if service == nil || service.journal == nil || ctx == nil ||
		!validEndpointReviewBaseStreamID(baseStreamID) {
		return nil, ErrEndpointReviewUnavailable
	}
	events, err := service.journal.ReadAll(ctx)
	if err != nil {
		return nil, ErrEndpointReviewUnavailable
	}
	generations := make(map[uint64]string)
	for _, event := range events {
		if event.StreamID != baseStreamID &&
			!strings.HasPrefix(event.StreamID, baseStreamID+"/") {
			continue
		}
		generation, ok := endpointReviewStreamGeneration(baseStreamID, event.StreamID)
		if !ok {
			return nil, ErrEndpointReviewUnavailable
		}
		if existing, found := generations[generation]; found && existing != event.StreamID {
			return nil, ErrEndpointReviewUnavailable
		}
		generations[generation] = event.StreamID
	}
	if len(generations) == 0 {
		return []endpointReviewGenerationState{}, nil
	}
	if _, found := generations[uint64(len(generations))]; !found {
		return nil, ErrEndpointReviewUnavailable
	}
	lineage := make([]endpointReviewGenerationState, 0, len(generations))
	for generation := uint64(1); generation <= uint64(len(generations)); generation++ {
		streamID, found := generations[generation]
		if !found {
			return nil, ErrEndpointReviewUnavailable
		}
		streamEvents, readErr := service.journal.ReadStream(ctx, streamID)
		if readErr != nil {
			return nil, ErrEndpointReviewUnavailable
		}
		state, decodeErr := decodeEndpointReviewGeneration(
			streamID, generation, streamEvents,
		)
		if decodeErr != nil {
			return nil, ErrEndpointReviewUnavailable
		}
		if len(lineage) > 0 {
			previous := lineage[len(lineage)-1]
			if !sameEndpointReviewSubject(previous.result, state.result) {
				return nil, ErrEndpointReviewUnavailable
			}
			closedAt := parseEndpointReviewTime(previous.result.ExpiresAt)
			if previous.superseded {
				closedAt = previous.supersededAt
			}
			if parseEndpointReviewTime(state.result.ApprovedAt).Before(closedAt) {
				return nil, ErrEndpointReviewUnavailable
			}
		}
		lineage = append(lineage, state)
	}
	return lineage, nil
}

func decodeEndpointReviewGeneration(
	streamID string,
	generation uint64,
	events []journal.Event,
) (endpointReviewGenerationState, error) {
	if len(events) < 2 || len(events) > 3 {
		return endpointReviewGenerationState{}, ErrEndpointReviewUnavailable
	}
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 {
			return endpointReviewGenerationState{}, ErrEndpointReviewUnavailable
		}
	}
	if events[0].Type != endpointReviewRequestedEvent ||
		events[1].Type != endpointReviewApprovedEvent {
		return endpointReviewGenerationState{}, ErrEndpointReviewUnavailable
	}
	var requested, approved EndpointReviewResult
	if json.Unmarshal(events[0].PayloadJSON, &requested) != nil ||
		json.Unmarshal(events[1].PayloadJSON, &approved) != nil ||
		!validEndpointReviewResult(requested, string(endpointapproval.StatusRequested), 1) ||
		!validEndpointReviewResult(approved, string(endpointapproval.StatusApproved), 2) ||
		!sameEndpointReviewSubject(requested, approved) ||
		requested.ExpiresAt != approved.ExpiresAt ||
		!parseEndpointReviewTime(requested.ApprovedAt).IsZero() ||
		!parseEndpointReviewTime(approved.ApprovedAt).Equal(events[1].EmittedAt.UTC()) {
		return endpointReviewGenerationState{}, ErrEndpointReviewUnavailable
	}
	state := endpointReviewGenerationState{
		result: approved, generation: generation, streamID: streamID,
		head: int64(len(events)),
	}
	if len(events) == 3 {
		var superseded EndpointReviewResult
		if events[2].Type != endpointReviewSupersededEvent ||
			json.Unmarshal(events[2].PayloadJSON, &superseded) != nil || superseded != approved ||
			events[2].EmittedAt.Before(events[1].EmittedAt) {
			return endpointReviewGenerationState{}, ErrEndpointReviewUnavailable
		}
		state.superseded = true
		state.supersededAt = events[2].EmittedAt.UTC()
	}
	return state, nil
}

func validEndpointReviewResult(
	result EndpointReviewResult,
	status string,
	revision uint64,
) bool {
	approvedAt := parseEndpointReviewTime(result.ApprovedAt)
	expiresAt := parseEndpointReviewTime(result.ExpiresAt)
	return validSetupDigest(result.CandidateID) &&
		validSetupDigest(result.CandidateDigest) &&
		validSetupDigest(result.AuthorityCandidateDigest) &&
		credentials.ValidProviderAccountIdentifier(
			result.ProviderID, result.ProviderAccountID,
		) && result.Protocol != "" && result.Endpoint != "" &&
		validSetupDigest(result.EndpointFingerprint) &&
		validSetupDigest(result.ModelDigest) && result.ReviewPolicyVersion > 0 &&
		validSetupDigest(result.ReviewPolicyDigest) && result.Status == status &&
		result.Revision == revision && validSetupDigest(result.ApprovalDigest) &&
		!expiresAt.IsZero() && (approvedAt.IsZero() || approvedAt.Before(expiresAt))
}

func sameEndpointReviewSubject(left, right EndpointReviewResult) bool {
	return left.CandidateID == right.CandidateID &&
		left.CandidateDigest == right.CandidateDigest &&
		left.AuthorityCandidateDigest == right.AuthorityCandidateDigest &&
		left.ProviderID == right.ProviderID &&
		left.ProviderAccountID == right.ProviderAccountID &&
		left.Protocol == right.Protocol && left.Endpoint == right.Endpoint &&
		left.EndpointFingerprint == right.EndpointFingerprint &&
		left.ModelDigest == right.ModelDigest &&
		left.ReviewPolicyVersion == right.ReviewPolicyVersion &&
		left.ReviewPolicyDigest == right.ReviewPolicyDigest
}

func validEndpointReviewBaseStreamID(streamID string) bool {
	const prefix = "provider-endpoint-review/"
	return strings.HasPrefix(streamID, prefix) &&
		validSetupDigest(strings.TrimPrefix(streamID, prefix))
}

func endpointReviewStreamGeneration(baseStreamID, streamID string) (uint64, bool) {
	if streamID == baseStreamID {
		return 1, true
	}
	const marker = "/generation/"
	if !strings.HasPrefix(streamID, baseStreamID+marker) {
		return 0, false
	}
	encoded := strings.TrimPrefix(streamID, baseStreamID+marker)
	generation, err := strconv.ParseUint(encoded, 10, 64)
	return generation, err == nil && generation > 1 && strconv.FormatUint(generation, 10) == encoded
}

func endpointReviewBaseStreamID(streamID string) (string, bool) {
	const prefix = "provider-endpoint-review/"
	if !strings.HasPrefix(streamID, prefix) || len(streamID) < len(prefix)+64 {
		return "", false
	}
	base := streamID[:len(prefix)+64]
	if !validEndpointReviewBaseStreamID(base) {
		return "", false
	}
	_, ok := endpointReviewStreamGeneration(base, streamID)
	return base, ok
}

func (service *LocalProductSetupService) activeEndpointReviewStates(
	ctx context.Context,
	now time.Time,
) ([]endpointReviewGenerationState, error) {
	events, err := service.journal.ReadAll(ctx)
	if err != nil {
		return nil, ErrEndpointReviewUnavailable
	}
	bases := make([]string, 0)
	seen := make(map[string]struct{})
	for _, event := range events {
		base, ok := endpointReviewBaseStreamID(event.StreamID)
		if !ok {
			continue
		}
		if _, found := seen[base]; found {
			continue
		}
		seen[base] = struct{}{}
		bases = append(bases, base)
	}
	active := make([]endpointReviewGenerationState, 0, len(bases))
	accounts := make(map[string]struct{})
	for _, base := range bases {
		lineage, lineageErr := service.readEndpointReviewLineage(ctx, base)
		if lineageErr != nil || len(lineage) == 0 {
			return nil, ErrEndpointReviewUnavailable
		}
		latest := lineage[len(lineage)-1]
		if latest.superseded || !now.Before(parseEndpointReviewTime(latest.result.ExpiresAt)) {
			continue
		}
		key := latest.result.ProviderID + "\x00" + latest.result.ProviderAccountID
		if _, duplicate := accounts[key]; duplicate {
			return nil, ErrEndpointReviewUnavailable
		}
		accounts[key] = struct{}{}
		active = append(active, latest)
	}
	return active, nil
}

func (service *LocalProductSetupService) activeEndpointReviewForAccount(
	ctx context.Context,
	providerID, accountID string,
	now time.Time,
) (endpointReviewGenerationState, error) {
	active, err := service.activeEndpointReviewStates(ctx, now)
	if err != nil {
		return endpointReviewGenerationState{}, err
	}
	for _, state := range active {
		if state.result.ProviderID == providerID &&
			state.result.ProviderAccountID == accountID {
			return state, nil
		}
	}
	return endpointReviewGenerationState{}, nil
}

func endpointReviewMatches(
	result EndpointReviewResult,
	candidate credentials.ImportCandidate,
	accountID string,
	authorityCandidateDigest string,
) bool {
	modelDigest, err := credentials.ImportCandidateModelDigest(candidate)
	return err == nil && result.CandidateID == candidate.CandidateID &&
		result.CandidateDigest == candidate.CandidateDigest &&
		result.AuthorityCandidateDigest == authorityCandidateDigest &&
		result.ProviderID == candidate.TargetProviderID &&
		result.ProviderAccountID == accountID && result.Protocol == candidate.Protocol &&
		result.Endpoint == candidate.Endpoint &&
		result.EndpointFingerprint == candidate.EndpointFingerprint &&
		result.ModelDigest == modelDigest &&
		result.ReviewPolicyVersion == candidate.ReviewPolicyVersion &&
		result.ReviewPolicyDigest == candidate.ReviewPolicyDigest
}

func parseEndpointReviewTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func (service *LocalProductSetupService) activeEndpointReviews(
	ctx context.Context,
	candidates []credentials.ImportCandidate,
) []EndpointReviewResult {
	if service == nil || service.journal == nil || ctx == nil || len(candidates) == 0 {
		return []EndpointReviewResult{}
	}
	allowed := make(map[string]credentials.ImportCandidate, len(candidates))
	for _, candidate := range candidates {
		if candidate.ImportMode == credentials.ImportModeCustomEndpointReview {
			allowed[candidate.CandidateDigest] = candidate
		}
	}
	active, err := service.activeEndpointReviewStates(ctx, service.now().UTC())
	if err != nil {
		return []EndpointReviewResult{}
	}
	result := make([]EndpointReviewResult, 0)
	for _, state := range active {
		review := state.result
		candidate, found := allowed[review.CandidateDigest]
		if !found || review.Status != "approved" || review.Revision != 2 ||
			!endpointReviewMatches(
				review, candidate, review.ProviderAccountID, review.AuthorityCandidateDigest,
			) {
			continue
		}
		result = append(result, review)
	}
	return result
}
