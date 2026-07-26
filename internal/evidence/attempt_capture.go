//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	"golang.org/x/sys/unix"
)

const (
	maxAttemptCaptureBytes = 1 << 20
	maxAttemptStateBytes   = 2 << 20
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

type AttemptReceipt struct {
	evidenceID string
	digest     string
}

type AttemptCaptureState struct {
	binding        AttemptCaptureInput
	frameCount     int
	resultObserved bool
}

type attemptCaptureDisk struct {
	Binding         AttemptCaptureInput `json:"binding"`
	Frames          []string            `json:"frames"`
	Terminal        *AttemptTerminal    `json:"terminal"`
	FinalizedDigest string              `json:"finalized_digest"`
}

type attemptReceiptDisk struct {
	EvidenceID string `json:"evidence_id"`
	Digest     string `json:"digest"`
}

type attemptArtifact struct {
	EvidenceID          string   `json:"evidence_id"`
	TeamInstanceID      string   `json:"team_instance_id"`
	PlanDigest          string   `json:"plan_digest"`
	LogicalNodeID       string   `json:"logical_node_id"`
	AttemptNumber       int      `json:"attempt_number"`
	WorkItemID          string   `json:"work_item_id"`
	RunID               string   `json:"run_id"`
	ClaimID             string   `json:"claim_id"`
	ClaimGeneration     int64    `json:"claim_generation"`
	RuntimeInstanceID   string   `json:"runtime_instance_id"`
	AgentInstanceID     string   `json:"agent_instance_id"`
	AuthorizedFrames    []string `json:"authorized_frames"`
	Status              string   `json:"status"`
	Reason              string   `json:"reason"`
	ChildResultObserved bool     `json:"child_result_observed"`
}

type attemptStateDirs struct {
	attempts int
	captures int
	receipts int
	staging  int
}

func (receipt AttemptReceipt) EvidenceID() string { return receipt.evidenceID }
func (receipt AttemptReceipt) Digest() string     { return receipt.digest }
func (state AttemptCaptureState) Binding() AttemptCaptureInput {
	return state.binding
}
func (state AttemptCaptureState) FrameCount() int      { return state.frameCount }
func (state AttemptCaptureState) ResultObserved() bool { return state.resultObserved }

func (store *Store) BeginAttemptCapture(
	ctx context.Context,
	input AttemptCaptureInput,
) error {
	if err := validateAttemptContext(ctx); err != nil {
		return err
	}
	if !validAttemptCaptureInput(input) {
		return ErrInvalidAttemptCapture
	}
	return store.withAttemptLock(input.EvidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		existing, found, err := readAttemptCaptureAt(
			dirs.captures,
			input.EvidenceID,
		)
		if err != nil {
			return err
		}
		if found {
			if existing.Binding == input {
				return nil
			}
			return ErrAttemptCaptureConflict
		}
		state := attemptCaptureDisk{
			Binding: input,
			Frames:  []string{},
		}
		return store.writeAttemptJSON(
			dirs,
			dirs.captures,
			input.EvidenceID,
			state,
		)
	})
}

func (store *Store) AppendAttemptFrame(
	ctx context.Context,
	evidenceID string,
	line []byte,
) error {
	if err := validateAttemptContext(ctx); err != nil {
		return err
	}
	if !validAttemptID(evidenceID) ||
		len(line) == 0 ||
		len(line) > bridgev1.MaxLineBytes ||
		bytes.Contains(line, []byte("loom_grant_v1.")) {
		return ErrInvalidAttemptCapture
	}
	frame, err := bridgev1.DecodeLine(line)
	if err != nil || !validCapturedFrameType(frame.Type()) {
		return ErrInvalidAttemptCapture
	}
	canonical, err := bridgev1.EncodeLine(frame)
	if err != nil || !bytes.Equal(canonical, line) {
		return ErrInvalidAttemptCapture
	}
	return store.withAttemptLock(evidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		state, found, err := readAttemptCaptureAt(dirs.captures, evidenceID)
		if err != nil {
			return err
		}
		if !found || state.Terminal != nil || state.FinalizedDigest != "" {
			return ErrAttemptCaptureConflict
		}
		if !frameMatchesAttempt(frame, state.Binding) {
			return ErrAttemptCaptureConflict
		}
		for _, existingLine := range state.Frames {
			existing, decodeErr := bridgev1.DecodeLine([]byte(existingLine))
			if decodeErr != nil {
				return ErrAttemptCaptureConflict
			}
			if existing.Sequence() != frame.Sequence() {
				continue
			}
			if existingLine == string(canonical) {
				return nil
			}
			return ErrAttemptCaptureConflict
		}
		stream, err := capturedBoundStream(state)
		if err != nil {
			return ErrAttemptCaptureConflict
		}
		if _, err := bridgev1.AdvanceBoundRunStream(stream, frame); err != nil {
			return ErrAttemptCaptureConflict
		}
		if len(state.Frames) >= bridgev1.MaxBufferedFrames ||
			capturedFrameBytes(state.Frames)+len(canonical) > maxAttemptCaptureBytes {
			return ErrInvalidAttemptCapture
		}
		state.Frames = append(state.Frames, string(canonical))
		return store.writeAttemptJSON(
			dirs,
			dirs.captures,
			evidenceID,
			state,
		)
	})
}

func (store *Store) RebindAttemptCapture(
	ctx context.Context,
	input AttemptCaptureInput,
) error {
	if err := validateAttemptContext(ctx); err != nil {
		return err
	}
	if !validAttemptCaptureInput(input) {
		return ErrInvalidAttemptCapture
	}
	return store.withAttemptLock(input.EvidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		state, found, err := readAttemptCaptureAt(
			dirs.captures,
			input.EvidenceID,
		)
		if err != nil {
			return err
		}
		if !found || state.Terminal != nil || state.FinalizedDigest != "" {
			return ErrAttemptCaptureConflict
		}
		if state.Binding == input {
			return nil
		}
		if len(state.Frames) != 0 ||
			!sameAttemptIdentity(state.Binding, input) ||
			input.ClaimGeneration != state.Binding.ClaimGeneration+1 ||
			input.ClaimID == state.Binding.ClaimID {
			return ErrAttemptCaptureConflict
		}
		state.Binding = input
		return store.writeAttemptJSON(
			dirs,
			dirs.captures,
			input.EvidenceID,
			state,
		)
	})
}

func (store *Store) FinalizeAttemptCapture(
	ctx context.Context,
	evidenceID string,
	terminal AttemptTerminal,
) (AttemptReceipt, error) {
	if err := validateAttemptContext(ctx); err != nil {
		return AttemptReceipt{}, err
	}
	if !validAttemptID(evidenceID) || !validAttemptTerminal(terminal) {
		return AttemptReceipt{}, ErrInvalidAttemptCapture
	}
	var pending attemptCaptureDisk
	var artifactBytes []byte
	var digest string
	var result AttemptReceipt
	completed := false
	err := store.withAttemptLock(evidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		state, found, err := readAttemptCaptureAt(dirs.captures, evidenceID)
		if err != nil {
			return err
		}
		if !found {
			return ErrAttemptCaptureIncomplete
		}
		if state.Terminal != nil {
			if *state.Terminal != terminal || state.FinalizedDigest == "" {
				return ErrAttemptCaptureConflict
			}
			receipt, found, receiptErr := readAttemptReceiptAt(
				dirs.receipts,
				evidenceID,
			)
			if receiptErr != nil {
				return receiptErr
			}
			if found && receipt.Digest != state.FinalizedDigest {
				return ErrAttemptCaptureConflict
			}
			if !found {
				if err := store.writeAttemptJSON(
					dirs,
					dirs.receipts,
					evidenceID,
					attemptReceiptDisk{
						EvidenceID: evidenceID,
						Digest:     state.FinalizedDigest,
					},
				); err != nil {
					return err
				}
			}
			result = AttemptReceipt{
				evidenceID: evidenceID,
				digest:     state.FinalizedDigest,
			}
			completed = true
			return nil
		}
		childResultObserved, reconcileErr := reconcileAttemptTerminal(
			state,
			terminal,
		)
		if reconcileErr != nil {
			return reconcileErr
		}
		artifactBytes, digest, err = canonicalAttemptArtifact(
			state,
			terminal,
			childResultObserved,
		)
		if err != nil {
			return err
		}
		pending = state
		return nil
	})
	if err != nil || completed {
		return result, err
	}
	if _, err := store.Publish(
		ctx,
		bytes.NewReader(artifactBytes),
		digest,
	); err != nil {
		return AttemptReceipt{}, err
	}
	err = store.withAttemptLock(evidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		state, found, err := readAttemptCaptureAt(
			dirs.captures,
			evidenceID,
		)
		if err != nil {
			return err
		}
		if !found {
			return ErrAttemptCaptureConflict
		}
		if state.Terminal != nil {
			if *state.Terminal != terminal ||
				state.FinalizedDigest != digest {
				return ErrAttemptCaptureConflict
			}
		} else {
			if !samePendingAttemptCapture(state, pending) {
				return ErrAttemptCaptureConflict
			}
			state.Terminal = &AttemptTerminal{
				Status: terminal.Status,
				Reason: terminal.Reason,
			}
			state.FinalizedDigest = digest
			if err := store.writeAttemptJSON(
				dirs,
				dirs.captures,
				evidenceID,
				state,
			); err != nil {
				return err
			}
		}
		receipt, found, err := readAttemptReceiptAt(
			dirs.receipts,
			evidenceID,
		)
		if err != nil {
			return err
		}
		if found && receipt.Digest != digest {
			return ErrAttemptCaptureConflict
		}
		if !found {
			if err := store.writeAttemptJSON(
				dirs,
				dirs.receipts,
				evidenceID,
				attemptReceiptDisk{
					EvidenceID: evidenceID,
					Digest:     digest,
				},
			); err != nil {
				return err
			}
		}
		result = AttemptReceipt{evidenceID: evidenceID, digest: digest}
		return nil
	})
	return result, err
}

func (store *Store) AttemptReceipt(
	ctx context.Context,
	evidenceID string,
) (AttemptReceipt, bool, error) {
	if err := validateAttemptContext(ctx); err != nil {
		return AttemptReceipt{}, false, err
	}
	if !validAttemptID(evidenceID) {
		return AttemptReceipt{}, false, ErrInvalidAttemptCapture
	}
	var result AttemptReceipt
	var found bool
	err := store.withAttemptLock(evidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		receipt, exists, err := readAttemptReceiptAt(
			dirs.receipts,
			evidenceID,
		)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		result = AttemptReceipt{
			evidenceID: receipt.EvidenceID,
			digest:     receipt.Digest,
		}
		found = true
		return nil
	})
	return result, found, err
}

func (store *Store) AttemptCapture(
	ctx context.Context,
	evidenceID string,
) (AttemptCaptureState, bool, error) {
	if err := validateAttemptContext(ctx); err != nil {
		return AttemptCaptureState{}, false, err
	}
	if !validAttemptID(evidenceID) {
		return AttemptCaptureState{}, false, ErrInvalidAttemptCapture
	}
	var result AttemptCaptureState
	var found bool
	err := store.withAttemptLock(evidenceID, func() error {
		dirs, err := store.openAttemptStateDirs()
		if err != nil {
			return err
		}
		defer dirs.close()
		state, exists, err := readAttemptCaptureAt(
			dirs.captures,
			evidenceID,
		)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		result = AttemptCaptureState{
			binding:        state.Binding,
			frameCount:     len(state.Frames),
			resultObserved: attemptResultObserved(state),
		}
		found = true
		return nil
	})
	return result, found, err
}

func (store *Store) withAttemptLock(
	evidenceID string,
	operation func() error,
) error {
	if store == nil || operation == nil {
		return ErrInvalidAttemptCapture
	}
	store.lifecycle.RLock()
	defer store.lifecycle.RUnlock()
	store.mu.Lock()
	if store.closed {
		store.mu.Unlock()
		return ErrStoreClosed
	}
	key := "attempt:" + evidenceID
	lock := store.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		store.locks[key] = lock
	}
	store.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	if err := store.revalidateRoot(); err != nil {
		return err
	}
	return operation()
}

func (store *Store) openAttemptStateDirs() (attemptStateDirs, error) {
	var dirs attemptStateDirs
	var err error
	dirs.attempts, err = ensureDirAt(store.rootFD, "attempts")
	if err != nil {
		return attemptStateDirs{}, err
	}
	dirs.captures, err = ensureDirAt(dirs.attempts, "captures")
	if err != nil {
		dirs.close()
		return attemptStateDirs{}, err
	}
	dirs.receipts, err = ensureDirAt(dirs.attempts, "receipts")
	if err != nil {
		dirs.close()
		return attemptStateDirs{}, err
	}
	dirs.staging, err = ensureDirAt(dirs.attempts, "staging")
	if err != nil {
		dirs.close()
		return attemptStateDirs{}, err
	}
	return dirs, nil
}

func (dirs *attemptStateDirs) close() {
	for _, fd := range []int{dirs.staging, dirs.receipts, dirs.captures, dirs.attempts} {
		if fd > 0 {
			_ = unix.Close(fd)
		}
	}
	dirs.attempts, dirs.captures, dirs.receipts, dirs.staging = 0, 0, 0, 0
}

func (store *Store) writeAttemptJSON(
	dirs attemptStateDirs,
	targetFD int,
	targetName string,
	value any,
) error {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || len(encoded) > maxAttemptStateBytes {
		return ErrInvalidAttemptCapture
	}
	tempName, file, err := createAttemptStagingFile(dirs.staging)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = unix.Unlinkat(dirs.staging, tempName, 0)
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := store.revalidateRoot(); err != nil {
		return err
	}
	if err := unix.Renameat(
		dirs.staging,
		tempName,
		targetFD,
		targetName,
	); err != nil {
		return err
	}
	cleanup = false
	return unix.Fsync(targetFD)
}

func createAttemptStagingFile(dirFD int) (string, *os.File, error) {
	for attempt := 0; attempt < 16; attempt++ {
		var material [16]byte
		if _, err := io.ReadFull(rand.Reader, material[:]); err != nil {
			return "", nil, err
		}
		name := "attempt-" + hex.EncodeToString(material[:])
		fd, err := unix.Openat(
			dirFD,
			name,
			unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC,
			uint32(privateFileMode),
		)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if err := unix.Fchmod(fd, uint32(privateFileMode)); err != nil {
			_ = unix.Close(fd)
			_ = unix.Unlinkat(dirFD, name, 0)
			return "", nil, err
		}
		return name, os.NewFile(uintptr(fd), name), nil
	}
	return "", nil, ErrAttemptCaptureConflict
}

func readAttemptCaptureAt(
	dirFD int,
	evidenceID string,
) (attemptCaptureDisk, bool, error) {
	var state attemptCaptureDisk
	found, err := readAttemptJSONAt(dirFD, evidenceID, &state)
	if err != nil || !found {
		return attemptCaptureDisk{}, found, err
	}
	if !validAttemptCaptureInput(state.Binding) ||
		state.Binding.EvidenceID != evidenceID ||
		state.Frames == nil ||
		len(state.Frames) > bridgev1.MaxBufferedFrames ||
		capturedFrameBytes(state.Frames) > maxAttemptCaptureBytes {
		return attemptCaptureDisk{}, false, ErrAttemptCaptureConflict
	}
	if _, err := capturedBoundStream(state); err != nil {
		return attemptCaptureDisk{}, false, ErrAttemptCaptureConflict
	}
	if state.Terminal == nil && state.FinalizedDigest != "" ||
		state.Terminal != nil &&
			(!validAttemptTerminal(*state.Terminal) ||
				validateDigest(state.FinalizedDigest) != nil) {
		return attemptCaptureDisk{}, false, ErrAttemptCaptureConflict
	}
	return state, true, nil
}

func readAttemptReceiptAt(
	dirFD int,
	evidenceID string,
) (attemptReceiptDisk, bool, error) {
	var receipt attemptReceiptDisk
	found, err := readAttemptJSONAt(dirFD, evidenceID, &receipt)
	if err != nil || !found {
		return attemptReceiptDisk{}, found, err
	}
	if receipt.EvidenceID != evidenceID ||
		validateDigest(receipt.Digest) != nil {
		return attemptReceiptDisk{}, false, ErrAttemptCaptureConflict
	}
	return receipt, true, nil
}

func readAttemptJSONAt(dirFD int, name string, target any) (bool, error) {
	fd, err := unix.Openat(
		dirFD,
		name,
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if errors.Is(err, unix.ELOOP) {
		return false, ErrStateSymlink
	}
	if err != nil {
		return false, err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	info, err := file.Stat()
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm() != privateFileMode ||
		info.Size() <= 0 ||
		info.Size() > maxAttemptStateBytes {
		return false, ErrAttemptCaptureConflict
	}
	content, err := io.ReadAll(io.LimitReader(file, maxAttemptStateBytes+1))
	if err != nil || int64(len(content)) != info.Size() ||
		hasDuplicateAttemptJSONKeys(content) {
		return false, ErrAttemptCaptureConflict
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false, ErrAttemptCaptureConflict
	}
	if err := requireAttemptJSONEOF(decoder); err != nil {
		return false, ErrAttemptCaptureConflict
	}
	return true, nil
}

func validateAttemptContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidAttemptCapture
	}
	return ctx.Err()
}

func validAttemptCaptureInput(input AttemptCaptureInput) bool {
	return validAttemptID(input.EvidenceID) &&
		validAttemptID(input.TeamInstanceID) &&
		validateDigest(input.PlanDigest) == nil &&
		validAttemptID(input.LogicalNodeID) &&
		input.AttemptNumber >= 1 && input.AttemptNumber <= 3 &&
		validAttemptID(input.WorkItemID) &&
		validAttemptID(input.RunID) &&
		validAttemptUUID(input.ClaimID) &&
		input.ClaimGeneration > 0 &&
		validAttemptID(input.RuntimeInstanceID) &&
		validAttemptID(input.AgentInstanceID)
}

func validAttemptID(value string) bool {
	if value == "" || len(value) > 128 ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value ||
		strings.ContainsAny(value, `/\`) {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current == 0x7f {
			return false
		}
	}
	return value != "." && value != ".."
}

func validAttemptUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' ||
		value[14] != '4' ||
		!strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, current := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", current) {
			return false
		}
	}
	return true
}

func validAttemptTerminal(terminal AttemptTerminal) bool {
	switch terminal.Status {
	case "succeeded":
		return terminal.Reason == ""
	case "failed", "cancelled":
		return validAttemptID(terminal.Reason)
	default:
		return false
	}
}

func validCapturedFrameType(messageType bridgev1.MessageType) bool {
	switch messageType {
	case bridgev1.MessageAck,
		bridgev1.MessageEvent,
		bridgev1.MessageEvidence,
		bridgev1.MessageHeartbeat,
		bridgev1.MessageResult:
		return true
	default:
		return false
	}
}

func frameMatchesAttempt(
	frame bridgev1.Frame,
	input AttemptCaptureInput,
) bool {
	return frame.WorkItemID() == input.WorkItemID &&
		frame.RunID() == input.RunID &&
		frame.ClaimGeneration() == input.ClaimGeneration &&
		frame.RuntimeInstanceID() == input.RuntimeInstanceID &&
		frame.SenderAgentInstanceID() == input.AgentInstanceID
}

func sameAttemptIdentity(
	left AttemptCaptureInput,
	right AttemptCaptureInput,
) bool {
	left.ClaimID, right.ClaimID = "", ""
	left.ClaimGeneration, right.ClaimGeneration = 0, 0
	return left == right
}

func samePendingAttemptCapture(
	left attemptCaptureDisk,
	right attemptCaptureDisk,
) bool {
	if left.Binding != right.Binding ||
		left.Terminal != nil ||
		right.Terminal != nil ||
		left.FinalizedDigest != "" ||
		right.FinalizedDigest != "" ||
		len(left.Frames) != len(right.Frames) {
		return false
	}
	for index := range left.Frames {
		if left.Frames[index] != right.Frames[index] {
			return false
		}
	}
	return true
}

func capturedBoundStream(
	state attemptCaptureDisk,
) (bridgev1.BoundRunStream, error) {
	stream, err := bridgev1.NewBoundRunStream(bridgev1.RunStreamBinding{
		WorkItemID:            state.Binding.WorkItemID,
		RunID:                 state.Binding.RunID,
		ClaimGeneration:       state.Binding.ClaimGeneration,
		RuntimeInstanceID:     state.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: state.Binding.AgentInstanceID,
	})
	if err != nil {
		return bridgev1.BoundRunStream{}, err
	}
	for _, line := range state.Frames {
		frame, err := bridgev1.DecodeLine([]byte(line))
		if err != nil || !frameMatchesAttempt(frame, state.Binding) ||
			!validCapturedFrameType(frame.Type()) ||
			bytes.Contains([]byte(line), []byte("loom_grant_v1.")) {
			return bridgev1.BoundRunStream{}, ErrAttemptCaptureConflict
		}
		stream, err = bridgev1.AdvanceBoundRunStream(stream, frame)
		if err != nil {
			return bridgev1.BoundRunStream{}, err
		}
	}
	return stream, nil
}

func capturedFrameBytes(frames []string) int {
	total := 0
	for _, line := range frames {
		total += len(line)
	}
	return total
}

func attemptResultObserved(state attemptCaptureDisk) bool {
	if len(state.Frames) == 0 {
		return false
	}
	frame, err := bridgev1.DecodeLine([]byte(state.Frames[len(state.Frames)-1]))
	return err == nil && frame.Type() == bridgev1.MessageResult
}

func reconcileAttemptTerminal(
	state attemptCaptureDisk,
	terminal AttemptTerminal,
) (bool, error) {
	resultObserved := attemptResultObserved(state)
	if !resultObserved {
		if terminal.Status == "succeeded" {
			return false, ErrAttemptCaptureIncomplete
		}
		return false, nil
	}
	frame, err := bridgev1.DecodeLine([]byte(state.Frames[len(state.Frames)-1]))
	if err != nil {
		return false, ErrAttemptCaptureConflict
	}
	var payload struct {
		Status *string `json:"status"`
		Reason *string `json:"reason"`
	}
	if hasDuplicateAttemptJSONKeys(frame.Payload()) {
		return false, ErrAttemptCaptureConflict
	}
	decoder := json.NewDecoder(bytes.NewReader(frame.Payload()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil ||
		requireAttemptJSONEOF(decoder) != nil ||
		payload.Status == nil ||
		payload.Reason == nil ||
		*payload.Status != terminal.Status ||
		*payload.Reason != terminal.Reason {
		return false, ErrAttemptCaptureConflict
	}
	return true, nil
}

func canonicalAttemptArtifact(
	state attemptCaptureDisk,
	terminal AttemptTerminal,
	childResultObserved bool,
) ([]byte, string, error) {
	artifact := attemptArtifact{
		EvidenceID:          state.Binding.EvidenceID,
		TeamInstanceID:      state.Binding.TeamInstanceID,
		PlanDigest:          state.Binding.PlanDigest,
		LogicalNodeID:       state.Binding.LogicalNodeID,
		AttemptNumber:       state.Binding.AttemptNumber,
		WorkItemID:          state.Binding.WorkItemID,
		RunID:               state.Binding.RunID,
		ClaimID:             state.Binding.ClaimID,
		ClaimGeneration:     state.Binding.ClaimGeneration,
		RuntimeInstanceID:   state.Binding.RuntimeInstanceID,
		AgentInstanceID:     state.Binding.AgentInstanceID,
		AuthorizedFrames:    append([]string(nil), state.Frames...),
		Status:              terminal.Status,
		Reason:              terminal.Reason,
		ChildResultObserved: childResultObserved,
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return nil, "", ErrInvalidAttemptCapture
	}
	sum := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(sum[:]), nil
}

func requireAttemptJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrAttemptCaptureConflict
	}
	return nil
}

func hasDuplicateAttemptJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return scanDuplicateAttemptJSONValue(decoder)
}

func scanDuplicateAttemptJSONValue(decoder *json.Decoder) bool {
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
			if scanDuplicateAttemptJSONValue(decoder) {
				return true
			}
		}
	case '[':
		for decoder.More() {
			if scanDuplicateAttemptJSONValue(decoder) {
				return true
			}
		}
	default:
		return true
	}
	_, err = decoder.Token()
	return err != nil
}
