package provider

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	maxOpenCodeModelCatalogBytes = 128 << 10
	maxOpenCodeModelCatalogItems = 512
)

var ErrOpenCodeModelDiscovery = errors.New("OpenCode model discovery unavailable")

// DiscoverSystemOpenCodeModels reads the installed CLI's current model
// directory without passing prompts or credentials to the process. The
// resulting identities are canonical, sorted, and safe to persist as runtime
// capability metadata.
func DiscoverSystemOpenCodeModels(
	ctx context.Context,
	executablePath string,
	homePath string,
) ([]string, error) {
	if ctx == nil || !cleanAbsolutePath(homePath) {
		return nil, ErrOpenCodeModelDiscovery
	}
	resolved, err := ResolveOpenCodeNativeExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return nil, ErrOpenCodeModelDiscovery
	}
	before, err := codexExecutableIdentity(executablePath)
	if err != nil {
		return nil, ErrOpenCodeModelDiscovery
	}
	command := exec.CommandContext(ctx, executablePath, "models")
	command.Dir = homePath
	command.Env = []string{
		"HOME=" + homePath,
		"PATH=" + filepath.Dir(executablePath) + ":/usr/bin:/bin",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "NO_COLOR=1",
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	command.WaitDelay = 2 * time.Second
	stdout := &boundedCodexBuffer{maximum: maxOpenCodeModelCatalogBytes}
	stderr := &boundedCodexBuffer{maximum: 4096}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil || stdout.err != nil || stderr.err != nil ||
		stderr.buffer.Len() != 0 {
		return nil, ErrOpenCodeModelDiscovery
	}
	after, err := codexExecutableIdentity(executablePath)
	if err != nil || !sameCodexExecutableIdentity(before, after) {
		return nil, ErrOpenCodeExecutableIdentityChanged
	}
	return ParseOpenCodeModelCatalog(stdout.buffer.Bytes())
}

// ParseOpenCodeModelCatalog parses the newline-delimited output of
// `opencode models`. It accepts model identities only; diagnostic text,
// duplicates, controls, and oversized catalogs fail closed.
func ParseOpenCodeModelCatalog(payload []byte) ([]string, error) {
	if len(payload) == 0 || len(payload) > maxOpenCodeModelCatalogBytes ||
		!utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 {
		return nil, ErrOpenCodeModelDiscovery
	}
	catalog := strings.TrimRight(string(payload), "\r\n")
	if catalog == "" {
		return nil, ErrOpenCodeModelDiscovery
	}
	lines := strings.Split(catalog, "\n")
	if len(lines) == 0 || len(lines) > maxOpenCodeModelCatalogItems {
		return nil, ErrOpenCodeModelDiscovery
	}
	models := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		modelID := strings.TrimSuffix(line, "\r")
		if modelID == "" || strings.TrimSpace(modelID) != modelID ||
			!ValidOpenCodeModelIdentity(modelID) {
			return nil, ErrOpenCodeModelDiscovery
		}
		if _, duplicate := seen[modelID]; duplicate {
			return nil, ErrOpenCodeModelDiscovery
		}
		seen[modelID] = struct{}{}
		models = append(models, modelID)
	}
	sort.Strings(models)
	return models, nil
}

// SelectOpenCodeNativeModel chooses an installed OpenCode-hosted route. It
// never treats a third-party provider model as native auth.
func SelectOpenCodeNativeModel(modelIDs []string) (string, bool) {
	for _, modelID := range modelIDs {
		if strings.HasPrefix(modelID, "opencode/") &&
			ValidOpenCodeModelIdentity(modelID) {
			return modelID, true
		}
	}
	return "", false
}
