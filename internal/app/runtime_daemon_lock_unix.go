//go:build darwin || linux

package app

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

type runtimeDaemonStateLock struct {
	file *os.File
}

func acquireRuntimeDaemonStateLock(path string) (*runtimeDaemonStateLock, error) {
	fd, err := unix.Open(
		path,
		unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, ErrLocalRuntimeObservationDaemonLocked
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 {
		_ = file.Close()
		return nil, ErrLocalRuntimeObservationDaemonLocked
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, ErrLocalRuntimeObservationDaemonLocked
	}
	return &runtimeDaemonStateLock{file: file}, nil
}

func openRuntimeDaemonStateFile(path string) (*os.File, error) {
	fd, err := unix.Open(
		path,
		unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 {
		_ = file.Close()
		return nil, ErrLocalRuntimeObservationDaemonState
	}
	return file, nil
}

func (l *runtimeDaemonStateLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	fd := int(l.file.Fd())
	unlockErr := unix.Flock(fd, unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}
