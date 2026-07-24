//go:build darwin

package evidence

import "golang.org/x/sys/unix"

func renameNoReplace(oldpath, newpath string, fromDirFD int, fromName string, toDirFD int, toName string) error {
	return unix.RenameatxNp(fromDirFD, fromName, toDirFD, toName, unix.RENAME_EXCL)
}
