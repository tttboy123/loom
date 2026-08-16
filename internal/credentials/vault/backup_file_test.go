package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishEncryptedBackupCreatesPrivateFileWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "loom-backup.loomvault")
	data := []byte(`{"schema_version":1,"ciphertext":"test-only"}`)
	if err := PublishEncryptedBackup(context.Background(), path, data); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != int64(len(data)) {
		t.Fatalf("published backup = %#v, %v", info, err)
	}
	if err := PublishEncryptedBackup(
		context.Background(), path, []byte("replacement"),
	); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("overwrite error = %v", err)
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != string(data) {
		t.Fatalf("published backup changed = %q, %v", stored, err)
	}
}

func TestPublishEncryptedBackupRejectsUnsafeDestination(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.loomvault")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"relative":        "backup.loomvault",
		"wrong_extension": filepath.Join(root, "backup.json"),
		"symlink":         filepath.Join(root, "link.loomvault"),
	} {
		t.Run(name, func(t *testing.T) {
			if name == "symlink" {
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := PublishEncryptedBackup(
				context.Background(), path, []byte("encrypted"),
			); err == nil {
				t.Fatal("unsafe backup destination accepted")
			}
		})
	}
}
