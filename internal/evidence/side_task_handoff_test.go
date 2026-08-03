//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
)

func TestReadArtifactIsBoundedDigestVerifiedAndReturnsCopies(t *testing.T) {
	store := newStoreWithPermissiveUmask(
		t,
		filepath.Join(tempRoot(t), "side-task-state"),
	)
	content := []byte(`{"schema_version":1,"authorized_findings":["bounded"]}`)
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	if _, err := store.Publish(context.Background(), bytes.NewReader(content), digest); err != nil {
		t.Fatal(err)
	}
	first, err := store.ReadArtifact(context.Background(), digest, 1024)
	if err != nil {
		t.Fatal(err)
	}
	first[0] = '!'
	second, err := store.ReadArtifact(context.Background(), digest, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second, content) {
		t.Fatalf("second = %q", second)
	}
	if _, err := store.ReadArtifact(context.Background(), digest, 8); !errors.Is(
		err, ErrArtifactTooLarge,
	) {
		t.Fatalf("bounded read error = %v", err)
	}
	if _, err := store.ReadArtifact(context.Background(), "not-a-digest", 1024); !errors.Is(
		err, ErrInvalidDigest,
	) {
		t.Fatalf("invalid digest error = %v", err)
	}
}
