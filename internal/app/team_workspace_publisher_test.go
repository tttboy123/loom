package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/supervisor"
)

func TestAtomicTeamWorkspacePublisherAppliesAcceptedChangesAndMatchesDigest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "remove.txt"), []byte("remove\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseline, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	expected := t.TempDir()
	if err := os.WriteFile(filepath.Join(expected, "old.txt"), []byte("updated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(expected, "web"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expected, "web", "index.html"), []byte("<main>Snake</main>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	finalSnapshot, err := supervisor.ObserveSourceSnapshot(expected)
	if err != nil {
		t.Fatal(err)
	}
	publisher := NewAtomicTeamWorkspacePublisher()
	err = publisher.PublishAcceptedWorkspace(context.Background(), TeamWorkspacePublication{
		TeamInstanceID: "team-snake", PlanDigest: strings.Repeat("1", 64),
		LogicalNodeID: "main", AttemptNumber: 1, SourcePath: root,
		ExpectedSourceDigest: baseline.TreeDigest(), WorkspaceDigest: finalSnapshot.TreeDigest(),
		Changes: []TeamWorkspaceChange{
			teamWorkspaceTestChange("old.txt", supervisor.WorkspaceChangeModified, []byte("updated\n")),
			teamWorkspaceTestChange("web/index.html", supervisor.WorkspaceChangeAdded, []byte("<main>Snake</main>\n")),
			{Path: "remove.txt", Kind: supervisor.WorkspaceChangeDeleted, Mode: 0o600,
				Digest: teamWorkspaceTestDigest([]byte("remove\n"))},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil || actual.TreeDigest() != finalSnapshot.TreeDigest() {
		t.Fatalf("published digest=%q want=%q err=%v", actual.TreeDigest(), finalSnapshot.TreeDigest(), err)
	}
	content, err := os.ReadFile(filepath.Join(root, "web", "index.html"))
	if err != nil || string(content) != "<main>Snake</main>\n" {
		t.Fatalf("published file=%q err=%v", content, err)
	}
}

func TestAtomicTeamWorkspacePublisherRejectsSourceDriftAndOutsidePath(t *testing.T) {
	root := t.TempDir()
	baseline, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "drift.txt"), []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	publisher := NewAtomicTeamWorkspacePublisher()
	err = publisher.PublishAcceptedWorkspace(context.Background(), TeamWorkspacePublication{
		TeamInstanceID: "team-snake", PlanDigest: strings.Repeat("1", 64),
		LogicalNodeID: "main", AttemptNumber: 1, SourcePath: root,
		ExpectedSourceDigest: baseline.TreeDigest(), WorkspaceDigest: strings.Repeat("2", 64),
		Changes: []TeamWorkspaceChange{
			teamWorkspaceTestChange("index.html", supervisor.WorkspaceChangeAdded, []byte("snake")),
		},
	})
	if !errors.Is(err, supervisor.ErrSourceChanged) {
		t.Fatalf("source drift error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("drifted publish created file: %v", err)
	}
}

func TestAtomicTeamWorkspacePublisherRejectsDestructiveLargeFileShrink(t *testing.T) {
	root := t.TempDir()
	original := []byte(strings.Repeat("playable-snake-game\n", 128))
	path := filepath.Join(root, "index.html")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	baseline, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	truncated := []byte("<meta name=\"acceptance\">\n")
	candidateRoot := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(candidateRoot, "index.html"), truncated, 0o600,
	); err != nil {
		t.Fatal(err)
	}
	candidate, err := supervisor.ObserveSourceSnapshot(candidateRoot)
	if err != nil {
		t.Fatal(err)
	}
	publisher := NewAtomicTeamWorkspacePublisher()
	err = publisher.PublishAcceptedWorkspace(
		context.Background(),
		TeamWorkspacePublication{
			TeamInstanceID: "team-snake", PlanDigest: strings.Repeat("1", 64),
			LogicalNodeID: "main", AttemptNumber: 1, SourcePath: root,
			ExpectedSourceDigest: baseline.TreeDigest(),
			WorkspaceDigest:      candidate.TreeDigest(),
			Changes: []TeamWorkspaceChange{
				teamWorkspaceTestChange(
					"index.html", supervisor.WorkspaceChangeModified, truncated,
				),
			},
		},
	)
	if !errors.Is(err, ErrTeamWorkspacePublish) ||
		!errors.Is(err, ErrTeamWorkspaceDestructiveChange) {
		t.Fatalf("destructive shrink error = %v", err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("source changed after rejected shrink: bytes=%d err=%v", len(content), readErr)
	}
}

func TestAtomicTeamWorkspacePublisherClassifiesFinalDigestFailure(t *testing.T) {
	root := t.TempDir()
	baseline, err := supervisor.ObserveSourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("<main>Snake</main>\n")
	err = NewAtomicTeamWorkspacePublisher().PublishAcceptedWorkspace(
		context.Background(),
		TeamWorkspacePublication{
			TeamInstanceID: "team-snake", PlanDigest: strings.Repeat("1", 64),
			LogicalNodeID: "main", AttemptNumber: 1, SourcePath: root,
			ExpectedSourceDigest: baseline.TreeDigest(),
			WorkspaceDigest:      strings.Repeat("2", 64),
			Changes: []TeamWorkspaceChange{
				teamWorkspaceTestChange("index.html", supervisor.WorkspaceChangeAdded, content),
			},
		},
	)
	if !errors.Is(err, ErrTeamWorkspacePublish) {
		t.Fatalf("final digest error = %v", err)
	}
	if stage, ok := TeamWorkspacePublishStage(err); !ok || stage != "final_digest" {
		t.Fatalf("final digest stage = %q, %t", stage, ok)
	}
	if _, statErr := os.Stat(filepath.Join(root, "index.html")); !errors.Is(
		statErr, os.ErrNotExist,
	) {
		t.Fatalf("failed publication was not rolled back: %v", statErr)
	}
}

func teamWorkspaceTestChange(
	path string,
	kind supervisor.WorkspaceChangeKind,
	content []byte,
) TeamWorkspaceChange {
	return TeamWorkspaceChange{
		Path: path, Kind: kind, Mode: 0o600,
		Digest: teamWorkspaceTestDigest(content), Content: append([]byte(nil), content...),
	}
}

func teamWorkspaceTestDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
