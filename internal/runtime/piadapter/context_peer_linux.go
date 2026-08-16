//go:build linux

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
		credential, err := unix.GetsockoptUcred(
			int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED,
		)
		if err != nil {
			credentialErr = err
			return
		}
		processID = int(credential.Pid)
	}); err != nil {
		return -1, err
	}
	if credentialErr != nil || processID <= 0 {
		return -1, errors.Join(syscall.EPERM, credentialErr)
	}
	return processID, nil
}
