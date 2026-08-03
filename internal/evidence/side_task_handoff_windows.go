//go:build windows

package evidence

import "context"

var ErrArtifactTooLarge = ErrUnsupportedPlatform

func (*Store) ReadArtifact(context.Context, string, int64) ([]byte, error) {
	return nil, ErrUnsupportedPlatform
}
