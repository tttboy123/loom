package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrInvalidStructuredTeamDraft               = errors.New("invalid structured team draft")
	ErrStructuredTeamDraftCatalogMismatch       = errors.New("structured team draft catalog mismatch")
	ErrStructuredTeamDraftReferenceMismatch     = errors.New("structured team draft reference mismatch")
	ErrStructuredTeamDraftBindingDigestMismatch = errors.New("structured team draft binding digest mismatch")
	ErrStructuredTeamDraftContentNotReady       = errors.New("structured team draft content not ready")
	ErrStructuredTeamDraftGapRequiresQuestion   = errors.New("structured team draft gap requires question")
)

type StructuredTeamDraft struct {
	draft         TeamDraft
	content       TeamDraftContentSnapshot
	bindingDigest string
}

type StructuredTeamDraftAcceptanceCandidate struct {
	Eligible              bool
	DraftID               string
	Revision              int
	CatalogDigest         string
	ContentDigest         string
	BindingDigest         string
	MainAgentDefinitionID string
	RoleCount             int
	TaskCount             int
}

func NewStructuredTeamDraft(
	id string,
	catalog TeamDraftCatalogSnapshot,
	content TeamDraftContentSnapshot,
) (StructuredTeamDraft, error) {
	if _, err := ValidateTeamDraftContent(content, catalog); err != nil {
		return StructuredTeamDraft{}, err
	}
	draft, err := NewTeamDraft(id, catalog, content.References())
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	return buildStructuredTeamDraft(draft, content)
}

func PresentStructuredTeamDraft(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	question *DraftQuestion,
) (StructuredTeamDraft, error) {
	contentCandidate, err := validateStructuredTeamDraft(current, expectedRevision, catalog)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	if !contentCandidate.AcceptanceReady && question == nil {
		return StructuredTeamDraft{}, ErrStructuredTeamDraftGapRequiresQuestion
	}
	nextDraft, err := PresentTeamDraft(current.draft, expectedRevision, catalog, question)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	return buildStructuredTeamDraft(nextDraft, current.content)
}

func AnswerStructuredTeamDraft(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	questionID string,
	answerText string,
	nextContent TeamDraftContentSnapshot,
	nextQuestion *DraftQuestion,
) (StructuredTeamDraft, error) {
	if _, err := validateStructuredTeamDraft(current, expectedRevision, catalog); err != nil {
		return StructuredTeamDraft{}, err
	}
	contentCandidate, err := ValidateTeamDraftContent(nextContent, catalog)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	if !contentCandidate.AcceptanceReady && nextQuestion == nil {
		return StructuredTeamDraft{}, ErrStructuredTeamDraftGapRequiresQuestion
	}
	nextDraft, err := AnswerTeamDraft(
		current.draft,
		expectedRevision,
		catalog,
		questionID,
		answerText,
		nextContent.References(),
		nextQuestion,
	)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	return buildStructuredTeamDraft(nextDraft, nextContent)
}

func EditStructuredTeamDraft(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	nextContent TeamDraftContentSnapshot,
	question *DraftQuestion,
) (StructuredTeamDraft, error) {
	if _, err := validateStructuredTeamDraft(current, expectedRevision, catalog); err != nil {
		return StructuredTeamDraft{}, err
	}
	contentCandidate, err := ValidateTeamDraftContent(nextContent, catalog)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	if !contentCandidate.AcceptanceReady && question == nil {
		return StructuredTeamDraft{}, ErrStructuredTeamDraftGapRequiresQuestion
	}
	nextDraft, err := EditTeamDraft(
		current.draft,
		expectedRevision,
		catalog,
		nextContent.References(),
		question,
	)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	return buildStructuredTeamDraft(nextDraft, nextContent)
}

func CheckStructuredTeamDraftAcceptable(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
) (StructuredTeamDraftAcceptanceCandidate, error) {
	contentCandidate, err := validateStructuredTeamDraft(current, expectedRevision, catalog)
	if err != nil {
		return StructuredTeamDraftAcceptanceCandidate{}, err
	}
	if !contentCandidate.AcceptanceReady {
		return StructuredTeamDraftAcceptanceCandidate{}, ErrStructuredTeamDraftContentNotReady
	}
	if _, err := CheckTeamDraftAcceptable(current.draft, expectedRevision, catalog); err != nil {
		return StructuredTeamDraftAcceptanceCandidate{}, err
	}
	return StructuredTeamDraftAcceptanceCandidate{
		Eligible:              true,
		DraftID:               current.draft.ID(),
		Revision:              current.draft.Revision(),
		CatalogDigest:         current.draft.CatalogDigest(),
		ContentDigest:         current.content.Digest(),
		BindingDigest:         current.bindingDigest,
		MainAgentDefinitionID: contentCandidate.MainAgentDefinitionID,
		RoleCount:             contentCandidate.RoleCount,
		TaskCount:             contentCandidate.TaskCount,
	}, nil
}

func (d StructuredTeamDraft) ID() string {
	return d.draft.ID()
}

func (d StructuredTeamDraft) Revision() int {
	return d.draft.Revision()
}

func (d StructuredTeamDraft) State() TeamDraftState {
	return d.draft.State()
}

func (d StructuredTeamDraft) CatalogDigest() string {
	return d.draft.CatalogDigest()
}

func (d StructuredTeamDraft) ContentDigest() string {
	return d.content.Digest()
}

func (d StructuredTeamDraft) BindingDigest() string {
	return d.bindingDigest
}

func (d StructuredTeamDraft) References() TeamDraftReferences {
	return d.draft.References()
}

func (d StructuredTeamDraft) Content() TeamDraftContentSnapshot {
	return cloneTeamDraftContentSnapshot(d.content)
}

func (d StructuredTeamDraft) Roles() []TeamDraftRoleSelection {
	return d.content.Roles()
}

func (d StructuredTeamDraft) Tasks() []TeamDraftTaskCandidate {
	return d.content.Tasks()
}

func (d StructuredTeamDraft) ApprovalMarkers() []TeamDraftApprovalMarker {
	return d.content.ApprovalMarkers()
}

func (d StructuredTeamDraft) CapabilityGaps() []TeamDraftCapabilityGap {
	return d.content.CapabilityGaps()
}

func (d StructuredTeamDraft) Question() (DraftQuestion, bool) {
	return d.draft.Question()
}

func (d StructuredTeamDraft) LastAnsweredQuestionID() string {
	return d.draft.LastAnsweredQuestionID()
}

func (d StructuredTeamDraft) LastAnswerText() string {
	return d.draft.LastAnswerText()
}

func buildStructuredTeamDraft(
	draft TeamDraft,
	content TeamDraftContentSnapshot,
) (StructuredTeamDraft, error) {
	if draft.CatalogDigest() == "" || content.Digest() == "" {
		return StructuredTeamDraft{}, ErrInvalidStructuredTeamDraft
	}
	if draft.CatalogDigest() != content.CatalogDigest() {
		return StructuredTeamDraft{}, ErrStructuredTeamDraftCatalogMismatch
	}
	if !equalStructuredReferences(draft.References(), content.References()) {
		return StructuredTeamDraft{}, ErrStructuredTeamDraftReferenceMismatch
	}
	structured := StructuredTeamDraft{
		draft:   copyStructuredCoreDraft(draft),
		content: cloneTeamDraftContentSnapshot(content),
	}
	digest, err := digestStructuredTeamDraft(structured)
	if err != nil {
		return StructuredTeamDraft{}, err
	}
	structured.bindingDigest = digest
	return structured, nil
}

func validateStructuredTeamDraft(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
) (TeamDraftContentValidationCandidate, error) {
	if current.bindingDigest == "" || current.content.Digest() == "" {
		return TeamDraftContentValidationCandidate{}, ErrInvalidStructuredTeamDraft
	}
	digest, err := digestStructuredTeamDraft(current)
	if err != nil {
		return TeamDraftContentValidationCandidate{}, err
	}
	if digest != current.bindingDigest {
		return TeamDraftContentValidationCandidate{}, ErrStructuredTeamDraftBindingDigestMismatch
	}
	if err := validateDraftCommand(current.draft, expectedRevision, catalog); err != nil {
		return TeamDraftContentValidationCandidate{}, err
	}
	contentCandidate, err := ValidateTeamDraftContent(current.content, catalog)
	if err != nil {
		return TeamDraftContentValidationCandidate{}, err
	}
	if current.draft.CatalogDigest() != current.content.CatalogDigest() {
		return TeamDraftContentValidationCandidate{}, ErrStructuredTeamDraftCatalogMismatch
	}
	if !equalStructuredReferences(current.draft.References(), current.content.References()) {
		return TeamDraftContentValidationCandidate{}, ErrStructuredTeamDraftReferenceMismatch
	}
	return contentCandidate, nil
}

func digestStructuredTeamDraft(input StructuredTeamDraft) (string, error) {
	question, hasQuestion := input.draft.Question()
	canonical := struct {
		DraftID              string
		Revision             int
		State                TeamDraftState
		CatalogDigest        string
		References           TeamDraftReferences
		Question             DraftQuestion
		HasQuestion          bool
		LastAnsweredQuestion string
		LastAnswerText       string
		ContentDigest        string
	}{
		DraftID:              input.draft.ID(),
		Revision:             input.draft.Revision(),
		State:                input.draft.State(),
		CatalogDigest:        input.draft.CatalogDigest(),
		References:           input.draft.References(),
		Question:             question,
		HasQuestion:          hasQuestion,
		LastAnsweredQuestion: input.draft.LastAnsweredQuestionID(),
		LastAnswerText:       input.draft.LastAnswerText(),
		ContentDigest:        input.content.Digest(),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: binding digest: %v", ErrInvalidStructuredTeamDraft, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func equalStructuredReferences(left, right TeamDraftReferences) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func copyStructuredCoreDraft(input TeamDraft) TeamDraft {
	return TeamDraft{
		id:                     input.id,
		revision:               input.revision,
		state:                  input.state,
		catalogDigest:          input.catalogDigest,
		references:             copyTeamDraftReferences(input.references),
		question:               input.question,
		hasQuestion:            input.hasQuestion,
		lastAnsweredQuestionID: input.lastAnsweredQuestionID,
		lastAnswerText:         input.lastAnswerText,
	}
}

func cloneStructuredTeamDraft(input StructuredTeamDraft) StructuredTeamDraft {
	input.draft = copyStructuredCoreDraft(input.draft)
	input.content = cloneTeamDraftContentSnapshot(input.content)
	return input
}
