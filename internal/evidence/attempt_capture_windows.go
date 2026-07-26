//go:build windows

package evidence

import (
	"context"
	"errors"
)

var (
	ErrInvalidAttemptCapture    = errors.New("invalid attempt capture")
	ErrAttemptCaptureConflict   = errors.New("attempt capture conflict")
	ErrAttemptCaptureIncomplete = errors.New("attempt capture incomplete")
)

type AttemptCaptureInput struct {
	EvidenceID        string
	TeamInstanceID    string
	PlanDigest        string
	LogicalNodeID     string
	AttemptNumber     int
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
}

type AttemptTerminal struct {
	Status string
	Reason string
}

type AttemptReceipt struct{}
type AttemptOutputSummary struct{}
type AttemptCaptureState struct{}

func (AttemptReceipt) EvidenceID() string { return "" }
func (AttemptReceipt) Digest() string     { return "" }
func (AttemptReceipt) OutputSummary() AttemptOutputSummary {
	return AttemptOutputSummary{}
}
func (AttemptOutputSummary) EvidenceID() string          { return "" }
func (AttemptOutputSummary) EvidenceDigest() string      { return "" }
func (AttemptOutputSummary) AuthorizedFrameCount() int   { return 0 }
func (AttemptOutputSummary) OutputFrameCount() int       { return 0 }
func (AttemptOutputSummary) OutputPayloadBytes() int     { return 0 }
func (AttemptOutputSummary) ResultObserved() bool        { return false }
func (AttemptOutputSummary) TerminalStatus() string      { return "" }
func (AttemptOutputSummary) Digest() string              { return "" }
func (AttemptCaptureState) Binding() AttemptCaptureInput { return AttemptCaptureInput{} }
func (AttemptCaptureState) FrameCount() int              { return 0 }
func (AttemptCaptureState) ResultObserved() bool         { return false }
func (*Store) BeginAttemptCapture(context.Context, AttemptCaptureInput) error {
	return ErrUnsupportedPlatform
}
func (*Store) AppendAttemptFrame(context.Context, string, []byte) error {
	return ErrUnsupportedPlatform
}
func (*Store) RebindAttemptCapture(context.Context, AttemptCaptureInput) error {
	return ErrUnsupportedPlatform
}
func (*Store) FinalizeAttemptCapture(
	context.Context,
	string,
	AttemptTerminal,
) (AttemptReceipt, error) {
	return AttemptReceipt{}, ErrUnsupportedPlatform
}
func (*Store) AttemptReceipt(
	context.Context,
	string,
) (AttemptReceipt, bool, error) {
	return AttemptReceipt{}, false, ErrUnsupportedPlatform
}
func (*Store) AttemptCapture(
	context.Context,
	string,
) (AttemptCaptureState, bool, error) {
	return AttemptCaptureState{}, false, ErrUnsupportedPlatform
}
