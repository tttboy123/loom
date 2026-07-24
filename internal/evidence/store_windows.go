//go:build windows

package evidence

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var (
	ErrRootSymlink         = errors.New("evidence root is a symlink")
	ErrRootChanged         = errors.New("evidence root changed")
	ErrInvalidRoot         = errors.New("invalid evidence root")
	ErrStateSymlink        = errors.New("evidence state path is a symlink")
	ErrInvalidDigest       = errors.New("invalid evidence digest")
	ErrDigestMismatch      = errors.New("evidence digest mismatch")
	ErrCorruptArtifact     = errors.New("corrupt evidence artifact")
	ErrShardChanged        = errors.New("evidence shard changed")
	ErrStagingCleanup      = errors.New("evidence staging cleanup failed")
	ErrStoreClosed         = errors.New("evidence store is closed")
	ErrUnsupportedPlatform = errors.New("evidence atomic no-replace publish is unsupported on this platform")
)

type Artifact struct {
	Digest string
}

type Store struct{}

func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: empty root", ErrInvalidRoot)
	}
	return nil, ErrUnsupportedPlatform
}

func (s *Store) Publish(ctx context.Context, source io.Reader, expectedDigest string) (Artifact, error) {
	return Artifact{}, ErrUnsupportedPlatform
}

func (s *Store) Close() error {
	return nil
}
