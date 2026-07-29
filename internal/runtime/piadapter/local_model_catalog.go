package piadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const piLocalModelCatalogBaseURL = "http://127.0.0.1:18427/v1"

var (
	ErrInvalidPiLocalModelCatalog = errors.New("invalid Pi local model catalog")
	ErrPiLocalModelCatalogWrite   = errors.New("Pi local model catalog write failed")
)

type PiLocalModelCatalogConfig struct {
	PrivateRoot    string
	ExecutablePath string
	ModelPath      string
}

type piLocalModelCatalog struct {
	config  PiLocalModelServerConfig
	binding piLocalServerBinding
	content []byte
	digest  string

	validateBinding func() error
}

func bindPiLocalModelCatalog(
	input PiLocalModelCatalogConfig,
	expectedModelDigest string,
) (*piLocalModelCatalog, error) {
	config := PiLocalModelServerConfig{
		PrivateRoot:    input.PrivateRoot,
		ExecutablePath: input.ExecutablePath,
		ModelPath:      input.ModelPath,
		Host:           "127.0.0.1",
		Port:           18427,
		StartupTimeout: time.Second,
		CancelGrace:    100 * time.Millisecond,
	}
	binding, err := inspectPiLocalModelServerBinding(config, expectedModelDigest)
	if err != nil {
		return nil, errors.Join(ErrInvalidPiLocalModelCatalog, err)
	}
	content, err := piLocalModelCatalogJSON(piLocalModelCatalogBaseURL)
	if err != nil {
		return nil, errors.Join(ErrInvalidPiLocalModelCatalog, err)
	}
	catalog := &piLocalModelCatalog{
		config:  config,
		binding: binding,
		content: append([]byte(nil), content...),
		digest:  expectedModelDigest,
	}
	catalog.validateBinding = func() error {
		return revalidatePiLocalServerBinding(
			catalog.binding,
			catalog.config,
			catalog.digest,
		)
	}
	return catalog, nil
}

func (catalog *piLocalModelCatalog) validate() error {
	if catalog == nil || catalog.validateBinding == nil {
		return ErrInvalidPiLocalModelCatalog
	}
	if err := catalog.validateBinding(); err != nil {
		return errors.Join(ErrInvalidPiLocalModelCatalog, err)
	}
	return nil
}

func (catalog *piLocalModelCatalog) materialize(agentDirectory string) error {
	if err := catalog.validate(); err != nil {
		return err
	}
	return catalog.materializeValidated(agentDirectory)
}

func (catalog *piLocalModelCatalog) materializeValidated(
	agentDirectory string,
) error {
	if catalog == nil || catalog.validateBinding == nil {
		return ErrInvalidPiLocalModelCatalog
	}
	if err := ensurePiLocalPrivateDirectory(agentDirectory); err != nil {
		return errors.Join(ErrPiLocalModelCatalogWrite, err)
	}

	modelsPath := filepath.Join(agentDirectory, "models.json")
	temporaryPath := filepath.Join(agentDirectory, ".models.json.tmp")
	if _, err := os.Lstat(modelsPath); !errors.Is(err, os.ErrNotExist) {
		return ErrPiLocalModelCatalogWrite
	}
	file, err := os.OpenFile(
		temporaryPath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return ErrPiLocalModelCatalogWrite
	}
	temporaryExists := true
	defer func() {
		if temporaryExists {
			_ = os.Remove(temporaryPath)
		}
	}()
	if written, err := file.Write(catalog.content); err != nil ||
		written != len(catalog.content) {
		_ = file.Close()
		return ErrPiLocalModelCatalogWrite
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return ErrPiLocalModelCatalogWrite
	}
	if err := file.Close(); err != nil {
		return ErrPiLocalModelCatalogWrite
	}
	if err := os.Rename(temporaryPath, modelsPath); err != nil {
		return ErrPiLocalModelCatalogWrite
	}
	temporaryExists = false

	info, err := os.Lstat(modelsPath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		return ErrPiLocalModelCatalogWrite
	}
	content, err := os.ReadFile(modelsPath)
	if err != nil || !bytes.Equal(content, catalog.content) {
		return ErrPiLocalModelCatalogWrite
	}
	return nil
}

func piLocalModelCatalogJSON(baseURL string) ([]byte, error) {
	return piModelCatalogJSON(piRPCProviderID, piRPCModelID, baseURL)
}

func piModelCatalogJSON(providerID, modelID, baseURL string) ([]byte, error) {
	if providerID != piRPCProviderID ||
		modelID != piRPCModelID ||
		!validPiRPCBaseURL(baseURL) {
		return nil, ErrInvalidPiLocalModelCatalog
	}
	type cost struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
	}
	type model struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Reasoning     bool     `json:"reasoning"`
		Input         []string `json:"input"`
		ContextWindow int      `json:"contextWindow"`
		MaxTokens     int      `json:"maxTokens"`
		Cost          cost     `json:"cost"`
	}
	type compatibility struct {
		SupportsDeveloperRole   bool `json:"supportsDeveloperRole"`
		SupportsReasoningEffort bool `json:"supportsReasoningEffort"`
	}
	type provider struct {
		BaseURL string        `json:"baseUrl"`
		API     string        `json:"api"`
		APIKey  string        `json:"apiKey"`
		Compat  compatibility `json:"compat"`
		Models  []model       `json:"models"`
	}
	value := struct {
		Providers map[string]provider `json:"providers"`
	}{
		Providers: map[string]provider{
			providerID: {
				BaseURL: baseURL,
				API:     "openai-completions",
				APIKey:  "loom-local-offline",
				Compat:  compatibility{},
				Models: []model{{
					ID:            modelID,
					Name:          "Loom Local Qwen 2.5 Coder 1.5B",
					Reasoning:     false,
					Input:         []string{"text"},
					ContextWindow: piLocalModelContextTokens,
					MaxTokens:     piLocalModelMaxOutputTokens,
					Cost:          cost{},
				}},
			},
		},
	}
	return json.Marshal(value)
}
