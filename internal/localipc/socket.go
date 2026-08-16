package localipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"time"
)

var ErrInvalidSocketPath = errors.New("invalid local IPC socket path")

var exactRemoveMu sync.Mutex

var socketPreparationLocks = struct {
	sync.Mutex
	entries map[string]*socketPreparationLock
}{entries: make(map[string]*socketPreparationLock)}

type socketPreparationLock struct {
	mu   sync.Mutex
	refs int
}

type fileIdentity struct {
	device uint64
	inode  uint64
	kind   os.FileMode
}

func validateSocketPath(path string, effectiveUID int) error {
	if path == "" || !filepath.IsAbs(path) ||
		filepath.Clean(path) != path ||
		len([]byte(path)) > 96 ||
		filepath.Base(path) != "loomd.sock" {
		return ErrInvalidSocketPath
	}
	parent := filepath.Dir(path)
	parentInfo, err := os.Lstat(parent)
	if err != nil || !parentInfo.IsDir() ||
		parentInfo.Mode().Perm() != 0o700 ||
		ownerUID(parentInfo) != effectiveUID {
		return ErrInvalidSocketPath
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil || resolvedParent != parent {
		return ErrInvalidSocketPath
	}
	resolvedInfo, err := os.Stat(resolvedParent)
	if err != nil || !os.SameFile(parentInfo, resolvedInfo) {
		return ErrInvalidSocketPath
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !isSocket(info) {
			return ErrInvalidSocketPath
		}
	} else if !os.IsNotExist(err) {
		return ErrInvalidSocketPath
	}
	return nil
}

func ownerUID(info os.FileInfo) int {
	value, ok := statUintField(info, "Uid")
	if !ok || value > uint64(^uint(0)>>1) {
		return -1
	}
	return int(value)
}

func identityOf(info os.FileInfo) (fileIdentity, bool) {
	device, deviceOK := statUintField(info, "Dev")
	inode, inodeOK := statUintField(info, "Ino")
	if !deviceOK || !inodeOK {
		return fileIdentity{}, false
	}
	return fileIdentity{
		device: device,
		inode:  inode,
		kind:   info.Mode() & os.ModeType,
	}, true
}

func statUintField(info os.FileInfo, name string) (uint64, bool) {
	if info == nil || info.Sys() == nil {
		return 0, false
	}
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return 0, false
	}
	field := value.FieldByName(name)
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint(), true
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		signed := field.Int()
		if signed < 0 {
			return 0, false
		}
		return uint64(signed), true
	default:
		return 0, false
	}
}

func isSocket(info os.FileInfo) bool {
	return info.Mode()&os.ModeSocket != 0
}

func prepareSocket(path string, effectiveUID int) (*os.File, error) {
	unlock := lockSocketPreparation(path)
	defer unlock()
	if err := validateSocketPath(path, effectiveUID); err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	lock, err := createOwnedLock(lockPath, effectiveUID)
	if err == nil {
		if err := prepareSocketTarget(path, effectiveUID); err != nil {
			_ = releaseLockFile(lockPath, lock)
			return nil, err
		}
		return lock, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, ErrInvalidSocketPath
	}

	abandoned, err := openOwnedLock(lockPath, effectiveUID)
	if err != nil {
		return nil, ErrInvalidSocketPath
	}
	if err := syscall.Flock(
		int(abandoned.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	); err != nil {
		_ = abandoned.Close()
		return nil, ErrInvalidSocketPath
	}
	abandonedInfo, err := abandoned.Stat()
	abandonedIdentity, identityOK := identityOf(abandonedInfo)
	if err != nil || !identityOK ||
		!validLockInfo(abandonedInfo, effectiveUID) ||
		!pathMatchesOwnedLock(
			lockPath,
			abandonedIdentity,
			effectiveUID,
		) {
		_ = abandoned.Close()
		return nil, ErrInvalidSocketPath
	}
	if err := prepareSocketTarget(path, effectiveUID); err != nil {
		_ = abandoned.Close()
		return nil, err
	}
	if err := removeExactFile(lockPath, abandonedIdentity); err != nil {
		_ = abandoned.Close()
		return nil, ErrInvalidSocketPath
	}
	fresh, err := createOwnedLock(lockPath, effectiveUID)
	if err != nil {
		_ = abandoned.Close()
		return nil, ErrInvalidSocketPath
	}
	if err := abandoned.Close(); err != nil {
		_ = releaseLockFile(lockPath, fresh)
		return nil, ErrInvalidSocketPath
	}
	return fresh, nil
}

func lockSocketPreparation(path string) func() {
	socketPreparationLocks.Lock()
	entry := socketPreparationLocks.entries[path]
	if entry == nil {
		entry = &socketPreparationLock{}
		socketPreparationLocks.entries[path] = entry
	}
	entry.refs++
	socketPreparationLocks.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		socketPreparationLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(socketPreparationLocks.entries, path)
		}
		socketPreparationLocks.Unlock()
	}
}

func createOwnedLock(path string, effectiveUID int) (*os.File, error) {
	lock, err := os.OpenFile(
		path,
		os.O_RDWR|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return nil, err
	}
	lockInfo, statErr := lock.Stat()
	lockIdentity, identityOK := identityOf(lockInfo)
	if statErr != nil || !identityOK {
		_ = lock.Close()
		return nil, ErrInvalidSocketPath
	}
	if err := syscall.Flock(
		int(lock.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	); err != nil {
		_ = removeExactFile(path, lockIdentity)
		_ = lock.Close()
		return nil, ErrInvalidSocketPath
	}
	if err := lock.Chmod(0o600); err != nil {
		_ = releaseLockFile(path, lock)
		return nil, ErrInvalidSocketPath
	}
	lockInfo, statErr = lock.Stat()
	lockIdentityAfter, identityOK := identityOf(lockInfo)
	if statErr != nil || !identityOK ||
		lockIdentityAfter != lockIdentity ||
		!validLockInfo(lockInfo, effectiveUID) ||
		!pathMatchesOwnedLock(
			path,
			lockIdentity,
			effectiveUID,
		) {
		if identityOK {
			_ = removeExactFile(path, lockIdentity)
		}
		_ = lock.Close()
		return nil, ErrInvalidSocketPath
	}
	return lock, nil
}

func openOwnedLock(path string, effectiveUID int) (*os.File, error) {
	fd, err := syscall.Open(
		path,
		syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC,
		0,
	)
	if err != nil {
		return nil, err
	}
	lock := os.NewFile(uintptr(fd), path)
	if lock == nil {
		_ = syscall.Close(fd)
		return nil, ErrInvalidSocketPath
	}
	info, err := lock.Stat()
	if err != nil || !validLockInfo(info, effectiveUID) {
		_ = lock.Close()
		return nil, ErrInvalidSocketPath
	}
	return lock, nil
}

func validLockInfo(info os.FileInfo, effectiveUID int) bool {
	linkCount, linkOK := statUintField(info, "Nlink")
	return info != nil &&
		info.Mode().IsRegular() &&
		info.Mode().Perm() == 0o600 &&
		info.Size() == 0 &&
		ownerUID(info) == effectiveUID &&
		linkOK && linkCount == 1
}

func pathMatchesIdentity(path string, identity fileIdentity) bool {
	info, err := os.Lstat(path)
	current, ok := identityOf(info)
	return err == nil && ok && current == identity
}

func pathMatchesOwnedLock(
	path string,
	identity fileIdentity,
	effectiveUID int,
) bool {
	info, err := os.Lstat(path)
	current, ok := identityOf(info)
	return err == nil && ok && current == identity &&
		validLockInfo(info, effectiveUID)
}

func prepareSocketTarget(path string, effectiveUID int) error {
	if info, err := os.Lstat(path); err == nil {
		before, ok := identityOf(info)
		if !ok || !isSocket(info) ||
			info.Mode().Perm() != 0o600 ||
			ownerUID(info) != effectiveUID {
			return ErrInvalidSocketPath
		}
		connection, dialErr := net.DialTimeout("unix", path, 25*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			return ErrInvalidSocketPath
		}
		if !isStaleSocketDialError(dialErr) {
			return ErrInvalidSocketPath
		}
		afterInfo, statErr := os.Lstat(path)
		after, identityOK := identityOf(afterInfo)
		if statErr != nil || !identityOK || before != after ||
			!isSocket(afterInfo) ||
			removeExactFile(path, before) != nil {
			return ErrInvalidSocketPath
		}
	} else if !os.IsNotExist(err) {
		return ErrInvalidSocketPath
	}
	return nil
}

func releaseLockFile(
	path string,
	lock *os.File,
) error {
	if lock == nil {
		return nil
	}
	info, statErr := lock.Stat()
	identity, identityOK := identityOf(info)
	var releaseErr error
	if statErr != nil || !identityOK {
		releaseErr = ErrInvalidSocketPath
	} else {
		releaseErr = removeExactFile(path, identity)
	}
	return errors.Join(releaseErr, lock.Close())
}

func isStaleSocketDialError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENOENT)
}

func removeExactFile(path string, identity fileIdentity) error {
	return removeExactFileObserved(path, identity, nil)
}

func removeExactFileObserved(
	path string,
	identity fileIdentity,
	afterInitialCheck func(),
) error {
	exactRemoveMu.Lock()
	defer exactRemoveMu.Unlock()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	current, ok := identityOf(info)
	if err != nil || !ok || current != identity {
		return ErrInvalidSocketPath
	}
	if afterInitialCheck != nil {
		afterInitialCheck()
	}
	info, err = os.Lstat(path)
	current, ok = identityOf(info)
	if err != nil || !ok || current != identity {
		return ErrInvalidSocketPath
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%w: cleanup", ErrInvalidSocketPath)
	}
	return nil
}
