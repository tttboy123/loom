package vault

import (
	"context"
	"errors"
	"sync"
	"time"
)

const maximumCredentialLeaseTTL = 5 * time.Minute

var (
	ErrCredentialLeaseClosed  = errors.New("credential lease closed")
	ErrCredentialLeaseExpired = errors.New("credential lease expired")
	ErrCredentialLeaseRevoked = errors.New("credential lease revoked")
)

type CredentialReader interface {
	ReadCredential(context.Context, CredentialIdentity) ([]byte, error)
}

type CredentialLeaseManager struct {
	mu         sync.Mutex
	reader     CredentialReader
	sequence   uint64
	generation uint64
	leases     map[uint64]*CredentialLease
	revoked    map[CredentialIdentity]struct{}
	rotating   bool
	closed     bool
}

type credentialLeaseState uint8

const (
	leaseActive credentialLeaseState = iota
	leaseClosed
	leaseExpired
	leaseRevoked
)

type CredentialLease struct {
	mu       sync.Mutex
	manager  *CredentialLeaseManager
	id       uint64
	identity CredentialIdentity
	secret   []byte
	state    credentialLeaseState
	context  context.Context
	cancel   context.CancelFunc
	parent   <-chan struct{}
	timer    *time.Timer
	done     chan struct{}
}

func NewCredentialLeaseManager(
	reader CredentialReader,
) (*CredentialLeaseManager, error) {
	if reader == nil {
		return nil, ErrInvalidVaultInput
	}
	return &CredentialLeaseManager{
		reader:  reader,
		leases:  make(map[uint64]*CredentialLease),
		revoked: make(map[CredentialIdentity]struct{}),
	}, nil
}

func (manager *CredentialLeaseManager) Acquire(
	ctx context.Context,
	identity CredentialIdentity,
	ttl time.Duration,
) (*CredentialLease, error) {
	if manager == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialIdentity(identity) || ttl <= 0 ||
		ttl > maximumCredentialLeaseTTL {
		return nil, ErrInvalidVaultInput
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil, ErrCredentialLeaseClosed
	}
	if manager.rotating {
		manager.mu.Unlock()
		return nil, ErrCredentialLeaseRevoked
	}
	if _, revoked := manager.revoked[identity]; revoked {
		manager.mu.Unlock()
		return nil, ErrCredentialLeaseRevoked
	}
	generation := manager.generation
	manager.mu.Unlock()
	secret, err := manager.reader.ReadCredential(ctx, identity)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		clearBytes(secret)
		return nil, err
	}
	if len(secret) == 0 || len(secret) > MaximumSecretBytes {
		clearBytes(secret)
		return nil, ErrVaultAuthentication
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		clearBytes(secret)
		return nil, ErrCredentialLeaseClosed
	}
	if manager.rotating || manager.generation != generation {
		manager.mu.Unlock()
		clearBytes(secret)
		return nil, ErrCredentialLeaseRevoked
	}
	if _, revoked := manager.revoked[identity]; revoked {
		manager.mu.Unlock()
		clearBytes(secret)
		return nil, ErrCredentialLeaseRevoked
	}
	manager.sequence++
	leaseContext, cancel := context.WithCancel(ctx)
	lease := &CredentialLease{
		manager: manager, id: manager.sequence, identity: identity,
		secret: secret, state: leaseActive, timer: time.NewTimer(ttl),
		context: leaseContext, cancel: cancel, parent: ctx.Done(),
		done: make(chan struct{}),
	}
	manager.leases[lease.id] = lease
	manager.mu.Unlock()
	go lease.waitForExpiry()
	return lease, nil
}

// WithRotationBarrier invalidates all current and in-flight leases before
// running rotate, then permits fresh acquisitions after rotate returns.
func (manager *CredentialLeaseManager) WithRotationBarrier(
	ctx context.Context,
	rotate func() error,
) error {
	if manager == nil || ctx == nil || ctx.Err() != nil || rotate == nil {
		return ErrInvalidVaultInput
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return ErrCredentialLeaseClosed
	}
	if manager.rotating {
		manager.mu.Unlock()
		return ErrCredentialLeaseRevoked
	}
	manager.rotating = true
	manager.generation++
	leasing := make([]*CredentialLease, 0, len(manager.leases))
	for id, lease := range manager.leases {
		delete(manager.leases, id)
		leasing = append(leasing, lease)
	}
	manager.mu.Unlock()
	for _, lease := range leasing {
		lease.terminate(leaseRevoked)
	}
	defer func() {
		manager.mu.Lock()
		manager.rotating = false
		manager.mu.Unlock()
	}()
	return rotate()
}

func (manager *CredentialLeaseManager) Revoke(
	identity CredentialIdentity,
) int {
	if manager == nil || !validCredentialIdentity(identity) {
		return 0
	}
	manager.mu.Lock()
	manager.revoked[identity] = struct{}{}
	leasing := make([]*CredentialLease, 0)
	for id, lease := range manager.leases {
		if lease.identity == identity {
			delete(manager.leases, id)
			leasing = append(leasing, lease)
		}
	}
	manager.mu.Unlock()
	for _, lease := range leasing {
		lease.terminate(leaseRevoked)
	}
	return len(leasing)
}

func (manager *CredentialLeaseManager) Close() error {
	if manager == nil {
		return nil
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil
	}
	manager.closed = true
	leasing := make([]*CredentialLease, 0, len(manager.leases))
	for id, lease := range manager.leases {
		delete(manager.leases, id)
		leasing = append(leasing, lease)
	}
	manager.mu.Unlock()
	for _, lease := range leasing {
		lease.terminate(leaseClosed)
	}
	return nil
}

func (lease *CredentialLease) WithSecret(
	use func(context.Context, []byte) error,
) error {
	if lease == nil || use == nil {
		return ErrInvalidVaultInput
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.state != leaseActive || len(lease.secret) == 0 {
		return leaseStateError(lease.state)
	}
	if err := lease.context.Err(); err != nil {
		return err
	}
	return use(lease.context, lease.secret)
}

func (lease *CredentialLease) Close() error {
	if lease == nil {
		return nil
	}
	changed := lease.terminate(leaseClosed)
	if changed && lease.manager != nil {
		lease.manager.remove(lease.id)
	}
	return nil
}

func (lease *CredentialLease) expire() {
	if lease == nil {
		return
	}
	if lease.terminate(leaseExpired) && lease.manager != nil {
		lease.manager.remove(lease.id)
	}
}

func (lease *CredentialLease) waitForExpiry() {
	select {
	case <-lease.timer.C:
		lease.expire()
	case <-lease.parent:
		lease.terminate(leaseClosed)
		if lease.manager != nil {
			lease.manager.remove(lease.id)
		}
	case <-lease.done:
	}
}

func (lease *CredentialLease) terminate(state credentialLeaseState) bool {
	if lease.cancel != nil {
		lease.cancel()
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.state != leaseActive {
		return false
	}
	lease.state = state
	if lease.timer != nil {
		lease.timer.Stop()
	}
	clearBytes(lease.secret)
	lease.secret = nil
	close(lease.done)
	return true
}

func (manager *CredentialLeaseManager) remove(id uint64) {
	manager.mu.Lock()
	delete(manager.leases, id)
	manager.mu.Unlock()
}

func leaseStateError(state credentialLeaseState) error {
	switch state {
	case leaseExpired:
		return ErrCredentialLeaseExpired
	case leaseRevoked:
		return ErrCredentialLeaseRevoked
	default:
		return ErrCredentialLeaseClosed
	}
}
