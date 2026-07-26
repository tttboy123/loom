//go:build unix

package piadapter

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureExecutionProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func terminateExecutionProcess(
	command *exec.Cmd,
	wait <-chan error,
	grace time.Duration,
) error {
	if command == nil || command.Process == nil {
		return nil
	}
	processGroup := -command.Process.Pid
	childReaped := false
	select {
	case <-wait:
		childReaped = true
	default:
	}
	if childReaped && !executionProcessGroupExists(processGroup) {
		return nil
	}
	termErr := syscall.Kill(processGroup, syscall.SIGTERM)
	if errors.Is(termErr, syscall.EPERM) {
		timer := time.NewTimer(25 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-wait:
			childReaped = true
			if !executionProcessGroupExists(processGroup) {
				return nil
			}
		case <-timer.C:
		}
	}
	if errors.Is(termErr, syscall.ESRCH) {
		termErr = nil
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	if !childReaped {
		select {
		case <-wait:
			childReaped = true
			if !executionProcessGroupExists(processGroup) {
				return termErr
			}
		case <-timer.C:
		}
	} else {
		<-timer.C
	}
	killErr := syscall.Kill(processGroup, syscall.SIGKILL)
	if errors.Is(killErr, syscall.ESRCH) {
		killErr = nil
	}
	if !childReaped {
		<-wait
	}
	return errors.Join(termErr, killErr)
}

func executionProcessGroupExists(processGroup int) bool {
	err := syscall.Kill(processGroup, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func executionProcessExists(processID int) bool {
	err := syscall.Kill(processID, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
