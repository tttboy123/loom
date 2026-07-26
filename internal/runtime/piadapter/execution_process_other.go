//go:build !unix

package piadapter

import (
	"errors"
	"os/exec"
	"time"
)

func configureExecutionProcess(_ *exec.Cmd) error {
	return ErrPiExecutionCleanup
}

func terminateExecutionProcess(
	command *exec.Cmd,
	wait <-chan error,
	_ time.Duration,
) error {
	if command == nil || command.Process == nil {
		return nil
	}
	killErr := command.Process.Kill()
	<-wait
	return errors.Join(ErrPiExecutionCleanup, killErr)
}

func executionProcessExists(_ int) bool {
	return false
}
