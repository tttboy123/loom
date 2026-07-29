//go:build !darwin && !linux

package localipc

import (
	"errors"
	"net"
)

func peerEffectiveUID(*net.UnixConn) (int, error) {
	return -1, errors.New("unsupported platform")
}
