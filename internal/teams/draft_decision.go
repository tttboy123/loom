package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrInvalidTeamDraftDecisionCommand   = errors.New("invalid team draft decision command")
	ErrInvalidTeamDraftDecisionSemantics = errors.New("invalid team draft decision semantics")
	ErrTeamDraftDecisionSourceMismatch   = errors.New("team draft decision source mismatch")
	ErrIneligibleTeamDraftAcceptance     = errors.New("ineligible team draft acceptance")
	ErrInvalidDecidedTeamDraft           = errors.New("invalid decided team draft")
	ErrTeamDraftDecisionDigestMismatch   = errors.New("team draft decision digest mismatch")
)

type TeamDraftDecisionKind string

const (
	TeamDraftDecisionAccepted TeamDraftDecisionKind = "accepted"
	TeamDraftDecisionRejected TeamDraftDecisionKind = "rejected"
	TeamDraftDecisionExpired  TeamDraftDecisionKind = "expired"
)

type TeamDraftDecisionActorKind string

const (
	TeamDraftDecisionActorUser   TeamDraftDecisionActorKind = "user"
	TeamDraftDecisionActorSystem TeamDraftDecisionActorKind = "system"
)

type TeamDraftDecisionAction string

const (
	TeamDraftDecisionActionConfirmAndStart TeamDraftDecisionAction = "confirm_and_start"
	TeamDraftDecisionActionReject          TeamDraftDecisionAction = "reject"
	TeamDraftDecisionActionExpire          TeamDraftDecisionAction = "expire"
)

type TeamDraftDecisionCommand struct {
	DecisionID          string
	Kind                TeamDraftDecisionKind
	ActorKind           TeamDraftDecisionActorKind
	ActorID             string
	Action              TeamDraftDecisionAction
	DraftID             string
	Revision            int
	CatalogDigest       string
	ContentDigest       string
	BindingDigest       string
	DecidedAtUnixMillis int64
	Reason              string
}

type DecidedTeamDraft struct {
	source         StructuredTeamDraft
	revision       int
	kind           TeamDraftDecisionKind
	command        TeamDraftDecisionCommand
	decisionDigest string
}

type DecidedTeamDraftValidationCandidate struct {
	Valid          bool
	Kind           TeamDraftDecisionKind
	DraftID        string
	SourceRevision int
	Revision       int
	CatalogDigest  string
	ContentDigest  string
	BindingDigest  string
	DecisionDigest string
}

func DecideStructuredTeamDraft(
	current StructuredTeamDraft,
	expectedRevision int,
	catalog TeamDraftCatalogSnapshot,
	command TeamDraftDecisionCommand,
) (DecidedTeamDraft, error) {
	if _, err := validateStructuredTeamDraft(current, expectedRevision, catalog); err != nil {
		return DecidedTeamDraft{}, err
	}
	if err := validateTeamDraftDecisionCommand(command, current); err != nil {
		return DecidedTeamDraft{}, err
	}
	if command.Kind == TeamDraftDecisionAccepted {
		if _, err := CheckStructuredTeamDraftAcceptable(current, expectedRevision, catalog); err != nil {
			return DecidedTeamDraft{}, fmt.Errorf("%w: %w", ErrIneligibleTeamDraftAcceptance, err)
		}
	}

	decided := DecidedTeamDraft{
		source:   cloneStructuredTeamDraft(current),
		revision: current.Revision() + 1,
		kind:     command.Kind,
		command:  command,
	}
	digest, err := digestDecidedTeamDraft(decided)
	if err != nil {
		return DecidedTeamDraft{}, err
	}
	decided.decisionDigest = digest
	return decided, nil
}

func ValidateDecidedTeamDraft(
	current DecidedTeamDraft,
	catalog TeamDraftCatalogSnapshot,
) (DecidedTeamDraftValidationCandidate, error) {
	if current.revision <= 0 || current.decisionDigest == "" {
		return DecidedTeamDraftValidationCandidate{}, ErrInvalidDecidedTeamDraft
	}
	if _, err := validateStructuredTeamDraft(current.source, current.source.Revision(), catalog); err != nil {
		return DecidedTeamDraftValidationCandidate{}, err
	}
	if current.revision != current.source.Revision()+1 || current.kind != current.command.Kind {
		return DecidedTeamDraftValidationCandidate{}, ErrInvalidDecidedTeamDraft
	}
	if err := validateTeamDraftDecisionCommand(current.command, current.source); err != nil {
		return DecidedTeamDraftValidationCandidate{}, err
	}
	if current.kind == TeamDraftDecisionAccepted {
		if _, err := CheckStructuredTeamDraftAcceptable(current.source, current.source.Revision(), catalog); err != nil {
			return DecidedTeamDraftValidationCandidate{}, fmt.Errorf("%w: %w", ErrIneligibleTeamDraftAcceptance, err)
		}
	}
	digest, err := digestDecidedTeamDraft(current)
	if err != nil {
		return DecidedTeamDraftValidationCandidate{}, err
	}
	if digest != current.decisionDigest {
		return DecidedTeamDraftValidationCandidate{}, ErrTeamDraftDecisionDigestMismatch
	}
	return DecidedTeamDraftValidationCandidate{
		Valid:          true,
		Kind:           current.kind,
		DraftID:        current.source.ID(),
		SourceRevision: current.source.Revision(),
		Revision:       current.revision,
		CatalogDigest:  current.source.CatalogDigest(),
		ContentDigest:  current.source.ContentDigest(),
		BindingDigest:  current.source.BindingDigest(),
		DecisionDigest: current.decisionDigest,
	}, nil
}

func (d DecidedTeamDraft) Kind() TeamDraftDecisionKind {
	return d.kind
}

func (d DecidedTeamDraft) DraftID() string {
	return d.source.ID()
}

func (d DecidedTeamDraft) SourceRevision() int {
	return d.source.Revision()
}

func (d DecidedTeamDraft) Revision() int {
	return d.revision
}

func (d DecidedTeamDraft) CatalogDigest() string {
	return d.source.CatalogDigest()
}

func (d DecidedTeamDraft) ContentDigest() string {
	return d.source.ContentDigest()
}

func (d DecidedTeamDraft) BindingDigest() string {
	return d.source.BindingDigest()
}

func (d DecidedTeamDraft) References() TeamDraftReferences {
	return d.source.References()
}

func (d DecidedTeamDraft) Content() TeamDraftContentSnapshot {
	return d.source.Content()
}

func (d DecidedTeamDraft) Source() StructuredTeamDraft {
	return cloneStructuredTeamDraft(d.source)
}

func (d DecidedTeamDraft) Command() TeamDraftDecisionCommand {
	return d.command
}

func (d DecidedTeamDraft) DecisionDigest() string {
	return d.decisionDigest
}

func validateTeamDraftDecisionCommand(
	command TeamDraftDecisionCommand,
	source StructuredTeamDraft,
) error {
	if command.DecisionID == "" || command.ActorID == "" ||
		command.DecidedAtUnixMillis <= 0 || command.DraftID == "" ||
		command.Revision <= 0 || command.CatalogDigest == "" ||
		command.ContentDigest == "" || command.BindingDigest == "" {
		return ErrInvalidTeamDraftDecisionCommand
	}
	switch command.Kind {
	case TeamDraftDecisionAccepted, TeamDraftDecisionRejected, TeamDraftDecisionExpired:
	default:
		return ErrInvalidTeamDraftDecisionCommand
	}
	switch command.ActorKind {
	case TeamDraftDecisionActorUser, TeamDraftDecisionActorSystem:
	default:
		return ErrInvalidTeamDraftDecisionCommand
	}
	switch command.Action {
	case TeamDraftDecisionActionConfirmAndStart, TeamDraftDecisionActionReject, TeamDraftDecisionActionExpire:
	default:
		return ErrInvalidTeamDraftDecisionCommand
	}
	if command.DraftID != source.ID() ||
		command.Revision != source.Revision() ||
		command.CatalogDigest != source.CatalogDigest() ||
		command.ContentDigest != source.ContentDigest() ||
		command.BindingDigest != source.BindingDigest() {
		return ErrTeamDraftDecisionSourceMismatch
	}
	switch command.Kind {
	case TeamDraftDecisionAccepted:
		if command.ActorKind != TeamDraftDecisionActorUser ||
			command.Action != TeamDraftDecisionActionConfirmAndStart ||
			command.Reason != "" {
			return ErrInvalidTeamDraftDecisionSemantics
		}
	case TeamDraftDecisionRejected:
		if command.ActorKind != TeamDraftDecisionActorUser ||
			command.Action != TeamDraftDecisionActionReject ||
			command.Reason == "" {
			return ErrInvalidTeamDraftDecisionSemantics
		}
	case TeamDraftDecisionExpired:
		if command.ActorKind != TeamDraftDecisionActorSystem ||
			command.Action != TeamDraftDecisionActionExpire ||
			command.Reason == "" {
			return ErrInvalidTeamDraftDecisionSemantics
		}
	}
	return nil
}

func digestDecidedTeamDraft(input DecidedTeamDraft) (string, error) {
	canonical := struct {
		DraftID         string
		SourceRevision  int
		Revision        int
		SourceState     TeamDraftState
		CatalogDigest   string
		ContentDigest   string
		BindingDigest   string
		References      TeamDraftReferences
		DecisionID      string
		Kind            TeamDraftDecisionKind
		ActorKind       TeamDraftDecisionActorKind
		ActorID         string
		Action          TeamDraftDecisionAction
		DecidedAtMillis int64
		Reason          string
	}{
		DraftID:         input.source.ID(),
		SourceRevision:  input.source.Revision(),
		Revision:        input.revision,
		SourceState:     input.source.State(),
		CatalogDigest:   input.source.CatalogDigest(),
		ContentDigest:   input.source.ContentDigest(),
		BindingDigest:   input.source.BindingDigest(),
		References:      input.source.References(),
		DecisionID:      input.command.DecisionID,
		Kind:            input.command.Kind,
		ActorKind:       input.command.ActorKind,
		ActorID:         input.command.ActorID,
		Action:          input.command.Action,
		DecidedAtMillis: input.command.DecidedAtUnixMillis,
		Reason:          input.command.Reason,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: decision digest: %v", ErrInvalidDecidedTeamDraft, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneDecidedTeamDraft(input DecidedTeamDraft) DecidedTeamDraft {
	input.source = cloneStructuredTeamDraft(input.source)
	return input
}
