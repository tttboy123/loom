//go:build darwin

package piadapter

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func piContextPeerPID(connection *net.UnixConn) (int, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return -1, err
	}
	processID := -1
	var credentialErr error
	if err := raw.Control(func(fd uintptr) {
		processID, credentialErr = unix.GetsockoptInt(
			int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID,
		)
	}); err != nil {
		return -1, err
	}
	if credentialErr != nil || processID <= 0 {
		return -1, errors.Join(syscall.EPERM, credentialErr)
	}
	return processID, nil
}
