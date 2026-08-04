package queue

// ConflictArbiter decides whether two admitted Jobs may be dispatched in
// parallel under the frozen Decomposition Compiler rules 2/3/5 (owned-path,
// mutex, and resource conflicts). It is a pure function of the queue
// projection; it creates no state and no authority.
type ConflictArbiter struct{}

func NewConflictArbiter() *ConflictArbiter {
	return &ConflictArbiter{}
}

// CanRunInParallel reports whether left and right may be dispatched in
// parallel, with a human-readable reason when they may not.
func (arbiter *ConflictArbiter) CanRunInParallel(left, right QueueJob) (bool, string) {
	if path, ok := arbiter.OwnedPathOverlap(left, right); ok {
		return false, "shared owned path " + path
	}
	if key, ok := arbiter.MutexOverlap(left, right); ok {
		return false, "shared mutex " + key
	}
	if reason, ok := arbiter.ResourceConflict(left, right); ok {
		return false, reason
	}
	return true, ""
}

func (arbiter *ConflictArbiter) OwnedPathOverlap(left, right QueueJob) (string, bool) {
	leftPaths := make(map[string]bool, len(left.OwnedPaths))
	for _, path := range left.OwnedPaths {
		leftPaths[path] = true
	}
	for _, path := range right.OwnedPaths {
		if leftPaths[path] {
			return path, true
		}
	}
	return "", false
}

func (arbiter *ConflictArbiter) MutexOverlap(left, right QueueJob) (string, bool) {
	leftMutexes := make(map[string]bool, len(left.MutexKeys))
	for _, key := range left.MutexKeys {
		leftMutexes[key] = true
	}
	for _, key := range right.MutexKeys {
		if leftMutexes[key] {
			return key, true
		}
	}
	return "", false
}

func (arbiter *ConflictArbiter) ResourceConflict(left, right QueueJob) (string, bool) {
	if left.ResourceClaims.Runtime == "" || right.ResourceClaims.Runtime == "" {
		return "", false
	}
	if left.ResourceClaims.Runtime != right.ResourceClaims.Runtime {
		return "", false
	}
	if left.ResourceClaims.Slots+right.ResourceClaims.Slots > 2 {
		return "resource slots oversell on runtime " + left.ResourceClaims.Runtime, true
	}
	return "", false
}
