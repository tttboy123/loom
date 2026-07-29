package piadapter

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPiLocalModelCatalogSharesRPCBytesAndMaterializesPrivately(t *testing.T) {
	root := piLocalModelPrivateRoot(t, "metadata-catalog")
	serverConfig, digest := piLocalModelInspectorFixture(t, root)
	config := PiLocalModelCatalogConfig{
		PrivateRoot:    serverConfig.PrivateRoot,
		ExecutablePath: serverConfig.ExecutablePath,
		ModelPath:      serverConfig.ModelPath,
	}
	catalog, err := bindPiLocalModelCatalog(config, digest)
	if err != nil {
		t.Fatalf("bindPiLocalModelCatalog() error = %v", err)
	}

	adapter := &piRPCBridgeAdapter{
		providerID: piRPCProviderID,
		modelID:    piRPCModelID,
		baseURL:    piLocalModelCatalogBaseURL,
	}
	rpcBytes, err := adapter.modelsJSON()
	if err != nil {
		t.Fatalf("modelsJSON() error = %v", err)
	}
	if !bytes.Equal(catalog.content, rpcBytes) {
		t.Fatalf("metadata catalog differs from accepted RPC catalog")
	}

	agentDirectory := privateTempDir(t)
	if err := catalog.materialize(agentDirectory); err != nil {
		t.Fatalf("materialize() error = %v", err)
	}
	modelsPath := filepath.Join(agentDirectory, "models.json")
	info, err := os.Lstat(modelsPath)
	if err != nil {
		t.Fatalf("stat models.json: %v", err)
	}
	if !info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o600 {
		t.Fatalf("models.json mode = %v, want regular 0600", info.Mode())
	}
	content, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if !bytes.Equal(content, rpcBytes) {
		t.Fatalf("materialized catalog differs from accepted RPC catalog")
	}
}

func TestPiLocalModelCatalogRejectsBindingDriftBeforeMaterialization(t *testing.T) {
	root := piLocalModelPrivateRoot(t, "metadata-catalog-drift")
	serverConfig, digest := piLocalModelInspectorFixture(t, root)
	config := PiLocalModelCatalogConfig{
		PrivateRoot:    serverConfig.PrivateRoot,
		ExecutablePath: serverConfig.ExecutablePath,
		ModelPath:      serverConfig.ModelPath,
	}
	catalog, err := bindPiLocalModelCatalog(config, digest)
	if err != nil {
		t.Fatalf("bindPiLocalModelCatalog() error = %v", err)
	}
	if err := os.Chmod(config.ModelPath, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := catalog.materialize(privateTempDir(t)); err == nil {
		t.Fatal("materialize() error = nil, want fail-closed binding drift")
	}
}
