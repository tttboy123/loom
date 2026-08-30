//go:build !unix

package piadapter

import (
	"os"
	"os/exec"
	"time"
)

func piLocalCurrentUserOwns(_ os.FileInfo) bool {
	return false
}

func piLocalFileUID(_ os.FileInfo) (uint32, bool) {
	return 0, false
}

func piLocalFileHasSingleLink(_ os.FileInfo) bool {
	return false
}

func configureLocalModelProcess(_ *exec.Cmd) error {
	return ErrInvalidPiLocalModel
}

func terminateLocalModelProcess(
	_ *exec.Cmd,
	_ <-chan error,
	_ time.Duration,
) error {
	return ErrInvalidPiLocalModel
}
