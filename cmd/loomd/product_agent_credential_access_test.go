package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

type productAgentCredentialSourceFixture struct {
	record     projection.ProviderCredentialRecord
	err        error
	calls      int
	providerID string
	accountID  string
}

func (source *productAgentCredentialSourceFixture) CurrentAgentCredential(
	_ context.Context,
	providerID string,
	providerAccountID string,
) (projection.ProviderCredentialRecord, error) {
	source.calls++
	source.providerID = providerID
	source.accountID = providerAccountID
	return source.record, source.err
}

type productAgentSecretStoreFixture struct {
	secret    []byte
	err       error
	reads     int
	reference string
}

type productAgentCredentialLeaseFixture struct {
	err      error
	calls    int
	identity credentialvault.CredentialIdentity
}

func (leasing *productAgentCredentialLeaseFixture) UseCredential(
	_ context.Context,
	identity credentialvault.CredentialIdentity,
	_ func(context.Context, []byte) error,
) error {
	leasing.calls++
	leasing.identity = identity
	return leasing.err
}

type productAgentVaultReaderFixture struct {
	err error
}

type productAgentHTTPDoerFixture struct {
	calls int
}

func (doer *productAgentHTTPDoerFixture) Do(*http.Request) (*http.Response, error) {
	doer.calls++
	return nil, errors.New("unexpected Provider request")
}

func (reader *productAgentVaultReaderFixture) ReadCredential(
	context.Context,
	credentialvault.CredentialIdentity,
) ([]byte, error) {
	return nil, reader.err
}

func (*productAgentSecretStoreFixture) Put(context.Context, string, []byte) error {
	return errors.New("unexpected put")
}

func (store *productAgentSecretStoreFixture) Read(
	_ context.Context,
	reference string,
) ([]byte, error) {
	store.reads++
	store.reference = reference
	return append([]byte(nil), store.secret...), store.err
}

func (*productAgentSecretStoreFixture) Delete(context.Context, string) error {
	return errors.New("unexpected delete")
}

func TestProductAgentCredentialAccessResolvesExactFrozenAccountRevision(t *testing.T) {
	binding := productDeepSeekFrozenBinding(t)
	source := &productAgentCredentialSourceFixture{record: projection.ProviderCredentialRecord{
		ProviderID:          binding.ProviderID,
		ProviderAccountID:   binding.ProviderAccountID,
		CredentialReference: binding.CredentialReference,
		Revision:            binding.CredentialRevision,
		Status:              string(credentials.CredentialVerified),
	}}
	store := &productAgentSecretStoreFixture{secret: []byte("private-agent-key")}
	access, err := newProductAgentCredentialAccess(
		source, productAgentTestLeasing(t, store),
	)
	if err != nil {
		t.Fatal(err)
	}
	var observedBinding loomruntime.FrozenExecutionBinding
	var observedSecret []byte
	err = access.UseCredential(
		context.Background(),
		binding,
		func(_ context.Context, secret []byte) error {
			observedBinding = binding
			observedSecret = secret
			if string(secret) != "private-agent-key" {
				t.Fatalf("secret = %q", secret)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || source.providerID != "deepseek" ||
		source.accountID != "deepseek.primary" || store.reads != 1 ||
		store.reference != binding.CredentialReference ||
		!reflect.DeepEqual(observedBinding, binding) {
		t.Fatalf("source=%#v store=%#v binding=%#v", source, store, observedBinding)
	}
	for index, value := range observedSecret {
		if value != 0 {
			t.Fatalf("secret byte %d was not cleared", index)
		}
	}
}

func TestProductAgentCredentialAccessRejectsStaleOrAmbiguousAccountBeforeKeychain(t *testing.T) {
	valid := productDeepSeekFrozenBinding(t)
	tests := []struct {
		name   string
		mutate func(*loomruntime.FrozenExecutionBinding, *projection.ProviderCredentialRecord)
	}{
		{name: "account drift", mutate: func(_ *loomruntime.FrozenExecutionBinding, record *projection.ProviderCredentialRecord) {
			record.ProviderAccountID = "deepseek.other"
		}},
		{name: "credential reference drift", mutate: func(_ *loomruntime.FrozenExecutionBinding, record *projection.ProviderCredentialRecord) {
			record.CredentialReference = "credential-ref-deepseek-new"
		}},
		{name: "credential revision drift", mutate: func(_ *loomruntime.FrozenExecutionBinding, record *projection.ProviderCredentialRecord) {
			record.Revision++
		}},
		{name: "credential is not verified", mutate: func(_ *loomruntime.FrozenExecutionBinding, record *projection.ProviderCredentialRecord) {
			record.Status = string(credentials.CredentialRejected)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := valid
			record := projection.ProviderCredentialRecord{
				ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialReference: binding.CredentialReference,
				Revision:            binding.CredentialRevision, Status: string(credentials.CredentialVerified),
			}
			test.mutate(&binding, &record)
			source := &productAgentCredentialSourceFixture{record: record}
			store := &productAgentSecretStoreFixture{secret: []byte("must-not-be-read")}
			access, err := newProductAgentCredentialAccess(
				source, productAgentTestLeasing(t, store),
			)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			err = access.UseCredential(
				context.Background(), binding,
				func(context.Context, []byte) error { called = true; return nil },
			)
			if !errors.Is(err, nativeadapter.ErrAgentCredentialUnavailable) {
				t.Fatalf("UseCredential() error = %v", err)
			}
			if called || store.reads != 0 {
				t.Fatalf("rejected binding called=%t keychain reads=%d", called, store.reads)
			}
		})
	}
}

func TestProductAgentCredentialAccessPreservesClosedVaultFailureStage(t *testing.T) {
	binding := productDeepSeekFrozenBinding(t)
	source := &productAgentCredentialSourceFixture{record: projection.ProviderCredentialRecord{
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		CredentialReference: binding.CredentialReference,
		Revision:            binding.CredentialRevision,
		Status:              string(credentials.CredentialVerified),
	}}
	privateFailure := errors.New("private-vault-path-must-not-escape")
	leasing := &productAgentCredentialLeaseFixture{err: credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultDecrypt,
		privateFailure,
	)}
	access, err := newProductAgentCredentialAccess(source, leasing)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = access.UseCredential(
		context.Background(), binding,
		func(context.Context, []byte) error { called = true; return nil },
	)
	if !errors.Is(err, nativeadapter.ErrAgentCredentialUnavailable) ||
		credentials.CredentialFailureStage(err) !=
			credentials.CredentialStageVaultDecrypt ||
		strings.Contains(err.Error(), privateFailure.Error()) {
		t.Fatalf("closed staged error = %v", err)
	}
	if called || leasing.calls != 1 ||
		leasing.identity.ProviderID != binding.ProviderID ||
		leasing.identity.ProviderAccountID != binding.ProviderAccountID ||
		leasing.identity.CredentialReference != binding.CredentialReference ||
		leasing.identity.CredentialRevision != binding.CredentialRevision {
		t.Fatalf("called=%t leasing=%#v", called, leasing)
	}
}

func TestProductAgentCredentialAccessDoesNotRewriteProviderFailure(t *testing.T) {
	binding := productDeepSeekFrozenBinding(t)
	source := &productAgentCredentialSourceFixture{record: projection.ProviderCredentialRecord{
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		CredentialReference: binding.CredentialReference,
		Revision:            binding.CredentialRevision,
		Status:              string(credentials.CredentialVerified),
	}}
	store := &productAgentSecretStoreFixture{secret: []byte("provider-call-secret")}
	access, err := newProductAgentCredentialAccess(
		source, productAgentTestLeasing(t, store),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = access.UseCredential(
		context.Background(), binding,
		func(context.Context, []byte) error {
			return harnessadapter.ErrHarnessProviderRateLimit
		},
	)
	if !errors.Is(err, harnessadapter.ErrHarnessProviderRateLimit) ||
		errors.Is(err, nativeadapter.ErrAgentCredentialUnavailable) ||
		credentials.CredentialFailureStage(err) != "" {
		t.Fatalf("provider failure was rewritten: %v", err)
	}
}

func TestProductVaultCredentialLeaseAccessPreservesVaultAndRevokeStages(t *testing.T) {
	identity := credentialvault.CredentialIdentity{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-stage",
		CredentialRevision:  9,
	}
	privateFailure := errors.New("private-cipher-detail-must-not-escape")
	for _, test := range []struct {
		name   string
		reader credentialvault.CredentialReader
		revoke bool
		stage  string
	}{
		{
			name: "AAD validation",
			reader: &productAgentVaultReaderFixture{err: credentials.WithCredentialFailureStage(
				credentials.CredentialStageVaultAADValidation,
				privateFailure,
			)},
			stage: credentials.CredentialStageVaultAADValidation,
		},
		{
			name:   "revoked lease",
			reader: &productAgentVaultReaderFixture{},
			revoke: true,
			stage:  credentials.CredentialStageLeaseRevoke,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, err := credentialvault.NewCredentialLeaseManager(test.reader)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			if test.revoke {
				manager.Revoke(identity)
			}
			access, err := newProductVaultCredentialLeaseAccess(manager)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			err = access.UseCredential(
				context.Background(), identity,
				func(context.Context, []byte) error { called = true; return nil },
			)
			if !errors.Is(err, credentials.ErrCredentialStoreUnavailable) ||
				credentials.CredentialFailureStage(err) != test.stage ||
				strings.Contains(err.Error(), privateFailure.Error()) {
				t.Fatalf("closed lease error = %v", err)
			}
			if called {
				t.Fatal("credential callback ran after lease failure")
			}
		})
	}
}

func TestCorruptVaultRecordProjectsExactAgentDiagnosticWithoutProviderCall(t *testing.T) {
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := productDeepSeekFrozenBinding(t)
	identity := credentialvault.CredentialIdentity{
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		CredentialReference: binding.CredentialReference,
		CredentialRevision:  binding.CredentialRevision,
	}
	secret := []byte("vault-agent-diagnostic-secret-must-not-escape")
	secretMarker := append([]byte(nil), secret...)
	defer func() {
		for index := range secretMarker {
			secretMarker[index] = 0
		}
	}()
	if err := store.PutCredential(context.Background(), identity, secret); err != nil {
		t.Fatal(err)
	}
	for index := range secret {
		secret[index] = 0
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := database.Exec(
		`UPDATE encrypted_credentials
		    SET ciphertext = zeroblob(length(ciphertext))
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?`,
		identity.CredentialReference, identity.ProviderID,
		identity.ProviderAccountID, identity.CredentialRevision,
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = database.Close()
		t.Fatalf("corrupted rows = %d, %v", rows, rowsErr)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	material, err = (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, err = credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	leasing, err := credentialvault.NewCredentialLeaseManager(store)
	if err != nil {
		t.Fatal(err)
	}
	defer leasing.Close()
	leaseAccess, err := newProductVaultCredentialLeaseAccess(leasing)
	if err != nil {
		t.Fatal(err)
	}
	credentialAccess, err := newProductAgentCredentialAccess(
		&productAgentCredentialSourceFixture{record: projection.ProviderCredentialRecord{
			ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			CredentialReference: binding.CredentialReference,
			Revision:            binding.CredentialRevision,
			Status:              string(credentials.CredentialVerified),
		}},
		leaseAccess,
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDirectory, "loom.db"),
		4<<10,
		func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.setCredentialRuntime(productCredentialRuntimeVault); err != nil {
		t.Fatal(err)
	}
	doer := &productAgentHTTPDoerFixture{}
	adapter, err := nativeadapter.NewDeepSeekAgentAdapter(
		nativeadapter.DeepSeekAgentAdapterConfig{
			RuntimeInstanceID: binding.RuntimeInstanceID,
			CredentialAccess:  credentialAccess,
			Diagnostics:       diagnostics,
			Client:            doer,
			Now: func() time.Time {
				return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
			},
			MaxResponseBytes: 64 << 10,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	const incidentID = "77777777-7777-4777-8777-777777777777"
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "88888888-8888-4888-8888-888888888888",
		CorrelationID:         incidentID,
		WorkItemID:            "work-vault-diagnostic",
		RunID:                 "run-vault-diagnostic",
		ClaimGeneration:       1,
		RuntimeInstanceID:     binding.RuntimeInstanceID,
		SenderAgentInstanceID: "agent-deepseek-vault",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             time.Date(2026, 8, 12, 11, 59, 59, 0, time.UTC),
		Payload: []byte(
			`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"private prompt must not escape"}`,
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := &productAgentFrameSinkFixture{}
	adapterResult, err := adapter.Execute(context.Background(), supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-vault-diagnostic", RunID: "run-vault-diagnostic",
			ClaimGeneration: 1, RuntimeInstanceID: binding.RuntimeInstanceID,
			SenderAgentInstanceID: "agent-deepseek-vault",
		},
		ExecutionBinding: binding, Dispatch: dispatch, FrameSink: sink,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if doer.calls != 0 || !adapterResult.DispatchAcknowledged() ||
		!adapterResult.ResultAcknowledged() || len(sink.frames) != 2 ||
		string(sink.frames[1].Payload()) !=
			`{"status":"failed","reason":"credential_unavailable"}` {
		t.Fatalf("Provider calls=%d result=%#v frames=%#v", doer.calls, adapterResult, sink.frames)
	}
	summaries, err := diagnostics.AgentAttemptDiagnostics(
		context.Background(),
		[]api.AgentAttemptDiagnosticQuery{{
			IncidentID: incidentID, ProviderID: binding.ProviderID,
			ProviderAccountID: binding.ProviderAccountID, ModelID: binding.ModelID,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := api.AgentAttemptDiagnosticSummary{
		IncidentID: incidentID, ProviderID: binding.ProviderID,
		ProviderAccountID: binding.ProviderAccountID, ModelID: binding.ModelID,
		FailureStage: credentials.CredentialStageVaultDecrypt,
		FailureCode:  "credential_unavailable", Retryable: false,
	}
	if !reflect.DeepEqual(summaries, []api.AgentAttemptDiagnosticSummary{want}) {
		t.Fatalf("diagnostic summaries = %#v, want %#v", summaries, want)
	}
	contents, err := os.ReadFile(filepath.Join(root, "diagnostics", "operational.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		secretMarker,
		[]byte(binding.CredentialReference),
		[]byte("private prompt must not escape"),
		[]byte("ciphertext"),
	} {
		if strings.Contains(string(contents), string(forbidden)) {
			t.Fatalf("operational diagnostics contain forbidden data: %s", contents)
		}
	}
}

func productAgentTestLeasing(
	t testing.TB,
	store credentials.SecretStore,
) productCredentialLeaseAccess {
	t.Helper()
	leasing, err := newProductLegacyCredentialLeaseAccess(store)
	if err != nil {
		t.Fatal(err)
	}
	return leasing
}

func productDeepSeekFrozenBinding(t testing.TB) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                  "deepseek-agent-primary-v1",
		AdapterType:         nativeadapter.DeepSeekAgentAdapterType,
		ProviderID:          nativeadapter.DeepSeekAgentProviderID,
		ProviderAccountID:   "deepseek.primary",
		ModelID:             nativeadapter.DeepSeekAgentModelID,
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: nativeadapter.DeepSeekAgentEndpointFingerprint,
		CredentialReference: "credential-ref-deepseek-current",
		CredentialRevision:  7,
		Timeout:             30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.loom-native.local", DeviceID: "device.local",
		AdapterType: nativeadapter.DeepSeekAgentAdapterType,
		DisplayName: "Loom Native", Status: loomruntime.RuntimeOnline, Capacity: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func TestProductMissionExecutorSelectsSupervisorPerAgentBinding(t *testing.T) {
	piSupervisor := &supervisor.Supervisor{}
	deepSeekSupervisor := &supervisor.Supervisor{}
	codexSupervisor := &supervisor.Supervisor{}
	executor := &productMissionExecutor{
		supervisor: piSupervisor,
		supervisors: map[string]*supervisor.Supervisor{
			"pi-cli\x00runtime.pi.local":               piSupervisor,
			"loom-native\x00runtime.loom-native.local": deepSeekSupervisor,
			"codex\x00runtime.codex.local":             codexSupervisor,
		},
	}
	tests := []struct {
		name     string
		profile  loomruntime.RuntimeProfile
		instance loomruntime.RuntimeInstance
		want     *supervisor.Supervisor
		wantErr  bool
	}{
		{
			name:     "Pi Agent",
			profile:  loomruntime.RuntimeProfile{AdapterType: "pi-cli"},
			instance: loomruntime.RuntimeInstance{ID: "runtime.pi.local"},
			want:     piSupervisor,
		},
		{
			name:     "DeepSeek Loom Native Agent",
			profile:  loomruntime.RuntimeProfile{AdapterType: "loom-native"},
			instance: loomruntime.RuntimeInstance{ID: "runtime.loom-native.local"},
			want:     deepSeekSupervisor,
		},
		{
			name:     "Codex OpenAI Agent",
			profile:  loomruntime.RuntimeProfile{AdapterType: "codex"},
			instance: loomruntime.RuntimeInstance{ID: "runtime.codex.local"},
			want:     codexSupervisor,
		},
		{
			name:     "unknown Provider runtime cannot fall back to Pi",
			profile:  loomruntime.RuntimeProfile{AdapterType: "loom-native"},
			instance: loomruntime.RuntimeInstance{ID: "runtime.unknown"},
			wantErr:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected, err := executor.supervisorFor(supervisor.ExecuteInput{
				Profile: test.profile, Instance: test.instance,
			})
			if (err != nil) != test.wantErr || selected != test.want {
				t.Fatalf("supervisorFor() = %p, %v; want %p error=%t", selected, err, test.want, test.wantErr)
			}
		})
	}
}

func TestEnsureProductNativeAgentRuntimeIsAuthoritativeAndIdempotent(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	for range 2 {
		if err := ensureProductNativeAgentRuntime(
			context.Background(), store, readModel, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ReadStream(
		context.Background(),
		"runtime_instance:runtime.loom-native.local",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "RuntimeInstanceDiscovered" {
		t.Fatalf("runtime events = %#v", events)
	}
	instance, ok := readModel.GlobalReadView().RuntimeInstance("runtime.loom-native.local")
	if !ok || instance.AdapterType != nativeadapter.DeepSeekAgentAdapterType ||
		instance.Status != string(loomruntime.RuntimeOnline) || instance.Capacity != 3 ||
		!reflect.DeepEqual(
			instance.ObservedCapabilities,
			[]string{loomruntime.CapabilityContextRetrieval},
		) ||
		!reflect.DeepEqual(instance.ModelIDs, []string{nativeadapter.DeepSeekAgentModelID}) {
		t.Fatalf("native runtime = %#v, %t", instance, ok)
	}
	for _, expected := range []struct {
		runtimeInstanceID string
		modelID           string
	}{
		{runtimeInstanceID: "runtime.loom-native.kimi", modelID: nativeadapter.KimiAgentModelID},
		{runtimeInstanceID: "runtime.loom-native.minimax", modelID: nativeadapter.MiniMaxAgentModelID},
	} {
		providerRuntime, providerOK := readModel.GlobalReadView().RuntimeInstance(
			expected.runtimeInstanceID,
		)
		if !providerOK || providerRuntime.AdapterType != nativeadapter.LoomNativeAgentAdapterType ||
			providerRuntime.Status != string(loomruntime.RuntimeOnline) ||
			providerRuntime.Capacity != 3 ||
			!reflect.DeepEqual(
				providerRuntime.ObservedCapabilities,
				[]string{loomruntime.CapabilityContextRetrieval},
			) ||
			!reflect.DeepEqual(providerRuntime.ModelIDs, []string{expected.modelID}) {
			t.Fatalf("provider runtime %s = %#v, %t", expected.runtimeInstanceID, providerRuntime, providerOK)
		}
	}
}

func TestEnsureProductNativeAgentRuntimeMigratesOnlyExactHistoricalCapabilitySet(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	now := time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC)
	definition, ok := productNativeAgentRuntimeDefinitionForProvider(
		nativeadapter.DeepSeekAgentProviderID,
	)
	if !ok {
		t.Fatal("DeepSeek runtime definition unavailable")
	}
	if validHistoricalProductNativeAgentRuntime(projection.RuntimeInstance{
		ID: definition.RuntimeInstanceID, DeviceID: "device.local",
		AdapterType: nativeadapter.LoomNativeAgentAdapterType,
		DisplayName: "Loom Native", ExecutableVersion: "v1",
		Status: string(loomruntime.RuntimeOnline), Capacity: 3,
		ObservedCapabilities: []string{"unexpected_capability"},
		ModelIDs:             []string{definition.ModelID},
	}, definition) {
		t.Fatal("runtime with unknown capability accepted as historical migration")
	}
	historical, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{productSetupProjectionProbe{
			id: definition.ProbeID,
			observations: []loomruntime.RuntimeObservation{{
				Instance: loomruntime.RuntimeInstance{
					ID: definition.RuntimeInstanceID, DeviceID: "device.local",
					AdapterType: nativeadapter.LoomNativeAgentAdapterType,
					DisplayName: "Loom Native", ExecutableVersion: "v1",
					Status: loomruntime.RuntimeOnline, Capacity: 3,
				},
				ModelIDs: []string{definition.ModelID},
			}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := state.CommitRuntimeDiscoverySnapshot(
		context.Background(), store, historical,
		state.RuntimeDiscoveryCommitInput{
			DiscoveryID: productDeterministicUUID("historical-native-runtime", "discovery"),
			EmittedAt:   now,
			Events: []state.RuntimeDiscoveryEventInput{{
				RuntimeInstanceID: definition.RuntimeInstanceID,
				EventID:           productDeterministicUUID("historical-native-runtime", "event"),
				IdempotencyKey: "historical-native-runtime." +
					productDeterministicUUID("historical-native-runtime", "idempotency"),
				Seq: 1,
			}},
		},
	)
	if err != nil || !commit.Committed() {
		t.Fatalf("historical commit = %#v, %v", commit, err)
	}
	if err := ensureProductNativeAgentProviderRuntime(
		context.Background(), store, readModel, now.Add(time.Second), definition.ProviderID,
	); err != nil {
		t.Fatal(err)
	}
	if err := ensureProductNativeAgentProviderRuntime(
		context.Background(), store, readModel, now.Add(2*time.Second), definition.ProviderID,
	); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadStream(
		context.Background(), "runtime_instance:"+definition.RuntimeInstanceID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Seq != 2 {
		t.Fatalf("runtime events = %#v", events)
	}
	instance, found := readModel.GlobalReadView().RuntimeInstance(definition.RuntimeInstanceID)
	if !found || !reflect.DeepEqual(
		instance.ObservedCapabilities,
		[]string{loomruntime.CapabilityContextRetrieval},
	) {
		t.Fatalf("migrated native runtime = %#v, %t", instance, found)
	}
}

func TestEnsureProductClaudeCodeAgentRuntimeIsExecutableBoundAndIdempotent(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	executable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC)
	for range 2 {
		if err := ensureProductClaudeCodeAgentRuntime(
			context.Background(), store, readModel, now, executable,
		); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ReadStream(
		context.Background(), "runtime_instance:"+productClaudeCodeRuntimeInstanceID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "RuntimeInstanceDiscovered" {
		t.Fatalf("runtime events = %#v", events)
	}
	instance, ok := readModel.GlobalReadView().RuntimeInstance(
		productClaudeCodeRuntimeInstanceID,
	)
	if !ok || instance.AdapterType != harnessadapter.ClaudeCodeAdapterType ||
		instance.DisplayName != "Claude Code" ||
		instance.Status != string(loomruntime.RuntimeOnline) || instance.Capacity != 2 ||
		!strings.HasPrefix(instance.ExecutableVersion, "sha256:") ||
		!reflect.DeepEqual(instance.ObservedCapabilities, []string{"workspace_edit"}) ||
		!reflect.DeepEqual(instance.ModelIDs, []string{harnessadapter.ClaudeCodeModelID}) {
		t.Fatalf("Claude Code runtime = %#v, %t", instance, ok)
	}
}

func TestHarnessRuntimeProbesPublishContextRetrievalOnlyForAttestedExecutable(t *testing.T) {
	for _, test := range []struct {
		name         string
		probe        loomruntime.RuntimeProbe
		capabilities []string
	}{
		{
			name: "Claude attested",
			probe: productClaudeCodeAgentRuntimeProbe{
				executableVersion: "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a",
			},
			capabilities: []string{
				loomruntime.CapabilityContextRetrieval,
				loomruntime.CapabilityGovernedToolLoop,
				"workspace_edit",
			},
		},
		{
			name: "Codex attested",
			probe: productCodexAgentRuntimeProbe{
				executableVersion: "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a",
			},
			capabilities: []string{
				loomruntime.CapabilityContextRetrieval,
				loomruntime.CapabilityGovernedToolLoop,
				"reasoning_effort", "workspace_edit",
			},
		},
		{
			name: "Claude drift",
			probe: productClaudeCodeAgentRuntimeProbe{
				executableVersion: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			},
			capabilities: []string{"workspace_edit"},
		},
		{
			name: "Codex drift",
			probe: productCodexAgentRuntimeProbe{
				executableVersion: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			},
			capabilities: []string{"reasoning_effort", "workspace_edit"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			observations, err := test.probe.ObserveRuntime(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(observations) != 1 || !reflect.DeepEqual(
				observations[0].Instance.ObservedCapabilities, test.capabilities,
			) {
				t.Fatalf("observations = %#v", observations)
			}
		})
	}
}

func TestHistoricalHarnessRuntimeUpgradeRequiresExactPriorCapabilities(t *testing.T) {
	claudeVersion := "sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a"
	claude := projection.RuntimeInstance{
		ID: productClaudeCodeRuntimeInstanceID, DeviceID: "device.local",
		AdapterType: harnessadapter.ClaudeCodeAdapterType, DisplayName: "Claude Code",
		ExecutableVersion: claudeVersion, Status: string(loomruntime.RuntimeOnline),
		ObservedCapabilities: []string{"workspace_edit"}, Capacity: 2,
		ModelIDs: []string{harnessadapter.ClaudeCodeModelID},
	}
	if !validHistoricalProductClaudeCodeAgentRuntime(claude, claudeVersion) {
		t.Fatal("exact historical Claude runtime was not eligible")
	}
	claude.ObservedCapabilities = []string{"unknown", "workspace_edit"}
	if validHistoricalProductClaudeCodeAgentRuntime(claude, claudeVersion) {
		t.Fatal("unknown Claude capability set was eligible")
	}
	codexVersion := "sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a"
	codex := projection.RuntimeInstance{
		ID: productCodexRuntimeInstanceID, DeviceID: "device.local",
		AdapterType: harnessadapter.CodexAdapterType, DisplayName: "Codex",
		ExecutableVersion: codexVersion, Status: string(loomruntime.RuntimeOnline),
		ObservedCapabilities: []string{"reasoning_effort", "workspace_edit"}, Capacity: 2,
		ModelIDs: []string{harnessadapter.CodexModelID},
	}
	if !validHistoricalProductCodexAgentRuntime(codex, codexVersion) {
		t.Fatal("exact historical Codex runtime was not eligible")
	}
	codex.DisplayName = "drift"
	if validHistoricalProductCodexAgentRuntime(codex, codexVersion) {
		t.Fatal("identity-drifted Codex runtime was eligible")
	}
}

func TestHarnessProfileCapabilitiesFollowObservedConformance(t *testing.T) {
	base := []string{"workspace_edit"}
	attested := loomruntime.RuntimeObservation{Instance: loomruntime.RuntimeInstance{
		ObservedCapabilities: []string{
			loomruntime.CapabilityContextRetrieval,
			loomruntime.CapabilityGovernedToolLoop,
			"workspace_edit",
		},
	}}
	if got := productHarnessProfileCapabilities(attested, base); !reflect.DeepEqual(
		got, []string{
			loomruntime.CapabilityContextRetrieval,
			loomruntime.CapabilityGovernedToolLoop,
			"workspace_edit",
		},
	) {
		t.Fatalf("attested capabilities = %#v", got)
	}
	drifted := loomruntime.RuntimeObservation{Instance: loomruntime.RuntimeInstance{
		ObservedCapabilities: []string{"workspace_edit"},
	}}
	if got := productHarnessProfileCapabilities(drifted, base); !reflect.DeepEqual(
		got, []string{"workspace_edit"},
	) {
		t.Fatalf("drifted capabilities = %#v", got)
	}
	if !reflect.DeepEqual(base, []string{"workspace_edit"}) {
		t.Fatalf("base capabilities mutated = %#v", base)
	}
}

func TestEnsureProductCodexAgentRuntimeIsExecutableBoundAndIdempotent(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 16, 0, 0, 0, time.UTC)
	for range 2 {
		if err := ensureProductCodexAgentRuntime(
			context.Background(), store, readModel, now, executable,
		); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ReadStream(
		context.Background(), "runtime_instance:"+productCodexRuntimeInstanceID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "RuntimeInstanceDiscovered" {
		t.Fatalf("runtime events = %#v", events)
	}
	instance, ok := readModel.GlobalReadView().RuntimeInstance(
		productCodexRuntimeInstanceID,
	)
	if !ok || instance.AdapterType != harnessadapter.CodexAdapterType ||
		instance.DisplayName != "Codex" ||
		instance.Status != string(loomruntime.RuntimeOnline) || instance.Capacity != 2 ||
		!strings.HasPrefix(instance.ExecutableVersion, "sha256:") ||
		!reflect.DeepEqual(
			instance.ObservedCapabilities,
			[]string{"reasoning_effort", "workspace_edit"},
		) || !reflect.DeepEqual(instance.ModelIDs, []string{harnessadapter.CodexModelID}) {
		t.Fatalf("Codex runtime = %#v, %t", instance, ok)
	}
}

func TestProductSetupCatalogPublishesVerifiedCodexOpenAIProfile(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 16, 30, 0, 0, time.UTC)
	if err := ensureProductCodexAgentRuntime(
		context.Background(), store, readModel, now, executable,
	); err != nil {
		t.Fatal(err)
	}
	unconfigured, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range unconfigured.RuntimeProfiles {
		if profile.ProviderID == harnessadapter.CodexProviderID {
			t.Fatalf("native auth alone published brokered Codex profile = %#v", profile)
		}
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "configure-openai-agent-catalog", ProviderID: "openai",
			CredentialReference: "credential-ref-openai-agent-catalog",
			ExpectedRevision:    0, OccurredAt: now.Add(time.Second),
			Status: credentials.CredentialConfigured,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "verify-openai-agent-catalog", ProviderID: "openai",
			CredentialReference: configured.CredentialReference,
			ExpectedRevision:    configured.Revision, OccurredAt: now.Add(2 * time.Second),
			Status: credentials.CredentialVerified,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	profiles := 0
	options := 0
	for _, profile := range catalog.RuntimeProfiles {
		if profile.ProviderID != harnessadapter.CodexProviderID {
			continue
		}
		profiles++
		if profile.AdapterType != harnessadapter.CodexAdapterType ||
			profile.ProviderAccountID != "openai.primary" ||
			profile.ModelID != harnessadapter.CodexModelID ||
			profile.AuthMode != loomruntime.AuthBrokered ||
			profile.EndpointFingerprint != harnessadapter.CodexEndpointFingerprint ||
			profile.CredentialReference != verified.CredentialReference ||
			profile.CredentialRevision != verified.Revision ||
			profile.ReasoningEffort != "high" ||
			!reflect.DeepEqual(
				profile.RequiredCapabilities,
				[]string{loomruntime.CapabilityReasoningEffort, "workspace_edit"},
			) {
			t.Fatalf("Codex profile = %#v", profile)
		}
		if _, err := loomruntime.ValidateExecutionProfile(profile); err != nil {
			t.Fatalf("Codex execution profile is not freezable: %v", err)
		}
	}
	for _, option := range catalog.RoleOptions {
		if !strings.HasPrefix(option.RuntimeProfileID, "loom-openai-") {
			continue
		}
		options++
		if option.RuntimeInstanceID != productCodexRuntimeInstanceID {
			t.Fatalf("Codex option = %#v", option)
		}
	}
	if profiles != 2 || options != 5 {
		t.Fatalf("Codex profiles=%d options=%d catalog=%#v", profiles, options, catalog)
	}
	assertProductSetupCatalogRoleOptionsFreezable(t, catalog)
}

func TestProductSetupCatalogPublishesVerifiedClaudeCodeAnthropicProfile(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	executable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 15, 30, 0, 0, time.UTC)
	if err := ensureProductClaudeCodeAgentRuntime(
		context.Background(), store, readModel, now, executable,
	); err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "configure-anthropic-agent-catalog", ProviderID: "anthropic",
			CredentialReference: "credential-ref-anthropic-agent-catalog",
			ExpectedRevision:    0, OccurredAt: now.Add(time.Second),
			Status: credentials.CredentialConfigured,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID: "verify-anthropic-agent-catalog", ProviderID: "anthropic",
			CredentialReference: configured.CredentialReference,
			ExpectedRevision:    configured.Revision, OccurredAt: now.Add(2 * time.Second),
			Status: credentials.CredentialVerified,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	profiles := 0
	options := 0
	for _, profile := range catalog.RuntimeProfiles {
		if profile.ProviderID != harnessadapter.ClaudeCodeProviderID {
			continue
		}
		profiles++
		if profile.AdapterType != harnessadapter.ClaudeCodeAdapterType ||
			profile.ProviderAccountID != "anthropic.primary" ||
			profile.ModelID != harnessadapter.ClaudeCodeModelID ||
			profile.AuthMode != loomruntime.AuthBrokered ||
			profile.EndpointFingerprint != harnessadapter.ClaudeCodeEndpointFingerprint ||
			profile.CredentialReference != verified.CredentialReference ||
			profile.CredentialRevision != verified.Revision ||
			!reflect.DeepEqual(profile.RequiredCapabilities, []string{"workspace_edit"}) {
			t.Fatalf("Claude Code profile = %#v", profile)
		}
	}
	for _, option := range catalog.RoleOptions {
		if !strings.HasPrefix(option.RuntimeProfileID, "loom-anthropic-") {
			continue
		}
		options++
		if option.RuntimeInstanceID != productClaudeCodeRuntimeInstanceID {
			t.Fatalf("Claude Code option = %#v", option)
		}
	}
	if profiles != 2 || options != 5 {
		t.Fatalf("Claude Code profiles=%d options=%d catalog=%#v", profiles, options, catalog)
	}
	assertProductSetupCatalogRoleOptionsFreezable(t, catalog)
}

type productAgentFailureAdapterFixture struct{}

func (*productAgentFailureAdapterFixture) AdapterType() string { return "loom-native" }

func (*productAgentFailureAdapterFixture) RuntimeInstanceID() string {
	return productNativeAgentRuntimeInstanceID
}

func (*productAgentFailureAdapterFixture) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	frames := make([]bridgev1.Frame, 0, 2)
	for index, record := range []struct {
		kind    bridgev1.MessageType
		payload any
	}{
		{kind: bridgev1.MessageAck, payload: struct {
			MessageID string `json:"message_id"`
		}{request.Dispatch.MessageID()}},
		{kind: bridgev1.MessageResult, payload: struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}{Status: "failed", Reason: "provider_auth"}},
	} {
		payload, err := json.Marshal(record.payload)
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID:             productDeterministicUUID("agent-failure", string(record.kind)),
			CorrelationID:         request.Dispatch.CorrelationID(),
			WorkItemID:            request.Binding.WorkItemID,
			RunID:                 request.Binding.RunID,
			ClaimGeneration:       request.Binding.ClaimGeneration,
			RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
			SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
			Sequence:              int64(index + 2),
			Type:                  record.kind,
			EmittedAt:             request.Dispatch.EmittedAt(),
			Payload:               payload,
		})
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
		frames = append(frames, frame)
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: frames, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

type productAgentFrameSinkFixture struct {
	frames []bridgev1.Frame
}

func (sink *productAgentFrameSinkFixture) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	return nil
}

func TestProductRuntimeWrapperPreservesAgentProviderFailureReason(t *testing.T) {
	adapter, err := newProductPiRuntimeAdapter(&productAgentFailureAdapterFixture{})
	if err != nil {
		t.Fatal(err)
	}
	binding := productDeepSeekFrozenBinding(t)
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "11111111-1111-4111-8111-111111111111",
		CorrelationID:         "22222222-2222-4222-8222-222222222222",
		WorkItemID:            "work-agent-failure",
		RunID:                 "run-agent-failure",
		ClaimGeneration:       1,
		RuntimeInstanceID:     binding.RuntimeInstanceID,
		SenderAgentInstanceID: "agent-deepseek",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC),
		Payload:               []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"bounded task"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := &productAgentFrameSinkFixture{}
	result, err := adapter.Execute(context.Background(), supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-agent-failure", RunID: "run-agent-failure",
			ClaimGeneration: 1, RuntimeInstanceID: binding.RuntimeInstanceID,
			SenderAgentInstanceID: "agent-deepseek",
		},
		ExecutionBinding: binding,
		Dispatch:         dispatch,
		FrameSink:        sink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DispatchAcknowledged() || !result.ResultAcknowledged() ||
		len(sink.frames) != 2 || sink.frames[1].Type() != bridgev1.MessageResult ||
		string(sink.frames[1].Payload()) != `{"status":"failed","reason":"provider_auth"}` {
		t.Fatalf("result=%#v frames=%#v", result, sink.frames)
	}
}

func TestProductSetupCatalogPublishesOnlyVerifiedExecutableDeepSeekProfiles(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	if err := ensureProductNativeAgentRuntime(
		context.Background(), store, readModel, now,
	); err != nil {
		t.Fatal(err)
	}

	unconfigured, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range unconfigured.RuntimeProfiles {
		if profile.ProviderID == nativeadapter.DeepSeekAgentProviderID {
			t.Fatalf("unverified DeepSeek profile published: %#v", profile)
		}
	}

	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "configure-deepseek-agent-catalog",
			ProviderID:          nativeadapter.DeepSeekAgentProviderID,
			CredentialReference: "credential-ref-deepseek-agent-catalog",
			ExpectedRevision:    0,
			OccurredAt:          now.Add(time.Second),
			Status:              credentials.CredentialConfigured,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "verify-deepseek-agent-catalog",
			ProviderID:          nativeadapter.DeepSeekAgentProviderID,
			CredentialReference: configured.CredentialReference,
			ExpectedRevision:    configured.Revision,
			OccurredAt:          now.Add(2 * time.Second),
			Status:              credentials.CredentialVerified,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	deepSeekProfiles := 0
	for _, profile := range catalog.RuntimeProfiles {
		if profile.ProviderID != nativeadapter.DeepSeekAgentProviderID {
			continue
		}
		deepSeekProfiles++
		if profile.AdapterType != nativeadapter.DeepSeekAgentAdapterType ||
			profile.ProviderAccountID != "deepseek.primary" ||
			profile.ModelID != nativeadapter.DeepSeekAgentModelID ||
			profile.AuthMode != loomruntime.AuthBrokered ||
			profile.EndpointFingerprint != nativeadapter.DeepSeekAgentEndpointFingerprint ||
			profile.CredentialReference != verified.CredentialReference ||
			profile.CredentialRevision != verified.Revision ||
			!reflect.DeepEqual(
				profile.RequiredCapabilities,
				[]string{loomruntime.CapabilityContextRetrieval},
			) {
			t.Fatalf("DeepSeek profile = %#v", profile)
		}
	}
	deepSeekOptions := 0
	for _, option := range catalog.RoleOptions {
		if strings.HasPrefix(option.RuntimeProfileID, "loom-deepseek-") {
			deepSeekOptions++
			if option.RuntimeInstanceID != productNativeAgentRuntimeInstanceID {
				t.Fatalf("DeepSeek option = %#v", option)
			}
		}
	}
	if deepSeekProfiles != 2 || deepSeekOptions != 5 {
		t.Fatalf("DeepSeek profiles=%d options=%d catalog=%#v", deepSeekProfiles, deepSeekOptions, catalog)
	}
	assertProductSetupCatalogRoleOptionsFreezable(t, catalog)
}

func TestProductSetupCatalogPublishesVerifiedKimiAndMiniMaxProfilesIndependently(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	now := time.Date(2026, 8, 10, 10, 30, 0, 0, time.UTC)
	if err := ensureProductNativeAgentRuntime(
		context.Background(), store, readModel, now,
	); err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		providerID          string
		modelID             string
		endpointFingerprint string
		runtimeInstanceID   string
	}{
		{
			providerID:          nativeadapter.KimiAgentProviderID,
			modelID:             nativeadapter.KimiAgentModelID,
			endpointFingerprint: nativeadapter.KimiAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.kimi",
		},
		{
			providerID:          nativeadapter.MiniMaxAgentProviderID,
			modelID:             nativeadapter.MiniMaxAgentModelID,
			endpointFingerprint: nativeadapter.MiniMaxAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.minimax",
		},
	}
	verified := make(map[string]credentials.MetadataResult, len(tests))
	for index, test := range tests {
		configured, commitErr := writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID:           "configure-" + test.providerID + "-agent-catalog",
				ProviderID:          test.providerID,
				CredentialReference: "credential-ref-" + test.providerID + "-agent-catalog",
				ExpectedRevision:    0,
				OccurredAt:          now.Add(time.Duration(index+1) * time.Second),
				Status:              credentials.CredentialConfigured,
			},
		)
		if commitErr != nil {
			t.Fatal(commitErr)
		}
		verified[test.providerID], commitErr = writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID:           "verify-" + test.providerID + "-agent-catalog",
				ProviderID:          test.providerID,
				CredentialReference: configured.CredentialReference,
				ExpectedRevision:    configured.Revision,
				OccurredAt:          now.Add(time.Duration(index+3) * time.Second),
				Status:              credentials.CredentialVerified,
			},
		)
		if commitErr != nil {
			t.Fatal(commitErr)
		}
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		profiles := 0
		options := 0
		for _, profile := range catalog.RuntimeProfiles {
			if profile.ProviderID != test.providerID {
				continue
			}
			profiles++
			if profile.AdapterType != nativeadapter.LoomNativeAgentAdapterType ||
				profile.ProviderAccountID != test.providerID+".primary" ||
				profile.ModelID != test.modelID ||
				profile.AuthMode != loomruntime.AuthBrokered ||
				profile.EndpointFingerprint != test.endpointFingerprint ||
				profile.CredentialReference != verified[test.providerID].CredentialReference ||
				profile.CredentialRevision != verified[test.providerID].Revision ||
				!reflect.DeepEqual(
					profile.RequiredCapabilities,
					[]string{loomruntime.CapabilityContextRetrieval},
				) {
				t.Fatalf("%s profile = %#v", test.providerID, profile)
			}
		}
		for _, option := range catalog.RoleOptions {
			if !strings.HasPrefix(option.RuntimeProfileID, "loom-"+test.providerID+"-") {
				continue
			}
			options++
			if option.RuntimeInstanceID != test.runtimeInstanceID {
				t.Fatalf("%s option = %#v", test.providerID, option)
			}
		}
		if profiles != 2 || options != 5 {
			t.Fatalf("%s profiles=%d options=%d catalog=%#v", test.providerID, profiles, options, catalog)
		}
	}
	assertProductSetupCatalogRoleOptionsFreezable(t, catalog)
}

func TestProductSetupCatalogPublishesAndRemovesDeepSeekAccountsIndependently(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	now := time.Date(2026, 8, 10, 11, 0, 0, 0, time.UTC)
	if err := ensureProductNativeAgentRuntime(
		context.Background(), store, readModel, now,
	); err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	verified := make(map[string]credentials.MetadataResult)
	for index, accountID := range []string{"deepseek.primary", "deepseek.work"} {
		configured, commitErr := writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID:           "configure-" + accountID,
				ProviderID:          nativeadapter.DeepSeekAgentProviderID,
				ProviderAccountID:   accountID,
				CredentialReference: "credential-ref-" + strings.ReplaceAll(accountID, ".", "-"),
				ExpectedRevision:    0,
				OccurredAt:          now.Add(time.Duration(index+1) * time.Second),
				Status:              credentials.CredentialConfigured,
			},
		)
		if commitErr != nil {
			t.Fatal(commitErr)
		}
		verified[accountID], commitErr = writer.CommitCredentialMetadata(
			context.Background(),
			credentials.MetadataCommand{
				CommandID:           "verify-" + accountID,
				ProviderID:          nativeadapter.DeepSeekAgentProviderID,
				ProviderAccountID:   accountID,
				CredentialReference: configured.CredentialReference,
				ExpectedRevision:    configured.Revision,
				OccurredAt:          now.Add(time.Duration(index+3) * time.Second),
				Status:              credentials.CredentialVerified,
			},
		)
		if commitErr != nil {
			t.Fatal(commitErr)
		}
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	assertProductDeepSeekAccountProfiles(t, catalog, verified)
	initialDigest := catalog.CatalogDigest

	work := verified["deepseek.work"]
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "revoke-deepseek.work",
			ProviderID:          nativeadapter.DeepSeekAgentProviderID,
			ProviderAccountID:   "deepseek.work",
			CredentialReference: work.CredentialReference,
			ExpectedRevision:    work.Revision,
			OccurredAt:          now.Add(6 * time.Second),
			Status:              credentials.CredentialRevoked,
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	afterRevoke, err := productSetupCatalogForView(
		context.Background(), readModel.GlobalReadView(),
	)
	if err != nil {
		t.Fatal(err)
	}
	assertProductDeepSeekAccountProfiles(t, afterRevoke, map[string]credentials.MetadataResult{
		"deepseek.primary": verified["deepseek.primary"],
	})
	if afterRevoke.CatalogDigest == initialDigest {
		t.Fatal("account revocation did not change catalog digest")
	}
}

func assertProductDeepSeekAccountProfiles(
	t testing.TB,
	catalog app.LocalProductSetupCatalog,
	want map[string]credentials.MetadataResult,
) {
	t.Helper()
	profiles := make(map[string][]loomruntime.RuntimeProfile)
	for _, profile := range catalog.RuntimeProfiles {
		if profile.ProviderID == nativeadapter.DeepSeekAgentProviderID {
			profiles[profile.ProviderAccountID] = append(
				profiles[profile.ProviderAccountID], profile,
			)
		}
	}
	if len(profiles) != len(want) {
		t.Fatalf("DeepSeek account profiles=%#v want accounts=%#v", profiles, want)
	}
	for accountID, result := range want {
		accountProfiles := profiles[accountID]
		if len(accountProfiles) != 2 {
			t.Fatalf("account %q profiles=%#v", accountID, accountProfiles)
		}
		for _, profile := range accountProfiles {
			if profile.CredentialReference != result.CredentialReference ||
				profile.CredentialRevision != result.Revision ||
				profile.ModelID != nativeadapter.DeepSeekAgentModelID ||
				!reflect.DeepEqual(
					profile.RequiredCapabilities,
					[]string{loomruntime.CapabilityContextRetrieval},
				) {
				t.Fatalf("account %q profile=%#v result=%#v", accountID, profile, result)
			}
		}
	}
	options := 0
	for _, option := range catalog.RoleOptions {
		if strings.HasPrefix(option.RuntimeProfileID, "loom-deepseek-") {
			options++
		}
	}
	if options != len(want)*5 {
		t.Fatalf("DeepSeek options=%d want=%d", options, len(want)*5)
	}
}

func assertProductSetupCatalogRoleOptionsFreezable(
	t testing.TB,
	catalog app.LocalProductSetupCatalog,
) {
	t.Helper()
	profiles := make(map[string]loomruntime.RuntimeProfile, len(catalog.RuntimeProfiles))
	for _, profile := range catalog.RuntimeProfiles {
		profiles[profile.ID] = profile
	}
	instances := make(map[string]loomruntime.RuntimeObservation)
	for _, observation := range catalog.RuntimeDiscovery.Observations() {
		instances[observation.Instance.ID] = observation
	}
	for _, option := range catalog.RoleOptions {
		profile, profileOK := profiles[option.RuntimeProfileID]
		observation, instanceOK := instances[option.RuntimeInstanceID]
		if !profileOK || !instanceOK {
			t.Fatalf("role option %q has no exact execution profile/runtime", option.ID)
		}
		if !productAgentContainsString(observation.ModelIDs, profile.ModelID) {
			t.Fatalf(
				"role option %q model %q is absent from runtime %q: %v",
				option.ID,
				profile.ModelID,
				observation.Instance.ID,
				observation.ModelIDs,
			)
		}
		if _, err := loomruntime.FreezeExecutionBinding(
			profile,
			observation.Instance,
		); err != nil {
			t.Fatalf("role option %q is not freezable: %v", option.ID, err)
		}
	}
}
