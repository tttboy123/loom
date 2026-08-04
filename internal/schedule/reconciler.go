package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
)

// Reconciler rebuilds the worker/attempt projection from the Journal and
// decides, for each open or crashed attempt, exactly one legal reclamation.
// It writes nothing itself.
type Reconciler struct {
	Attempts map[string]Attempt
	Leases   map[string]Lease
}

func NewReconciler() *Reconciler {
	return &Reconciler{
		Attempts: map[string]Attempt{},
		Leases:   map[string]Lease{},
	}
}

// Replay rebuilds the attempt/lease projection. Unknown event types are
// ignored (shared Journal); malformed worker events fail closed.
func (r *Reconciler) Replay(events []journal.Event) error {
	for _, event := range events {
		switch event.Type {
		case "AttemptClaimed":
			var attempt Attempt
			if err := json.Unmarshal(event.PayloadJSON, &attempt); err != nil {
				return fmt.Errorf("reconciler invalid attempt: %w", err)
			}
			if attempt.AttemptID == "" {
				return errors.New("reconciler empty attempt_id")
			}
			r.Attempts[attempt.AttemptID] = attempt
			if expiry, err := time.Parse(time.RFC3339Nano, attempt.LeaseExpiresAt); err == nil {
				r.Leases[attempt.AttemptID] = Lease{
					AttemptID: attempt.AttemptID, JobID: attempt.JobID,
					Generation: attempt.Generation, ExpiresAt: expiry,
				}
			}
		case "AttemptResultRecorded", "AttemptCrashed", "LeaseReclaimed",
			"GenerationAdvanced", "StaleResultRejected":
			var update struct {
				AttemptID              string        `json:"attempt_id"`
				Generation             int64         `json:"generation"`
				Status                 AttemptStatus `json:"status"`
				CrashSeam              CrashSeam     `json:"crash_seam"`
				CrashEffectCardinality string        `json:"crash_effect_cardinality"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &update); err != nil {
				return fmt.Errorf("reconciler invalid transition: %w", err)
			}
			if attempt, ok := r.Attempts[update.AttemptID]; ok {
				attempt.Status = update.Status
				if update.CrashSeam != "" {
					attempt.CrashSeam = update.CrashSeam
				}
				if update.CrashEffectCardinality != "" {
					attempt.CrashEffectCardinality = update.CrashEffectCardinality
				}
				if update.Generation > attempt.Generation {
					attempt.Generation = update.Generation
				}
				r.Attempts[update.AttemptID] = attempt
			}
		}
	}
	return nil
}

// OpenAttempts returns non-terminal attempts with their jobs and lanes.
type OpenAttempt struct {
	Attempt Attempt
	Lane    Lane
}

func (r *Reconciler) OpenAttempts(now time.Time) []OpenAttempt {
	out := make([]OpenAttempt, 0, len(r.Attempts))
	for _, attempt := range r.Attempts {
		if attempt.Status == StatusSucceeded || attempt.Status == StatusFailed {
			continue
		}
		lane := attempt.Lane
		if lane == "" {
			lane = LaneDevelopment
		}
		out = append(out, OpenAttempt{Attempt: attempt, Lane: lane})
	}
	return out
}

// Reclaimable returns attempts whose lease expired or which crashed, with the
// next generation (exactly one per reclamation).
func (r *Reconciler) Reclaimable(now time.Time, ttl time.Duration) []Lease {
	var out []Lease
	for _, attempt := range r.Attempts {
		if attempt.Status != StatusCrashed && attempt.Status != StatusStaleRejected {
			lease, ok := r.Leases[attempt.AttemptID]
			if !ok || !lease.Expired(now) {
				continue
			}
		}
		out = append(out, Lease{
			AttemptID: attempt.AttemptID, JobID: attempt.JobID,
			Generation: attempt.Generation + 1, ExpiresAt: now.Add(ttl),
		})
	}
	return out
}
