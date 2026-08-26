package harnessadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

const (
	codexContextRetrievalExecutableVersion  = "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a"
	claudeContextRetrievalExecutableVersion = "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a"
	codexAgentInputExecutableVersion        = "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a"
	claudeAgentInputExecutableVersion       = "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a"
	codexGovernedToolExecutableVersion      = "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a"
	claudeGovernedToolExecutableVersion     = "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a"
	opencodeGovernedToolExecutableVersion   = "sha256:43f7083d450567706a80b6441331a25b5ed6d6c9f742826790545b068229cbb2"
)

// HasContextRetrievalConformance reports only executable byte identities that
// passed the real CLI Attempt-scoped MCP component gate.
func HasContextRetrievalConformance(adapterType, executableVersion string) bool {
	switch adapterType {
	case CodexAdapterType:
		return executableVersion == codexContextRetrievalExecutableVersion
	case ClaudeCodeAdapterType:
		return executableVersion == claudeContextRetrievalExecutableVersion
	default:
		return false
	}
}

// HasAgentInputContinuationConformance reports only executable byte identities
// whose persistent same-process protocol was audited for governed inputs.
func HasAgentInputContinuationConformance(adapterType, executableVersion string) bool {
	switch adapterType {
	case CodexAdapterType:
		return executableVersion == codexAgentInputExecutableVersion
	case ClaudeCodeAdapterType:
		return executableVersion == claudeAgentInputExecutableVersion
	default:
		return false
	}
}

// HasGovernedToolMCPConformance reports only executable byte identities whose
// Attempt-scoped MCP tool transport was audited independently of Context reads.
func HasGovernedToolMCPConformance(adapterType, executableVersion string) bool {
	switch adapterType {
	case CodexAdapterType:
		return executableVersion == codexGovernedToolExecutableVersion
	case ClaudeCodeAdapterType:
		return executableVersion == claudeGovernedToolExecutableVersion
	case OpenCodeAdapterType:
		return executableVersion == opencodeGovernedToolExecutableVersion
	default:
		return false
	}
}

func harnessContextExecutableVersion(path string) (string, error) {
	resolved, err := ResolveHarnessExecutable(path)
	if err != nil || resolved != path {
		return "", ErrHarnessProcessUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return "", ErrHarnessProcessUnavailable
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, (1<<30)+1))
	if err != nil || written <= 0 || written > 1<<30 {
		return "", ErrHarnessProcessUnavailable
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func executableHasContextRetrievalConformance(adapterType, path string) bool {
	version, err := harnessContextExecutableVersion(path)
	return err == nil && HasContextRetrievalConformance(adapterType, version)
}

func executableHasAgentInputContinuationConformance(adapterType, path string) bool {
	version, err := harnessContextExecutableVersion(path)
	return err == nil && HasAgentInputContinuationConformance(adapterType, version)
}

func executableHasGovernedToolMCPConformance(adapterType, path string) bool {
	version, err := harnessContextExecutableVersion(path)
	return err == nil && HasGovernedToolMCPConformance(adapterType, version)
}
