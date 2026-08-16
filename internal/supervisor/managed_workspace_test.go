package supervisor

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestManagedWorkspaceCopyDigestChangesCleanup(t *testing.T) { // s3_w4_workspace_copy_digest_changes_cleanup
	root := privateDirectory(t, "managed-root")
	source := privateDirectory(t, "source")
	writeManagedTestFile(t, filepath.Join(source, "alpha.txt"), []byte("alpha\n"), 0o600)
	writeManagedTestFile(
		t,
		filepath.Join(source, "bin", "run"),
		[]byte("#!/fixture\n"),
		0o700,
	)
	writeManagedTestFile(
		t,
		filepath.Join(source, ".git", "config"),
		[]byte("credential=must-not-copy\n"),
		0o600,
	)

	workspace, err := prepareManagedWorkspace(root, source)
	if err != nil {
		t.Fatalf("prepareManagedWorkspace() error = %v", err)
	}
	invocationPath := workspace.invocationPath
	for _, path := range []string{
		workspace.invocationPath,
		workspace.workspacePath,
		workspace.homePath,
		workspace.tempPath,
	} {
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatalf("private directory %q = %#v, %v", path, info, statErr)
		}
	}
	if _, err := os.Lstat(filepath.Join(workspace.workspacePath, ".git")); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf(".git copied into workspace: %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(workspace.workspacePath, "alpha.txt"))
	if err != nil || string(copied) != "alpha\n" {
		t.Fatalf("copied alpha = %q, %v", copied, err)
	}
	executable, err := os.Lstat(filepath.Join(workspace.workspacePath, "bin", "run"))
	if err != nil || executable.Mode().Perm() != 0o700 {
		t.Fatalf("executable mode = %#o, %v", executable.Mode().Perm(), err)
	}
	if workspace.sourceDigest == "" || workspace.initialWorkspaceDigest == "" {
		t.Fatalf("empty workspace digests: %#v", workspace)
	}
	if err := workspace.verifySourceUnchanged(); err != nil {
		t.Fatalf("verifySourceUnchanged() error = %v", err)
	}

	writeManagedTestFile(
		t,
		filepath.Join(workspace.workspacePath, "alpha.txt"),
		[]byte("changed\n"),
		0o600,
	)
	if err := os.Remove(filepath.Join(workspace.workspacePath, "bin", "run")); err != nil {
		t.Fatal(err)
	}
	writeManagedTestFile(
		t,
		filepath.Join(workspace.workspacePath, "new.txt"),
		[]byte("new\n"),
		0o600,
	)
	digest, changes, err := workspace.collectChanges()
	if err != nil {
		t.Fatalf("collectChanges() error = %v", err)
	}
	if digest == "" || digest == workspace.initialWorkspaceDigest {
		t.Fatalf("final workspace digest = %q", digest)
	}
	got := make([]string, 0, len(changes))
	for _, change := range changes {
		got = append(got, change.Path()+":"+string(change.Kind()))
	}
	want := []string{
		"alpha.txt:modified",
		"bin/run:deleted",
		"new.txt:added",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %#v, want %#v", got, want)
	}
	if string(changes[0].Content()) != "changed\n" ||
		len(changes[1].Content()) != 0 ||
		string(changes[2].Content()) != "new\n" {
		t.Fatalf("change content = %#v", changes)
	}
	content := changes[0].Content()
	content[0] = 'X'
	if string(changes[0].Content()) != "changed\n" {
		t.Fatal("WorkspaceChange.Content aliases caller")
	}
	if err := workspace.cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Lstat(invocationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invocation root remains after cleanup: %v", err)
	}
}

func TestManagedWorkspaceRejectsHardlinksSymlinksAndBounds(t *testing.T) { // s3_w4_bounds_failure_fuzz_static
	t.Run("source symlink", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		outside := filepath.Join(t.TempDir(), "outside")
		writeManagedTestFile(t, outside, []byte("outside"), 0o600)
		if err := os.Symlink(outside, filepath.Join(source, "escape")); err != nil {
			t.Fatal(err)
		}
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("symlink workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("source hardlink", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		outside := filepath.Join(t.TempDir(), "outside")
		writeManagedTestFile(t, outside, []byte("outside"), 0o600)
		if err := os.Link(outside, filepath.Join(source, "alias")); err != nil {
			t.Fatal(err)
		}
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("hardlink workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("two in-tree names for one inode", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		first := filepath.Join(source, "first")
		writeManagedTestFile(t, first, []byte("shared"), 0o600)
		if err := os.Link(first, filepath.Join(source, "second")); err != nil {
			t.Fatal(err)
		}
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("in-tree hardlink workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("workspace hardlink", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		writeManagedTestFile(t, filepath.Join(source, "safe"), []byte("safe"), 0o600)
		workspace, err := prepareManagedWorkspace(root, source)
		if err != nil {
			t.Fatal(err)
		}
		defer workspace.cleanup()
		outside := filepath.Join(t.TempDir(), "outside")
		writeManagedTestFile(t, outside, []byte("secret"), 0o600)
		if err := os.Link(outside, filepath.Join(workspace.workspacePath, "alias")); err != nil {
			t.Fatal(err)
		}
		if _, changes, err := workspace.collectChanges(); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || len(changes) != 0 {
			t.Fatalf("hardlink changes = %#v, %v", changes, err)
		}
	})

	t.Run("two in-workspace names for one inode", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		writeManagedTestFile(t, filepath.Join(source, "safe"), []byte("safe"), 0o600)
		workspace, err := prepareManagedWorkspace(root, source)
		if err != nil {
			t.Fatal(err)
		}
		defer workspace.cleanup()
		first := filepath.Join(workspace.workspacePath, "first")
		writeManagedTestFile(t, first, []byte("shared"), 0o600)
		if err := os.Link(first, filepath.Join(workspace.workspacePath, "second")); err != nil {
			t.Fatal(err)
		}
		if _, changes, err := workspace.collectChanges(); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || len(changes) != 0 {
			t.Fatalf("in-workspace hardlink changes = %#v, %v", changes, err)
		}
	})

	t.Run("symlink replacement during no-follow open", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		target := filepath.Join(source, "target")
		writeManagedTestFile(t, target, []byte("before"), 0o600)
		outside := filepath.Join(t.TempDir(), "outside")
		writeManagedTestFile(t, outside, []byte("outside"), 0o600)

		originalHook := managedTraversalBeforeOpen
		var replace sync.Once
		managedTraversalBeforeOpen = func(relative string) {
			if relative == "target" {
				replace.Do(func() {
					if err := os.Rename(target, target+".original"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, target); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
		defer func() { managedTraversalBeforeOpen = originalHook }()

		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("replacement workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("intermediate directory replacement during rooted open", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		directory := filepath.Join(source, "nested")
		writeManagedTestFile(t, filepath.Join(directory, "target"), []byte("same inode"), 0o600)

		originalHook := managedTraversalBeforeOpen
		var replace sync.Once
		managedTraversalBeforeOpen = func(relative string) {
			if relative == "nested" {
				replace.Do(func() {
					renamed := directory + "-original"
					if err := os.Rename(directory, renamed); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(renamed, directory); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
		defer func() { managedTraversalBeforeOpen = originalHook }()

		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("intermediate replacement workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("source file bound", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		path := filepath.Join(source, "large")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxManagedSourceFileBytes + 1); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("oversized workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("source entry bound", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		for index := 0; index <= maxManagedSourceEntries; index++ {
			writeManagedTestFile(
				t,
				filepath.Join(source, fmt.Sprintf("entry-%04d", index)),
				nil,
				0o600,
			)
		}
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("entry-bound workspace = %#v, %v", workspace, err)
		}
	})

	t.Run("changed file and count bounds", func(t *testing.T) {
		root := privateDirectory(t, "root")
		source := privateDirectory(t, "source")
		writeManagedTestFile(t, filepath.Join(source, "safe"), []byte("safe"), 0o600)
		workspace, err := prepareManagedWorkspace(root, source)
		if err != nil {
			t.Fatal(err)
		}
		defer workspace.cleanup()
		largePath := filepath.Join(workspace.workspacePath, "large")
		large, err := os.OpenFile(largePath, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := large.Truncate(maxManagedChangeFileBytes + 1); err != nil {
			large.Close()
			t.Fatal(err)
		}
		if err := large.Close(); err != nil {
			t.Fatal(err)
		}
		if _, changes, err := workspace.collectChanges(); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || len(changes) != 0 {
			t.Fatalf("large change = %#v, %v", changes, err)
		}
		if err := os.Remove(largePath); err != nil {
			t.Fatal(err)
		}
		for index := 0; index <= maxManagedChanges; index++ {
			writeManagedTestFile(
				t,
				filepath.Join(workspace.workspacePath, fmt.Sprintf("change-%04d", index)),
				nil,
				0o600,
			)
		}
		if _, changes, err := workspace.collectChanges(); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || len(changes) != 0 {
			t.Fatalf("change count = %#v, %v", changes, err)
		}
	})

	t.Run("special filesystem entry", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix socket fixture")
		}
		root := privateDirectory(t, "root")
		source, err := os.MkdirTemp("/tmp", "loom-w4-socket-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(source) })
		if err := os.Chmod(source, 0o700); err != nil {
			t.Fatal(err)
		}
		listener, err := net.Listen("unix", filepath.Join(source, "socket"))
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		if workspace, err := prepareManagedWorkspace(root, source); !errors.Is(
			err,
			ErrManagedWorkspace,
		) || workspace != nil {
			t.Fatalf("special-entry workspace = %#v, %v", workspace, err)
		}
	})
}

func FuzzManagedWorkspaceAndSessionNeverPanic(f *testing.F) {
	for _, seed := range []string{
		"",
		"alpha.txt",
		"../escape",
		"/absolute",
		"a\x00b",
		strings.Repeat("x", 4096),
	} {
		f.Add(seed, []byte(`{"status":"succeeded","reason":""}`))
	}
	f.Fuzz(func(t *testing.T, path string, payload []byte) {
		_ = validManagedRelativePath(path)
		_, _, _ = parseManagedResultPayload(payload)
		_, _ = bridgev1.DecodeLine(payload)
		if !validManagedRelativePath(path) ||
			len(path) > 256 ||
			len(payload) > 1024 ||
			filepath.ToSlash(path) == ".git" {
			return
		}
		root := privateDirectory(t, "fuzz-root")
		source := privateDirectory(t, "fuzz-source")
		target := filepath.Join(source, path)
		if relative, err := filepath.Rel(source, target); err != nil ||
			!validManagedRelativePath(relative) {
			return
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return
		}
		if err := os.WriteFile(target, append([]byte(nil), payload...), 0o600); err != nil {
			return
		}
		if err := os.Chmod(target, 0o600); err != nil {
			return
		}
		workspace, err := prepareManagedWorkspace(root, source)
		if err == nil {
			if cleanupErr := workspace.cleanup(); cleanupErr != nil {
				t.Fatal(cleanupErr)
			}
		}
	})
}

func TestS3W4StaticOwnedBoundary(t *testing.T) {
	supervisorDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(supervisorDirectory, "managed_execution.go"),
		filepath.Join(supervisorDirectory, "managed_workspace.go"),
		filepath.Join(supervisorDirectory, "managed_workspace_unix.go"),
		filepath.Join(supervisorDirectory, "managed_workspace_other.go"),
		filepath.Join(supervisorDirectory, "..", "runtime", "piadapter", "execution_adapter.go"),
		filepath.Join(supervisorDirectory, "..", "runtime", "piadapter", "execution_process_unix.go"),
		filepath.Join(supervisorDirectory, "..", "runtime", "piadapter", "execution_process_other.go"),
	}
	allowedExports := map[string]struct{}{
		"ErrInvalidManagedExecution": {}, "ErrManagedWorkspace": {},
		"ErrSourceChanged": {}, "ErrRuntimeAdapter": {},
		"ErrBridgeSession": {}, "ErrRuntimeTimeout": {},
		"ErrRuntimeCancelled": {}, "ErrProcessCleanup": {},
		"ErrAuthorizedFrameObserver": {},
		"WorkspaceChangeKind":        {}, "WorkspaceChangeAdded": {},
		"WorkspaceChangeModified": {}, "WorkspaceChangeDeleted": {},
		"RuntimeAdapter": {}, "AdapterRequest": {}, "AdapterResultInput": {},
		"FrameSink": {}, "AcceptFrame": {}, "AuthorizedFrame": {},
		"Frame": {}, "Tentative": {}, "AuthorizedFrameObserver": {},
		"ObserveAuthorizedFrame": {}, "FrameObserver": {},
		"AdapterResult": {}, "Config": {}, "ExecuteInput": {},
		"WorkspaceChange": {}, "Outcome": {}, "Supervisor": {},
		"NewAdapterResult": {}, "New": {}, "InboundFrames": {}, "Stderr": {},
		"ExitCode": {}, "DispatchAcknowledged": {}, "ResultAcknowledged": {},
		"CancelAcknowledged": {}, "Accounting": {}, "Path": {}, "Kind": {}, "Mode": {},
		"Digest": {}, "Content": {}, "WorkItem": {}, "Run": {}, "Stream": {},
		"SourceDigest": {}, "WorkspaceDigest": {}, "Changes": {}, "Execute": {},
		"SourceSnapshot": {}, "ObserveSourceSnapshot": {}, "Valid": {},
		"TreeDigest": {}, "EntryCount": {}, "FileCount": {},
		"DirectoryCount": {}, "TotalBytes": {},
		"AdapterType": {}, "RuntimeInstanceID": {}, "WorkspacePath": {},
		"WorkspaceRoot": {},
		"HomePath":      {}, "TempPath": {}, "Binding": {}, "Dispatch": {},
		"ClaimID": {}, "IncidentID": {},
		"ExpectedSourceDigest": {},
		"ExecutionBinding":     {},
		"ContextCapsule":       {},
		"RouteSegment":         {},
		"ContextRetriever":     {},
		"ContextDelivery":      {},
		"AgentInputs":          {},
		"Grant":                {}, "CleanupTimeout": {}, "SourcePath": {}, "Profile": {},
		"Instance": {}, "Generation": {},
		"ErrInvalidPiExecutionAdapter": {}, "ErrPiExecutionBindingChanged": {},
		"ErrPiExecutionProcess": {}, "ErrPiExecutionProtocol": {},
		"ErrPiExecutionOutputTooLarge": {}, "ErrPiExecutionCleanup": {},
		"PiExecutionAdapterConfig": {}, "ExecutablePath": {}, "Arguments": {},
		"RuntimeSearchPaths": {}, "CancelGrace": {}, "Now": {}, "Random": {},
		"NewPiExecutionAdapter": {},
	}
	forbiddenImports := map[string]struct{}{
		"net": {}, "net/http": {}, "os/user": {},
		"loom-pi-rebuild/internal/app":        {},
		"loom-pi-rebuild/internal/agents":     {},
		"loom-pi-rebuild/internal/evidence":   {},
		"loom-pi-rebuild/internal/projection": {},
		"loom-pi-rebuild/internal/state":      {},
		"loom-pi-rebuild/internal/teams":      {},
	}
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, forbidden := range [][]byte{
			[]byte("http://"), []byte("https://"), []byte("npm "),
			[]byte("pnpm "), []byte("yarn "), []byte("bash"),
			[]byte("zsh"), []byte("sh -c"), []byte("os.Getenv"),
			[]byte("os.Environ"), []byte("exec.LookPath"),
			[]byte("exec.CommandContext"),
			[]byte("StdoutPipe"), []byte("StderrPipe"),
		} {
			if bytes.Contains(content, forbidden) {
				t.Fatalf("%s contains forbidden capability %q", path, forbidden)
			}
		}
		file, parseErr := parser.ParseFile(
			token.NewFileSet(),
			path,
			content,
			0,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, imported := range file.Imports {
			name := strings.Trim(imported.Path.Value, `"`)
			if _, forbidden := forbiddenImports[name]; forbidden {
				t.Fatalf("%s imports forbidden package %q", path, name)
			}
		}
		checkExport := func(identifier *ast.Ident) {
			if identifier == nil || !identifier.IsExported() {
				return
			}
			if _, allowed := allowedExports[identifier.Name]; !allowed {
				t.Errorf("%s exposes unexpected identifier %q", path, identifier.Name)
			}
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				checkExport(value.Name)
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					switch item := specification.(type) {
					case *ast.TypeSpec:
						checkExport(item.Name)
						switch shape := item.Type.(type) {
						case *ast.StructType:
							for _, field := range shape.Fields.List {
								for _, name := range field.Names {
									checkExport(name)
								}
							}
						case *ast.InterfaceType:
							for _, field := range shape.Methods.List {
								for _, name := range field.Names {
									checkExport(name)
								}
							}
						}
					case *ast.ValueSpec:
						for _, name := range item.Names {
							checkExport(name)
						}
					}
				}
			}
		}
	}
}

func privateDirectory(t testing.TB, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeManagedTestFile(
	t testing.TB,
	path string,
	content []byte,
	mode os.FileMode,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte(nil), content...), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func sortedManagedChangePaths(changes []WorkspaceChange) []string {
	output := make([]string, 0, len(changes))
	for _, change := range changes {
		output = append(output, change.Path())
	}
	sort.Strings(output)
	return output
}

func assertManagedBytesAbsent(
	t testing.TB,
	secret []byte,
	values ...[]byte,
) {
	t.Helper()
	for _, value := range values {
		if bytes.Contains(value, secret) {
			t.Fatalf("secret leaked in %q", value)
		}
	}
}
