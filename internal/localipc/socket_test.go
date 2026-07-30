package localipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

type identityFixtureFileInfo struct {
	mode os.FileMode
}

func (info identityFixtureFileInfo) Name() string       { return "fixture" }
func (info identityFixtureFileInfo) Size() int64        { return 0 }
func (info identityFixtureFileInfo) Mode() os.FileMode  { return info.mode }
func (info identityFixtureFileInfo) ModTime() time.Time { return time.Time{} }
func (info identityFixtureFileInfo) IsDir() bool        { return info.mode.IsDir() }
func (identityFixtureFileInfo) Sys() any {
	return struct {
		Dev uint64
		Ino uint64
	}{Dev: 7, Ino: 11}
}

func TestFileIdentityIncludesFileKind(t *testing.T) {
	socket, socketOK := identityOf(identityFixtureFileInfo{
		mode: os.ModeSocket | 0o600,
	})
	regular, regularOK := identityOf(identityFixtureFileInfo{mode: 0o600})
	if !socketOK || !regularOK {
		t.Fatalf(
			"identity availability socket=%v regular=%v",
			socketOK,
			regularOK,
		)
	}
	if socket == regular {
		t.Fatalf(
			"equal device/inode socket and regular identities collided: %#v",
			socket,
		)
	}
}

func TestValidateSocketPathRequiresPrivateOwnedCleanNonSymlinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket path validation is compile-only on Windows")
	}
	root := shortPrivateSocketRoot(t)
	valid := filepath.Join(root, "loomd.sock")
	if err := validateSocketPath(valid, os.Geteuid()); err != nil {
		t.Fatalf("validateSocketPath(valid) error = %v", err)
	}

	worldWritable := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(worldWritable, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(worldWritable, 0o777); err != nil {
		t.Fatal(err)
	}
	symlinkRoot := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, symlinkRoot); err != nil {
		t.Fatal(err)
	}
	longName := filepath.Join(root, string(make([]byte, 97)))

	for _, path := range []string{
		"",
		"relative/loomd.sock",
		filepath.Join(root, "other.sock"),
		filepath.Join(worldWritable, "loomd.sock"),
		filepath.Join(symlinkRoot, "loomd.sock"),
		longName,
	} {
		if err := validateSocketPath(path, os.Geteuid()); err == nil {
			t.Fatalf("validateSocketPath(%q) error = nil", path)
		}
	}
}

func TestSocketLockAndExactIdentityCleanupFailClosed(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	lock, err := prepareSocket(socketPath, os.Geteuid())
	if err != nil {
		t.Fatal(err)
	}
	lockInfo, err := lock.Stat()
	if err != nil {
		t.Fatal(err)
	}
	lockIdentity, ok := identityOf(lockInfo)
	if !ok || ownerUID(lockInfo) != os.Geteuid() {
		t.Fatalf("lock identity=%#v ok=%v", lockIdentity, ok)
	}
	if _, err := prepareSocket(socketPath, os.Geteuid()); err == nil {
		t.Fatal("second socket lock was accepted")
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := removeExactFile(socketPath+".lock", lockIdentity); err != nil {
		t.Fatal(err)
	}
	if err := removeExactFile(
		socketPath+".lock",
		lockIdentity,
	); err != nil {
		t.Fatalf("missing exact file cleanup = %v", err)
	}

	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeExactFile(
		replacement,
		fileIdentity{device: 1, inode: 1},
	); err == nil {
		t.Fatal("wrong identity removal error = nil")
	}
	if _, err := os.Stat(replacement); err != nil {
		t.Fatalf("wrong identity removed replacement: %v", err)
	}
	if _, ok := statUintField(nil, "Uid"); ok {
		t.Fatal("nil file info produced stat field")
	}
}

func TestExactRemovePreservesReplacementInsertedAfterInitialCheck(
	t *testing.T,
) {
	root := shortPrivateSocketRoot(t)
	path := filepath.Join(root, "replacement")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := identityOf(info)
	if !ok {
		t.Fatal("original identity unavailable")
	}
	replacement := []byte("replacement")
	err = removeExactFileObserved(path, identity, func() {
		if removeErr := os.Remove(path); removeErr != nil {
			t.Fatal(removeErr)
		}
		if writeErr := os.WriteFile(path, replacement, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	})
	if !errors.Is(err, ErrInvalidSocketPath) {
		t.Fatalf("replacement remove error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != string(replacement) {
		t.Fatalf("replacement data=%q error=%v", data, readErr)
	}
}

func TestPrepareSocketReclaimsAbandonedOwnedLockWithMissingSocket(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	lockPath := socketPath + ".lock"
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(lockPath, 0o600); err != nil {
		t.Fatal(err)
	}

	lock, err := prepareSocket(socketPath, os.Geteuid())
	if err != nil {
		t.Fatalf("prepareSocket(abandoned owned lock) error = %v", err)
	}
	lockInfo, err := lock.Stat()
	if err != nil {
		t.Fatal(err)
	}
	lockIdentity, ok := identityOf(lockInfo)
	if !ok || !lockInfo.Mode().IsRegular() ||
		lockInfo.Mode().Perm() != 0o600 ||
		ownerUID(lockInfo) != os.Geteuid() {
		t.Fatalf(
			"fresh lock info=%#v identity=%#v ok=%v",
			lockInfo,
			lockIdentity,
			ok,
		)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := removeExactFile(lockPath, lockIdentity); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSocketRejectsUnsafeExistingLockShapesWithoutDeletion(
	t *testing.T,
) {
	tests := []struct {
		name string
		make func(t *testing.T, root, lockPath string)
	}{
		{
			name: "symlink",
			make: func(t *testing.T, root, lockPath string) {
				target := filepath.Join(root, "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, lockPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "hard_link",
			make: func(t *testing.T, root, lockPath string) {
				target := filepath.Join(root, "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(target, lockPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "wrong_mode",
			make: func(t *testing.T, _, lockPath string) {
				if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(lockPath, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "nonzero",
			make: func(t *testing.T, _, lockPath string) {
				if err := os.WriteFile(
					lockPath,
					[]byte("occupied"),
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := shortPrivateSocketRoot(t)
			socketPath := filepath.Join(root, "loomd.sock")
			lockPath := socketPath + ".lock"
			test.make(t, root, lockPath)
			before, err := os.Lstat(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeIdentity, ok := identityOf(before)
			if !ok {
				t.Fatal("unsafe fixture identity unavailable")
			}
			if lock, err := prepareSocket(
				socketPath,
				os.Geteuid(),
			); err == nil {
				_ = releaseLockFile(lockPath, lock)
				t.Fatal("unsafe existing lock was accepted")
			}
			after, err := os.Lstat(lockPath)
			afterIdentity, ok := identityOf(after)
			if err != nil || !ok || afterIdentity != beforeIdentity {
				t.Fatalf(
					"unsafe lock changed: before=%#v after=%#v err=%v",
					beforeIdentity,
					afterIdentity,
					err,
				)
			}
		})
	}
}

func TestPrepareSocketRejectsActiveAdvisoryOwnerWithoutDeletion(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	lockPath := socketPath + ".lock"
	owner, err := os.OpenFile(
		lockPath,
		os.O_RDWR|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err := syscall.Flock(
		int(owner.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(owner.Fd()), syscall.LOCK_UN)
	before, err := owner.Stat()
	if err != nil {
		t.Fatal(err)
	}
	beforeIdentity, ok := identityOf(before)
	if !ok {
		t.Fatal("active lock identity unavailable")
	}
	if _, err := prepareSocket(socketPath, os.Geteuid()); err == nil {
		t.Fatal("active advisory owner was accepted")
	}
	if !pathMatchesIdentity(lockPath, beforeIdentity) {
		t.Fatal("active advisory lock path changed")
	}
}

func TestPrepareSocketReclaimsLegacyStaleSocketAndLock(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(socketPath+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := prepareSocket(socketPath, os.Geteuid())
	if err != nil {
		t.Fatalf("legacy stale pair was not reclaimed: %v", err)
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("stale socket remains: %v", err)
	}
	if err := releaseLockFile(socketPath+".lock", lock); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareSocketConcurrentReclaimHasSingleWinnerFiftyTimes(
	t *testing.T,
) {
	for iteration := 0; iteration < 50; iteration++ {
		root := shortPrivateSocketRoot(t)
		socketPath := filepath.Join(root, "loomd.sock")
		lockPath := socketPath + ".lock"
		if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan *os.File, 2)
		var wait sync.WaitGroup
		for contender := 0; contender < 2; contender++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				lock, err := prepareSocket(socketPath, os.Geteuid())
				if err != nil {
					results <- nil
					return
				}
				results <- lock
			}()
		}
		close(start)
		wait.Wait()
		close(results)
		winners := 0
		for lock := range results {
			if lock == nil {
				continue
			}
			winners++
			if err := releaseLockFile(lockPath, lock); err != nil {
				t.Fatal(err)
			}
		}
		if winners != 1 {
			t.Fatalf(
				"iteration %d winners=%d, want 1",
				iteration,
				winners,
			)
		}
	}
}

func TestPrepareSocketRefusesLiveOwnerAndRegularTarget(t *testing.T) {
	root := shortPrivateSocketRoot(t)
	socketPath := filepath.Join(root, "loomd.sock")
	listener, err := net.ListenUnix(
		"unix",
		&net.UnixAddr{Name: socketPath, Net: "unix"},
	)
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if _, err := prepareSocket(socketPath, os.Geteuid()); err == nil {
		t.Fatal("live socket owner was replaced")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := identityOf(info)
	if !ok {
		t.Fatal("socket identity unavailable")
	}
	if err := removeExactFile(socketPath, identity); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(socketPath, []byte("regular"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSocket(socketPath, os.Geteuid()); err == nil {
		t.Fatal("regular socket target was accepted")
	}
}

func TestStaleSocketClassificationAllowsOnlyRefusedOrMissing(t *testing.T) {
	for _, accepted := range []error{
		syscall.ECONNREFUSED,
		syscall.ENOENT,
		&net.OpError{Err: syscall.ECONNREFUSED},
	} {
		if !isStaleSocketDialError(accepted) {
			t.Fatalf("stale socket error %v was rejected", accepted)
		}
	}
	for _, rejected := range []error{
		contextDeadlineError{},
		syscall.EACCES,
		errors.New("ambiguous dial failure"),
	} {
		if isStaleSocketDialError(rejected) {
			t.Fatalf("ambiguous socket error %v was accepted", rejected)
		}
	}
}

type contextDeadlineError struct{}

func (contextDeadlineError) Error() string   { return "timeout" }
func (contextDeadlineError) Timeout() bool   { return true }
func (contextDeadlineError) Temporary() bool { return true }
