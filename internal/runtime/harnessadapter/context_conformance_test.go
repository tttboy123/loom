package harnessadapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHarnessContextRetrievalConformanceIsExactExecutableBound(t *testing.T) {
	for _, test := range []struct {
		name, adapterType, executableVersion string
		want                                 bool
	}{
		{
			name: "Codex 0.144.1", adapterType: CodexAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
			want:              true,
		},
		{
			name: "Claude Code 2.1.196", adapterType: ClaudeCodeAdapterType,
			executableVersion: "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a",
			want:              true,
		},
		{
			name: "Codex digest drift", adapterType: CodexAdapterType,
			executableVersion: "sha256:19915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
		{
			name: "cross adapter", adapterType: ClaudeCodeAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
		{name: "empty", adapterType: CodexAdapterType},
		{name: "unknown adapter", adapterType: "loom-native", executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := HasContextRetrievalConformance(
				test.adapterType, test.executableVersion,
			); got != test.want {
				t.Fatalf("HasContextRetrievalConformance() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestHarnessContextExecutableVersionIsContentBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harness")
	if err := os.WriteFile(path, []byte("first"), 0o700); err != nil {
		t.Fatal(err)
	}
	first, err := harnessContextExecutableVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o700); err != nil {
		t.Fatal(err)
	}
	second, err := harnessContextExecutableVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != len("sha256:")+64 || len(second) != len(first) {
		t.Fatalf("versions = %q %q", first, second)
	}
}

func TestHarnessAgentInputContinuationConformanceIsExactExecutableBound(t *testing.T) {
	for _, test := range []struct {
		adapterType, executableVersion string
		want                           bool
	}{
		{
			adapterType:       CodexAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
			want:              true,
		},
		{
			adapterType:       ClaudeCodeAdapterType,
			executableVersion: "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a",
			want:              true,
		},
		{
			adapterType:       CodexAdapterType,
			executableVersion: "sha256:19915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
		{
			adapterType:       ClaudeCodeAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
	} {
		if got := HasAgentInputContinuationConformance(
			test.adapterType, test.executableVersion,
		); got != test.want {
			t.Fatalf("HasAgentInputContinuationConformance(%q, %q) = %t, want %t",
				test.adapterType, test.executableVersion, got, test.want)
		}
	}
}

func TestHarnessGovernedToolMCPConformanceIsExactExecutableBound(t *testing.T) {
	for _, test := range []struct {
		adapterType, executableVersion string
		want                           bool
	}{
		{
			adapterType:       CodexAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
			want:              true,
		},
		{
			adapterType:       ClaudeCodeAdapterType,
			executableVersion: "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a",
			want:              true,
		},
		{
			adapterType:       OpenCodeAdapterType,
			executableVersion: "sha256:43f7083d450567706a80b6441331a25b5ed6d6c9f742826790545b068229cbb2",
			want:              true,
		},
		{
			adapterType:       CodexAdapterType,
			executableVersion: "sha256:19915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
		{
			adapterType:       ClaudeCodeAdapterType,
			executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
		},
		{
			adapterType:       OpenCodeAdapterType,
			executableVersion: "sha256:53f7083d450567706a80b6441331a25b5ed6d6c9f742826790545b068229cbb2",
		},
	} {
		if got := HasGovernedToolMCPConformance(
			test.adapterType, test.executableVersion,
		); got != test.want {
			t.Fatalf("HasGovernedToolMCPConformance(%q, %q) = %t, want %t",
				test.adapterType, test.executableVersion, got, test.want)
		}
	}
}
