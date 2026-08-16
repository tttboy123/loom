//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const AggregationScopeAuthorizedOutputEvents = "authorized_output_events"

type AggregationOutput struct {
	binding             AttemptCaptureInput
	evidenceDigest      string
	outputSummaryDigest string
	content             []byte
}

func (output AggregationOutput) Binding() AttemptCaptureInput { return output.binding }
func (output AggregationOutput) EvidenceDigest() string       { return output.evidenceDigest }
func (output AggregationOutput) OutputSummaryDigest() string  { return output.outputSummaryDigest }
func (output AggregationOutput) Content() []byte {
	return append([]byte(nil), output.content...)
}

func (output *AggregationOutput) Close() {
	if output == nil {
		return
	}
	for index := range output.content {
		output.content[index] = 0
	}
	output.content = nil
}

// ReadAggregationOutput returns only authorized output-event payloads from an
// exact finalized Attempt. Evidence, result, tool, and private runtime frames
// stay outside the aggregation disclosure boundary.
func (store *Store) ReadAggregationOutput(
	ctx context.Context,
	receipt AttemptReceipt,
	maximumBytes int64,
) (AggregationOutput, error) {
	if store == nil || ctx == nil || maximumBytes <= 0 ||
		receipt.evidenceID == "" || receipt.digest == "" ||
		receipt.summary.digest == "" {
		return AggregationOutput{}, ErrInvalidAttemptCapture
	}
	exact, found, err := store.AttemptReceipt(ctx, receipt.evidenceID)
	if err != nil || !found || exact.digest != receipt.digest ||
		exact.summary.digest != receipt.summary.digest ||
		!exact.summary.resultObserved || exact.summary.terminalStatus != "succeeded" {
		return AggregationOutput{}, ErrAttemptCaptureConflict
	}
	artifactBytes, err := store.ReadArtifact(ctx, receipt.digest, maximumBytes)
	if err != nil {
		return AggregationOutput{}, err
	}
	defer zeroAggregationBytes(artifactBytes)
	var artifact attemptArtifact
	decoder := json.NewDecoder(bytes.NewReader(artifactBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&artifact) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		artifact.EvidenceID != receipt.evidenceID ||
		artifact.Status != "succeeded" || !artifact.ChildResultObserved {
		return AggregationOutput{}, ErrAttemptCaptureConflict
	}
	events := make([]json.RawMessage, 0, exact.summary.outputFrameCount)
	for _, line := range artifact.AuthorizedFrames {
		frame, decodeErr := bridgev1.DecodeLine([]byte(line))
		if decodeErr != nil || !frameMatchesAttempt(frame, AttemptCaptureInput{
			EvidenceID: artifact.EvidenceID, TeamInstanceID: artifact.TeamInstanceID,
			PlanDigest: artifact.PlanDigest, LogicalNodeID: artifact.LogicalNodeID,
			AttemptNumber: artifact.AttemptNumber, WorkItemID: artifact.WorkItemID,
			RunID: artifact.RunID, ClaimID: artifact.ClaimID,
			ClaimGeneration:   artifact.ClaimGeneration,
			RuntimeInstanceID: artifact.RuntimeInstanceID,
			AgentInstanceID:   artifact.AgentInstanceID,
		}) {
			return AggregationOutput{}, ErrAttemptCaptureConflict
		}
		if frame.Type() != bridgev1.MessageEvent {
			continue
		}
		payload := frame.Payload()
		if !json.Valid(payload) {
			return AggregationOutput{}, ErrAttemptCaptureConflict
		}
		events = append(events, append(json.RawMessage(nil), payload...))
	}
	if len(events) == 0 {
		return AggregationOutput{}, ErrAttemptCaptureIncomplete
	}
	content, err := json.Marshal(struct {
		SchemaVersion int               `json:"schema_version"`
		Scope         string            `json:"scope"`
		Events        []json.RawMessage `json:"events"`
	}{1, AggregationScopeAuthorizedOutputEvents, events})
	if err != nil || int64(len(content)) > maximumBytes {
		zeroAggregationBytes(content)
		return AggregationOutput{}, ErrArtifactTooLarge
	}
	return AggregationOutput{
		binding: AttemptCaptureInput{
			EvidenceID: artifact.EvidenceID, TeamInstanceID: artifact.TeamInstanceID,
			PlanDigest: artifact.PlanDigest, LogicalNodeID: artifact.LogicalNodeID,
			AttemptNumber: artifact.AttemptNumber, WorkItemID: artifact.WorkItemID,
			RunID: artifact.RunID, ClaimID: artifact.ClaimID,
			ClaimGeneration:   artifact.ClaimGeneration,
			RuntimeInstanceID: artifact.RuntimeInstanceID,
			AgentInstanceID:   artifact.AgentInstanceID,
		},
		evidenceDigest: receipt.digest, outputSummaryDigest: receipt.summary.digest,
		content: content,
	}, nil
}

func zeroAggregationBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
