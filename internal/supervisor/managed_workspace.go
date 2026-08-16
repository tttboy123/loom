package supervisor

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var managedTraversalBeforeOpen = func(string) {}

const (
	maxManagedSourceEntries    = 4096
	maxManagedSourceBytes      = int64(64 << 20)
	maxManagedSourceFileBytes  = int64(1 << 20)
	maxManagedChanges          = 1024
	maxManagedChangeBytes      = int64(16 << 20)
	maxManagedChangeFileBytes  = int64(8 << 20)
	maxManagedRelativePathByte = 4096
)

type workspaceEntry struct {
	path       string
	mode       fs.FileMode
	size       int64
	digest     string
	content    []byte
	directory  bool
	executable bool
}

type managedWorkspace struct {
	rootPath               string
	sourcePath             string
	invocationPath         string
	workspacePath          string
	homePath               string
	tempPath               string
	sourceDigest           string
	initialWorkspaceDigest string
	initialEntries         map[string]workspaceEntry
	rootInfo               fs.FileInfo
	sourceInfo             fs.FileInfo
	invocationInfo         fs.FileInfo
	workspaceInfo          fs.FileInfo
	homeInfo               fs.FileInfo
	tempInfo               fs.FileInfo
}

// SourceSnapshot is a content-addressed, path-free observation of the source
// tree that will be copied into a managed workspace.
type SourceSnapshot struct {
	treeDigest     string
	entryCount     int
	fileCount      int
	directoryCount int
	totalBytes     int64
}

func ObserveSourceSnapshot(sourcePath string) (SourceSnapshot, error) {
	entries, digest, err := scanManagedTree(
		sourcePath,
		maxManagedSourceEntries,
		maxManagedSourceBytes,
		maxManagedSourceFileBytes,
		true,
	)
	if err != nil {
		return SourceSnapshot{}, err
	}
	snapshot := SourceSnapshot{
		treeDigest: digest,
		entryCount: len(entries),
	}
	for _, entry := range entries {
		if entry.directory {
			snapshot.directoryCount++
			continue
		}
		snapshot.fileCount++
		snapshot.totalBytes += entry.size
	}
	if !snapshot.Valid() {
		return SourceSnapshot{}, ErrManagedWorkspace
	}
	return snapshot, nil
}

func (snapshot SourceSnapshot) Valid() bool {
	if !validManagedTreeDigest(snapshot.treeDigest) || snapshot.entryCount < 0 ||
		snapshot.fileCount < 0 || snapshot.directoryCount < 0 ||
		snapshot.totalBytes < 0 ||
		snapshot.fileCount+snapshot.directoryCount != snapshot.entryCount {
		return false
	}
	return true
}

func validManagedTreeDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (snapshot SourceSnapshot) TreeDigest() string { return snapshot.treeDigest }

func (snapshot SourceSnapshot) EntryCount() int { return snapshot.entryCount }

func (snapshot SourceSnapshot) FileCount() int { return snapshot.fileCount }

func (snapshot SourceSnapshot) DirectoryCount() int { return snapshot.directoryCount }

func (snapshot SourceSnapshot) TotalBytes() int64 { return snapshot.totalBytes }

func prepareManagedWorkspace(
	workspaceRoot string,
	sourcePath string,
) (*managedWorkspace, error) {
	if err := validateAbsoluteCleanDirectory(sourcePath, false); err != nil {
		return nil, fmt.Errorf("%w: source: %v", ErrManagedWorkspace, err)
	}
	if err := validateAbsoluteCleanDirectory(workspaceRoot, true); err != nil {
		return nil, fmt.Errorf("%w: root: %v", ErrManagedWorkspace, err)
	}
	rootInfo, err := os.Lstat(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: root identity: %v", ErrManagedWorkspace, err)
	}
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("%w: source identity: %v", ErrManagedWorkspace, err)
	}
	sourceEntries, sourceDigest, err := scanManagedTree(
		sourcePath,
		maxManagedSourceEntries,
		maxManagedSourceBytes,
		maxManagedSourceFileBytes,
		true,
	)
	if err != nil {
		return nil, err
	}
	if err := verifyManagedDirectoryIdentity(sourcePath, sourceInfo, false); err != nil {
		return nil, fmt.Errorf("%w: source identity: %v", ErrManagedWorkspace, err)
	}
	if err := verifyManagedDirectoryIdentity(workspaceRoot, rootInfo, true); err != nil {
		return nil, fmt.Errorf("%w: root identity: %v", ErrManagedWorkspace, err)
	}
	invocationPath, err := os.MkdirTemp(workspaceRoot, "loom-managed-")
	if err != nil {
		return nil, fmt.Errorf("%w: create invocation: %v", ErrManagedWorkspace, err)
	}
	workspace := &managedWorkspace{
		rootPath:       workspaceRoot,
		sourcePath:     sourcePath,
		invocationPath: invocationPath,
		workspacePath:  filepath.Join(invocationPath, "workspace"),
		homePath:       filepath.Join(invocationPath, "home"),
		tempPath:       filepath.Join(invocationPath, "tmp"),
		sourceDigest:   sourceDigest,
		rootInfo:       rootInfo,
		sourceInfo:     sourceInfo,
	}
	failed := true
	defer func() {
		if failed {
			_ = os.RemoveAll(workspace.invocationPath)
		}
	}()
	if err := os.Chmod(invocationPath, 0o700); err != nil {
		return nil, fmt.Errorf("%w: invocation permissions: %v", ErrManagedWorkspace, err)
	}
	for _, path := range []string{
		workspace.workspacePath,
		workspace.homePath,
		workspace.tempPath,
	} {
		if err := os.Mkdir(path, 0o700); err != nil {
			return nil, fmt.Errorf("%w: create private directory: %v", ErrManagedWorkspace, err)
		}
	}
	workspace.invocationInfo, err = os.Lstat(workspace.invocationPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invocation identity: %v", ErrManagedWorkspace, err)
	}
	workspace.workspaceInfo, err = os.Lstat(workspace.workspacePath)
	if err != nil {
		return nil, fmt.Errorf("%w: workspace identity: %v", ErrManagedWorkspace, err)
	}
	workspace.homeInfo, err = os.Lstat(workspace.homePath)
	if err != nil {
		return nil, fmt.Errorf("%w: home identity: %v", ErrManagedWorkspace, err)
	}
	workspace.tempInfo, err = os.Lstat(workspace.tempPath)
	if err != nil {
		return nil, fmt.Errorf("%w: temp identity: %v", ErrManagedWorkspace, err)
	}
	if err := copyManagedEntries(workspace.workspacePath, sourceEntries); err != nil {
		return nil, err
	}
	initialEntries, initialDigest, err := scanManagedTree(
		workspace.workspacePath,
		maxManagedSourceEntries,
		maxManagedSourceBytes,
		maxManagedSourceFileBytes,
		false,
	)
	if err != nil {
		return nil, err
	}
	workspace.initialEntries = initialEntries
	workspace.initialWorkspaceDigest = initialDigest
	failed = false
	return workspace, nil
}

func (workspace *managedWorkspace) verifySourceUnchanged() error {
	if workspace == nil {
		return ErrManagedWorkspace
	}
	if err := verifyManagedDirectoryIdentity(
		workspace.sourcePath,
		workspace.sourceInfo,
		false,
	); err != nil {
		return ErrSourceChanged
	}
	_, digest, err := scanManagedTree(
		workspace.sourcePath,
		maxManagedSourceEntries,
		maxManagedSourceBytes,
		maxManagedSourceFileBytes,
		true,
	)
	if err != nil {
		return err
	}
	if err := verifyManagedDirectoryIdentity(
		workspace.sourcePath,
		workspace.sourceInfo,
		false,
	); err != nil {
		return ErrSourceChanged
	}
	if digest != workspace.sourceDigest {
		return ErrSourceChanged
	}
	return nil
}

func (workspace *managedWorkspace) containsBytes(value []byte) bool {
	if workspace == nil || len(value) == 0 {
		return false
	}
	for _, entry := range workspace.initialEntries {
		if bytes.Contains([]byte(entry.path), value) ||
			bytes.Contains(entry.content, value) {
			return true
		}
	}
	return false
}

func (workspace *managedWorkspace) collectChanges() (
	string,
	[]WorkspaceChange,
	error,
) {
	if workspace == nil {
		return "", nil, ErrManagedWorkspace
	}
	if err := workspace.verifyPrivateBindings(); err != nil {
		return "", nil, err
	}
	finalEntries, digest, err := scanManagedTree(
		workspace.workspacePath,
		maxManagedSourceEntries+maxManagedChanges,
		maxManagedSourceBytes+maxManagedChangeBytes,
		maxManagedChangeFileBytes,
		false,
	)
	if err != nil {
		return "", nil, err
	}
	paths := make(map[string]struct{}, len(workspace.initialEntries)+len(finalEntries))
	for path := range workspace.initialEntries {
		paths[path] = struct{}{}
	}
	for path := range finalEntries {
		paths[path] = struct{}{}
	}
	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)
	changes := make([]WorkspaceChange, 0)
	var contentBytes int64
	for _, path := range sortedPaths {
		before, hadBefore := workspace.initialEntries[path]
		after, hasAfter := finalEntries[path]
		if (hadBefore && before.directory) || (hasAfter && after.directory) {
			continue
		}
		var kind WorkspaceChangeKind
		var entry workspaceEntry
		switch {
		case !hadBefore && hasAfter:
			kind = WorkspaceChangeAdded
			entry = after
		case hadBefore && !hasAfter:
			kind = WorkspaceChangeDeleted
			entry = before
		case hadBefore && hasAfter &&
			(before.digest != after.digest ||
				before.executable != after.executable ||
				before.size != after.size):
			kind = WorkspaceChangeModified
			entry = after
		default:
			continue
		}
		if len(changes) >= maxManagedChanges {
			return "", nil, fmt.Errorf("%w: too many changes", ErrManagedWorkspace)
		}
		var content []byte
		if kind != WorkspaceChangeDeleted {
			contentBytes += int64(len(entry.content))
			if contentBytes > maxManagedChangeBytes {
				return "", nil, fmt.Errorf("%w: changed content limit", ErrManagedWorkspace)
			}
			content = bytes.Clone(entry.content)
		}
		changes = append(changes, WorkspaceChange{
			path:    path,
			kind:    kind,
			mode:    normalizedManagedMode(entry.mode),
			digest:  entry.digest,
			content: content,
		})
	}
	return digest, changes, nil
}

func (workspace *managedWorkspace) cleanup() error {
	if workspace == nil || workspace.invocationPath == "" {
		return ErrManagedWorkspace
	}
	if err := verifyManagedDirectoryIdentity(
		workspace.rootPath,
		workspace.rootInfo,
		true,
	); err != nil {
		return errors.Join(ErrProcessCleanup, err)
	}
	if err := verifyManagedDirectoryIdentity(
		workspace.invocationPath,
		workspace.invocationInfo,
		true,
	); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.Join(ErrProcessCleanup, err)
	}
	if err := os.RemoveAll(workspace.invocationPath); err != nil {
		return fmt.Errorf("%w: %v", ErrProcessCleanup, err)
	}
	return nil
}

func (workspace *managedWorkspace) verifyPrivateBindings() error {
	if workspace == nil {
		return ErrManagedWorkspace
	}
	bindings := []struct {
		path string
		info fs.FileInfo
	}{
		{workspace.rootPath, workspace.rootInfo},
		{workspace.invocationPath, workspace.invocationInfo},
		{workspace.workspacePath, workspace.workspaceInfo},
		{workspace.homePath, workspace.homeInfo},
		{workspace.tempPath, workspace.tempInfo},
	}
	for _, binding := range bindings {
		if err := verifyManagedDirectoryIdentity(
			binding.path,
			binding.info,
			true,
		); err != nil {
			return fmt.Errorf("%w: private directory identity: %v", ErrManagedWorkspace, err)
		}
	}
	return nil
}

func validateAbsoluteCleanDirectory(path string, requirePrivate bool) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("path is not absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("path is not a non-symlink directory")
	}
	if requirePrivate && info.Mode().Perm() != 0o700 {
		return errors.New("directory permissions are not 0700")
	}
	handle, err := os.Open(path)
	if err != nil {
		return err
	}
	defer handle.Close()
	opened, err := handle.Stat()
	if err != nil {
		return err
	}
	after, err := os.Lstat(path)
	if err != nil ||
		!opened.IsDir() ||
		!os.SameFile(info, opened) ||
		!os.SameFile(opened, after) {
		return errors.New("directory identity changed")
	}
	return nil
}

func verifyManagedDirectoryIdentity(
	path string,
	expected fs.FileInfo,
	requirePrivate bool,
) error {
	if expected == nil {
		return errors.New("missing directory identity")
	}
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !current.IsDir() ||
		current.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(expected, current) ||
		current.Mode() != expected.Mode() ||
		(requirePrivate && current.Mode().Perm() != 0o700) {
		return errors.New("directory identity changed")
	}
	return nil
}

func scanManagedTree(
	root string,
	maxEntries int,
	maxBytes int64,
	maxFileBytes int64,
	ignoreTopGit bool,
) (map[string]workspaceEntry, string, error) {
	if err := validateAbsoluteCleanDirectory(root, false); err != nil {
		return nil, "", fmt.Errorf("%w: tree root: %v", ErrManagedWorkspace, err)
	}
	entries, err := scanManagedTreePlatform(
		root,
		maxEntries,
		maxBytes,
		maxFileBytes,
		ignoreTopGit,
	)
	if err != nil {
		return nil, "", fmt.Errorf("%w: scan: %v", ErrManagedWorkspace, err)
	}
	return entries, managedManifestDigest(entries), nil
}

func copyManagedEntries(root string, entries map[string]workspaceEntry) error {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(left, right int) bool {
		leftDepth := strings.Count(paths[left], "/")
		rightDepth := strings.Count(paths[right], "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return paths[left] < paths[right]
	})
	for _, relative := range paths {
		entry := entries[relative]
		path := filepath.Join(root, filepath.FromSlash(relative))
		if entry.directory {
			if err := os.Mkdir(path, 0o700); err != nil {
				return fmt.Errorf("%w: copy directory: %v", ErrManagedWorkspace, err)
			}
			continue
		}
		mode := fs.FileMode(0o600)
		if entry.executable {
			mode = 0o700
		}
		if err := os.WriteFile(path, bytes.Clone(entry.content), mode); err != nil {
			return fmt.Errorf("%w: copy file: %v", ErrManagedWorkspace, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("%w: copied mode: %v", ErrManagedWorkspace, err)
		}
	}
	return nil
}

func managedManifestDigest(entries map[string]workspaceEntry) string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		entry := entries[path]
		writeManagedManifestField(hash, []byte(path))
		if entry.directory {
			writeManagedManifestField(hash, []byte("directory"))
			continue
		}
		writeManagedManifestField(hash, []byte("regular"))
		if entry.executable {
			writeManagedManifestField(hash, []byte("executable"))
		} else {
			writeManagedManifestField(hash, []byte("non-executable"))
		}
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(entry.size))
		writeManagedManifestField(hash, size[:])
		writeManagedManifestField(hash, []byte(entry.digest))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func writeManagedManifestField(writer io.Writer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func validManagedRelativePath(path string) bool {
	if path == "" ||
		!utf8.ValidString(path) ||
		strings.IndexByte(path, 0) >= 0 ||
		len(path) > maxManagedRelativePathByte ||
		filepath.IsAbs(path) ||
		filepath.Clean(path) != path ||
		path == "." {
		return false
	}
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func normalizedManagedMode(mode fs.FileMode) fs.FileMode {
	if mode.Perm()&0o111 != 0 {
		return 0o700
	}
	return 0o600
}
