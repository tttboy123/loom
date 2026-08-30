//go:build darwin

package piadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const (
	lockedPiComponentLineage = "p2a-w2-pi-component-20260730-001"
	lockedPiComponentRoot    = "/Users/lune/Library/Application Support/Loom/p2a-w2-pi-component-20260730-001"
	lockedPiManifestPath     = lockedPiComponentRoot + "/manifest.json"
	lockedPiIsolationRoot    = lockedPiComponentRoot + "/isolation"

	lockedPiLocalModelPrivateRoot = "/Users/lune/Library/Application Support/Loom/phase1-live"
	lockedPiSearchEntry           = "/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin/pi"
	lockedPiResolvedEntry         = "/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/@earendil-works/pi-coding-agent/dist/cli.js"
	lockedPiEntrySHA256           = "af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca"
	lockedPiEntrySize             = int64(681)
	lockedPiNodeExecutable        = "/Users/lune/Documents/Codex/devtools/node-v24.16.0-darwin-arm64/bin/node"
	lockedPiNodeSHA256            = "1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8"
	lockedPiNodeSize              = int64(120573328)
	lockedPiLlamaArchive          = "/Users/lune/Library/Application Support/Loom/phase1-live/sources/llama-b10107-bin-macos-arm64.tar.gz"
	lockedPiLlamaArchiveSHA256    = "b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32"
	lockedPiLlamaArchiveSize      = int64(10804162)
	lockedPiLlamaExecutable       = "/Users/lune/Library/Application Support/Loom/phase1-live/runtime/llama-b10107/llama-server"
	lockedPiLlamaSHA256           = "a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b"
	lockedPiLlamaSize             = int64(33472)
	lockedPiModelPath             = "/Users/lune/Library/Application Support/Loom/phase1-live/models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
	lockedPiModelSize             = int64(1117320768)
	lockedPiModelSHA256           = "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046"
	lockedPiExpectedVersion       = "0.82.1"
	lockedPiExpectedModelID       = "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"
)

var lockedPiRuntimeSearchPaths = []string{
	"/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin",
	"/Users/lune/Documents/Codex/devtools/node-v24.16.0-darwin-arm64/bin",
}

type lockedPiComponentManifest struct {
	SchemaVersion         int                          `json:"schema_version"`
	LineageID             string                       `json:"lineage_id"`
	ComponentRoot         string                       `json:"component_root"`
	IsolationRoot         string                       `json:"isolation_root"`
	LocalModelPrivateRoot string                       `json:"local_model_private_root"`
	PiSearchEntry         lockedPiManifestResolvedFile `json:"pi_search_entry"`
	NodeExecutable        lockedPiManifestFile         `json:"node_executable"`
	RuntimeSearchPaths    []string                     `json:"runtime_search_paths"`
	LlamaArchive          lockedPiManifestFile         `json:"llama_archive"`
	LlamaExecutable       lockedPiManifestFile         `json:"llama_executable"`
	Model                 lockedPiManifestModel        `json:"model"`
	Timeout               string                       `json:"timeout"`
	ExpectedVersion       string                       `json:"expected_version"`
	ExpectedModelID       string                       `json:"expected_model_id"`
	NetworkPolicy         string                       `json:"network_policy"`
}

type lockedPiManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type lockedPiManifestResolvedFile struct {
	Path         string `json:"path"`
	ResolvedPath string `json:"resolved_path"`
	SHA256       string `json:"sha256"`
}

type lockedPiManifestModel struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type lockedPiFileIdentity struct {
	path   string
	info   os.FileInfo
	mode   os.FileMode
	size   int64
	digest string
}

func TestLockedPiComponent(t *testing.T) {
	if os.Getenv("LOOM_P2A_W2_LOCKED_PI_COMPONENT") !=
		lockedPiComponentLineage ||
		os.Getenv("LOOM_P2A_W2_LOCKED_PI_MANIFEST") !=
			lockedPiManifestPath {
		t.Skip("locked Pi component gate closed")
	}

	manifest, before, gateReason := validateLockedPiComponentGate()
	if gateReason != "" {
		t.Skip("locked Pi component gate closed: " + gateReason)
	}
	defer func() {
		_, _, postReason := validateLockedPiComponentGate()
		if postReason != "" {
			t.Errorf(
				"locked Pi component postcondition failed: %s",
				postReason,
			)
			return
		}
		if reason := revalidateLockedPiComponentFiles(before); reason != "" {
			t.Errorf(
				"locked Pi component postcondition failed: %s",
				reason,
			)
		}
	}()

	timeout, err := time.ParseDuration(manifest.Timeout)
	if err != nil {
		t.Skip("locked Pi component gate closed: manifest_timeout")
	}
	factory, err := NewPiLocalRuntimeProbeFactory(
		PiLocalRuntimeProbeFactoryConfig{
			ProbeID:            "p2a-w2-locked-pi-probe",
			InstanceID:         "p2a-w2-locked-pi-runtime",
			DeviceID:           "p2a-w2-locked-local-device",
			DisplayName:        "Locked Pi 0.82.1",
			IsolationRoot:      manifest.IsolationRoot,
			RuntimeSearchPaths: append([]string(nil), manifest.RuntimeSearchPaths...),
			Timeout:            timeout,
			LocalModelCatalog: &PiLocalModelCatalogConfig{
				PrivateRoot:        manifest.LocalModelPrivateRoot,
				RuntimeArchivePath: manifest.LlamaArchive.Path,
				ExecutablePath:     manifest.LlamaExecutable.Path,
				ModelPath:          manifest.Model.Path,
			},
		},
	)
	if err != nil {
		t.Fatalf("locked Pi component failed: component_factory")
	}
	probe, present, err := factory.BuildProbe(context.Background())
	if err != nil || !present || probe == nil {
		t.Fatalf("locked Pi component failed: component_probe")
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{probe},
	)
	if err != nil {
		t.Fatalf(
			"locked Pi component failed: %s",
			lockedPiComponentFailureReason(err),
		)
	}
	observations := snapshot.Observations()
	if len(observations) != 1 ||
		observations[0].SourceProbeID != "p2a-w2-locked-pi-probe" ||
		observations[0].Instance.ID != "p2a-w2-locked-pi-runtime" ||
		observations[0].Instance.ExecutableVersion !=
			manifest.ExpectedVersion ||
		!reflect.DeepEqual(
			observations[0].ModelIDs,
			[]string{manifest.ExpectedModelID},
		) {
		t.Fatalf("locked Pi component failed: component_observation")
	}
	if !lockedPiDirectoryEmpty(manifest.IsolationRoot) {
		t.Fatalf("locked Pi component failed: component_residue")
	}
}

func TestLockedPiManifestDecoderIsStrictAndDuplicateClosed(t *testing.T) {
	valid, err := json.Marshal(expectedLockedPiManifest())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeLockedPiManifest(valid)
	if err != nil || !validLockedPiManifest(decoded) {
		t.Fatalf("valid manifest rejected")
	}

	tests := []struct {
		name    string
		content []byte
	}{
		{
			name: "top-level duplicate",
			content: bytes.Replace(
				valid,
				[]byte(`"schema_version":1`),
				[]byte(`"schema_version":1,"schema_version":1`),
				1,
			),
		},
		{
			name: "nested duplicate",
			content: bytes.Replace(
				valid,
				[]byte(`"sha256":"`+lockedPiEntrySHA256+`"`),
				[]byte(
					`"sha256":"`+lockedPiEntrySHA256+
						`","sha256":"`+lockedPiEntrySHA256+`"`,
				),
				1,
			),
		},
		{
			name: "top-level unknown",
			content: bytes.Replace(
				valid,
				[]byte(`"schema_version":1`),
				[]byte(`"schema_version":1,"unknown":true`),
				1,
			),
		},
		{
			name: "nested unknown",
			content: bytes.Replace(
				valid,
				[]byte(`"path":"`+lockedPiNodeExecutable+`"`),
				[]byte(
					`"path":"`+lockedPiNodeExecutable+
						`","resolved_path":"unaccepted"`,
				),
				1,
			),
		},
		{
			name:    "trailing value",
			content: append(append([]byte(nil), valid...), []byte(` {}`)...),
		},
		{
			name:    "malformed",
			content: []byte(`{"schema_version":1`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeLockedPiManifest(test.content); err == nil {
				t.Fatal("unsafe manifest accepted")
			}
		})
	}
}

func validateLockedPiComponentGate() (
	lockedPiComponentManifest,
	[]lockedPiFileIdentity,
	string,
) {
	if os.Geteuid() != 501 {
		return lockedPiComponentManifest{}, nil, "component_owner"
	}
	if !lockedPiPathChainHasNoSymlink(lockedPiComponentRoot) ||
		!lockedPiDirectory(
			lockedPiComponentRoot,
			0o700,
		) ||
		!lockedPiDirectory(
			lockedPiIsolationRoot,
			0o700,
		) ||
		!lockedPiDirectoryEmpty(lockedPiIsolationRoot) ||
		!lockedPiComponentTreeExact() {
		return lockedPiComponentManifest{}, nil, "component_root"
	}
	manifestInfo, err := os.Lstat(lockedPiManifestPath)
	if err != nil ||
		!manifestInfo.Mode().IsRegular() ||
		manifestInfo.Mode()&os.ModeSymlink != 0 ||
		manifestInfo.Mode().Perm() != 0o600 ||
		!piLocalCurrentUserOwns(manifestInfo) {
		return lockedPiComponentManifest{}, nil, "component_manifest"
	}
	content, err := os.ReadFile(lockedPiManifestPath)
	if err != nil {
		return lockedPiComponentManifest{}, nil, "component_manifest"
	}
	manifest, err := decodeLockedPiManifest(content)
	if err != nil || !validLockedPiManifest(manifest) {
		return lockedPiComponentManifest{}, nil, "component_manifest"
	}

	piEntryInfo, err := os.Lstat(manifest.PiSearchEntry.Path)
	if err != nil ||
		piEntryInfo.Mode()&os.ModeSymlink == 0 ||
		!piLocalCurrentUserOwns(piEntryInfo) ||
		!lockedPiPathChainHasNoSymlink(filepath.Dir(manifest.PiSearchEntry.Path)) {
		return lockedPiComponentManifest{}, nil, "component_pi_entry"
	}
	resolved, err := filepath.EvalSymlinks(manifest.PiSearchEntry.Path)
	if err != nil || resolved != manifest.PiSearchEntry.ResolvedPath {
		return lockedPiComponentManifest{}, nil, "component_pi_entry"
	}

	checks := []struct {
		path       string
		mode       os.FileMode
		size       int64
		digest     string
		executable bool
		reason     string
	}{
		{
			path:       manifest.PiSearchEntry.ResolvedPath,
			mode:       0o700,
			size:       lockedPiEntrySize,
			digest:     manifest.PiSearchEntry.SHA256,
			executable: true,
			reason:     "component_pi_identity",
		},
		{
			path:       manifest.NodeExecutable.Path,
			mode:       0o755,
			size:       lockedPiNodeSize,
			digest:     manifest.NodeExecutable.SHA256,
			executable: true,
			reason:     "component_node_identity",
		},
		{
			path:   manifest.LlamaArchive.Path,
			mode:   0o600,
			size:   lockedPiLlamaArchiveSize,
			digest: manifest.LlamaArchive.SHA256,
			reason: "component_llama_archive_identity",
		},
		{
			path:       manifest.LlamaExecutable.Path,
			mode:       0o700,
			size:       lockedPiLlamaSize,
			digest:     manifest.LlamaExecutable.SHA256,
			executable: true,
			reason:     "component_llama_identity",
		},
		{
			path:   manifest.Model.Path,
			mode:   0o600,
			size:   manifest.Model.Size,
			digest: manifest.Model.SHA256,
			reason: "component_model_identity",
		},
	}
	before := make([]lockedPiFileIdentity, 0, len(checks))
	for _, check := range checks {
		identity, ok := inspectLockedPiFile(
			check.path,
			check.mode,
			check.size,
			check.digest,
			check.executable,
		)
		if !ok {
			return lockedPiComponentManifest{}, nil, check.reason
		}
		before = append(before, identity)
	}
	if !lockedPiDirectory(
		manifest.LocalModelPrivateRoot,
		0o700,
	) {
		return lockedPiComponentManifest{}, nil, "component_model_root"
	}
	searchModes := []os.FileMode{0o700, 0o755}
	for index, path := range manifest.RuntimeSearchPaths {
		if !lockedPiDirectory(path, searchModes[index]) {
			return lockedPiComponentManifest{}, nil, "component_search_path"
		}
	}
	if !lockedPiOfflineEnvironment(manifest) {
		return lockedPiComponentManifest{}, nil, "component_environment"
	}
	if !lockedPiComponentQuiescent() {
		return lockedPiComponentManifest{}, nil, "component_activity"
	}
	return manifest, before, ""
}

func decodeLockedPiManifest(
	content []byte,
) (lockedPiComponentManifest, error) {
	if err := rejectLockedPiDuplicateJSONKeys(content); err != nil {
		return lockedPiComponentManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest lockedPiComponentManifest
	if err := decoder.Decode(&manifest); err != nil {
		return lockedPiComponentManifest{}, err
	}
	if err := requireLockedPiJSONEOF(decoder); err != nil {
		return lockedPiComponentManifest{}, err
	}
	return manifest, nil
}

func validLockedPiManifest(manifest lockedPiComponentManifest) bool {
	return reflect.DeepEqual(manifest, expectedLockedPiManifest())
}

func expectedLockedPiManifest() lockedPiComponentManifest {
	return lockedPiComponentManifest{
		SchemaVersion:         1,
		LineageID:             lockedPiComponentLineage,
		ComponentRoot:         lockedPiComponentRoot,
		IsolationRoot:         lockedPiIsolationRoot,
		LocalModelPrivateRoot: lockedPiLocalModelPrivateRoot,
		PiSearchEntry: lockedPiManifestResolvedFile{
			Path:         lockedPiSearchEntry,
			ResolvedPath: lockedPiResolvedEntry,
			SHA256:       lockedPiEntrySHA256,
		},
		NodeExecutable: lockedPiManifestFile{
			Path:   lockedPiNodeExecutable,
			SHA256: lockedPiNodeSHA256,
		},
		RuntimeSearchPaths: append(
			[]string(nil),
			lockedPiRuntimeSearchPaths...,
		),
		LlamaArchive: lockedPiManifestFile{
			Path:   lockedPiLlamaArchive,
			SHA256: lockedPiLlamaArchiveSHA256,
		},
		LlamaExecutable: lockedPiManifestFile{
			Path:   lockedPiLlamaExecutable,
			SHA256: lockedPiLlamaSHA256,
		},
		Model: lockedPiManifestModel{
			Path:   lockedPiModelPath,
			Size:   lockedPiModelSize,
			SHA256: lockedPiModelSHA256,
		},
		Timeout:         "10s",
		ExpectedVersion: lockedPiExpectedVersion,
		ExpectedModelID: lockedPiExpectedModelID,
		NetworkPolicy:   "offline_no_listener",
	}
}

func rejectLockedPiDuplicateJSONKeys(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := consumeLockedPiJSONValue(decoder); err != nil {
		return err
	}
	return requireLockedPiJSONEOF(decoder)
}

func consumeLockedPiJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid locked Pi manifest object key")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate locked Pi manifest object key")
			}
			seen[key] = struct{}{}
			if err := consumeLockedPiJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid locked Pi manifest object")
		}
	case '[':
		for decoder.More() {
			if err := consumeLockedPiJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid locked Pi manifest array")
		}
	default:
		return errors.New("invalid locked Pi manifest delimiter")
	}
	return nil
}

func requireLockedPiJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing locked Pi manifest value")
		}
		return err
	}
	return nil
}

func lockedPiPathChainHasNoSymlink(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	current := string(filepath.Separator)
	relative := path[1:]
	for relative != "." && relative != "" {
		first := relative
		if separator := bytes.IndexByte(
			[]byte(relative),
			byte(os.PathSeparator),
		); separator >= 0 {
			first = relative[:separator]
			relative = relative[separator+1:]
		} else {
			relative = ""
		}
		if first == "" || first == "." || first == ".." {
			return false
		}
		current = filepath.Join(current, first)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func lockedPiDirectory(path string, mode os.FileMode) bool {
	if !lockedPiPathChainHasNoSymlink(path) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil &&
		info.IsDir() &&
		info.Mode()&os.ModeSymlink == 0 &&
		info.Mode().Perm() == mode &&
		piLocalCurrentUserOwns(info)
}

func lockedPiDirectoryEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}

func lockedPiComponentTreeExact() bool {
	entries, err := os.ReadDir(lockedPiComponentRoot)
	if err != nil || len(entries) != 2 {
		return false
	}
	return entries[0].Name() == "isolation" &&
		entries[0].IsDir() &&
		entries[1].Name() == "manifest.json" &&
		entries[1].Type().IsRegular()
}

func lockedPiOfflineEnvironment(manifest lockedPiComponentManifest) bool {
	searchPaths := make(
		[]piMetadataDirectoryBinding,
		0,
		len(manifest.RuntimeSearchPaths),
	)
	for _, path := range manifest.RuntimeSearchPaths {
		searchPaths = append(
			searchPaths,
			piMetadataDirectoryBinding{path: path},
		)
	}
	directories := piMetadataInvocationDirectories{
		home:     filepath.Join(manifest.IsolationRoot, "home"),
		agent:    filepath.Join(manifest.IsolationRoot, "agent"),
		sessions: filepath.Join(manifest.IsolationRoot, "sessions"),
		temp:     filepath.Join(manifest.IsolationRoot, "tmp"),
	}
	runner := piMetadataProcessRunner{searchPaths: searchPaths}
	got := runner.environment(directories)
	want := []string{
		"HOME=" + directories.home,
		"TMPDIR=" + directories.temp,
		"PI_CODING_AGENT_DIR=" + directories.agent,
		"PI_CODING_AGENT_SESSION_DIR=" + directories.sessions,
		"PATH=" + strings.Join(
			manifest.RuntimeSearchPaths,
			string(os.PathListSeparator),
		),
		"LANG=C",
		"LC_ALL=C",
		"NO_COLOR=1",
		"TERM=dumb",
		"PI_OFFLINE=1",
		"PI_SKIP_VERSION_CHECK=1",
		"PI_TELEMETRY=0",
	}
	return reflect.DeepEqual(got, want)
}

func lockedPiComponentQuiescent() bool {
	if !lockedPiComponentArtifactsAbsent() ||
		!lockedPiComponentProcessesAbsent() {
		return false
	}
	command := exec.Command(
		"/usr/sbin/lsof",
		"-nP",
		"-a",
		"-iTCP:18427",
		"-sTCP:LISTEN",
	)
	command.Env = []string{"LANG=C", "LC_ALL=C"}
	command.Stdin = nil
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	err := command.Run()
	if err == nil {
		return false
	}
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.ExitCode() == 1
}

func lockedPiComponentArtifactsAbsent() bool {
	for _, name := range []string{
		"state.sqlite3",
		"state.sqlite3-wal",
		"state.sqlite3-shm",
		"state.sqlite3-journal",
		"loom.sock",
		"daemon.sock",
		"product.sock",
		"attempt.pid",
	} {
		if _, err := os.Lstat(
			filepath.Join(lockedPiComponentRoot, name),
		); !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	return true
}

func lockedPiComponentProcessesAbsent() bool {
	command := exec.Command("/bin/ps", "-axo", "command=")
	command.Env = []string{"LANG=C", "LC_ALL=C"}
	command.Stdin = nil
	output := newPiMetadataBoundedWriter(maxPiMetadataOutputBytes)
	command.Stdout = output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil || output.overflow {
		return false
	}
	processes := output.String()
	return !strings.Contains(processes, lockedPiComponentRoot) &&
		!strings.Contains(processes, lockedPiLlamaExecutable) &&
		!strings.Contains(processes, "p2a-w2-live-20260730-004")
}

func inspectLockedPiFile(
	path string,
	mode os.FileMode,
	size int64,
	digest string,
	executable bool,
) (lockedPiFileIdentity, bool) {
	if !lockedPiPathChainHasNoSymlink(path) {
		return lockedPiFileIdentity{}, false
	}
	info, err := os.Lstat(path)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != mode ||
		!piLocalCurrentUserOwns(info) ||
		(size > 0 && info.Size() != size) ||
		(executable && info.Mode().Perm()&0o111 == 0) {
		return lockedPiFileIdentity{}, false
	}
	file, err := os.OpenFile(path, os.O_RDONLY|piNoFollowFlag(), 0)
	if err != nil {
		return lockedPiFileIdentity{}, false
	}
	opened, err := file.Stat()
	if err != nil ||
		!os.SameFile(info, opened) ||
		opened.Mode() != info.Mode() ||
		opened.Size() != info.Size() {
		_ = file.Close()
		return lockedPiFileIdentity{}, false
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return lockedPiFileIdentity{}, false
	}
	after, err := os.Lstat(path)
	if err != nil ||
		!os.SameFile(opened, after) ||
		after.Mode() != opened.Mode() ||
		after.Size() != opened.Size() {
		return lockedPiFileIdentity{}, false
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if actualDigest != digest {
		return lockedPiFileIdentity{}, false
	}
	return lockedPiFileIdentity{
		path:   path,
		info:   after,
		mode:   after.Mode(),
		size:   after.Size(),
		digest: actualDigest,
	}, true
}

func revalidateLockedPiComponentFiles(
	before []lockedPiFileIdentity,
) string {
	reasons := []string{
		"component_pi_identity",
		"component_node_identity",
		"component_llama_identity",
		"component_model_identity",
	}
	if len(before) != len(reasons) {
		return "component_identity"
	}
	for index, expected := range before {
		current, ok := inspectLockedPiFile(
			expected.path,
			expected.mode.Perm(),
			expected.size,
			expected.digest,
			expected.mode.Perm()&0o111 != 0,
		)
		if !ok ||
			!os.SameFile(expected.info, current.info) ||
			expected.mode != current.mode ||
			expected.size != current.size ||
			expected.digest != current.digest {
			return reasons[index]
		}
	}
	return ""
}

func lockedPiComponentFailureReason(err error) string {
	command, commandFailure := loomruntime.PiMetadataFailureCommand(err)
	prefix := "component_"
	switch command {
	case loomruntime.PiMetadataVersion:
		prefix = "component_version_"
	case loomruntime.PiMetadataListModels:
		prefix = "component_models_"
	case "":
	default:
		return "component_unknown"
	}
	switch {
	case errors.Is(err, ErrPiMetadataBindingChanged),
		errors.Is(err, ErrPiLocalRuntimeProbeBindingChanged):
		return "component_binding"
	case errors.Is(err, ErrPiMetadataProcessTimeout):
		return prefix + "timeout"
	case errors.Is(err, ErrPiMetadataProcessOutputTooLarge):
		return prefix + "output_limit"
	case errors.Is(err, ErrPiMetadataProcessFailed):
		return prefix + "process"
	case errors.Is(err, loomruntime.ErrPiMetadataStderr):
		return prefix + "stderr"
	case errors.Is(err, loomruntime.ErrDuplicatePiRuntimeModel):
		return prefix + "duplicate"
	case errors.Is(err, loomruntime.ErrInvalidPiMetadataOutput),
		errors.Is(err, loomruntime.ErrPiMetadataOutputTooLarge):
		return prefix + "output"
	case commandFailure:
		return prefix + "unknown"
	default:
		return "component_unknown"
	}
}
