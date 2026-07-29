package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/state"

	_ "modernc.org/sqlite"
)

const (
	minRuntimeObservationInterval = 10 * time.Millisecond
	maxRuntimeObservationInterval = 24 * time.Hour
	maxRuntimeObservationTimeout  = 30 * time.Second
	maxRuntimeObservationCycles   = 100000
)

var (
	ErrInvalidLocalRuntimeObservationDaemon = errors.New(
		"invalid local runtime observation daemon",
	)
	ErrLocalRuntimeObservationDaemonLocked = errors.New(
		"local runtime observation daemon state locked",
	)
	ErrLocalRuntimeObservationDaemonRunning = errors.New(
		"local runtime observation daemon already running",
	)
	ErrLocalRuntimeObservationDaemonClosed = errors.New(
		"local runtime observation daemon closed",
	)
	ErrLocalRuntimeObservationDaemonState = errors.New(
		"local runtime observation daemon state unavailable",
	)
	ErrLocalRuntimeObservationDaemonMetadata = errors.New(
		"local runtime observation daemon metadata invalid",
	)
	ErrLocalRuntimeObservationDaemonCycle = errors.New(
		"local runtime observation daemon cycle failed",
	)
)

type LocalRuntimeObservationDaemonConfig struct {
	StatePath           string
	IsolationRoot       string
	RuntimeSearchPaths  []string
	ProbeID             string
	RuntimeInstanceID   string
	DeviceID            string
	DisplayName         string
	ObservationInterval time.Duration
	ProcessTimeout      time.Duration
	MaxCycles           int
	LocalModelCatalog   *piadapter.PiLocalModelCatalogConfig
}

type RuntimeObservationDaemonClock interface {
	Now() time.Time
	Wait(context.Context, time.Duration) error
}

type RuntimeObservationIdentitySource interface {
	NextRuntimeObservationIdentity(context.Context) (string, error)
}

type LocalRuntimeObservationFact struct {
	RuntimeInstanceID string
	ExecutableVersion string
	Status            string
	ModelIDs          []string
	DiscoverySequence int64
	StatusSequence    int64
}

type LocalRuntimeObservationDaemonResult struct {
	CompletedCycles int
	DiscoveryEvents int
	StatusEvents    int
	NoWriteCycles   int
	RuntimeFacts    []LocalRuntimeObservationFact
}

type LocalRuntimeObservationDaemon struct {
	mu        sync.Mutex
	running   bool
	closed    bool
	config    LocalRuntimeObservationDaemonConfig
	clock     RuntimeObservationDaemonClock
	readModel *projection.Projection
	observer  *PreparedProjectedRuntimeObserver
	db        *sql.DB
	stateFile *os.File
	stateLock *runtimeDaemonStateLock
}

type systemRuntimeObservationDaemonClock struct{}

func NewSystemRuntimeObservationDaemonClock() RuntimeObservationDaemonClock {
	return systemRuntimeObservationDaemonClock{}
}

func (systemRuntimeObservationDaemonClock) Now() time.Time {
	return time.Now().UTC()
}

func (systemRuntimeObservationDaemonClock) Wait(
	ctx context.Context,
	interval time.Duration,
) error {
	timer := time.NewTimer(interval)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type cryptographicRuntimeObservationIdentitySource struct{}

func NewCryptographicRuntimeObservationIdentitySource() RuntimeObservationIdentitySource {
	return cryptographicRuntimeObservationIdentitySource{}
}

func (cryptographicRuntimeObservationIdentitySource) NextRuntimeObservationIdentity(
	ctx context.Context,
) (string, error) {
	if ctx == nil {
		return "", ErrLocalRuntimeObservationDaemonMetadata
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", ErrLocalRuntimeObservationDaemonMetadata
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func NewLocalRuntimeObservationDaemon(
	input LocalRuntimeObservationDaemonConfig,
	clock RuntimeObservationDaemonClock,
	identities RuntimeObservationIdentitySource,
) (_ *LocalRuntimeObservationDaemon, resultErr error) {
	config, err := validateLocalRuntimeObservationDaemonConfig(input)
	if err != nil || nilAppInterface(clock) || nilAppInterface(identities) {
		return nil, ErrInvalidLocalRuntimeObservationDaemon
	}

	factory, err := piadapter.NewPiLocalRuntimeProbeFactory(
		piadapter.PiLocalRuntimeProbeFactoryConfig{
			ProbeID:            config.ProbeID,
			InstanceID:         config.RuntimeInstanceID,
			DeviceID:           config.DeviceID,
			DisplayName:        config.DisplayName,
			IsolationRoot:      config.IsolationRoot,
			RuntimeSearchPaths: config.RuntimeSearchPaths,
			Timeout:            config.ProcessTimeout,
			LocalModelCatalog:  config.LocalModelCatalog,
		},
	)
	if err != nil {
		return nil, ErrInvalidLocalRuntimeObservationDaemon
	}

	lock, err := acquireRuntimeDaemonStateLock(config.StatePath + ".lock")
	if err != nil {
		return nil, err
	}
	var stateFile *os.File
	var db *sql.DB
	defer func() {
		if resultErr == nil {
			return
		}
		if db != nil {
			_ = db.Close()
		}
		if stateFile != nil {
			_ = stateFile.Close()
		}
		_ = lock.Close()
	}()

	stateFile, err = openRuntimeDaemonStateFile(config.StatePath)
	if err != nil {
		return nil, fmt.Errorf("%w: open", ErrLocalRuntimeObservationDaemonState)
	}
	db, err = openRuntimeDaemonSQLite(config.StatePath)
	if err != nil {
		return nil, fmt.Errorf("%w: sqlite", ErrLocalRuntimeObservationDaemonState)
	}
	if err := journal.Migrate(context.Background(), db); err != nil {
		return nil, fmt.Errorf("%w: migrate", ErrLocalRuntimeObservationDaemonState)
	}
	if !sameRuntimeDaemonStateFile(config.StatePath, stateFile) {
		return nil, fmt.Errorf("%w: identity", ErrLocalRuntimeObservationDaemonState)
	}

	readModel := projection.New(db)
	provider := &runtimeObservationCommitInputProvider{
		readModel:  readModel,
		clock:      clock,
		identities: identities,
	}
	store := journal.NewStore(db)
	discoveryCommitter, err := NewPreparedRuntimeDiscoveryCommitter(store, provider)
	if err != nil {
		return nil, ErrInvalidLocalRuntimeObservationDaemon
	}
	statusCommitter, err := NewPreparedRuntimeStatusCommitter(store, provider)
	if err != nil {
		return nil, ErrInvalidLocalRuntimeObservationDaemon
	}
	observer, err := NewPreparedProjectedRuntimeObserver(
		[]discoveryscan.ProbeFactory{factory},
		readModel,
		discoveryCommitter,
		statusCommitter,
	)
	if err != nil {
		return nil, ErrInvalidLocalRuntimeObservationDaemon
	}

	return &LocalRuntimeObservationDaemon{
		config:    config,
		clock:     clock,
		readModel: readModel,
		observer:  observer,
		db:        db,
		stateFile: stateFile,
		stateLock: lock,
	}, nil
}

func (d *LocalRuntimeObservationDaemon) Run(
	ctx context.Context,
) (result LocalRuntimeObservationDaemonResult, resultErr error) {
	if d == nil || ctx == nil {
		return LocalRuntimeObservationDaemonResult{},
			ErrInvalidLocalRuntimeObservationDaemon
	}
	d.mu.Lock()
	switch {
	case d.closed:
		d.mu.Unlock()
		return LocalRuntimeObservationDaemonResult{},
			ErrLocalRuntimeObservationDaemonClosed
	case d.running:
		d.mu.Unlock()
		return LocalRuntimeObservationDaemonResult{},
			ErrLocalRuntimeObservationDaemonRunning
	default:
		d.running = true
		d.mu.Unlock()
	}
	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		return LocalRuntimeObservationDaemonResult{}, err
	}

	trigger := &intervalRuntimeObservationTrigger{
		clock:    d.clock,
		interval: d.config.ObservationInterval,
		first:    true,
	}
	for d.config.MaxCycles == 0 ||
		result.CompletedCycles < d.config.MaxCycles {
		_, plan, discovery, _, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				ctx, trigger, d.observer,
			)
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return cloneLocalRuntimeObservationDaemonResult(result), err
			}
			return cloneLocalRuntimeObservationDaemonResult(result),
				fmt.Errorf("%w: %w", ErrLocalRuntimeObservationDaemonCycle, err)
		}

		result.CompletedCycles++
		switch plan.Kind() {
		case RuntimeObservationWriteNone:
			result.NoWriteCycles++
		case RuntimeObservationWriteDiscovery:
			result.DiscoveryEvents += discovery.EventCount()
		case RuntimeObservationWriteStatus:
			result.StatusEvents += status.EventCount()
		default:
			return cloneLocalRuntimeObservationDaemonResult(result),
				ErrLocalRuntimeObservationDaemonCycle
		}
		result.RuntimeFacts = runtimeObservationFacts(
			d.readModel.Snapshot(),
		)
	}
	return cloneLocalRuntimeObservationDaemonResult(result), nil
}

func (d *LocalRuntimeObservationDaemon) Close() error {
	if d == nil {
		return ErrInvalidLocalRuntimeObservationDaemon
	}
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return ErrLocalRuntimeObservationDaemonRunning
	}
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	db := d.db
	stateFile := d.stateFile
	lock := d.stateLock
	d.db = nil
	d.stateFile = nil
	d.stateLock = nil
	d.mu.Unlock()

	var closeErrors []error
	if db != nil {
		closeErrors = append(closeErrors, db.Close())
	}
	if stateFile != nil {
		closeErrors = append(closeErrors, stateFile.Close())
	}
	if lock != nil {
		closeErrors = append(closeErrors, lock.Close())
	}
	return errors.Join(closeErrors...)
}

type intervalRuntimeObservationTrigger struct {
	mu       sync.Mutex
	clock    RuntimeObservationDaemonClock
	interval time.Duration
	first    bool
}

func (t *intervalRuntimeObservationTrigger) AwaitRuntimeObservation(
	ctx context.Context,
) error {
	if t == nil || ctx == nil || nilAppInterface(t.clock) {
		return ErrInvalidLocalRuntimeObservationDaemon
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if t.first {
		t.first = false
		return nil
	}
	return t.clock.Wait(ctx, t.interval)
}

type runtimeObservationCommitInputProvider struct {
	mu         sync.Mutex
	readModel  *projection.Projection
	clock      RuntimeObservationDaemonClock
	identities RuntimeObservationIdentitySource
}

func (p *runtimeObservationCommitInputProvider) PrepareRuntimeDiscoveryCommit(
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitInput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.valid(ctx); err != nil {
		return state.RuntimeDiscoveryCommitInput{}, err
	}
	observations := snapshot.Observations()
	projected := p.readModel.Snapshot()
	discoveryID, err := p.nextIdentity(ctx)
	if err != nil {
		return state.RuntimeDiscoveryCommitInput{}, err
	}
	emittedAt, err := p.emittedAt()
	if err != nil {
		return state.RuntimeDiscoveryCommitInput{}, err
	}
	events := make([]state.RuntimeDiscoveryEventInput, len(observations))
	seen := map[string]struct{}{discoveryID: {}}
	for index, observation := range observations {
		if err := ctx.Err(); err != nil {
			return state.RuntimeDiscoveryCommitInput{}, err
		}
		current, exists := projected.RuntimeInstances[observation.Instance.ID]
		if exists {
			if current.ID != observation.Instance.ID {
				return state.RuntimeDiscoveryCommitInput{},
					ErrLocalRuntimeObservationDaemonMetadata
			}
		}
		sequence, err := nextRuntimeObservationSequence(current, exists)
		if err != nil {
			return state.RuntimeDiscoveryCommitInput{}, err
		}
		eventID, err := p.nextUniqueIdentity(ctx, seen)
		if err != nil {
			return state.RuntimeDiscoveryCommitInput{}, err
		}
		idempotencyKey, err := p.nextUniqueIdentity(ctx, seen)
		if err != nil {
			return state.RuntimeDiscoveryCommitInput{}, err
		}
		events[index] = state.RuntimeDiscoveryEventInput{
			RuntimeInstanceID: observation.Instance.ID,
			EventID:           eventID,
			IdempotencyKey:    idempotencyKey,
			Seq:               sequence,
		}
	}
	return state.RuntimeDiscoveryCommitInput{
		DiscoveryID: discoveryID,
		EmittedAt:   emittedAt,
		Events:      events,
	}, nil
}

func nextRuntimeObservationSequence(
	current projection.RuntimeInstance,
	exists bool,
) (int64, error) {
	if !exists {
		return 1, nil
	}
	latest := max(current.DiscoverySequence, current.StatusSequence)
	if current.ID == "" || latest <= 0 || latest == math.MaxInt64 {
		return 0, ErrLocalRuntimeObservationDaemonMetadata
	}
	return latest + 1, nil
}

func (p *runtimeObservationCommitInputProvider) PrepareRuntimeStatusCommit(
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitInput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.valid(ctx); err != nil {
		return state.RuntimeStatusCommitInput{}, err
	}
	reconciliationID, err := p.nextIdentity(ctx)
	if err != nil {
		return state.RuntimeStatusCommitInput{}, err
	}
	emittedAt, err := p.emittedAt()
	if err != nil {
		return state.RuntimeStatusCommitInput{}, err
	}
	transitions := candidate.Transitions()
	events := make([]state.RuntimeStatusEventInput, len(transitions))
	seen := map[string]struct{}{reconciliationID: {}}
	for index, transition := range transitions {
		if err := ctx.Err(); err != nil {
			return state.RuntimeStatusCommitInput{}, err
		}
		if transition.PreviousSequence <= 0 ||
			transition.PreviousSequence == math.MaxInt64 {
			return state.RuntimeStatusCommitInput{},
				ErrLocalRuntimeObservationDaemonMetadata
		}
		eventID, err := p.nextUniqueIdentity(ctx, seen)
		if err != nil {
			return state.RuntimeStatusCommitInput{}, err
		}
		idempotencyKey, err := p.nextUniqueIdentity(ctx, seen)
		if err != nil {
			return state.RuntimeStatusCommitInput{}, err
		}
		events[index] = state.RuntimeStatusEventInput{
			RuntimeInstanceID: transition.RuntimeInstanceID,
			EventID:           eventID,
			IdempotencyKey:    idempotencyKey,
			Seq:               transition.PreviousSequence + 1,
		}
	}
	return state.RuntimeStatusCommitInput{
		ReconciliationID: reconciliationID,
		EmittedAt:        emittedAt,
		Events:           events,
	}, nil
}

func (p *runtimeObservationCommitInputProvider) valid(ctx context.Context) error {
	if p == nil ||
		ctx == nil ||
		p.readModel == nil ||
		nilAppInterface(p.clock) ||
		nilAppInterface(p.identities) {
		return ErrLocalRuntimeObservationDaemonMetadata
	}
	return ctx.Err()
}

func (p *runtimeObservationCommitInputProvider) emittedAt() (time.Time, error) {
	value := p.clock.Now()
	if value.IsZero() || value.Location() != time.UTC {
		return time.Time{}, ErrLocalRuntimeObservationDaemonMetadata
	}
	return value, nil
}

func (p *runtimeObservationCommitInputProvider) nextIdentity(
	ctx context.Context,
) (string, error) {
	value, err := p.identities.NextRuntimeObservationIdentity(ctx)
	if err != nil {
		return "", err
	}
	if !validRuntimeObservationIdentity(value) {
		return "", ErrLocalRuntimeObservationDaemonMetadata
	}
	return value, nil
}

func (p *runtimeObservationCommitInputProvider) nextUniqueIdentity(
	ctx context.Context,
	seen map[string]struct{},
) (string, error) {
	value, err := p.nextIdentity(ctx)
	if err != nil {
		return "", err
	}
	if _, exists := seen[value]; exists {
		return "", ErrLocalRuntimeObservationDaemonMetadata
	}
	seen[value] = struct{}{}
	return value, nil
}

func validRuntimeObservationIdentity(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func validateLocalRuntimeObservationDaemonConfig(
	input LocalRuntimeObservationDaemonConfig,
) (LocalRuntimeObservationDaemonConfig, error) {
	if input.ProbeID == "" ||
		input.RuntimeInstanceID == "" ||
		input.DeviceID == "" ||
		input.DisplayName == "" ||
		input.ObservationInterval < minRuntimeObservationInterval ||
		input.ObservationInterval > maxRuntimeObservationInterval ||
		input.ProcessTimeout <= 0 ||
		input.ProcessTimeout > maxRuntimeObservationTimeout ||
		input.MaxCycles < 0 ||
		input.MaxCycles > maxRuntimeObservationCycles ||
		!validRuntimeDaemonPath(input.StatePath) ||
		!validRuntimeDaemonPath(input.IsolationRoot) ||
		len(input.RuntimeSearchPaths) == 0 {
		return LocalRuntimeObservationDaemonConfig{},
			ErrInvalidLocalRuntimeObservationDaemon
	}
	parent := filepath.Dir(input.StatePath)
	if !validPrivateRuntimeDaemonDirectory(parent) ||
		!validPrivateRuntimeDaemonDirectory(input.IsolationRoot) {
		return LocalRuntimeObservationDaemonConfig{},
			ErrInvalidLocalRuntimeObservationDaemon
	}
	if err := validateExistingRuntimeDaemonState(input.StatePath); err != nil {
		return LocalRuntimeObservationDaemonConfig{}, err
	}

	searchPaths := make([]string, 0, len(input.RuntimeSearchPaths))
	seen := make(map[string]struct{}, len(input.RuntimeSearchPaths))
	for _, path := range input.RuntimeSearchPaths {
		if !validRuntimeDaemonPath(path) {
			return LocalRuntimeObservationDaemonConfig{},
				ErrInvalidLocalRuntimeObservationDaemon
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !filepath.IsAbs(resolved) {
			return LocalRuntimeObservationDaemonConfig{},
				ErrInvalidLocalRuntimeObservationDaemon
		}
		info, err := os.Lstat(resolved)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return LocalRuntimeObservationDaemonConfig{},
				ErrInvalidLocalRuntimeObservationDaemon
		}
		if _, duplicate := seen[resolved]; duplicate {
			return LocalRuntimeObservationDaemonConfig{},
				ErrInvalidLocalRuntimeObservationDaemon
		}
		seen[resolved] = struct{}{}
		searchPaths = append(searchPaths, resolved)
	}
	input.RuntimeSearchPaths = searchPaths
	if input.LocalModelCatalog != nil {
		copied := *input.LocalModelCatalog
		input.LocalModelCatalog = &copied
	}
	return input, nil
}

func validRuntimeDaemonPath(path string) bool {
	return path != "" &&
		!strings.ContainsRune(path, '\x00') &&
		filepath.IsAbs(path) &&
		filepath.Clean(path) == path
}

func validPrivateRuntimeDaemonDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil &&
		info.IsDir() &&
		info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().Perm() == 0o700
}

func validateExistingRuntimeDaemonState(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		return ErrInvalidLocalRuntimeObservationDaemon
	}
	return nil
}

func openRuntimeDaemonSQLite(path string) (*sql.DB, error) {
	values := url.Values{}
	values.Add("mode", "rwc")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "foreign_keys(1)")
	uri := url.URL{Scheme: "file", Path: path}
	uri.RawQuery = values.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sameRuntimeDaemonStateFile(path string, opened *os.File) bool {
	openedInfo, openedErr := opened.Stat()
	pathInfo, pathErr := os.Lstat(path)
	return openedErr == nil &&
		pathErr == nil &&
		pathInfo.Mode().IsRegular() &&
		pathInfo.Mode()&os.ModeSymlink == 0 &&
		pathInfo.Mode().Perm() == 0o600 &&
		os.SameFile(openedInfo, pathInfo)
}

func runtimeObservationFacts(
	snapshot projection.Snapshot,
) []LocalRuntimeObservationFact {
	facts := make([]LocalRuntimeObservationFact, 0, len(snapshot.RuntimeInstances))
	for _, instance := range snapshot.RuntimeInstances {
		facts = append(facts, LocalRuntimeObservationFact{
			RuntimeInstanceID: instance.ID,
			ExecutableVersion: instance.ExecutableVersion,
			Status:            instance.Status,
			ModelIDs:          append([]string(nil), instance.ModelIDs...),
			DiscoverySequence: instance.DiscoverySequence,
			StatusSequence:    instance.StatusSequence,
		})
	}
	sort.Slice(facts, func(i, j int) bool {
		return facts[i].RuntimeInstanceID < facts[j].RuntimeInstanceID
	})
	return facts
}

func cloneLocalRuntimeObservationDaemonResult(
	input LocalRuntimeObservationDaemonResult,
) LocalRuntimeObservationDaemonResult {
	result := input
	result.RuntimeFacts = make([]LocalRuntimeObservationFact, len(input.RuntimeFacts))
	for index, fact := range input.RuntimeFacts {
		fact.ModelIDs = append([]string(nil), fact.ModelIDs...)
		result.RuntimeFacts[index] = fact
	}
	return result
}
