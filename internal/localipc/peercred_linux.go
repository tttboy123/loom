//go:build linux

package localipc

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func peerEffectiveUID(connection *net.UnixConn) (int, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return -1, err
	}
	uid := -1
	var credentialErr error
	if err := raw.Control(func(fd uintptr) {
		credential, err := unix.GetsockoptUcred(
			int(fd),
			unix.SOL_SOCKET,
			unix.SO_PEERCRED,
		)
		if err != nil {
			credentialErr = err
			return
		}
		uid = int(credential.Uid)
	}); err != nil {
		return -1, err
	}
	if credentialErr != nil || uid < 0 {
		return -1, errors.Join(syscall.EPERM, credentialErr)
	}
	return uid, nil
}
