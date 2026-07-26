//go:build unix

package supervisor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type managedUnixScan struct {
	entries      map[string]workspaceEntry
	maxEntries   int
	maxBytes     int64
	maxFileBytes int64
	totalBytes   int64
	ignoreTopGit bool
}

func scanManagedTreePlatform(
	root string,
	maxEntries int,
	maxBytes int64,
	maxFileBytes int64,
	ignoreTopGit bool,
) (map[string]workspaceEntry, error) {
	var pathBefore unix.Stat_t
	if err := unix.Lstat(root, &pathBefore); err != nil {
		return nil, err
	}
	rootFD, err := unix.Open(
		root,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return nil, err
	}
	rootFile := os.NewFile(uintptr(rootFD), root)
	if rootFile == nil {
		_ = unix.Close(rootFD)
		return nil, errors.New("bind tree root")
	}
	defer rootFile.Close()

	var opened unix.Stat_t
	if err := unix.Fstat(rootFD, &opened); err != nil ||
		!managedUnixDirectory(&opened) ||
		!sameManagedUnixStat(&pathBefore, &opened) {
		return nil, errors.New("tree root identity mismatch")
	}

	scan := &managedUnixScan{
		entries:      make(map[string]workspaceEntry),
		maxEntries:   maxEntries,
		maxBytes:     maxBytes,
		maxFileBytes: maxFileBytes,
		ignoreTopGit: ignoreTopGit,
	}
	if err := scan.directory(rootFile, ""); err != nil {
		return nil, err
	}

	var descriptorAfter unix.Stat_t
	var pathAfter unix.Stat_t
	if err := unix.Fstat(rootFD, &descriptorAfter); err != nil {
		return nil, err
	}
	if err := unix.Lstat(root, &pathAfter); err != nil ||
		!sameManagedUnixStat(&opened, &descriptorAfter) ||
		!sameManagedUnixStat(&descriptorAfter, &pathAfter) {
		return nil, errors.New("tree root changed during scan")
	}
	return scan.entries, nil
}

func (scan *managedUnixScan) directory(directory *os.File, prefix string) error {
	children, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	sort.Slice(children, func(left, right int) bool {
		return children[left].Name() < children[right].Name()
	})

	dirFD := int(directory.Fd())
	for _, child := range children {
		name := child.Name()
		if prefix == "" && scan.ignoreTopGit && name == ".git" {
			continue
		}
		if !validManagedUnixName(name) {
			return errors.New("invalid directory entry name")
		}
		relative := name
		if prefix != "" {
			relative = prefix + "/" + name
		}
		if !validManagedRelativePath(relative) {
			return errors.New("invalid relative path")
		}
		if len(scan.entries) >= scan.maxEntries {
			return errors.New("entry limit")
		}

		var before unix.Stat_t
		if err := unix.Fstatat(
			dirFD,
			name,
			&before,
			unix.AT_SYMLINK_NOFOLLOW,
		); err != nil {
			return err
		}
		managedTraversalBeforeOpen(relative)

		switch {
		case managedUnixDirectory(&before):
			if err := scan.childDirectory(dirFD, name, relative, &before); err != nil {
				return err
			}
		case managedUnixRegular(&before):
			if err := scan.regular(dirFD, name, relative, &before); err != nil {
				return err
			}
		default:
			return errors.New("unsupported filesystem entry")
		}
	}
	return nil
}

func (scan *managedUnixScan) childDirectory(
	parentFD int,
	name string,
	relative string,
	before *unix.Stat_t,
) error {
	childFD, err := unix.Openat(
		parentFD,
		name,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return err
	}
	childFile := os.NewFile(uintptr(childFD), relative)
	if childFile == nil {
		_ = unix.Close(childFD)
		return errors.New("bind child directory")
	}
	defer childFile.Close()

	var opened unix.Stat_t
	if err := unix.Fstat(childFD, &opened); err != nil ||
		!managedUnixDirectory(&opened) ||
		!sameManagedUnixStat(before, &opened) {
		return errors.New("child directory identity mismatch")
	}
	scan.entries[relative] = workspaceEntry{
		path:      relative,
		mode:      managedUnixFileMode(&opened, true),
		directory: true,
	}
	if err := scan.directory(childFile, relative); err != nil {
		return err
	}

	var descriptorAfter unix.Stat_t
	var parentAfter unix.Stat_t
	if err := unix.Fstat(childFD, &descriptorAfter); err != nil {
		return err
	}
	if err := unix.Fstatat(
		parentFD,
		name,
		&parentAfter,
		unix.AT_SYMLINK_NOFOLLOW,
	); err != nil ||
		!sameManagedUnixStat(&opened, &descriptorAfter) ||
		!sameManagedUnixStat(&descriptorAfter, &parentAfter) {
		return errors.New("child directory changed during scan")
	}
	return nil
}

func (scan *managedUnixScan) regular(
	parentFD int,
	name string,
	relative string,
	before *unix.Stat_t,
) error {
	if uint64(before.Nlink) != 1 ||
		before.Size < 0 ||
		before.Size > scan.maxFileBytes {
		return errors.New("regular file link or size limit")
	}
	fileFD, err := unix.Openat(
		parentFD,
		name,
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fileFD), relative)
	if file == nil {
		_ = unix.Close(fileFD)
		return errors.New("bind regular file")
	}
	defer file.Close()

	var opened unix.Stat_t
	if err := unix.Fstat(fileFD, &opened); err != nil ||
		!managedUnixRegular(&opened) ||
		uint64(opened.Nlink) != 1 ||
		!sameManagedUnixStat(before, &opened) {
		return errors.New("regular file identity mismatch")
	}
	content, err := io.ReadAll(io.LimitReader(file, scan.maxFileBytes+1))
	if err != nil || int64(len(content)) > scan.maxFileBytes {
		return errors.New("bounded file read failed")
	}

	var descriptorAfter unix.Stat_t
	var parentAfter unix.Stat_t
	if err := unix.Fstat(fileFD, &descriptorAfter); err != nil {
		return err
	}
	if err := unix.Fstatat(
		parentFD,
		name,
		&parentAfter,
		unix.AT_SYMLINK_NOFOLLOW,
	); err != nil ||
		uint64(descriptorAfter.Nlink) != 1 ||
		uint64(parentAfter.Nlink) != 1 ||
		!sameManagedUnixStat(&opened, &descriptorAfter) ||
		!sameManagedUnixStat(&descriptorAfter, &parentAfter) ||
		int64(len(content)) != descriptorAfter.Size {
		return errors.New("regular file changed during read")
	}
	scan.totalBytes += int64(len(content))
	if scan.totalBytes > scan.maxBytes {
		return errors.New("tree byte limit")
	}
	digest := sha256.Sum256(content)
	mode := managedUnixFileMode(&descriptorAfter, false)
	scan.entries[relative] = workspaceEntry{
		path:       relative,
		mode:       mode,
		size:       int64(len(content)),
		digest:     hex.EncodeToString(digest[:]),
		content:    content,
		executable: mode.Perm()&0o111 != 0,
	}
	return nil
}

func validManagedUnixName(name string) bool {
	return name != "" &&
		name != "." &&
		name != ".." &&
		utf8.ValidString(name) &&
		!strings.ContainsRune(name, 0) &&
		!strings.ContainsRune(name, '/')
}

func managedUnixDirectory(stat *unix.Stat_t) bool {
	return stat != nil && uint32(stat.Mode)&unix.S_IFMT == unix.S_IFDIR
}

func managedUnixRegular(stat *unix.Stat_t) bool {
	return stat != nil && uint32(stat.Mode)&unix.S_IFMT == unix.S_IFREG
}

func managedUnixFileMode(stat *unix.Stat_t, directory bool) fs.FileMode {
	mode := fs.FileMode(uint32(stat.Mode) & 0o777)
	if directory {
		mode |= fs.ModeDir
	}
	return mode
}

func sameManagedUnixStat(left *unix.Stat_t, right *unix.Stat_t) bool {
	return left != nil &&
		right != nil &&
		uint64(left.Dev) == uint64(right.Dev) &&
		uint64(left.Ino) == uint64(right.Ino) &&
		uint32(left.Mode) == uint32(right.Mode) &&
		uint64(left.Nlink) == uint64(right.Nlink) &&
		left.Size == right.Size
}
