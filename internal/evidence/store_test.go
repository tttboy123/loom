//go:build !windows

package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNewStoreRejectsRootSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	base := tempRoot(t)
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := NewStore(link); !errors.Is(err, ErrRootSymlink) {
		t.Fatalf("NewStore() error = %v, want %v", err, ErrRootSymlink)
	}
}

func TestNewStoreRejectsEmptyRootWithTypedInvalidRoot(t *testing.T) {
	if _, err := NewStore(""); !errors.Is(err, ErrInvalidRoot) || errors.Is(err, ErrInvalidDigest) {
		t.Fatalf("NewStore(empty) error = %v, want ErrInvalidRoot only", err)
	}
}

func TestNewStoreRejectsSymlinkedRootAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	base := tempRoot(t)
	realParent := filepath.Join(base, "real-parent")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatalf("Mkdir(real parent) error = %v", err)
	}
	linkParent := filepath.Join(base, "link-parent")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Fatalf("Symlink(parent) error = %v", err)
	}

	if _, err := NewStore(filepath.Join(linkParent, "state")); !errors.Is(err, ErrRootSymlink) {
		t.Fatalf("NewStore() error = %v, want %v", err, ErrRootSymlink)
	}
}

func TestNewStoreCreatesPrivateStateDirectories(t *testing.T) {
	root := filepath.Join(tempRoot(t), "state")

	store := newStoreWithPermissiveUmask(t, root)

	assertDirMode(t, root, 0o700)
	assertDirMode(t, filepath.Join(root, "artifacts"), 0o700)
	assertDirMode(t, filepath.Join(root, "artifacts", "sha256"), 0o700)
	assertDirMode(t, store.stagingDir, 0o700)
}

func TestPublishKnownBytesToCanonicalDigestPath(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	store := newStoreWithPermissiveUmask(t, root)
	content := []byte("deterministic evidence bytes\n")
	digest := sha256Hex(content)

	var artifact Artifact
	withPermissiveUmask(t, func() {
		var err error
		artifact, err = store.Publish(ctx, bytes.NewReader(content), digest)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	})

	wantPath := artifactPath(root, digest)
	if artifact != (Artifact{Digest: digest}) {
		t.Fatalf("Publish() = %#v, want digest %s", artifact, digest)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile(final) error = %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("final bytes = %q, want %q", got, content)
	}
	assertDirMode(t, filepath.Dir(wantPath), 0o700)
	assertFileMode(t, wantPath, 0o600)
	assertNoStagingResidue(t, store)
}

func TestArtifactPublicIdentityIsDigestOnly(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, filepath.Join(tempRoot(t), "state"))
	content := []byte("public identity is digest only")
	digest := sha256Hex(content)

	artifact, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if artifact != (Artifact{Digest: digest}) {
		t.Fatalf("Publish() = %#v, want only digest identity", artifact)
	}
}

func TestPublishDigestMismatchLeavesNoFinalArtifactOrStagingResidue(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte("actual")
	expected := sha256Hex([]byte("expected"))

	_, err := store.Publish(ctx, bytes.NewReader(content), expected)
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrDigestMismatch)
	}
	if _, statErr := os.Stat(artifactPath(root, expected)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishRepeatedAndConcurrentSameContentIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte(strings.Repeat("same evidence\n", 64))
	digest := sha256Hex(content)

	first, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	const publishers = 12
	artifacts := make([]Artifact, publishers)
	var wg sync.WaitGroup
	errs := make(chan error, publishers)
	for i := range publishers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			artifact, err := store.Publish(ctx, bytes.NewReader(content), digest)
			if err != nil {
				errs <- err
				return
			}
			artifacts[i] = artifact
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Publish() error = %v", err)
	}
	for i, artifact := range artifacts {
		if artifact != first {
			t.Fatalf("artifact[%d] = %#v, want %#v", i, artifact, first)
		}
	}
	assertNoStagingResidue(t, store)
}

func TestPublishTwoStoreInstancesSameRootSameDigestIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	content := []byte(strings.Repeat("cross store same digest\n", 64))
	digest := sha256Hex(content)
	firstReady := make(chan struct{})
	secondReady := make(chan struct{})
	release := make(chan struct{})
	var onceFirst sync.Once
	var onceSecond sync.Once

	storeA := newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			onceFirst.Do(func() { close(firstReady) })
			<-release
		},
	})
	storeB := newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			onceSecond.Do(func() { close(secondReady) })
			<-release
		},
	})

	results := make(chan error, 2)
	go func() {
		artifact, err := storeA.Publish(ctx, bytes.NewReader(content), digest)
		if err == nil && artifact != (Artifact{Digest: digest}) {
			err = fmt.Errorf("storeA artifact = %#v, want digest %s", artifact, digest)
		}
		results <- err
	}()
	go func() {
		artifact, err := storeB.Publish(ctx, bytes.NewReader(content), digest)
		if err == nil && artifact != (Artifact{Digest: digest}) {
			err = fmt.Errorf("storeB artifact = %#v, want digest %s", artifact, digest)
		}
		results <- err
	}()
	<-firstReady
	<-secondReady
	close(release)

	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	assertArtifactBytes(t, artifactPath(root, digest), content)
	assertNoStagingResidue(t, storeA)
	assertNoStagingResidue(t, storeB)
}

func TestPublishExistingTargetWithCorruptBytesFailsClosed(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte("canonical")
	digest := sha256Hex(content)
	target := artifactPath(root, digest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("MkdirAll(target shard) error = %v", err)
	}
	if err := os.WriteFile(target, []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupt target) error = %v", err)
	}

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrCorruptArtifact) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrCorruptArtifact)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("ReadFile(corrupt target) error = %v", readErr)
	}
	if string(got) != "corrupt" {
		t.Fatalf("existing target changed to %q, want corrupt bytes preserved", got)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishReaderFailureCleansStagingState(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	readErr := errors.New("read failed")

	_, err := store.Publish(ctx, errReader{err: readErr}, sha256Hex([]byte("unused")))
	if !errors.Is(err, readErr) {
		t.Fatalf("Publish() error = %v, want reader error %v", err, readErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishInjectedAtomicPublishFailureCleansStagingState(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	renameErr := errors.New("rename failed")
	store := newStoreWithOps(t, root, storeOps{
		renameNoReplace: func(oldpath, newpath string, fromDirFD int, fromName string, toDirFD int, toName string) error {
			assertFileMode(t, oldpath, 0o600)
			return renameErr
		},
	})
	content := []byte("will not commit")
	digest := sha256Hex(content)

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, renameErr) {
		t.Fatalf("Publish() error = %v, want rename error %v", err, renameErr)
	}
	if _, statErr := os.Stat(artifactPath(root, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishCommitSuccessIgnoresPostCommitContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	root := filepath.Join(tempRoot(t), "state")
	var store *Store
	store = newStoreWithOps(t, root, storeOps{
		afterRename: func(oldpath, newpath string) {
			cancel()
		},
	})
	content := []byte("commit wins over later cancellation")
	digest := sha256Hex(content)

	artifact, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if err != nil {
		t.Fatalf("Publish() error = %v, want committed success", err)
	}
	if artifact != (Artifact{Digest: digest}) {
		t.Fatalf("Publish() = %#v, want digest %s", artifact, digest)
	}
	assertArtifactBytes(t, artifactPath(root, digest), content)
	assertNoStagingResidue(t, store)
}

func TestPublishCorruptTargetRaceFailsClosedWithoutOverwrite(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	content := []byte("canonical no replace")
	digest := sha256Hex(content)
	corrupt := []byte("corrupt target created during race")
	var store *Store
	store = newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			if err := os.WriteFile(newpath, corrupt, 0o600); err != nil {
				t.Fatalf("WriteFile(corrupt race target) error = %v", err)
			}
		},
	})

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrCorruptArtifact) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrCorruptArtifact)
	}
	assertArtifactBytes(t, artifactPath(root, digest), corrupt)
	assertNoStagingResidue(t, store)
}

func TestPublishRejectsShardRebindImmediatelyPreRename(t *testing.T) {
	ctx := context.Background()
	parent := tempRoot(t)
	root := filepath.Join(parent, "state")
	escape := filepath.Join(parent, "escape")
	oldRoot := filepath.Join(parent, "old-root")
	content := []byte("shard rebind before rename")
	digest := sha256Hex(content)
	shardName := digest[:2]
	oldShard := filepath.Join(oldRoot, "artifacts", "sha256", shardName)
	newShard := filepath.Join(root, "artifacts", "sha256", shardName)
	escapeShard := filepath.Join(escape, "artifacts", "sha256", shardName)

	store := newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			if err := os.MkdirAll(filepath.Dir(oldShard), 0o700); err != nil {
				t.Fatalf("MkdirAll(old shard parent) error = %v", err)
			}
			if err := os.Rename(newShard, oldShard); err != nil {
				t.Fatalf("Rename(opened shard) error = %v", err)
			}
			if err := os.Mkdir(newShard, 0o700); err != nil {
				t.Fatalf("Mkdir(replacement shard) error = %v", err)
			}
			if err := os.MkdirAll(escapeShard, 0o700); err != nil {
				t.Fatalf("MkdirAll(escape shard) error = %v", err)
			}
		},
	})

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrShardChanged) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrShardChanged)
	}
	for _, path := range []string{
		filepath.Join(oldShard, digest),
		filepath.Join(newShard, digest),
		filepath.Join(escapeShard, digest),
	} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("artifact stat %s error = %v, want not exist", path, statErr)
		}
	}
	assertNoStagingResidue(t, store)
}

func TestPublishRejectsFIFOExistingTargetWithoutBlocking(t *testing.T) {
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte("fifo target must not block")
	digest := sha256Hex(content)
	target := artifactPath(root, digest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("MkdirAll(shard) error = %v", err)
	}
	if err := unix.Mkfifo(target, 0o600); err != nil {
		t.Fatalf("Mkfifo(target) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := store.Publish(ctx, bytes.NewReader(content), digest)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrCorruptArtifact) {
			t.Fatalf("Publish() error = %v, want %v", err, ErrCorruptArtifact)
		}
	case <-ctx.Done():
		fd, err := unix.Open(target, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			_ = unix.Close(fd)
		}
		select {
		case err := <-done:
			t.Fatalf("Publish() blocked on FIFO until unblocked; final error = %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("Publish() remained blocked on FIFO after forced unblock")
		}
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat(target) error = %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("target mode = %v, want FIFO preserved", info.Mode())
	}
	assertNoStagingResidue(t, store)

	socketRoot := filepath.Join(shortTempRoot(t), "s")
	socketStore := newStore(t, socketRoot)
	socketContent := []byte("socket target must return typed corruption")
	socketDigest := sha256Hex(socketContent)
	socketTarget := artifactPath(socketRoot, socketDigest)
	if err := os.MkdirAll(filepath.Dir(socketTarget), 0o700); err != nil {
		t.Fatalf("MkdirAll(socket shard) error = %v", err)
	}
	listener, err := net.Listen("unix", socketTarget)
	if err != nil {
		t.Fatalf("Listen(unix socket target) error = %v", err)
	}
	defer listener.Close()

	_, err = socketStore.Publish(context.Background(), bytes.NewReader(socketContent), socketDigest)
	if !errors.Is(err, ErrCorruptArtifact) {
		t.Fatalf("Publish(socket target) error = %v, want %v", err, ErrCorruptArtifact)
	}
	socketInfo, err := os.Lstat(socketTarget)
	if err != nil {
		t.Fatalf("Lstat(socket target) error = %v", err)
	}
	if socketInfo.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket target mode = %v, want socket preserved", socketInfo.Mode())
	}
	assertNoStagingResidue(t, socketStore)
}

func TestPublishEEXISTVerificationHonorsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	root := filepath.Join(tempRoot(t), "state")
	content := []byte("existing target after race")
	digest := sha256Hex(content)
	store := newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			if err := os.WriteFile(newpath, content, 0o600); err != nil {
				t.Fatalf("WriteFile(race target) error = %v", err)
			}
			cancel()
		},
	})

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish() error = %v, want %v", err, context.Canceled)
	}
	assertArtifactBytes(t, artifactPath(root, digest), content)
	assertNoStagingResidue(t, store)
}

func TestPublishRejectsShardSymlinkSwapWithoutFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	escape := filepath.Join(tempRoot(t), "escape")
	if err := os.Mkdir(escape, 0o700); err != nil {
		t.Fatalf("Mkdir(escape) error = %v", err)
	}
	store := newStore(t, root)
	content := []byte("must not escape through shard symlink")
	digest := sha256Hex(content)
	shard := filepath.Join(root, "artifacts", "sha256", digest[:2])
	if err := os.Symlink(escape, shard); err != nil {
		t.Fatalf("Symlink(shard) error = %v", err)
	}

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrStateSymlink) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrStateSymlink)
	}
	if _, statErr := os.Stat(filepath.Join(escape, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("escaped artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishRejectsTargetSymlinkSwapWithoutFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	ctx := context.Background()
	root := filepath.Join(tempRoot(t), "state")
	escape := filepath.Join(tempRoot(t), "escape")
	if err := os.Mkdir(escape, 0o700); err != nil {
		t.Fatalf("Mkdir(escape) error = %v", err)
	}
	store := newStore(t, root)
	content := []byte("must not escape through target symlink")
	digest := sha256Hex(content)
	target := artifactPath(root, digest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("MkdirAll(shard) error = %v", err)
	}
	if err := os.Symlink(filepath.Join(escape, "escaped"), target); err != nil {
		t.Fatalf("Symlink(target) error = %v", err)
	}

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrCorruptArtifact) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrCorruptArtifact)
	}
	if _, statErr := os.Stat(filepath.Join(escape, "escaped")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("escaped artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishRootPathSymlinkSwapFailsBeforeCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	ctx := context.Background()
	parent := tempRoot(t)
	root := filepath.Join(parent, "state")
	escape := filepath.Join(parent, "escape")
	store := newStore(t, root)
	if err := os.Mkdir(escape, 0o700); err != nil {
		t.Fatalf("Mkdir(escape) error = %v", err)
	}
	if err := os.Rename(root, root+".real"); err != nil {
		t.Fatalf("Rename(root) error = %v", err)
	}
	if err := os.Symlink(escape, root); err != nil {
		t.Fatalf("Symlink(root) error = %v", err)
	}
	content := []byte("publish stays inside opened root")
	digest := sha256Hex(content)

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrRootChanged) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrRootChanged)
	}
	if _, statErr := os.Stat(artifactPath(escape, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("escaped artifact stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(artifactPath(root+".real", digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old root artifact stat error = %v, want not exist", statErr)
	}
}

func TestPublishRootRebindBeforeStagingFailsWithoutCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	ctx := context.Background()
	parent := tempRoot(t)
	root := filepath.Join(parent, "state")
	escape := filepath.Join(parent, "escape")
	store := newStoreWithOps(t, root, storeOps{
		beforeStaging: func() {
			if err := os.Mkdir(escape, 0o700); err != nil {
				t.Fatalf("Mkdir(escape) error = %v", err)
			}
			if err := os.Rename(root, root+".old"); err != nil {
				t.Fatalf("Rename(root) error = %v", err)
			}
			if err := os.Symlink(escape, root); err != nil {
				t.Fatalf("Symlink(root) error = %v", err)
			}
		},
	})
	content := []byte("root rebind before staging")
	digest := sha256Hex(content)

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrRootChanged) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrRootChanged)
	}
	if _, statErr := os.Stat(artifactPath(root+".old", digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old root artifact stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(artifactPath(escape, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("escaped artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishRootRebindImmediatelyPreRenameFailsAndCleansStaging(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are platform-specific")
	}
	ctx := context.Background()
	parent := tempRoot(t)
	root := filepath.Join(parent, "state")
	escape := filepath.Join(parent, "escape")
	store := newStoreWithOps(t, root, storeOps{
		beforeRename: func(oldpath, newpath string) {
			if err := os.Mkdir(escape, 0o700); err != nil {
				t.Fatalf("Mkdir(escape) error = %v", err)
			}
			if err := os.Rename(root, root+".old"); err != nil {
				t.Fatalf("Rename(root) error = %v", err)
			}
			if err := os.Symlink(escape, root); err != nil {
				t.Fatalf("Symlink(root) error = %v", err)
			}
		},
	})
	content := []byte("root rebind before rename")
	digest := sha256Hex(content)

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, ErrRootChanged) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrRootChanged)
	}
	if _, statErr := os.Stat(artifactPath(root+".old", digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("old root artifact stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(artifactPath(escape, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("escaped artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishPrecommitCleanupFailureSurfacesTypedError(t *testing.T) {
	ctx := context.Background()
	cleanupErr := errors.New("cleanup refused")
	store := newStoreWithOps(t, filepath.Join(tempRoot(t), "state"), storeOps{
		unlinkStaging: func(dirFD int, name string) error {
			return cleanupErr
		},
	})
	content := []byte("mismatch cleanup failure")
	expected := sha256Hex([]byte("different"))

	_, err := store.Publish(ctx, bytes.NewReader(content), expected)
	if !errors.Is(err, ErrDigestMismatch) || !errors.Is(err, ErrStagingCleanup) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Publish() error = %v, want digest mismatch joined with typed cleanup error", err)
	}
}

func TestCreateStagingFileChmodFailureJoinsCleanupFailure(t *testing.T) {
	chmodErr := errors.New("chmod refused")
	cleanupErr := errors.New("cleanup refused")
	store := newStoreWithOps(t, filepath.Join(tempRoot(t), "state"), storeOps{
		chmodStaging: func(fd int) error {
			return chmodErr
		},
		unlinkStaging: func(dirFD int, name string) error {
			if err := unix.Unlinkat(dirFD, name, 0); err != nil {
				t.Fatalf("Unlinkat(staging) error = %v", err)
			}
			return cleanupErr
		},
	})

	_, _, _, err := store.createStagingFile(store.stagingFD)
	if !errors.Is(err, chmodErr) || !errors.Is(err, ErrStagingCleanup) || !errors.Is(err, cleanupErr) {
		t.Fatalf("createStagingFile() error = %v, want chmod joined with typed cleanup error", err)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishContextCancellationBeforePublicationFailsWithoutCommit(t *testing.T) {
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte("context canceled")
	digest := sha256Hex(content)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Publish(ctx, bytes.NewReader(content), digest)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish() error = %v, want %v", err, context.Canceled)
	}
	if _, statErr := os.Stat(artifactPath(root, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func TestPublishContextCancellationDuringPublicationFailsWithoutCommit(t *testing.T) {
	root := filepath.Join(tempRoot(t), "state")
	store := newStore(t, root)
	content := []byte("context canceled during read")
	digest := sha256Hex(content)
	ctx, cancel := context.WithCancel(context.Background())
	reader := cancelAfterFirstRead{
		reader: bytes.NewReader(content),
		cancel: cancel,
	}

	_, err := store.Publish(ctx, reader, digest)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish() error = %v, want %v", err, context.Canceled)
	}
	if _, statErr := os.Stat(artifactPath(root, digest)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final artifact stat error = %v, want not exist", statErr)
	}
	assertNoStagingResidue(t, store)
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func newStore(t *testing.T, root string) *Store {
	t.Helper()
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() cleanup error = %v", err)
		}
	})
	return store
}

func newStoreWithOps(t *testing.T, root string, ops storeOps) *Store {
	t.Helper()
	store, err := newStoreWithOptions(root, ops)
	if err != nil {
		t.Fatalf("newStoreWithOptions() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() cleanup error = %v", err)
		}
	})
	return store
}

func newStoreWithPermissiveUmask(t *testing.T, root string) *Store {
	t.Helper()
	var store *Store
	withPermissiveUmask(t, func() {
		store = newStore(t, root)
	})
	return store
}

func withPermissiveUmask(t *testing.T, fn func()) {
	t.Helper()
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	fn()
}

func tempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(temp root) error = %v", err)
	}
	return root
}

func shortTempRoot(t *testing.T) string {
	t.Helper()
	var root string
	for i := 0; i < 16; i++ {
		candidate := fmt.Sprintf("/tmp/l%x", i)
		err := os.Mkdir(candidate, 0o700)
		if err == nil {
			root = candidate
			break
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("Mkdir(short root) error = %v", err)
		}
	}
	if root == "" {
		t.Fatal("no available short temp root")
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Fatalf("RemoveAll(short root) error = %v", err)
		}
	})
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(short root) error = %v", err)
	}
	return root
}

func assertDirMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}

func assertNoStagingResidue(t *testing.T, store *Store) {
	t.Helper()
	entries, err := os.ReadDir(store.stagingDir)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir(staging) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("staging entries = %d, want 0", len(entries))
	}
}

func assertArtifactBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s bytes = %q, want %q", path, got, want)
	}
}

func artifactPath(root, digest string) string {
	return filepath.Join(root, "artifacts", "sha256", digest[:2], digest)
}

func TestStoreCloseReleasesDirectoryDescriptors(t *testing.T) {
	store := newStore(t, filepath.Join(tempRoot(t), "state"))
	rootFD := store.rootFD
	if rootFD < 0 {
		t.Fatalf("rootFD = %d, want open descriptor", rootFD)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := unix.Fstat(rootFD, &unix.Stat_t{}); !errors.Is(err, unix.EBADF) {
		t.Fatalf("Fstat(closed rootFD) error = %v, want EBADF", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCloseWaitsForActivePublishAndRejectsLaterPublish(t *testing.T) {
	ctx := context.Background()
	store := newStoreWithOps(t, filepath.Join(tempRoot(t), "state"), storeOps{})
	started := make(chan struct{})
	release := make(chan struct{})
	store.ops.beforeRename = func(oldpath, newpath string) {
		close(started)
		<-release
	}
	content := []byte("close waits for active publish")
	digest := sha256Hex(content)
	publishDone := make(chan error, 1)
	go func() {
		_, err := store.Publish(ctx, bytes.NewReader(content), digest)
		publishDone <- err
	}()
	<-started
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- store.Close()
	}()
	select {
	case err := <-closeDone:
		t.Fatalf("Close() finished before active Publish released: %v", err)
	default:
	}
	close(release)
	if err := <-publishDone; err != nil {
		t.Fatalf("active Publish() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := store.Publish(ctx, bytes.NewReader(content), digest); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("later Publish() error = %v, want %v", err, ErrStoreClosed)
	}
}

func TestCloseWaitsForSameDigestQueuedPublish(t *testing.T) {
	ctx := context.Background()
	store := newStoreWithOps(t, filepath.Join(tempRoot(t), "state"), storeOps{})
	firstStarted := make(chan struct{})
	secondLeased := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	store.ops.afterLease = func() {
		select {
		case <-firstStarted:
			select {
			case secondLeased <- struct{}{}:
			default:
			}
		default:
		}
	}
	store.ops.beforeRename = func(oldpath, newpath string) {
		select {
		case <-firstStarted:
		default:
			close(firstStarted)
			<-releaseFirst
		}
	}
	content := []byte("same digest queued close")
	digest := sha256Hex(content)
	firstDone := make(chan error, 1)
	go func() {
		_, err := store.Publish(ctx, bytes.NewReader(content), digest)
		firstDone <- err
	}()
	<-firstStarted
	secondDone := make(chan error, 1)
	go func() {
		_, err := store.Publish(ctx, bytes.NewReader(content), digest)
		secondDone <- err
	}()
	<-secondLeased
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- store.Close()
	}()
	select {
	case err := <-closeDone:
		t.Fatalf("Close() finished while same-digest Publish was queued: %v", err)
	default:
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("queued Publish() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := store.Publish(ctx, bytes.NewReader(content), digest); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("later Publish() error = %v, want %v", err, ErrStoreClosed)
	}
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

type cancelAfterFirstRead struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (r cancelAfterFirstRead) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}
