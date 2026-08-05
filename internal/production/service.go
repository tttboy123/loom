package production

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

const productionPrefix = "prod1"

type Service struct {
	store     *journal.Store
	paths     Paths
	now       Now
	adminLock AdminLockReader
}

func NewService(store *journal.Store, paths Paths, now Now, adminLock AdminLockReader) (*Service, error) {
	if store == nil || now == nil || paths.AppSupport == "" ||
		paths.LaunchAgents == "" || paths.DaemonPath == "" {
		return nil, ErrInvalidProductionInput
	}
	if adminLock == nil {
		adminLock = func(context.Context) (bool, error) { return false, nil }
	}
	return &Service{store: store, paths: paths, now: now, adminLock: adminLock}, nil
}

func (s *Service) Snapshot(ctx context.Context) (ProductionSnapshot, error) {
	if s == nil || s.store == nil {
		return ProductionSnapshot{}, ErrInvalidProductionInput
	}
	activated, activatedAt, mode, err := s.activationFromJournal(ctx)
	if err != nil {
		return ProductionSnapshot{}, err
	}
	recovery := s.recoveryStatus(ctx, activated)
	return ProductionSnapshot{
		ViewVersion: "v1", Activated: activated,
		ActivatedAt: activatedAt, TargetMode: mode, Recovery: recovery,
	}, nil
}

func (s *Service) Command(ctx context.Context, command ActivationCommand) (CommandResult, error) {
	if s == nil || s.store == nil {
		return CommandResult{}, ErrInvalidProductionInput
	}
	if command.OperationID == "" || command.JourneyID == "" {
		return CommandResult{}, ErrInvalidProductionInput
	}
	switch command.Operation {
	case "activation_preview":
		preview, err := s.preview(ctx, "activation", command.TargetMode)
		if err != nil {
			return CommandResult{}, err
		}
		return CommandResult{Operation: command.Operation, Preview: preview, Digest: preview.Digest}, nil
	case "deactivation_preview":
		preview, err := s.preview(ctx, "deactivation", "")
		if err != nil {
			return CommandResult{}, err
		}
		return CommandResult{Operation: command.Operation, Preview: preview, Digest: preview.Digest}, nil
	case "activation_confirm":
		return s.confirmActivation(ctx, command)
	case "deactivation_confirm":
		return s.confirmDeactivation(ctx, command)
	default:
		return CommandResult{}, fmt.Errorf("%w: unknown operation %q", ErrInvalidProductionInput, command.Operation)
	}
}

func (s *Service) Degraded(ctx context.Context) bool {
	activated, _, _, err := s.activationFromJournal(ctx)
	if err != nil {
		return true
	}
	return s.recoveryStatus(ctx, activated).Degraded
}

func (s *Service) preview(ctx context.Context, operation, targetMode string) (ActivationPreview, error) {
	adminLock, err := s.adminLock(ctx)
	if err != nil {
		return ActivationPreview{}, err
	}
	var files []FileChange
	var launchd []LaunchdChange
	switch operation {
	case "activation":
		configPath := s.paths.ConfigPath()
		configContent := desiredConfig(true, targetMode)
		files = append(files, FileChange{
			Path: configPath, Action: "write",
			CurrentDigest: digestFile(configPath), DesiredDigest: sha256Hex(configContent),
			Mode: "0600",
		})
		plistPath := s.paths.PlistPath()
		plistContent := desiredPlist(s.paths.DaemonPath, s.paths.AppSupport)
		files = append(files, FileChange{
			Path: plistPath, Action: "write",
			CurrentDigest: digestFile(plistPath), DesiredDigest: sha256Hex(plistContent),
			Mode: "0600",
		})
		launchd = append(launchd, LaunchdChange{
			Label: "com.loom.local.daemon", PlistPath: plistPath, Action: "install",
		})
	case "deactivation":
		for _, path := range []string{s.paths.ConfigPath(), s.paths.PlistPath()} {
			files = append(files, FileChange{
				Path: path, Action: "remove",
				CurrentDigest: digestFile(path), DesiredDigest: "absent",
				Mode: "0600",
			})
		}
		launchd = append(launchd, LaunchdChange{
			Label: "com.loom.local.daemon", PlistPath: s.paths.PlistPath(), Action: "remove",
		})
	default:
		return ActivationPreview{}, ErrInvalidProductionInput
	}
	preview := ActivationPreview{
		Operation: operation, Scope: "user", TargetMode: targetMode,
		Files: files, LaunchdItems: launchd, AdminLock: adminLock,
	}
	preview.Digest = previewDigest(preview)
	return preview, nil
}

func (s *Service) confirmActivation(ctx context.Context, command ActivationCommand) (CommandResult, error) {
	if command.AuthorizedBy == "" {
		return CommandResult{}, ErrAuthorizationRequired
	}
	events, err := s.store.ReadAll(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	if result, ok := existingProductionResult(events, "activation", command.OperationID); ok {
		return result, nil
	}
	expected, err := s.preview(ctx, "activation", command.TargetMode)
	if err != nil {
		return CommandResult{}, err
	}
	if command.PreviewDigest == "" || command.PreviewDigest != expected.Digest {
		return CommandResult{}, ErrPreviewDigestMismatch
	}
	if expected.AdminLock && command.TargetMode == "bypass_permissions" {
		return CommandResult{}, ErrAdminLockBlocksBypass
	}
	heads := streamHeads(events)
	now := s.now().UTC()
	activatedEvent, err := buildActivationEvent("activation", command, now)
	if err != nil {
		return CommandResult{}, err
	}
	committed, err := appendCAS(ctx, s.store, events, heads, streamActivation, activatedEvent)
	if err != nil {
		return CommandResult{}, err
	}
	ids := eventIDs(committed)
	// Backup existing state, then write. Any failure rolls back.
	configPath := s.paths.ConfigPath()
	plistPath := s.paths.PlistPath()
	backupConfig := s.paths.AppSupport + "/" + configBackupName
	backupPlist := s.paths.LaunchAgents + "/" + plistBackupName
	if digestFile(configPath) != "absent" {
		if err := copyFileAtomic(configPath, backupConfig); err != nil {
			return CommandResult{}, err
		}
	}
	if digestFile(plistPath) != "absent" {
		if err := copyFileAtomic(plistPath, backupPlist); err != nil {
			_ = removeFileIfExists(configPath)
			return CommandResult{}, err
		}
	}
	configContent := desiredConfig(true, command.TargetMode)
	if err := atomicWrite(configPath, configContent, privateFileMode); err != nil {
		_ = s.recordRollback(ctx, command, configPath, err)
		return CommandResult{}, err
	}
	if err := installPlist(s.paths); err != nil {
		_ = removeFileIfExists(configPath)
		_ = s.recordRollback(ctx, command, plistPath, err)
		return CommandResult{}, err
	}
	written, err := buildConfigEvent("written", configPath, command, now)
	if err == nil {
		if committed, casErr := appendCAS(ctx, s.store, events, streamHeads(events), streamConfigPrefix+"daemon.json", written); casErr == nil {
			ids = append(ids, eventIDs(committed)...)
		}
	}
	recovery := s.recoveryStatus(ctx, true)
	return CommandResult{
		Operation: command.Operation, Digest: expected.Digest,
		Activated: true, Recovery: recovery, EventIDs: ids,
		Note: "Loom resident daemon activated",
	}, nil
}

func (s *Service) confirmDeactivation(ctx context.Context, command ActivationCommand) (CommandResult, error) {
	if command.AuthorizedBy == "" {
		return CommandResult{}, ErrAuthorizationRequired
	}
	events, err := s.store.ReadAll(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	if result, ok := existingProductionResult(events, "deactivation", command.OperationID); ok {
		return result, nil
	}
	expected, err := s.preview(ctx, "deactivation", "")
	if err != nil {
		return CommandResult{}, err
	}
	if command.PreviewDigest == "" || command.PreviewDigest != expected.Digest {
		return CommandResult{}, ErrPreviewDigestMismatch
	}
	heads := streamHeads(events)
	now := s.now().UTC()
	deactivatedEvent, err := buildActivationEvent("deactivation", command, now)
	if err != nil {
		return CommandResult{}, err
	}
	committed, err := appendCAS(ctx, s.store, events, heads, streamActivation, deactivatedEvent)
	if err != nil {
		return CommandResult{}, err
	}
	ids := eventIDs(committed)
	configPath := s.paths.ConfigPath()
	plistPath := s.paths.PlistPath()
	backupConfig := s.paths.AppSupport + "/" + configBackupName
	backupPlist := s.paths.LaunchAgents + "/" + plistBackupName
	if err := removePlist(s.paths); err != nil {
		return CommandResult{}, err
	}
	if digestFile(backupPlist) != "absent" {
		if err := copyFileAtomic(backupPlist, plistPath); err != nil {
			return CommandResult{}, err
		}
	}
	if digestFile(backupConfig) != "absent" {
		if err := copyFileAtomic(backupConfig, configPath); err != nil {
			return CommandResult{}, err
		}
	} else if err := removeFileIfExists(configPath); err != nil {
		return CommandResult{}, err
	}
	return CommandResult{
		Operation: command.Operation, Digest: expected.Digest,
		Activated: false, EventIDs: ids,
		Note: "Loom resident daemon deactivated",
	}, nil
}

func (s *Service) recoveryStatus(ctx context.Context, activated bool) RecoveryStatus {
	if !activated {
		return RecoveryStatus{Degraded: false, LastActivated: false}
	}
	matches, err := configMatches(s.paths.ConfigPath())
	if err != nil || !matches {
		return RecoveryStatus{
			Degraded: true, LastActivated: true,
			ConfigDigestMismatch: true,
			Reason:               "journal says activated but disk config digest does not match",
		}
	}
	return RecoveryStatus{Degraded: false, LastActivated: true}
}

func configMatches(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var payload configPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return false, err
	}
	return payload.SchemaVersion == 1 && payload.Activated, nil
}

func (s *Service) activationFromJournal(ctx context.Context) (bool, string, string, error) {
	events, err := s.store.ReadAll(ctx)
	if err != nil {
		return false, "", "", err
	}
	activated := false
	activatedAt := ""
	mode := ""
	for _, event := range events {
		if event.Type == EventActivationActivated {
			var payload struct {
				Mode        string `json:"target_mode"`
				ActivatedAt string `json:"activated_at"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
				continue
			}
			activated = true
			activatedAt = payload.ActivatedAt
			mode = payload.Mode
		}
		if event.Type == EventActivationDeactivated {
			activated = false
			activatedAt = ""
			mode = ""
		}
	}
	return activated, activatedAt, mode, nil
}

func (s *Service) recordRollback(ctx context.Context, command ActivationCommand, path string, cause error) error {
	events, err := s.store.ReadAll(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	event, err := buildConfigEvent("rolled_back", path, command, now)
	if err != nil {
		return err
	}
	_, err = appendCAS(ctx, s.store, events, streamHeads(events), streamConfigPrefix+path, event)
	return err
}

func buildActivationEvent(operation string, command ActivationCommand, now interface{ Format(string) string }) (journal.Event, error) {
	eventType := EventActivationActivated
	if operation == "deactivation" {
		eventType = EventActivationDeactivated
	}
	payload := map[string]any{
		"scope": "user", "target_mode": command.TargetMode,
		"preview_digest": command.PreviewDigest,
		"authorized_by":  command.AuthorizedBy,
	}
	timeKey := "activated_at"
	if operation == "deactivation" {
		timeKey = "deactivated_at"
	}
	payload[timeKey] = now.Format("2006-01-02T15:04:05.999999999Z07:00")
	body, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, err
	}
	return journal.Event{
		ID:             deterministicEventID(productionPrefix, eventType, streamActivation, command.OperationID),
		StreamID:       streamActivation,
		IdempotencyKey: productionPrefix + "/" + eventType + "/" + command.OperationID,
		Type:           eventType, SchemaVersion: 1,
		CorrelationID: command.JourneyID, PayloadJSON: body,
	}, nil
}

func buildConfigEvent(kind, path string, command ActivationCommand, now interface{ Format(string) string }) (journal.Event, error) {
	eventType := EventConfigWritten
	if kind == "rolled_back" {
		eventType = EventConfigRolledBack
	}
	payload := map[string]any{
		"path": path, "digest": digestFile(path),
		"authorized_by": command.AuthorizedBy,
		"recorded_at":   now.Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, err
	}
	stream := streamConfigPrefix + strings.TrimPrefix(path, "/")
	return journal.Event{
		ID:             deterministicEventID(productionPrefix, eventType, stream, command.OperationID),
		StreamID:       stream,
		IdempotencyKey: productionPrefix + "/" + eventType + "/" + command.OperationID,
		Type:           eventType, SchemaVersion: 1,
		CorrelationID: command.JourneyID, PayloadJSON: body,
	}, nil
}

func previewDigest(preview ActivationPreview) string {
	files := append([]FileChange(nil), preview.Files...)
	launchd := append([]LaunchdChange(nil), preview.LaunchdItems...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(launchd, func(i, j int) bool { return launchd[i].Label < launchd[j].Label })
	body, _ := json.Marshal(map[string]any{
		"operation": preview.Operation, "target_mode": preview.TargetMode,
		"files": files, "launchd_items": launchd, "admin_lock": preview.AdminLock,
	})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func streamHeads(events []journal.Event) map[string]int64 {
	heads := make(map[string]int64)
	for _, event := range events {
		if event.Seq > heads[event.StreamID] {
			heads[event.StreamID] = event.Seq
		}
	}
	return heads
}

func appendCAS(ctx context.Context, store *journal.Store, events []journal.Event, heads map[string]int64, streamID string, event journal.Event) ([]journal.Event, error) {
	event.Seq = heads[streamID] + 1
	if event.EmittedAt.IsZero() {
		event.EmittedAt = time.Now().UTC()
	}
	for _, existing := range events {
		if existing.IdempotencyKey == event.IdempotencyKey {
			if existing.ID == event.ID && existing.Type == event.Type &&
				bytes.Equal(existing.PayloadJSON, event.PayloadJSON) {
				return []journal.Event{existing}, nil
			}
			return nil, journal.ErrIdempotencyConflict
		}
	}
	return store.AppendBatchIfStreamHeads(ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: heads[streamID]}},
		[]journal.Event{event},
	)
}

func deterministicEventID(prefix, eventType, streamID, operationID string) string {
	sum := sha256.Sum256([]byte(prefix + "\n" + eventType + "\n" + streamID + "\n" + operationID))
	return prefix + "-" + hex.EncodeToString(sum[:])[:32]
}

func eventIDs(events []journal.Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func existingProductionResult(events []journal.Event, operation, operationID string) (CommandResult, bool) {
	eventType := EventActivationActivated
	if operation == "deactivation" {
		eventType = EventActivationDeactivated
	}
	key := productionPrefix + "/" + eventType + "/" + operationID
	for _, event := range events {
		if event.IdempotencyKey == key {
			return CommandResult{
				Operation: operation + "_confirm", Activated: operation == "activation",
				Note: "idempotent production command already completed",
			}, true
		}
	}
	return CommandResult{}, false
}

var _ = errors.Is
