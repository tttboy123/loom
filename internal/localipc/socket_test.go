package localipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
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
