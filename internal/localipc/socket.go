package localipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"time"
)

var ErrInvalidSocketPath = errors.New("invalid local IPC socket path")

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
	if err := validateSocketPath(path, effectiveUID); err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(
		lockPath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return nil, ErrInvalidSocketPath
	}
	lockInfo, statErr := lock.Stat()
	lockIdentity, identityOK := identityOf(lockInfo)
	if statErr != nil || !identityOK ||
		ownerUID(lockInfo) != effectiveUID {
		_ = lock.Close()
		return nil, ErrInvalidSocketPath
	}
	cleanupLock := func() {
		_ = lock.Close()
		_ = removeExactFile(lockPath, lockIdentity)
	}
	if err := lock.Chmod(0o600); err != nil {
		cleanupLock()
		return nil, ErrInvalidSocketPath
	}
	if info, err := os.Lstat(path); err == nil {
		before, ok := identityOf(info)
		if !ok || !isSocket(info) ||
			info.Mode().Perm() != 0o600 ||
			ownerUID(info) != effectiveUID {
			cleanupLock()
			return nil, ErrInvalidSocketPath
		}
		connection, dialErr := net.DialTimeout("unix", path, 25*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			cleanupLock()
			return nil, ErrInvalidSocketPath
		}
		if !isStaleSocketDialError(dialErr) {
			cleanupLock()
			return nil, ErrInvalidSocketPath
		}
		afterInfo, statErr := os.Lstat(path)
		after, identityOK := identityOf(afterInfo)
		if statErr != nil || !identityOK || before != after ||
			!isSocket(afterInfo) ||
			removeExactFile(path, before) != nil {
			cleanupLock()
			return nil, ErrInvalidSocketPath
		}
	}
	return lock, nil
}

func isStaleSocketDialError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENOENT)
}

func removeExactFile(path string, identity fileIdentity) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	current, ok := identityOf(info)
	if err != nil || !ok || current != identity {
		return ErrInvalidSocketPath
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%w: cleanup", ErrInvalidSocketPath)
	}
	return nil
}
