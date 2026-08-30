package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/projection"
)

type productMissionAttemptOutputSource struct {
	evidence   *evidence.Store
	projection *projection.Projection
	now        func() time.Time
}

func newProductMissionAttemptOutputSource(
	evidenceStore *evidence.Store,
	readModel *projection.Projection,
	now func() time.Time,
) (*productMissionAttemptOutputSource, error) {
	if evidenceStore == nil || readModel == nil || now == nil {
		return nil, api.ErrInvalidLocalProductRequest
	}
	current := now()
	if current.IsZero() || current.Location() != time.UTC {
		return nil, api.ErrInvalidLocalProductRequest
	}
	return &productMissionAttemptOutputSource{
		evidence: evidenceStore, projection: readModel, now: now,
	}, nil
}

func (source *productMissionAttemptOutputSource) ReadMissionAttemptOutputs(
	ctx context.Context,
	request api.MissionAttemptOutputRequest,
) ([]api.MissionAttemptOutput, error) {
	if source == nil || source.evidence == nil || source.projection == nil ||
		ctx == nil || request.TeamInstanceID == "" || request.LogicalNodeID == "" ||
		request.AttemptNumber <= 0 || request.WorkItemID == "" || request.RunID == "" ||
		request.MaximumBytes <= 0 || request.MaximumBytes > 256<<10 {
		return nil, api.ErrInvalidLocalProductRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	execution, found := source.projection.GlobalReadView().TeamExecution(
		request.TeamInstanceID,
	)
	if !found || execution.TeamInstanceID != request.TeamInstanceID {
		return []api.MissionAttemptOutput{}, nil
	}
	attempt, found := productMissionOutputAttempt(execution, request)
	if !found || attempt.EvidenceID == "" || attempt.EvidenceDigest == "" ||
		attempt.OutputSummaryDigest == "" {
		return []api.MissionAttemptOutput{}, nil
	}
	receipt, found, err := source.evidence.AttemptReceipt(ctx, attempt.EvidenceID)
	if err != nil {
		return nil, err
	}
	if !found || receipt.Digest() != attempt.EvidenceDigest ||
		receipt.OutputSummary().Digest() != attempt.OutputSummaryDigest {
		return nil, errors.New("Mission Attempt output receipt mismatch")
	}
	output, err := source.evidence.ReadAggregationOutput(
		ctx, receipt, int64(request.MaximumBytes),
	)
	if err != nil {
		return nil, err
	}
	defer output.Close()
	return decodeProductMissionAttemptOutputs(
		output.Content(), request.MaximumBytes, source.now(),
	)
}

func productMissionOutputAttempt(
	execution projection.TeamExecution,
	request api.MissionAttemptOutputRequest,
) (projection.TeamExecutionAttempt, bool) {
	for _, node := range execution.Nodes {
		if node.LogicalNodeID != request.LogicalNodeID {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber == request.AttemptNumber &&
				attempt.WorkItemID == request.WorkItemID && attempt.RunID == request.RunID {
				return attempt, true
			}
		}
	}
	return projection.TeamExecutionAttempt{}, false
}

func decodeProductMissionAttemptOutputs(
	content []byte,
	maximumBytes int,
	occurredAt time.Time,
) ([]api.MissionAttemptOutput, error) {
	if len(content) == 0 || len(content) > maximumBytes || occurredAt.IsZero() ||
		occurredAt.Location() != time.UTC {
		return nil, api.ErrInvalidLocalProductRequest
	}
	var envelope struct {
		SchemaVersion int               `json:"schema_version"`
		Scope         string            `json:"scope"`
		Events        []json.RawMessage `json:"events"`
	}
	if decodeProductMissionOutputJSON(content, &envelope) != nil || envelope.SchemaVersion != 1 ||
		envelope.Scope != evidence.AggregationScopeAuthorizedOutputEvents ||
		len(envelope.Events) == 0 {
		return nil, api.ErrInvalidLocalProductRequest
	}
	result := make([]api.MissionAttemptOutput, 0, len(envelope.Events))
	total := 0
	for index, event := range envelope.Events {
		var payload struct {
			Delta string `json:"delta"`
		}
		if decodeProductMissionOutputJSON(event, &payload) != nil || payload.Delta == "" ||
			!utf8.ValidString(payload.Delta) {
			return nil, api.ErrInvalidLocalProductRequest
		}
		total += len(payload.Delta)
		if total > maximumBytes {
			return nil, api.ErrInvalidLocalProductRequest
		}
		digest := sha256.Sum256([]byte(payload.Delta))
		identityInput := append(
			[]byte("loom/mission-attempt-output/v1\x00"+strconv.Itoa(index+1)+"\x00"),
			digest[:]...,
		)
		identity := sha256.Sum256(identityInput)
		result = append(result, api.MissionAttemptOutput{
			SourceID:      "output-" + hex.EncodeToString(identity[:16]),
			Sequence:      int64(index + 1),
			ContentDigest: hex.EncodeToString(digest[:]),
			Text:          payload.Delta,
			OccurredAt:    occurredAt,
		})
	}
	return result, nil
}

func decodeProductMissionOutputJSON(content []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return api.ErrInvalidLocalProductRequest
	}
	return nil
}

var _ api.MissionAttemptOutputSource = (*productMissionAttemptOutputSource)(nil)
