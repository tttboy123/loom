package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"loom-pi-rebuild/internal/credentials"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--credential-helper" {
		if err := credentials.RunProductKeychainHelper(); err == nil {
			os.Exit(0)
		} else {
			os.Exit(credentials.ProductKeychainHelperExitCode(err))
		}
	}
	plan, err := prepareLocalAppInvocation(os.Args[1:], localAppServiceArgs)
	if err != nil {
		_, _ = os.Stderr.WriteString("daemon unavailable: local_app_service\n")
		os.Exit(exitInvalidInput)
	}
	if plan.Reexec {
		if err := reexecLocalAppService(plan.Args); err != nil {
			_, _ = os.Stderr.WriteString("daemon unavailable: local_app_service\n")
			os.Exit(exitUnavailable)
		}
	}
	signalContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	ctx, cancel := context.WithCancel(signalContext)
	defer cancel()
	if plan.ParentPID > 0 {
		go cancelWhenLocalAppParentExits(ctx, cancel, plan.ParentPID)
	}
	os.Exit(run(ctx, plan.Args, os.Stdout, os.Stderr, nil))
}

func reexecLocalAppService(args []string) error {
	executable, err := os.Executable()
	if err != nil || !filepath.IsAbs(executable) {
		return errors.New("local app executable unavailable")
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil || !filepath.IsAbs(executable) ||
		filepath.Clean(executable) != executable {
		return errors.New("local app executable unavailable")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("local app executable unavailable")
	}
	argv := append([]string{executable}, args...)
	return syscall.Exec(executable, argv, os.Environ())
}

func cancelWhenLocalAppParentExits(
	ctx context.Context,
	cancel context.CancelFunc,
	parentPID int,
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syscall.Kill(parentPID, 0); err != nil {
				cancel()
				return
			}
		}
	}
}
