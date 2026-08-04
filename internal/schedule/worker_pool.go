package schedule

// WorkerPool coordinates per-lane pools so capacity is never oversold and a
// worker holds exactly one claim.
type WorkerPool struct {
	pools map[Lane]*Pool
}

func NewWorkerPool(capacities map[Lane]int) *WorkerPool {
	pools := map[Lane]*Pool{}
	for lane, capacity := range capacities {
		pools[lane] = NewPool(lane, capacity)
	}
	return &WorkerPool{pools: pools}
}

func (wp *WorkerPool) Pool(lane Lane) *Pool {
	if wp == nil {
		return nil
	}
	return wp.pools[lane]
}

func (wp *WorkerPool) ActiveCount(lane Lane) int {
	pool := wp.Pool(lane)
	if pool == nil {
		return 0
	}
	return len(pool.Active)
}

func (wp *WorkerPool) Acquire(lane Lane, workerID string, attempt Attempt) (Worker, error) {
	pool := wp.Pool(lane)
	if pool == nil {
		return Worker{}, ErrCapacityOversold
	}
	return pool.Acquire(workerID, attempt)
}

func (wp *WorkerPool) Release(lane Lane, workerID string) {
	if pool := wp.Pool(lane); pool != nil {
		pool.Release(workerID)
	}
}
