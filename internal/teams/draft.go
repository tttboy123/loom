package teams

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalidTeamDraft                = errors.New("invalid team draft")
	ErrInvalidDraftQuestion            = errors.New("invalid draft question")
	ErrInvalidDraftAnswer              = errors.New("invalid draft answer")
	ErrInvalidTeamDraftStateTransition = errors.New("invalid team draft state transition")
	ErrStaleTeamDraftRevision          = errors.New("stale team draft revision")
	ErrTeamDraftCatalogMismatch        = errors.New("team draft catalog mismatch")
	ErrTeamDraftQuestionMismatch       = errors.New("team draft question mismatch")
)

type TeamDraftState string

const (
	TeamDraftStateDraft          TeamDraftState = "draft"
	TeamDraftStateAwaitingAnswer TeamDraftState = "awaiting_answer"
	TeamDraftStateProposed       TeamDraftState = "proposed"
)

type DraftQuestion struct {
	ID     string
	Prompt string
}

type TeamDraft struct {
	id                     string
	revision               int
	state                  TeamDraftState
	catalogDigest          string
	references             TeamDraftReferences
	question               DraftQuestion
	hasQuestion            bool
	lastAnsweredQuestionID string
	lastAnswerText         string
}

type TeamDraftAcceptanceCandidate struct {
	Eligible      bool
	DraftID       string
	Revision      int
	CatalogDigest string
}

func NewTeamDraft(id string, catalog TeamDraftCatalogSnapshot, references TeamDraftReferences) (TeamDraft, error) {
	if id == "" {
		return TeamDraft{}, ErrInvalidTeamDraft
	}
	normalized, err := validateAndNormalizeDraftReferences(catalog, references)
	if err != nil {
		return TeamDraft{}, err
	}
	return TeamDraft{
		id:            id,
		revision:      1,
		state:         TeamDraftStateDraft,
		catalogDigest: catalog.Digest(),
		references:    normalized,
	}, nil
}

func PresentTeamDraft(
	current TeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	nextQuestion *DraftQuestion,
) (TeamDraft, error) {
	if err := validateDraftCommand(current, expectedRevision, catalog); err != nil {
		return TeamDraft{}, err
	}
	if current.state != TeamDraftStateDraft {
		return TeamDraft{}, ErrInvalidTeamDraftStateTransition
	}
	question, hasQuestion, err := copyValidDraftQuestion(nextQuestion)
	if err != nil {
		return TeamDraft{}, err
	}
	normalized, err := validateAndNormalizeDraftReferences(catalog, current.references)
	if err != nil {
		return TeamDraft{}, err
	}

	next := nextDraftRevision(current, normalized, question, hasQuestion)
	return next, nil
}

func AnswerTeamDraft(
	current TeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	questionID string,
	answerText string,
	references TeamDraftReferences,
	nextQuestion *DraftQuestion,
) (TeamDraft, error) {
	if err := validateDraftCommand(current, expectedRevision, catalog); err != nil {
		return TeamDraft{}, err
	}
	if current.state != TeamDraftStateAwaitingAnswer || !current.hasQuestion {
		return TeamDraft{}, ErrInvalidTeamDraftStateTransition
	}
	if questionID == "" || questionID != current.question.ID {
		return TeamDraft{}, ErrTeamDraftQuestionMismatch
	}
	if answerText == "" {
		return TeamDraft{}, ErrInvalidDraftAnswer
	}
	question, hasQuestion, err := copyValidDraftQuestion(nextQuestion)
	if err != nil {
		return TeamDraft{}, err
	}
	normalized, err := validateAndNormalizeDraftReferences(catalog, references)
	if err != nil {
		return TeamDraft{}, err
	}

	next := nextDraftRevision(current, normalized, question, hasQuestion)
	next.lastAnsweredQuestionID = questionID
	next.lastAnswerText = answerText
	return next, nil
}

func EditTeamDraft(
	current TeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	references TeamDraftReferences,
	unresolvedQuestion *DraftQuestion,
) (TeamDraft, error) {
	if err := validateDraftCommand(current, expectedRevision, catalog); err != nil {
		return TeamDraft{}, err
	}
	if current.state != TeamDraftStateAwaitingAnswer && current.state != TeamDraftStateProposed {
		return TeamDraft{}, ErrInvalidTeamDraftStateTransition
	}
	question, hasQuestion, err := copyValidDraftQuestion(unresolvedQuestion)
	if err != nil {
		return TeamDraft{}, err
	}
	normalized, err := validateAndNormalizeDraftReferences(catalog, references)
	if err != nil {
		return TeamDraft{}, err
	}

	return nextDraftRevision(current, normalized, question, hasQuestion), nil
}

func CheckTeamDraftAcceptable(
	current TeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
) (TeamDraftAcceptanceCandidate, error) {
	if err := validateDraftCommand(current, expectedRevision, catalog); err != nil {
		return TeamDraftAcceptanceCandidate{}, err
	}
	if current.state != TeamDraftStateProposed || current.hasQuestion {
		return TeamDraftAcceptanceCandidate{}, ErrInvalidTeamDraftStateTransition
	}
	if _, err := validateAndNormalizeDraftReferences(catalog, current.references); err != nil {
		return TeamDraftAcceptanceCandidate{}, err
	}
	return TeamDraftAcceptanceCandidate{
		Eligible:      true,
		DraftID:       current.id,
		Revision:      current.revision,
		CatalogDigest: current.catalogDigest,
	}, nil
}

func (d TeamDraft) ID() string {
	return d.id
}

func (d TeamDraft) Revision() int {
	return d.revision
}

func (d TeamDraft) State() TeamDraftState {
	return d.state
}

func (d TeamDraft) CatalogDigest() string {
	return d.catalogDigest
}

func (d TeamDraft) References() TeamDraftReferences {
	return copyTeamDraftReferences(d.references)
}

func (d TeamDraft) Question() (DraftQuestion, bool) {
	if !d.hasQuestion {
		return DraftQuestion{}, false
	}
	return d.question, true
}

func (d TeamDraft) LastAnsweredQuestionID() string {
	return d.lastAnsweredQuestionID
}

func (d TeamDraft) LastAnswerText() string {
	return d.lastAnswerText
}

func validateDraftCommand(current TeamDraft, expectedRevision int, catalog TeamDraftCatalogSnapshot) error {
	if !current.valid() {
		return ErrInvalidTeamDraft
	}
	if expectedRevision != current.revision {
		return ErrStaleTeamDraftRevision
	}
	if catalog.Digest() == "" {
		return ErrInvalidTeamDraftCatalog
	}
	if catalog.Digest() != current.catalogDigest {
		return ErrTeamDraftCatalogMismatch
	}
	return nil
}

func (d TeamDraft) valid() bool {
	if d.id == "" || d.revision <= 0 || d.catalogDigest == "" {
		return false
	}
	switch d.state {
	case TeamDraftStateDraft, TeamDraftStateAwaitingAnswer, TeamDraftStateProposed:
		return true
	default:
		return false
	}
}

func copyValidDraftQuestion(input *DraftQuestion) (DraftQuestion, bool, error) {
	if input == nil {
		return DraftQuestion{}, false, nil
	}
	if input.ID == "" || input.Prompt == "" {
		return DraftQuestion{}, false, ErrInvalidDraftQuestion
	}
	return DraftQuestion{ID: input.ID, Prompt: input.Prompt}, true, nil
}

func nextDraftRevision(
	current TeamDraft,
	references TeamDraftReferences,
	question DraftQuestion,
	hasQuestion bool,
) TeamDraft {
	state := TeamDraftStateProposed
	if hasQuestion {
		state = TeamDraftStateAwaitingAnswer
	}
	return TeamDraft{
		id:                     current.id,
		revision:               current.revision + 1,
		state:                  state,
		catalogDigest:          current.catalogDigest,
		references:             copyTeamDraftReferences(references),
		question:               question,
		hasQuestion:            hasQuestion,
		lastAnsweredQuestionID: current.lastAnsweredQuestionID,
		lastAnswerText:         current.lastAnswerText,
	}
}

func validateAndNormalizeDraftReferences(
	catalog TeamDraftCatalogSnapshot,
	references TeamDraftReferences,
) (TeamDraftReferences, error) {
	if _, err := ValidateTeamDraftCatalog(catalog, references); err != nil {
		return TeamDraftReferences{}, fmt.Errorf("validate team draft references: %w", err)
	}
	normalized := copyTeamDraftReferences(references)
	sort.Strings(normalized.SubAgentDefinitionIDs)
	sort.Strings(normalized.RuntimeInstanceIDs)
	sort.Slice(normalized.RuntimeModels, func(i, j int) bool {
		if normalized.RuntimeModels[i].RuntimeInstanceID == normalized.RuntimeModels[j].RuntimeInstanceID {
			return normalized.RuntimeModels[i].ModelID < normalized.RuntimeModels[j].ModelID
		}
		return normalized.RuntimeModels[i].RuntimeInstanceID < normalized.RuntimeModels[j].RuntimeInstanceID
	})
	sort.Strings(normalized.SkillIDs)
	sort.Strings(normalized.MemberIDs)
	sort.Strings(normalized.PermissionIDs)
	return normalized, nil
}

func copyTeamDraftReferences(input TeamDraftReferences) TeamDraftReferences {
	return TeamDraftReferences{
		MainAgentDefinitionID: input.MainAgentDefinitionID,
		SubAgentDefinitionIDs: append([]string(nil), input.SubAgentDefinitionIDs...),
		RuntimeInstanceIDs:    append([]string(nil), input.RuntimeInstanceIDs...),
		RuntimeModels:         append([]RuntimeModelReference(nil), input.RuntimeModels...),
		SkillIDs:              append([]string(nil), input.SkillIDs...),
		MemberIDs:             append([]string(nil), input.MemberIDs...),
		PermissionIDs:         append([]string(nil), input.PermissionIDs...),
		RequestedBudget:       input.RequestedBudget,
		RequestedConcurrency:  input.RequestedConcurrency,
	}
}
