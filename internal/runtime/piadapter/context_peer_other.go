//go:build !darwin && !linux

package piadapter

import (
	"errors"
	"net"
)

func piContextPeerPID(*net.UnixConn) (int, error) {
	return -1, errors.New("Pi Context peer identity unsupported")
}
