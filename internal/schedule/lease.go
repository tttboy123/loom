package schedule

import (
	"time"
)

// Lease is the frozen attempt lease: one claim per attempt, generation
// monotonic, expiry drives exactly one reclamation.
type Lease struct {
	AttemptID  string
	JobID      string
	Generation int64
	ExpiresAt  time.Time
}

// IssueLease creates a fresh lease for a new generation.
func IssueLease(attemptID, jobID string, generation int64, ttl time.Duration, now time.Time) Lease {
	if generation < 1 {
		generation = 1
	}
	return Lease{
		AttemptID: attemptID, JobID: jobID,
		Generation: generation, ExpiresAt: now.Add(ttl),
	}
}

// Validate returns ErrLeaseExpired when the lease is past expiry and
// ErrStaleGeneration when the presented generation is behind the current one.
func (lease Lease) Validate(currentGeneration int64, now time.Time) error {
	if now.After(lease.ExpiresAt) {
		return ErrLeaseExpired
	}
	if lease.Generation < currentGeneration {
		return ErrStaleGeneration
	}
	return nil
}

// Expired reports whether the lease has lapsed at now.
func (lease Lease) Expired(now time.Time) bool {
	return now.After(lease.ExpiresAt)
}
