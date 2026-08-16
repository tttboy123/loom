package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"time"

	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/state"
)

const productNativeAgentRuntimeInstanceID = "runtime.loom-native.local"

const productClaudeCodeRuntimeInstanceID = "runtime.claude-code.local"

const productCodexRuntimeInstanceID = "runtime.codex.local"

const productOpenCodeRuntimeInstanceID = "runtime.opencode.local"

const (
	productKimiAgentRuntimeInstanceID    = "runtime.loom-native.kimi"
	productMiniMaxAgentRuntimeInstanceID = "runtime.loom-native.minimax"
)

type productNativeAgentRuntimeDefinition struct {
	RuntimeInstanceID   string
	ProbeID             string
	ProviderID          string
	ModelID             string
	EndpointFingerprint string
}

type productClaudeCodeAgentRuntimeProbe struct {
	executableVersion string
}

type productCodexAgentRuntimeProbe struct {
	executableVersion string
}

type productOpenCodeAgentRuntimeProbe struct {
	executableVersion string
}

func (productClaudeCodeAgentRuntimeProbe) ID() string {
	return "probe.claude-code.local"
}

func (probe productClaudeCodeAgentRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if probe.executableVersion == "" {
		return nil, errors.New("Claude Code runtime unavailable")
	}
	capabilities := productClaudeCodeCapabilities(probe.executableVersion)
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID: productClaudeCodeRuntimeInstanceID, DeviceID: "device.local",
			AdapterType: harnessadapter.ClaudeCodeAdapterType,
			DisplayName: "Claude Code", ExecutableVersion: probe.executableVersion,
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: capabilities, Capacity: 2,
		},
		ModelIDs: []string{harnessadapter.ClaudeCodeModelID},
	}}, nil
}

func (productCodexAgentRuntimeProbe) ID() string {
	return "probe.codex.local"
}

func (probe productCodexAgentRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if probe.executableVersion == "" {
		return nil, errors.New("Codex runtime unavailable")
	}
	capabilities := productCodexCapabilities(probe.executableVersion)
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID: productCodexRuntimeInstanceID, DeviceID: "device.local",
			AdapterType: harnessadapter.CodexAdapterType,
			DisplayName: "Codex", ExecutableVersion: probe.executableVersion,
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: capabilities,
			Capacity:             2,
		},
		ModelIDs: []string{harnessadapter.CodexModelID},
	}}, nil
}

func (productOpenCodeAgentRuntimeProbe) ID() string {
	return "probe.opencode.local"
}

func (probe productOpenCodeAgentRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if probe.executableVersion == "" {
		return nil, errors.New("OpenCode runtime unavailable")
	}
	capabilities := productOpenCodeCapabilities(probe.executableVersion)
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID: productOpenCodeRuntimeInstanceID, DeviceID: "device.local",
			AdapterType: harnessadapter.OpenCodeAdapterType,
			DisplayName: "OpenCode", ExecutableVersion: probe.executableVersion,
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: capabilities,
			Capacity:             1,
		},
		ModelIDs: nil,
	}}, nil
}

var productNativeAgentRuntimeDefinitions = []productNativeAgentRuntimeDefinition{
	{
		RuntimeInstanceID:   productNativeAgentRuntimeInstanceID,
		ProbeID:             "probe.loom-native.local",
		ProviderID:          nativeadapter.DeepSeekAgentProviderID,
		ModelID:             nativeadapter.DeepSeekAgentModelID,
		EndpointFingerprint: nativeadapter.DeepSeekAgentEndpointFingerprint,
	},
	{
		RuntimeInstanceID:   productKimiAgentRuntimeInstanceID,
		ProbeID:             "probe.loom-native.kimi",
		ProviderID:          nativeadapter.KimiAgentProviderID,
		ModelID:             nativeadapter.KimiAgentModelID,
		EndpointFingerprint: nativeadapter.KimiAgentEndpointFingerprint,
	},
	{
		RuntimeInstanceID:   productMiniMaxAgentRuntimeInstanceID,
		ProbeID:             "probe.loom-native.minimax",
		ProviderID:          nativeadapter.MiniMaxAgentProviderID,
		ModelID:             nativeadapter.MiniMaxAgentModelID,
		EndpointFingerprint: nativeadapter.MiniMaxAgentEndpointFingerprint,
	},
}

type productNativeAgentRuntimeProbe struct {
	definition productNativeAgentRuntimeDefinition
}

func (probe productNativeAgentRuntimeProbe) ID() string {
	return probe.definition.ProbeID
}

func (probe productNativeAgentRuntimeProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []loomruntime.RuntimeObservation{{
		Instance: loomruntime.RuntimeInstance{
			ID:                   probe.definition.RuntimeInstanceID,
			DeviceID:             "device.local",
			AdapterType:          nativeadapter.LoomNativeAgentAdapterType,
			DisplayName:          "Loom Native",
			ExecutableVersion:    "v1",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{loomruntime.CapabilityContextRetrieval},
			Capacity:             3,
		},
		ModelIDs: []string{probe.definition.ModelID},
	}}, nil
}

func ensureProductNativeAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
) error {
	if ctx == nil || store == nil || readModel == nil || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC {
		return errors.New("native Agent runtime unavailable")
	}
	for _, definition := range productNativeAgentRuntimeDefinitions {
		if err := ensureProductNativeAgentProviderRuntime(
			ctx, store, readModel, emittedAt, definition.ProviderID,
		); err != nil {
			return err
		}
	}
	return nil
}

func ensureProductNativeAgentProviderRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	providerID string,
) error {
	if ctx == nil || store == nil || readModel == nil || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC {
		return errors.New("native Agent runtime unavailable")
	}
	definition, ok := productNativeAgentRuntimeDefinitionForProvider(providerID)
	if !ok {
		return nil
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	current, found := readModel.GlobalReadView().RuntimeInstance(
		definition.RuntimeInstanceID,
	)
	migratingHistoricalRuntime := false
	if found {
		if validProductNativeAgentRuntime(current, definition) {
			return nil
		}
		if !validHistoricalProductNativeAgentRuntime(current, definition) {
			return errors.New("native Agent runtime identity drift")
		}
		migratingHistoricalRuntime = true
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{productNativeAgentRuntimeProbe{definition: definition}},
	)
	if err != nil {
		return err
	}
	sequence := int64(1)
	identitySuffix := "v2"
	if migratingHistoricalRuntime {
		sequence = max(current.DiscoverySequence, current.StatusSequence) + 1
		identitySuffix = "context-retrieval-v1"
	}
	commit, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		discovery,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: productDeterministicUUID(
				"native-agent-runtime", definition.RuntimeInstanceID, identitySuffix, "discovery",
			),
			EmittedAt: emittedAt,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: definition.RuntimeInstanceID,
				EventID: productDeterministicUUID(
					"native-agent-runtime", definition.RuntimeInstanceID, identitySuffix, "event",
				),
				IdempotencyKey: "native-agent-runtime." + productDeterministicUUID(
					"native-agent-runtime", definition.RuntimeInstanceID, identitySuffix, "idempotency",
				),
				Seq: sequence,
			}},
		},
	)
	if err != nil || !commit.Committed() || commit.EventCount() != 1 {
		return errors.New("native Agent runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	current, found = readModel.GlobalReadView().RuntimeInstance(
		definition.RuntimeInstanceID,
	)
	if !found || !validProductNativeAgentRuntime(current, definition) {
		return errors.New("native Agent runtime unavailable")
	}
	return nil
}

func ensureProductVerifiedNativeAgentRuntimes(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
) error {
	if ctx == nil || store == nil || readModel == nil {
		return errors.New("native Agent runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	view := readModel.GlobalReadView()
	for _, definition := range productNativeAgentRuntimeDefinitions {
		if len(productVerifiedAgentCredentials(view, definition.ProviderID)) == 0 {
			continue
		}
		if err := ensureProductNativeAgentProviderRuntime(
			ctx, store, readModel, emittedAt, definition.ProviderID,
		); err != nil {
			return err
		}
	}
	return nil
}

func ensureProductClaudeCodeAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if ctx == nil || store == nil || readModel == nil || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC {
		return errors.New("Claude Code runtime unavailable")
	}
	resolved, err := harnessadapter.ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return errors.New("Claude Code runtime unavailable")
	}
	executableVersion, err := productHarnessExecutableVersion(executablePath)
	if err != nil {
		return err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	migratingHistoricalRuntime := false
	if current, found := readModel.GlobalReadView().RuntimeInstance(
		productClaudeCodeRuntimeInstanceID,
	); found {
		if validProductClaudeCodeAgentRuntime(current, executableVersion) {
			return nil
		}
		if !validHistoricalProductClaudeCodeAgentRuntime(current, executableVersion) {
			return errors.New("Claude Code runtime identity drift")
		}
		migratingHistoricalRuntime = true
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{productClaudeCodeAgentRuntimeProbe{
			executableVersion: executableVersion,
		}},
	)
	if err != nil {
		return err
	}
	sequence := int64(1)
	if migratingHistoricalRuntime {
		current, _ := readModel.GlobalReadView().RuntimeInstance(
			productClaudeCodeRuntimeInstanceID,
		)
		sequence = max(current.DiscoverySequence, current.StatusSequence) + 1
	}
	commit, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		discovery,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: productHarnessRuntimeIdentity(
				productClaudeCodeRuntimeInstanceID, migratingHistoricalRuntime, "discovery",
			),
			EmittedAt: emittedAt,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: productClaudeCodeRuntimeInstanceID,
				EventID: productHarnessRuntimeIdentity(
					productClaudeCodeRuntimeInstanceID, migratingHistoricalRuntime, "event",
				),
				IdempotencyKey: "harness-agent-runtime." + productHarnessRuntimeIdentity(
					productClaudeCodeRuntimeInstanceID, migratingHistoricalRuntime, "idempotency",
				),
				Seq: sequence,
			}},
		},
	)
	if err != nil || !commit.Committed() || commit.EventCount() != 1 {
		return errors.New("Claude Code runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	current, found := readModel.GlobalReadView().RuntimeInstance(
		productClaudeCodeRuntimeInstanceID,
	)
	if !found || !validProductClaudeCodeAgentRuntime(current, executableVersion) {
		return errors.New("Claude Code runtime unavailable")
	}
	return nil
}

func ensureProductVerifiedClaudeCodeAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if executablePath == "" {
		return nil
	}
	if ctx == nil || store == nil || readModel == nil {
		return errors.New("Claude Code runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	if len(productVerifiedAgentCredentials(
		readModel.GlobalReadView(), harnessadapter.ClaudeCodeProviderID,
	)) == 0 {
		return nil
	}
	return ensureProductClaudeCodeAgentRuntime(
		ctx, store, readModel, emittedAt, executablePath,
	)
}

func ensureProductCodexAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if ctx == nil || store == nil || readModel == nil || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC {
		return errors.New("Codex runtime unavailable")
	}
	resolved, err := harnessadapter.ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return errors.New("Codex runtime unavailable")
	}
	executableVersion, err := productHarnessExecutableVersion(executablePath)
	if err != nil {
		return err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	migratingHistoricalRuntime := false
	if current, found := readModel.GlobalReadView().RuntimeInstance(
		productCodexRuntimeInstanceID,
	); found {
		if validProductCodexAgentRuntime(current, executableVersion) {
			return nil
		}
		if !validHistoricalProductCodexAgentRuntime(current, executableVersion) {
			return errors.New("Codex runtime identity drift")
		}
		migratingHistoricalRuntime = true
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{productCodexAgentRuntimeProbe{
			executableVersion: executableVersion,
		}},
	)
	if err != nil {
		return err
	}
	sequence := int64(1)
	if migratingHistoricalRuntime {
		current, _ := readModel.GlobalReadView().RuntimeInstance(
			productCodexRuntimeInstanceID,
		)
		sequence = max(current.DiscoverySequence, current.StatusSequence) + 1
	}
	commit, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		discovery,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: productHarnessRuntimeIdentity(
				productCodexRuntimeInstanceID, migratingHistoricalRuntime, "discovery",
			),
			EmittedAt: emittedAt,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: productCodexRuntimeInstanceID,
				EventID: productHarnessRuntimeIdentity(
					productCodexRuntimeInstanceID, migratingHistoricalRuntime, "event",
				),
				IdempotencyKey: "harness-agent-runtime." + productHarnessRuntimeIdentity(
					productCodexRuntimeInstanceID, migratingHistoricalRuntime, "idempotency",
				),
				Seq: sequence,
			}},
		},
	)
	if err != nil || !commit.Committed() || commit.EventCount() != 1 {
		return errors.New("Codex runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	current, found := readModel.GlobalReadView().RuntimeInstance(
		productCodexRuntimeInstanceID,
	)
	if !found || !validProductCodexAgentRuntime(current, executableVersion) {
		return errors.New("Codex runtime unavailable")
	}
	return nil
}

func ensureProductVerifiedCodexAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if executablePath == "" {
		return nil
	}
	if ctx == nil || store == nil || readModel == nil {
		return errors.New("Codex runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	if len(productVerifiedAgentCredentials(
		readModel.GlobalReadView(), harnessadapter.CodexProviderID,
	)) == 0 {
		return nil
	}
	return ensureProductCodexAgentRuntime(
		ctx, store, readModel, emittedAt, executablePath,
	)
}

func validProductOpenCodeAgentRuntime(
	current projection.RuntimeInstance,
	executableVersion string,
) bool {
	return current.ID == productOpenCodeRuntimeInstanceID &&
		current.AdapterType == harnessadapter.OpenCodeAdapterType &&
		current.DeviceID == "device.local" && current.DisplayName == "OpenCode" &&
		current.ExecutableVersion == executableVersion &&
		current.Status == string(loomruntime.RuntimeOnline) && current.Capacity == 1 &&
		reflect.DeepEqual(
			current.ObservedCapabilities,
			productOpenCodeCapabilities(executableVersion),
		)
}

func productOpenCodeCapabilities(executableVersion string) []string {
	return []string{"workspace_edit"}
}

func ensureProductVerifiedOpenCodeAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if executablePath == "" {
		return nil
	}
	if ctx == nil || store == nil || readModel == nil {
		return errors.New("OpenCode runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	return ensureProductOpenCodeAgentRuntime(
		ctx, store, readModel, emittedAt, executablePath,
	)
}

func ensureProductOpenCodeAgentRuntime(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	emittedAt time.Time,
	executablePath string,
) error {
	if ctx == nil || store == nil || readModel == nil || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC {
		return errors.New("OpenCode runtime unavailable")
	}
	resolved, err := harnessadapter.ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return errors.New("OpenCode runtime unavailable")
	}
	executableVersion, err := productHarnessExecutableVersion(executablePath)
	if err != nil {
		return err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	if current, found := readModel.GlobalReadView().RuntimeInstance(
		productOpenCodeRuntimeInstanceID,
	); found && validProductOpenCodeAgentRuntime(current, executableVersion) {
		return nil
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		[]loomruntime.RuntimeProbe{productOpenCodeAgentRuntimeProbe{
			executableVersion: executableVersion,
		}},
	)
	if err != nil {
		return err
	}
	commit, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		discovery,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: productHarnessRuntimeIdentity(
				productOpenCodeRuntimeInstanceID, false, "discovery",
			),
			EmittedAt: emittedAt,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: productOpenCodeRuntimeInstanceID,
				EventID: productHarnessRuntimeIdentity(
					productOpenCodeRuntimeInstanceID, false, "event",
				),
				IdempotencyKey: "harness-agent-runtime." + productHarnessRuntimeIdentity(
					productOpenCodeRuntimeInstanceID, false, "idempotency",
				),
				Seq: 1,
			}},
		},
	)
	if err != nil || !commit.Committed() || commit.EventCount() != 1 {
		return errors.New("OpenCode runtime unavailable")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return err
	}
	current, found := readModel.GlobalReadView().RuntimeInstance(
		productOpenCodeRuntimeInstanceID,
	)
	if !found || !validProductOpenCodeAgentRuntime(current, executableVersion) {
		return errors.New("OpenCode runtime unavailable")
	}
	return nil
}

func productHarnessExecutableVersion(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("Harness executable unavailable")
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, (1<<30)+1))
	if err != nil || written <= 0 || written > 1<<30 {
		return "", errors.New("Harness executable unavailable")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func productHarnessRuntimeIdentity(
	runtimeInstanceID string,
	contextRetrievalMigration bool,
	kind string,
) string {
	parts := []string{"harness-agent-runtime", runtimeInstanceID}
	if contextRetrievalMigration {
		parts = append(parts, "governed-tool-mcp-v1")
	}
	return productDeterministicUUID(append(parts, kind)...)
}

func validProductClaudeCodeAgentRuntime(
	current projection.RuntimeInstance,
	executableVersion string,
) bool {
	return current.ID == productClaudeCodeRuntimeInstanceID &&
		current.AdapterType == harnessadapter.ClaudeCodeAdapterType &&
		current.DeviceID == "device.local" && current.DisplayName == "Claude Code" &&
		current.ExecutableVersion == executableVersion &&
		current.Status == string(loomruntime.RuntimeOnline) && current.Capacity == 2 &&
		reflect.DeepEqual(
			current.ObservedCapabilities,
			productClaudeCodeCapabilities(executableVersion),
		) &&
		reflect.DeepEqual(current.ModelIDs, []string{harnessadapter.ClaudeCodeModelID})
}

func validProductCodexAgentRuntime(
	current projection.RuntimeInstance,
	executableVersion string,
) bool {
	return current.ID == productCodexRuntimeInstanceID &&
		current.AdapterType == harnessadapter.CodexAdapterType &&
		current.DeviceID == "device.local" && current.DisplayName == "Codex" &&
		current.ExecutableVersion == executableVersion &&
		current.Status == string(loomruntime.RuntimeOnline) && current.Capacity == 2 &&
		reflect.DeepEqual(current.ObservedCapabilities, productCodexCapabilities(executableVersion)) &&
		reflect.DeepEqual(current.ModelIDs, []string{harnessadapter.CodexModelID})
}

func validHistoricalProductClaudeCodeAgentRuntime(
	current projection.RuntimeInstance,
	executableVersion string,
) bool {
	if !harnessadapter.HasGovernedToolMCPConformance(
		harnessadapter.ClaudeCodeAdapterType, executableVersion,
	) || !reflect.DeepEqual(current.ObservedCapabilities, []string{"workspace_edit"}) &&
		!reflect.DeepEqual(current.ObservedCapabilities, []string{
			loomruntime.CapabilityContextRetrieval, "workspace_edit",
		}) {
		return false
	}
	current.ObservedCapabilities = productClaudeCodeCapabilities(executableVersion)
	return validProductClaudeCodeAgentRuntime(current, executableVersion)
}

func validHistoricalProductCodexAgentRuntime(
	current projection.RuntimeInstance,
	executableVersion string,
) bool {
	if !harnessadapter.HasGovernedToolMCPConformance(
		harnessadapter.CodexAdapterType, executableVersion,
	) || !reflect.DeepEqual(current.ObservedCapabilities, []string{
		"reasoning_effort", "workspace_edit",
	}) && !reflect.DeepEqual(current.ObservedCapabilities, []string{
		loomruntime.CapabilityContextRetrieval, "reasoning_effort", "workspace_edit",
	}) {
		return false
	}
	current.ObservedCapabilities = productCodexCapabilities(executableVersion)
	return validProductCodexAgentRuntime(current, executableVersion)
}

func productClaudeCodeCapabilities(executableVersion string) []string {
	capabilities := []string{"workspace_edit"}
	if harnessadapter.HasContextRetrievalConformance(
		harnessadapter.ClaudeCodeAdapterType, executableVersion,
	) {
		capabilities = append(capabilities, loomruntime.CapabilityContextRetrieval)
	}
	if harnessadapter.HasGovernedToolMCPConformance(
		harnessadapter.ClaudeCodeAdapterType, executableVersion,
	) {
		capabilities = append(capabilities, loomruntime.CapabilityGovernedToolLoop)
	}
	sort.Strings(capabilities)
	return capabilities
}

func productCodexCapabilities(executableVersion string) []string {
	capabilities := []string{"reasoning_effort", "workspace_edit"}
	if harnessadapter.HasContextRetrievalConformance(
		harnessadapter.CodexAdapterType, executableVersion,
	) {
		capabilities = append(capabilities, loomruntime.CapabilityContextRetrieval)
	}
	if harnessadapter.HasGovernedToolMCPConformance(
		harnessadapter.CodexAdapterType, executableVersion,
	) {
		capabilities = append(capabilities, loomruntime.CapabilityGovernedToolLoop)
	}
	sort.Strings(capabilities)
	return capabilities
}

func validProductNativeAgentRuntime(
	current projection.RuntimeInstance,
	definition productNativeAgentRuntimeDefinition,
) bool {
	return current.ID == definition.RuntimeInstanceID &&
		current.AdapterType == nativeadapter.LoomNativeAgentAdapterType &&
		current.DeviceID == "device.local" && current.DisplayName == "Loom Native" &&
		current.ExecutableVersion == "v1" &&
		current.Status == string(loomruntime.RuntimeOnline) && current.Capacity == 3 &&
		reflect.DeepEqual(
			current.ObservedCapabilities,
			[]string{loomruntime.CapabilityContextRetrieval},
		) &&
		reflect.DeepEqual(current.ModelIDs, []string{definition.ModelID})
}

func validHistoricalProductNativeAgentRuntime(
	current projection.RuntimeInstance,
	definition productNativeAgentRuntimeDefinition,
) bool {
	if len(current.ObservedCapabilities) != 0 {
		return false
	}
	current.ObservedCapabilities = []string{loomruntime.CapabilityContextRetrieval}
	return validProductNativeAgentRuntime(current, definition)
}

func productNativeAgentRuntimeDefinitionForProvider(
	providerID string,
) (productNativeAgentRuntimeDefinition, bool) {
	for _, definition := range productNativeAgentRuntimeDefinitions {
		if definition.ProviderID == providerID {
			return definition, true
		}
	}
	return productNativeAgentRuntimeDefinition{}, false
}

func productNativeAgentRuntimeDefinitionForInstance(
	runtimeInstanceID string,
) (productNativeAgentRuntimeDefinition, bool) {
	for _, definition := range productNativeAgentRuntimeDefinitions {
		if definition.RuntimeInstanceID == runtimeInstanceID {
			return definition, true
		}
	}
	return productNativeAgentRuntimeDefinition{}, false
}

type productAgentCredentialSource interface {
	CurrentAgentCredential(
		context.Context,
		string,
		string,
	) (projection.ProviderCredentialRecord, error)
}

type productProjectedAgentCredentialSource struct {
	projection *projection.Projection
}

func (source *productProjectedAgentCredentialSource) CurrentAgentCredential(
	ctx context.Context,
	providerID string,
	providerAccountID string,
) (projection.ProviderCredentialRecord, error) {
	if source == nil || source.projection == nil || ctx == nil ||
		!credentials.ValidProviderAccountIdentifier(providerID, providerAccountID) {
		return projection.ProviderCredentialRecord{}, nativeadapter.ErrAgentCredentialUnavailable
	}
	if err := source.projection.Rebuild(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return projection.ProviderCredentialRecord{}, ctxErr
		}
		return projection.ProviderCredentialRecord{}, nativeadapter.ErrAgentCredentialUnavailable
	}
	record, ok := source.projection.GlobalReadView().ProviderAccountCredential(
		providerID, providerAccountID,
	)
	if !ok {
		return projection.ProviderCredentialRecord{}, nativeadapter.ErrAgentCredentialUnavailable
	}
	return record, nil
}

type productAgentCredentialAccess struct {
	source  productAgentCredentialSource
	leasing productCredentialLeaseAccess
}

func newProductAgentCredentialAccess(
	source productAgentCredentialSource,
	leasing productCredentialLeaseAccess,
) (*productAgentCredentialAccess, error) {
	if nilProductAgentInterface(source) || nilProductAgentInterface(leasing) {
		return nil, nativeadapter.ErrAgentCredentialUnavailable
	}
	return &productAgentCredentialAccess{source: source, leasing: leasing}, nil
}

func (access *productAgentCredentialAccess) UseCredential(
	ctx context.Context,
	binding loomruntime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	if access == nil || access.source == nil || access.leasing == nil || ctx == nil || use == nil {
		return nativeadapter.ErrAgentCredentialUnavailable
	}
	validated, err := loomruntime.ValidateFrozenExecutionBinding(binding)
	if err != nil || validated.AuthMode != loomruntime.AuthBrokered ||
		!credentials.ValidProviderAccountIdentifier(
			validated.ProviderID, validated.ProviderAccountID,
		) {
		return nativeadapter.ErrAgentCredentialUnavailable
	}
	record, err := access.source.CurrentAgentCredential(
		ctx, validated.ProviderID, validated.ProviderAccountID,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return errors.Join(nativeadapter.ErrAgentCredentialUnavailable, err)
	}
	if record.ProviderID != validated.ProviderID ||
		record.ProviderAccountID != validated.ProviderAccountID ||
		record.CredentialReference != validated.CredentialReference ||
		record.Revision != validated.CredentialRevision ||
		record.Status != string(credentials.CredentialVerified) {
		return nativeadapter.ErrAgentCredentialUnavailable
	}
	err = access.leasing.UseCredential(
		ctx,
		credentialvault.CredentialIdentity{
			ProviderID:          validated.ProviderID,
			ProviderAccountID:   validated.ProviderAccountID,
			CredentialReference: validated.CredentialReference,
			CredentialRevision:  validated.CredentialRevision,
		},
		func(leaseContext context.Context, secret []byte) error {
			return use(leaseContext, secret)
		},
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if stage := credentials.CredentialFailureStage(err); stage != "" {
			return credentials.WithCredentialFailureStage(
				stage,
				nativeadapter.ErrAgentCredentialUnavailable,
			)
		}
		return err
	}
	return nil
}

func productAgentContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validProductAgentCredentialReference(value string) bool {
	const prefix = "credential-ref-"
	if len(value) <= len(prefix) || len(value) > 128 ||
		value[:len(prefix)] != prefix {
		return false
	}
	for _, character := range value[len(prefix):] {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func nilProductAgentInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
