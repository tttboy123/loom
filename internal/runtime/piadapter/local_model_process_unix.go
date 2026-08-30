//go:build unix

package piadapter

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func piLocalCurrentUserOwns(info os.FileInfo) bool {
	uid, ok := piLocalFileUID(info)
	return ok && uid == uint32(os.Geteuid())
}

func piLocalFileUID(info os.FileInfo) (uint32, bool) {
	if info == nil {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return stat.Uid, ok
}

func piLocalFileHasSingleLink(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink == 1
}

func configureLocalModelProcess(command *exec.Cmd) error {
	return configureExecutionProcess(command)
}

func terminateLocalModelProcess(
	command *exec.Cmd,
	wait <-chan error,
	grace time.Duration,
) error {
	return terminateExecutionProcess(command, wait, grace)
}
