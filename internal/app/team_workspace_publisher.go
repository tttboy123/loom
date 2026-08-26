package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/supervisor"
)

var ErrTeamWorkspacePublish = errors.New("Team workspace publish failed")

var ErrTeamWorkspaceDestructiveChange = errors.New(
	"Team workspace destructive change requires review",
)

const guardedTeamWorkspaceFileBytes int64 = 1024

type teamWorkspacePublishStageError string

func (stage teamWorkspacePublishStageError) Error() string {
	return "Team workspace publish stage: " + string(stage)
}

func TeamWorkspacePublishStage(err error) (string, bool) {
	var stage teamWorkspacePublishStageError
	if !errors.As(err, &stage) || stage == "" {
		return "", false
	}
	return string(stage), true
}

func teamWorkspacePublishError(stage string, causes ...error) error {
	values := []error{ErrTeamWorkspacePublish, teamWorkspacePublishStageError(stage)}
	values = append(values, causes...)
	return errors.Join(values...)
}

type atomicTeamWorkspacePublisher struct{}

func NewAtomicTeamWorkspacePublisher() TeamWorkspacePublisher {
	return &atomicTeamWorkspacePublisher{}
}

type stagedTeamWorkspaceChange struct {
	change  TeamWorkspaceChange
	target  string
	staged  string
	backup  string
	applied bool
}

func (*atomicTeamWorkspacePublisher) PublishAcceptedWorkspace(
	ctx context.Context,
	publication TeamWorkspacePublication,
) error {
	if ctx == nil || publication.TeamInstanceID == "" || publication.PlanDigest == "" ||
		publication.LogicalNodeID == "" || publication.AttemptNumber < 1 ||
		publication.SourcePath == "" || publication.ExpectedSourceDigest == "" ||
		publication.WorkspaceDigest == "" || len(publication.Changes) == 0 {
		return teamWorkspacePublishError("input_validation")
	}
	if err := ctx.Err(); err != nil {
		return teamWorkspacePublishError("cancelled", err)
	}
	root := filepath.Clean(publication.SourcePath)
	if !filepath.IsAbs(root) || root != publication.SourcePath {
		return teamWorkspacePublishError("input_validation")
	}
	baseline, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil {
		return teamWorkspacePublishError("source_snapshot", err)
	}
	if baseline.TreeDigest() != publication.ExpectedSourceDigest {
		return teamWorkspacePublishError("source_drift", supervisor.ErrSourceChanged)
	}
	stageRoot, err := os.MkdirTemp(filepath.Dir(root), ".loom-publish-")
	if err != nil {
		return teamWorkspacePublishError("stage_create", err)
	}
	defer os.RemoveAll(stageRoot)
	if err := os.Chmod(stageRoot, 0o700); err != nil {
		return teamWorkspacePublishError("stage_create", err)
	}
	changes := append([]TeamWorkspaceChange(nil), publication.Changes...)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	prepared := make([]stagedTeamWorkspaceChange, 0, len(changes))
	seen := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return teamWorkspacePublishError("cancelled", err)
		}
		relative, ok := validTeamWorkspacePublishPath(change.Path)
		if !ok {
			return teamWorkspacePublishError("change_validation")
		}
		if _, duplicate := seen[relative]; duplicate {
			return teamWorkspacePublishError("change_validation")
		}
		seen[relative] = struct{}{}
		target := filepath.Join(root, relative)
		if !teamWorkspacePathWithin(root, target) ||
			!teamWorkspaceAncestorsSafe(root, filepath.Dir(target)) {
			return teamWorkspacePublishError("change_validation")
		}
		entry := stagedTeamWorkspaceChange{
			change: change, target: target,
			staged: filepath.Join(stageRoot, "new", relative),
			backup: filepath.Join(stageRoot, "backup", relative),
		}
		info, statErr := os.Lstat(target)
		switch change.Kind {
		case supervisor.WorkspaceChangeAdded:
			if !errors.Is(statErr, fs.ErrNotExist) {
				return teamWorkspacePublishError("change_conflict", statErr)
			}
		case supervisor.WorkspaceChangeModified, supervisor.WorkspaceChangeDeleted:
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return teamWorkspacePublishError("change_conflict", statErr)
			}
		default:
			return teamWorkspacePublishError("change_validation")
		}
		if change.Kind != supervisor.WorkspaceChangeDeleted {
			content := append([]byte(nil), change.Content...)
			if change.Kind == supervisor.WorkspaceChangeModified &&
				info.Size() >= guardedTeamWorkspaceFileBytes &&
				int64(len(content))*2 < info.Size() {
				return teamWorkspacePublishError(
					"destructive_change", ErrTeamWorkspaceDestructiveChange,
				)
			}
			digest := sha256.Sum256(content)
			if change.Digest != hex.EncodeToString(digest[:]) ||
				(change.Mode.Perm() != 0o600 && change.Mode.Perm() != 0o700) {
				return teamWorkspacePublishError("change_validation")
			}
			if err := os.MkdirAll(filepath.Dir(entry.staged), 0o700); err != nil ||
				os.WriteFile(entry.staged, content, change.Mode.Perm()) != nil ||
				os.Chmod(entry.staged, change.Mode.Perm()) != nil ||
				syncTeamWorkspaceFile(entry.staged) != nil {
				return teamWorkspacePublishError("stage_write")
			}
		}
		prepared = append(prepared, entry)
	}
	rollback := func() {
		for index := len(prepared) - 1; index >= 0; index-- {
			entry := &prepared[index]
			if !entry.applied {
				continue
			}
			switch entry.change.Kind {
			case supervisor.WorkspaceChangeAdded:
				_ = os.Remove(entry.target)
				removeEmptyTeamWorkspaceParents(root, filepath.Dir(entry.target))
			case supervisor.WorkspaceChangeModified:
				_ = os.Remove(entry.target)
				_ = os.Rename(entry.backup, entry.target)
			case supervisor.WorkspaceChangeDeleted:
				_ = os.Rename(entry.backup, entry.target)
			}
		}
	}
	for index := range prepared {
		entry := &prepared[index]
		if err := ctx.Err(); err != nil {
			rollback()
			return teamWorkspacePublishError("cancelled", err)
		}
		if entry.change.Kind != supervisor.WorkspaceChangeAdded {
			if err := os.MkdirAll(filepath.Dir(entry.backup), 0o700); err != nil ||
				os.Rename(entry.target, entry.backup) != nil {
				rollback()
				return teamWorkspacePublishError("apply")
			}
		}
		if entry.change.Kind != supervisor.WorkspaceChangeDeleted {
			if err := os.MkdirAll(filepath.Dir(entry.target), 0o700); err != nil ||
				!teamWorkspaceAncestorsSafe(root, filepath.Dir(entry.target)) ||
				os.Rename(entry.staged, entry.target) != nil {
				if entry.change.Kind != supervisor.WorkspaceChangeAdded {
					_ = os.Rename(entry.backup, entry.target)
				}
				rollback()
				return teamWorkspacePublishError("apply")
			}
		}
		entry.applied = true
	}
	finalSnapshot, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil || finalSnapshot.TreeDigest() != publication.WorkspaceDigest {
		rollback()
		return teamWorkspacePublishError("final_digest", err)
	}
	if err := syncTeamWorkspaceDirectory(root); err != nil {
		rollback()
		return teamWorkspacePublishError("sync", err)
	}
	return nil
}

func removeEmptyTeamWorkspaceParents(root string, directory string) {
	for directory != root && teamWorkspacePathWithin(root, directory) {
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

func validTeamWorkspacePublishPath(path string) (string, bool) {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." ||
		strings.ContainsRune(path, 0) {
		return "", false
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return path, true
}

func teamWorkspacePathWithin(root string, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func teamWorkspaceAncestorsSafe(root string, directory string) bool {
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	current := root
	if relative == "." {
		info, statErr := os.Lstat(current)
		return statErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func syncTeamWorkspaceFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncTeamWorkspaceDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync workspace directory: %w", err)
	}
	return nil
}
