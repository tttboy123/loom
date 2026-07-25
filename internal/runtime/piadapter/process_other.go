//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package piadapter

import "os/exec"

func configurePiMetadataProcess(_ *exec.Cmd) {}
