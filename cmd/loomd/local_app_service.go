package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

const localAppServiceFlag = "--local-app-service"
const managedLocalAppParentFlag = "--managed-parent-pid"

type localAppInvocationPlan struct {
	Args      []string
	ParentPID int
	Reexec    bool
}

func prepareLocalAppInvocation(
	args []string,
	expand func() ([]string, error),
) (localAppInvocationPlan, error) {
	if expand == nil {
		return localAppInvocationPlan{}, errors.New("invalid local app invocation")
	}
	if len(args) > 0 && args[0] == localAppServiceFlag {
		parentPID, err := parseLocalAppServiceParentPID(args)
		if err != nil {
			return localAppInvocationPlan{}, err
		}
		expanded, err := expand()
		if err != nil || !validCanonicalLocalAppRunArgs(expanded) ||
			containsLocalAppInternalFlag(expanded) {
			return localAppInvocationPlan{}, errors.New("invalid local app handoff")
		}
		canonical := append([]string(nil), expanded...)
		if parentPID > 0 {
			canonical = append(
				canonical,
				managedLocalAppParentFlag,
				strconv.Itoa(parentPID),
			)
		}
		return localAppInvocationPlan{Args: canonical, Reexec: true}, nil
	}
	runArgs, parentPID, err := parseManagedLocalAppInvocation(args)
	if err != nil {
		return localAppInvocationPlan{}, err
	}
	return localAppInvocationPlan{
		Args: runArgs, ParentPID: parentPID,
	}, nil
}

func parseManagedLocalAppInvocation(args []string) ([]string, int, error) {
	managedIndex := -1
	for index, argument := range args {
		if argument == managedLocalAppParentFlag {
			if managedIndex >= 0 {
				return nil, 0, errors.New("duplicate managed local app parent")
			}
			managedIndex = index
		} else if strings.HasPrefix(argument, managedLocalAppParentFlag+"=") {
			return nil, 0, errors.New("invalid managed local app parent")
		}
	}
	if managedIndex < 0 {
		return append([]string(nil), args...), 0, nil
	}
	if managedIndex != len(args)-2 ||
		!validCanonicalLocalAppRunArgs(args[:managedIndex]) {
		return nil, 0, errors.New("invalid managed local app handoff")
	}
	parentPID, err := strconv.Atoi(args[managedIndex+1])
	if err != nil || parentPID <= 1 {
		return nil, 0, errors.New("invalid managed local app parent")
	}
	return append([]string(nil), args[:managedIndex]...), parentPID, nil
}

func validCanonicalLocalAppRunArgs(args []string) bool {
	required := map[string]string{
		"--state": "", "--isolation-root": "", "--socket": "",
	}
	for index := 0; index < len(args); index++ {
		for flag := range required {
			if strings.HasPrefix(args[index], flag+"=") {
				return false
			}
		}
		value, wanted := required[args[index]]
		if !wanted {
			continue
		}
		if value != "" || index+1 >= len(args) {
			return false
		}
		candidate := args[index+1]
		if !filepath.IsAbs(candidate) || filepath.Clean(candidate) != candidate {
			return false
		}
		required[args[index]] = candidate
		index++
	}
	for _, value := range required {
		if value == "" {
			return false
		}
	}
	return true
}

func containsLocalAppInternalFlag(args []string) bool {
	for _, argument := range args {
		if argument == localAppServiceFlag ||
			argument == managedLocalAppParentFlag ||
			strings.HasPrefix(argument, managedLocalAppParentFlag+"=") {
			return true
		}
	}
	return false
}

func localAppServiceArgs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return nil, errors.New("local app home unavailable")
	}
	appSupport := filepath.Join(home, "Library", "Application Support", "Loom")
	for _, directory := range []string{
		filepath.Join(appSupport, "state"),
		filepath.Join(appSupport, "isolation"),
		filepath.Join(appSupport, "run"),
	} {
		if err := ensureProductExecutionDirectory(directory); err != nil {
			return nil, errors.New("local app private directory unavailable")
		}
	}
	runtimeDirs, err := installedLocalAppRuntimeDirs(home)
	if err != nil {
		return nil, err
	}
	for _, executable := range []string{"node"} {
		path, lookupErr := exec.LookPath(executable)
		if lookupErr != nil {
			return nil, errors.New("local app runtime dependency unavailable")
		}
		runtimeDirs = appendUniqueCleanDir(runtimeDirs, filepath.Dir(path))
	}
	codexPath, _ := exec.LookPath("codex")
	opencodePath, _ := exec.LookPath("opencode")
	claudePath, _ := exec.LookPath("claude")
	if resolved, resolveErr := harnessadapter.ResolveHarnessExecutable(claudePath); resolveErr == nil {
		claudePath = resolved
	} else {
		claudePath = ""
	}
	return expandLocalAppServiceArgs(
		[]string{localAppServiceFlag}, home, runtimeDirs, codexPath, opencodePath, claudePath,
	)
}

func parseLocalAppServiceParentPID(args []string) (int, error) {
	if len(args) == 1 && args[0] == localAppServiceFlag {
		return 0, nil
	}
	if len(args) != 3 || args[0] != localAppServiceFlag ||
		args[1] != "--parent-pid" {
		return 0, errors.New("invalid local app service invocation")
	}
	parent, err := strconv.Atoi(args[2])
	if err != nil || parent <= 1 {
		return 0, errors.New("invalid local app parent")
	}
	return parent, nil
}

func installedLocalAppRuntimeDirs(home string) ([]string, error) {
	runtimeRoot := filepath.Join(
		home,
		"Library", "Application Support", "Loom", "runtimes", "pi",
	)
	entries, err := os.ReadDir(runtimeRoot)
	if err != nil {
		return nil, errors.New("local app runtime unavailable")
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !validLocalAppRuntimeVersion(entry.Name()) {
			continue
		}
		bin := filepath.Join(runtimeRoot, entry.Name(), "node_modules", ".bin")
		info, statErr := os.Stat(bin)
		if statErr == nil && info.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	if len(versions) == 0 {
		return nil, errors.New("local app runtime unavailable")
	}
	sort.Strings(versions)
	selected := versions[len(versions)-1]
	return []string{
		filepath.Join(runtimeRoot, selected, "node_modules", ".bin"),
	}, nil
}

func expandLocalAppServiceArgs(
	args []string,
	home string,
	runtimeDirs []string,
	codexExecutable string,
	opencodeExecutable string,
	harnessExecutables ...string,
) ([]string, error) {
	if len(args) != 1 || args[0] != localAppServiceFlag ||
		!filepath.IsAbs(home) || len(runtimeDirs) == 0 {
		return nil, errors.New("invalid local app service input")
	}
	version, ok := localAppRuntimeVersion(home, runtimeDirs[0])
	if !ok {
		return nil, errors.New("invalid local app runtime")
	}
	for _, directory := range runtimeDirs {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
			return nil, errors.New("invalid local app runtime")
		}
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			return nil, errors.New("local app runtime unavailable")
		}
	}
	appSupport := filepath.Join(home, "Library", "Application Support", "Loom")
	expanded := []string{
		"--state", filepath.Join(appSupport, "state", "loom.db"),
		"--isolation-root", filepath.Join(appSupport, "isolation"),
	}
	for _, directory := range runtimeDirs {
		expanded = append(expanded, "--runtime-dir", directory)
	}
	expanded = append(expanded,
		"--probe-id", "probe.pi.local-app",
		"--instance-id", "runtime.pi.earendil-works."+version,
		"--device-id", "device.local",
		"--display-name", "Pi "+version,
		"--interval", "10s",
		"--process-timeout", "30s",
		"--socket", filepath.Join(appSupport, "run", "loomd.sock"),
	)
	if codexExecutable != "" {
		if !filepath.IsAbs(codexExecutable) ||
			filepath.Clean(codexExecutable) != codexExecutable {
			return nil, errors.New("invalid local app codex executable")
		}
		expanded = append(expanded, "--codex-executable", codexExecutable)
	}
	if opencodeExecutable != "" {
		if !filepath.IsAbs(opencodeExecutable) ||
			filepath.Clean(opencodeExecutable) != opencodeExecutable {
			return nil, errors.New("invalid local app OpenCode executable")
		}
		expanded = append(expanded, "--opencode-executable", opencodeExecutable)
	}
	if len(harnessExecutables) > 1 {
		return nil, errors.New("invalid local app Harness executable")
	}
	if len(harnessExecutables) == 1 && harnessExecutables[0] != "" {
		claudeExecutable := harnessExecutables[0]
		if !filepath.IsAbs(claudeExecutable) ||
			filepath.Clean(claudeExecutable) != claudeExecutable {
			return nil, errors.New("invalid local app Claude executable")
		}
		expanded = append(expanded, "--claude-executable", claudeExecutable)
	}
	expanded = appendLocalAppPiModelArgs(
		expanded, home, piadapter.InspectPiLocalModelServerBinding,
	)
	credentialImportPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
	if _, err := os.Lstat(credentialImportPath); err == nil {
		expanded = append(
			expanded,
			"--credential-import-source", credentialImportPath,
		)
	}
	return expanded, nil
}

type localAppPiModelInspector func(
	piadapter.PiLocalModelServerConfig,
) (piadapter.PiLocalModelServerBinding, error)

func appendLocalAppPiModelArgs(
	args []string,
	home string,
	inspect localAppPiModelInspector,
) []string {
	result := append([]string(nil), args...)
	if inspect == nil || !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return result
	}
	root := filepath.Join(
		home, "Library", "Application Support", "Loom", "phase1-live",
	)
	config := piadapter.PiLocalModelServerConfig{
		PrivateRoot: root,
		RuntimeArchivePath: filepath.Join(
			root, "sources", "llama-b10107-bin-macos-arm64.tar.gz",
		),
		ExecutablePath: filepath.Join(
			root, "runtime", "llama-b10107", "llama-server",
		),
		ModelPath: filepath.Join(
			root, "models", "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
		),
		Host: "127.0.0.1", Port: 18427,
		StartupTimeout: 60 * time.Second, CancelGrace: 3 * time.Second,
	}
	if _, err := inspect(config); err != nil {
		return result
	}
	return append(
		result,
		"--local-model-private-root", config.PrivateRoot,
		"--local-model-runtime-archive", config.RuntimeArchivePath,
		"--local-model-executable", config.ExecutablePath,
		"--local-model-path", config.ModelPath,
	)
}

func localAppRuntimeVersion(home, runtimeDir string) (string, bool) {
	root := filepath.Join(
		home,
		"Library", "Application Support", "Loom", "runtimes", "pi",
	)
	relative, err := filepath.Rel(root, runtimeDir)
	if err != nil {
		return "", false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) != 3 || parts[1] != "node_modules" || parts[2] != ".bin" ||
		!validLocalAppRuntimeVersion(parts[0]) {
		return "", false
	}
	return parts[0], true
}

func validLocalAppRuntimeVersion(value string) bool {
	if value == "" || len(value) > 64 || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' || char == '+' {
			continue
		}
		return false
	}
	return true
}

func appendUniqueCleanDir(paths []string, candidate string) []string {
	candidate = filepath.Clean(candidate)
	for _, path := range paths {
		if path == candidate {
			return paths
		}
	}
	return append(paths, candidate)
}
