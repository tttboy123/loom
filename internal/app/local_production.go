package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/production"
)

var (
	ErrInvalidProductionRequest = errors.New("invalid local production request")
	ErrProductionUnavailable    = errors.New("production service unavailable")
)

type ProductionSnapshotRequest struct {
	JourneyID string `json:"-"`
}

type ProductionCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type ProductionCommandResult struct {
	OperationID string                     `json:"operation_id"`
	Action      string                     `json:"action"`
	ViewVersion string                     `json:"view_version"`
	Result      production.CommandResult   `json:"result,omitempty"`
	EventIDs    []string                   `json:"event_ids,omitempty"`
	Note        string                     `json:"note,omitempty"`
}

type LocalProductionService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	service     *production.Service
}

func NewLocalProductionService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
	service *production.Service,
) (*LocalProductionService, error) {
	if store == nil || now == nil || viewVersion == nil || service == nil {
		return nil, ErrInvalidProductionRequest
	}
	return &LocalProductionService{
		store: store, now: now, viewVersion: viewVersion, service: service,
	}, nil
}

func (service *LocalProductionService) ProductionSnapshot(
	ctx context.Context,
	request ProductionSnapshotRequest,
) (production.ProductionSnapshot, error) {
	if service == nil || service.service == nil {
		return production.ProductionSnapshot{}, ErrProductionUnavailable
	}
	return service.service.Snapshot(ctx)
}

func (service *LocalProductionService) ProductionCommand(
	ctx context.Context,
	request ProductionCommandRequest,
) (ProductionCommandResult, error) {
	if service == nil || service.service == nil {
		return ProductionCommandResult{}, ErrProductionUnavailable
	}
	if request.OperationID == "" || request.Action == "" {
		return ProductionCommandResult{}, ErrInvalidProductionRequest
	}
	var input struct {
		Operation     string `json:"operation"`
		TargetMode    string `json:"target_mode"`
		PreviewDigest string `json:"preview_digest"`
		AuthorizedBy  string `json:"authorized_by"`
	}
	if err := decodeExactProductionParams(request.Input, &input); err != nil {
		return ProductionCommandResult{}, ErrInvalidProductionRequest
	}
	if input.Operation == "" {
		return ProductionCommandResult{}, ErrInvalidProductionRequest
	}
	result, err := service.service.Command(ctx, production.ActivationCommand{
		Operation: input.Operation, TargetMode: input.TargetMode,
		PreviewDigest: input.PreviewDigest, AuthorizedBy: input.AuthorizedBy,
		JourneyID: request.JourneyID, OperationID: request.OperationID,
	})
	if err != nil {
		return ProductionCommandResult{}, err
	}
	return ProductionCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), Result: result,
		EventIDs: result.EventIDs, Note: result.Note,
	}, nil
}

func (service *LocalProductionService) Degraded(ctx context.Context) bool {
	if service == nil || service.service == nil {
		return true
	}
	return service.service.Degraded(ctx)
}

func decodeExactProductionParams(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data")
	}
	return nil
}
