package runtime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxPiMetadataOutputBytes  = 256 * 1024
	maxPiMetadataVersionBytes = 128
	maxPiMetadataTokenBytes   = 256
	maxPiMetadataModelRows    = 1024
	maxPiMetadataPathBytes    = 4096

	piLegacyNoModelsDiagnostic = "No models available."
	pi0821NoModelsDiagnostic   = "No models available. Use /login to log into a provider via OAuth or API key. See:"
)

var (
	ErrInvalidPiRuntimeProbe    = errors.New("invalid pi runtime probe")
	ErrPiMetadataCommandFailed  = errors.New("pi metadata command failed")
	ErrInvalidPiMetadataOutput  = errors.New("invalid pi metadata output")
	ErrPiMetadataOutputTooLarge = errors.New("pi metadata output too large")
	ErrPiMetadataStderr         = errors.New("pi metadata stderr")
	ErrDuplicatePiRuntimeModel  = errors.New("duplicate pi runtime model")
)

type PiMetadataCommand string

const (
	PiMetadataVersion    PiMetadataCommand = "version"
	PiMetadataListModels PiMetadataCommand = "list_models"
)

type PiMetadataRequest struct {
	Command PiMetadataCommand
	Args    []string
}

type PiMetadataResult struct {
	Stdout string
	Stderr string
}

type PiMetadataRunner interface {
	RunPiMetadata(context.Context, PiMetadataRequest) (PiMetadataResult, error)
}

type PiRuntimeProbeConfig struct {
	ProbeID     string
	InstanceID  string
	DeviceID    string
	DisplayName string
	Runner      PiMetadataRunner
}

type piRuntimeProbe struct {
	probeID     string
	instanceID  string
	deviceID    string
	displayName string
	runner      PiMetadataRunner
}

func NewPiRuntimeProbe(config PiRuntimeProbeConfig) (RuntimeProbe, error) {
	if config.ProbeID == "" ||
		config.InstanceID == "" ||
		config.DeviceID == "" ||
		config.DisplayName == "" ||
		isNilPiMetadataRunner(config.Runner) {
		return nil, ErrInvalidPiRuntimeProbe
	}
	return &piRuntimeProbe{
		probeID:     config.ProbeID,
		instanceID:  config.InstanceID,
		deviceID:    config.DeviceID,
		displayName: config.DisplayName,
		runner:      config.Runner,
	}, nil
}

func (p *piRuntimeProbe) ID() string {
	return p.probeID
}

func (p *piRuntimeProbe) ObserveRuntime(ctx context.Context) ([]RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	versionResult, err := p.runMetadata(ctx, PiMetadataRequest{
		Command: PiMetadataVersion,
		Args:    []string{"--version"},
	})
	if err != nil {
		return nil, err
	}
	version, err := parsePiVersion(versionResult.Stdout)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	modelResult, err := p.runMetadata(ctx, PiMetadataRequest{
		Command: PiMetadataListModels,
		Args: []string{
			"--offline",
			"--no-approve",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-themes",
			"--no-context-files",
			"--list-models",
		},
	})
	if err != nil {
		return nil, err
	}
	models, err := parsePiModels(modelResult.Stdout)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instance, err := NewRuntimeInstance(RuntimeInstance{
		ID:                   p.instanceID,
		DeviceID:             p.deviceID,
		AdapterType:          "pi-cli",
		DisplayName:          p.displayName,
		ExecutableVersion:    version,
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"pi.metadata.models", "pi.metadata.version"},
		Capacity:             1,
	})
	if err != nil {
		return nil, ErrInvalidPiRuntimeProbe
	}

	return []RuntimeObservation{{
		Instance: instance,
		ModelIDs: append([]string(nil), models...),
	}}, nil
}

func (p *piRuntimeProbe) runMetadata(ctx context.Context, request PiMetadataRequest) (PiMetadataResult, error) {
	if err := ctx.Err(); err != nil {
		return PiMetadataResult{}, err
	}
	request.Args = append([]string(nil), request.Args...)
	result, runErr := p.runner.RunPiMetadata(ctx, request)
	if err := ctx.Err(); err != nil {
		return PiMetadataResult{}, err
	}
	if runErr != nil {
		return PiMetadataResult{}, fmt.Errorf("%w: %s", ErrPiMetadataCommandFailed, safePiMetadataCommand(request.Command))
	}
	if len(result.Stdout) > maxPiMetadataOutputBytes || len(result.Stderr) > maxPiMetadataOutputBytes {
		return PiMetadataResult{}, ErrPiMetadataOutputTooLarge
	}
	if !validPiMetadataText(result.Stdout) || !validPiMetadataText(result.Stderr) {
		return PiMetadataResult{}, ErrInvalidPiMetadataOutput
	}
	if result.Stderr != "" {
		return PiMetadataResult{}, ErrPiMetadataStderr
	}
	return PiMetadataResult{Stdout: result.Stdout}, nil
}

func isNilPiMetadataRunner(runner PiMetadataRunner) bool {
	if runner == nil {
		return true
	}
	value := reflect.ValueOf(runner)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validPiMetadataText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func safePiMetadataCommand(command PiMetadataCommand) string {
	switch command {
	case PiMetadataVersion:
		return "version"
	case PiMetadataListModels:
		return "model-list"
	default:
		return "unknown"
	}
}

func parsePiVersion(stdout string) (string, error) {
	version := strings.TrimSpace(stdout)
	if version == "" ||
		len(version) > maxPiMetadataVersionBytes ||
		strings.ContainsAny(version, "\r\n") ||
		!isASCIIAlphaNumeric(version[0]) {
		return "", ErrInvalidPiMetadataOutput
	}
	for index := 0; index < len(version); index++ {
		if !isPiVersionByte(version[index]) {
			return "", ErrInvalidPiMetadataOutput
		}
	}
	return version, nil
}

func isPiVersionByte(value byte) bool {
	return isASCIIAlphaNumeric(value) ||
		value == '.' ||
		value == '+' ||
		value == '_' ||
		value == '-'
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9'
}

func parsePiModels(stdout string) ([]string, error) {
	if strings.ContainsRune(stdout, '\r') {
		return nil, ErrInvalidPiMetadataOutput
	}
	if stdout == piLegacyNoModelsDiagnostic ||
		stdout == piLegacyNoModelsDiagnostic+"\n" {
		return nil, nil
	}
	if validPi0821NoModelsDiagnostic(stdout) {
		return nil, nil
	}
	lines := nonEmptyPiMetadataLines(stdout)
	if len(lines) == 0 {
		return nil, ErrInvalidPiMetadataOutput
	}
	if !reflect.DeepEqual(
		strings.Fields(lines[0]),
		[]string{"provider", "model", "context", "max-out", "thinking", "images"},
	) {
		return nil, ErrInvalidPiMetadataOutput
	}
	if len(lines)-1 > maxPiMetadataModelRows {
		return nil, ErrInvalidPiMetadataOutput
	}

	models := make([]string, 0, len(lines)-1)
	seen := make(map[string]struct{}, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) != 6 ||
			!validPiMetadataToken(fields[0]) ||
			strings.Contains(fields[0], "/") ||
			!validPiMetadataToken(fields[1]) ||
			!validPiCountToken(fields[2]) ||
			!validPiCountToken(fields[3]) ||
			!validPiBooleanToken(fields[4]) ||
			!validPiBooleanToken(fields[5]) {
			return nil, ErrInvalidPiMetadataOutput
		}
		modelID := fields[0] + "/" + fields[1]
		if _, exists := seen[modelID]; exists {
			return nil, ErrDuplicatePiRuntimeModel
		}
		seen[modelID] = struct{}{}
		models = append(models, modelID)
	}
	sort.Strings(models)
	return models, nil
}

func validPi0821NoModelsDiagnostic(stdout string) bool {
	if strings.HasSuffix(stdout, "\n") {
		stdout = strings.TrimSuffix(stdout, "\n")
	}
	rawLines := strings.Split(stdout, "\n")
	if len(rawLines) != 3 ||
		rawLines[0] != pi0821NoModelsDiagnostic {
		return false
	}
	for _, rawLine := range rawLines {
		if rawLine == "" || strings.TrimSpace(rawLine) != rawLine {
			return false
		}
	}
	for _, rawPath := range rawLines[1:] {
		for _, character := range rawPath {
			if unicode.IsControl(character) {
				return false
			}
		}
	}
	providersPath := rawLines[1]
	modelsPath := rawLines[2]
	if !validPiMetadataDocPath(providersPath, "providers.md") ||
		!validPiMetadataDocPath(modelsPath, "models.md") {
		return false
	}
	providersParent := filepath.Dir(providersPath)
	modelsParent := filepath.Dir(modelsPath)
	return providersParent != "" &&
		providersParent != "." &&
		providersParent == modelsParent
}

func validPiMetadataDocPath(value string, basename string) bool {
	if value == "" ||
		len(value) > maxPiMetadataPathBytes ||
		!utf8.ValidString(value) ||
		!filepath.IsAbs(value) ||
		filepath.Clean(value) != value ||
		filepath.Base(value) != basename ||
		filepath.Base(filepath.Dir(value)) != "docs" {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func nonEmptyPiMetadataLines(stdout string) []string {
	rawLines := nonEmptyPiMetadataRawLines(stdout)
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		line := strings.TrimSpace(rawLine)
		lines = append(lines, line)
	}
	return lines
}

func nonEmptyPiMetadataRawLines(stdout string) []string {
	rawLines := strings.Split(stdout, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		if strings.TrimSpace(rawLine) != "" {
			lines = append(lines, rawLine)
		}
	}
	return lines
}

func validPiMetadataToken(value string) bool {
	if value == "" || len(value) > maxPiMetadataTokenBytes {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validPiCountToken(value string) bool {
	if value == "" {
		return false
	}
	if last := value[len(value)-1]; last == 'K' || last == 'M' {
		value = value[:len(value)-1]
	}
	if value == "" {
		return false
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && parts[1] == "" {
		return false
	}
	hasNonZero := false
	for _, part := range parts {
		for index := 0; index < len(part); index++ {
			if part[index] < '0' || part[index] > '9' {
				return false
			}
			hasNonZero = hasNonZero || part[index] != '0'
		}
	}
	return hasNonZero
}

func validPiBooleanToken(value string) bool {
	return value == "yes" || value == "no"
}
