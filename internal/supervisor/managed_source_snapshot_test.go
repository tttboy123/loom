package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveSourceSnapshotIsDeterministicContentAddressedAndPathFree(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "config"), []byte("private repository metadata\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := ObserveSourceSnapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ObserveSourceSnapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !first.Valid() || first.EntryCount() != 1 ||
		first.FileCount() != 1 || first.DirectoryCount() != 0 ||
		first.TotalBytes() != 6 {
		t.Fatalf("snapshot = %#v / %#v", first, second)
	}
	if strings.Contains(first.TreeDigest(), source) {
		t.Fatalf("snapshot digest leaked source path: %q", first.TreeDigest())
	}

	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := ObserveSourceSnapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Valid() || changed.TreeDigest() == first.TreeDigest() ||
		changed.TotalBytes() != 7 {
		t.Fatalf("changed snapshot = %#v", changed)
	}
}
