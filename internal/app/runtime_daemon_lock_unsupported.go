//go:build !darwin && !linux

package app

import "os"

type runtimeDaemonStateLock struct{}

func acquireRuntimeDaemonStateLock(string) (*runtimeDaemonStateLock, error) {
	return nil, ErrLocalRuntimeObservationDaemonLocked
}

func openRuntimeDaemonStateFile(string) (*os.File, error) {
	return nil, ErrLocalRuntimeObservationDaemonState
}

func (*runtimeDaemonStateLock) Close() error {
	return nil
}
