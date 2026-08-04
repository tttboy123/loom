package schedule

import "time"

// RecoveryPolicy is the frozen bounded-recovery surface. It never invents
// retry semantics; it only reports the next legal action.
type RecoveryPolicy struct {
	MaxAttempts int64
	TTL         time.Duration
}

func DefaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{MaxAttempts: 3, TTL: 10 * time.Second}
}

// NextAction returns reclaim (new generation), repair, or human_required.
// hidden infinite retry is impossible because attemptCount is capped.
type RecoveryAction string

const (
	ActionReclaim RecoveryAction = "reclaim"
	ActionRepair  RecoveryAction = "repair"
	ActionHuman   RecoveryAction = "human_required"
)

func (policy RecoveryPolicy) Next(attemptCount int64, class FailureClass) (RecoveryAction, error) {
	if class == ClassHumanRequired {
		return ActionHuman, nil
	}
	if attemptCount >= policy.MaxAttempts {
		if class == ClassInfraTransient || class == ClassCrossLayer {
			return ActionRepair, nil
		}
		return ActionHuman, nil
	}
	return ActionReclaim, nil
}
