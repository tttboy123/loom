//go:build !windows

package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

var ErrArtifactTooLarge = errors.New("evidence artifact exceeds read bound")

// ReadArtifact returns a private copy only after revalidating the root,
// opening the digest path without following links, bounding the bytes and
// verifying the exact content digest on the same descriptor.
func (s *Store) ReadArtifact(
	ctx context.Context,
	digest string,
	maximumBytes int64,
) ([]byte, error) {
	if s == nil || ctx == nil || validateDigest(digest) != nil ||
		maximumBytes <= 0 {
		return nil, ErrInvalidDigest
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.lifecycle.RLock()
	defer s.lifecycle.RUnlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrStoreClosed
	}
	s.mu.Unlock()
	unlock, err := s.lockDigest(digest)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.revalidateRoot(); err != nil {
		return nil, err
	}
	shardFD, err := unix.Openat(
		s.sha256FD,
		digest[:2],
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("%w: shard", ErrCorruptArtifact)
	}
	defer unix.Close(shardFD)
	fd, err := unix.Openat(
		shardFD,
		digest,
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("%w: target", ErrCorruptArtifact)
	}
	file := os.NewFile(uintptr(fd), digest)
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, ErrCorruptArtifact
	}
	reader := io.LimitReader(file, maximumBytes+1)
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(bytes)) > maximumBytes {
		return nil, ErrArtifactTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(bytes)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, ErrCorruptArtifact
	}
	return append([]byte{}, bytes...), nil
}
