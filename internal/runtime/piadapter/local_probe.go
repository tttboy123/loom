package piadapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const piLocalRuntimeExecutableName = "pi"

var (
	ErrInvalidPiLocalRuntimeProbeFactory     = errors.New("invalid pi local runtime probe factory")
	ErrInvalidPiLocalRuntimeProbeRequest     = errors.New("invalid pi local runtime probe request")
	ErrPiLocalRuntimeProbeBindingChanged     = errors.New("pi local runtime probe binding changed")
	ErrPiLocalRuntimeCandidateInvalid        = errors.New("pi local runtime candidate invalid")
	ErrPiLocalRuntimeProbeConstructionFailed = errors.New("pi local runtime probe construction failed")
)

type PiLocalRuntimeProbeFactoryConfig struct {
	ProbeID            string
	InstanceID         string
	DeviceID           string
	DisplayName        string
	IsolationRoot      string
	RuntimeSearchPaths []string
	Timeout            time.Duration
	LocalModelCatalog  *PiLocalModelCatalogConfig
}

type PiLocalRuntimeProbeFactory struct {
	probeID       string
	instanceID    string
	deviceID      string
	displayName   string
	isolationRoot piMetadataDirectoryBinding
	searchPaths   []piMetadataDirectoryBinding
	timeout       time.Duration
	catalog       *piLocalModelCatalog
}

func NewPiLocalRuntimeProbeFactory(
	config PiLocalRuntimeProbeFactoryConfig,
) (*PiLocalRuntimeProbeFactory, error) {
	return newPiLocalRuntimeProbeFactory(config, piLocalModelSHA256)
}

func newPiLocalRuntimeProbeFactory(
	config PiLocalRuntimeProbeFactoryConfig,
	expectedModelDigest string,
) (*PiLocalRuntimeProbeFactory, error) {
	if config.ProbeID == "" ||
		config.InstanceID == "" ||
		config.DeviceID == "" ||
		config.DisplayName == "" ||
		config.Timeout <= 0 ||
		config.Timeout > maxPiMetadataProcessTimeout ||
		len(config.RuntimeSearchPaths) == 0 {
		return nil, ErrInvalidPiLocalRuntimeProbeFactory
	}

	root, err := bindPiMetadataIsolationRoot(config.IsolationRoot)
	if err != nil {
		return nil, ErrInvalidPiLocalRuntimeProbeFactory
	}
	searchPaths, err := bindPiMetadataSearchPaths(config.RuntimeSearchPaths)
	if err != nil {
		return nil, ErrInvalidPiLocalRuntimeProbeFactory
	}
	var catalog *piLocalModelCatalog
	if config.LocalModelCatalog != nil {
		catalog, err = bindPiLocalModelCatalog(
			*config.LocalModelCatalog,
			expectedModelDigest,
		)
		if err != nil {
			return nil, ErrInvalidPiLocalRuntimeProbeFactory
		}
	}

	return &PiLocalRuntimeProbeFactory{
		probeID:       config.ProbeID,
		instanceID:    config.InstanceID,
		deviceID:      config.DeviceID,
		displayName:   config.DisplayName,
		isolationRoot: root,
		searchPaths:   append([]piMetadataDirectoryBinding(nil), searchPaths...),
		timeout:       config.Timeout,
		catalog:       catalog,
	}, nil
}

func (f *PiLocalRuntimeProbeFactory) BuildProbe(
	ctx context.Context,
) (loomruntime.RuntimeProbe, bool, error) {
	if ctx == nil || f == nil {
		return nil, false, ErrInvalidPiLocalRuntimeProbeRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if !f.validBindings() {
		return nil, false, ErrPiLocalRuntimeProbeBindingChanged
	}

	searchPaths := f.canonicalSearchPaths()
	for _, searchPath := range searchPaths {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}

		candidatePath := filepath.Join(searchPath, piLocalRuntimeExecutableName)
		if _, err := os.Lstat(candidatePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, false, ErrPiLocalRuntimeCandidateInvalid
		}
		if _, err := bindPiMetadataExecutable(candidatePath); err != nil {
			return nil, false, ErrPiLocalRuntimeCandidateInvalid
		}
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}

		runner, err := newPiMetadataProcessRunnerWithBoundCatalog(PiMetadataProcessRunnerConfig{
			ExecutablePath:     candidatePath,
			IsolationRoot:      f.isolationRoot.path,
			RuntimeSearchPaths: searchPaths,
			Timeout:            f.timeout,
		}, f.catalog)
		if err != nil {
			return nil, false, ErrPiLocalRuntimeProbeConstructionFailed
		}
		probe, err := loomruntime.NewPiRuntimeProbe(loomruntime.PiRuntimeProbeConfig{
			ProbeID:     f.probeID,
			InstanceID:  f.instanceID,
			DeviceID:    f.deviceID,
			DisplayName: f.displayName,
			Runner:      runner,
		})
		if err != nil {
			return nil, false, ErrPiLocalRuntimeProbeConstructionFailed
		}
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		return probe, true, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

func (f *PiLocalRuntimeProbeFactory) validBindings() bool {
	if !validPiMetadataDirectoryBinding(f.isolationRoot, true) {
		return false
	}
	for _, binding := range f.searchPaths {
		if !validPiLocalRuntimeSearchBinding(binding) {
			return false
		}
	}
	return true
}

func validPiLocalRuntimeSearchBinding(binding piMetadataDirectoryBinding) bool {
	if !validPiMetadataDirectoryBinding(binding, false) {
		return false
	}
	info, err := os.Lstat(binding.path)
	return err == nil && info.Mode().Perm() == binding.info.Mode().Perm()
}

func (f *PiLocalRuntimeProbeFactory) canonicalSearchPaths() []string {
	paths := make([]string, 0, len(f.searchPaths))
	for _, binding := range f.searchPaths {
		paths = append(paths, binding.path)
	}
	return paths
}
