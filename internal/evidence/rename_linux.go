//go:build linux

package evidence

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func renameNoReplace(oldpath, newpath string, fromDirFD int, fromName string, toDirFD int, toName string) error {
	err := unix.Renameat2(fromDirFD, fromName, toDirFD, toName, unix.RENAME_NOREPLACE)
	return mapRenameNoReplaceError(err)
}

func mapRenameNoReplaceError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
		return fmt.Errorf("%w: %w", ErrUnsupportedPlatform, err)
	default:
		return err
	}
}
