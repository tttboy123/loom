//go:build !darwin && !linux && !windows

package evidence

func renameNoReplace(oldpath, newpath string, fromDirFD int, fromName string, toDirFD int, toName string) error {
	return ErrUnsupportedPlatform
}
