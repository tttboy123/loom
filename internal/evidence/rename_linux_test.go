//go:build linux

package evidence

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRenameNoReplaceMapsUnsupportedLinuxErrors(t *testing.T) {
	for _, err := range []error{unix.ENOSYS, unix.EOPNOTSUPP, unix.EINVAL} {
		if got := mapRenameNoReplaceError(err); !errors.Is(got, ErrUnsupportedPlatform) || !errors.Is(got, err) {
			t.Fatalf("mapRenameNoReplaceError(%v) = %v, want wrapped ErrUnsupportedPlatform", err, got)
		}
	}

	if got := mapRenameNoReplaceError(unix.EEXIST); !errors.Is(got, unix.EEXIST) || errors.Is(got, ErrUnsupportedPlatform) {
		t.Fatalf("mapRenameNoReplaceError(EEXIST) = %v, want raw existence error", got)
	}
}
