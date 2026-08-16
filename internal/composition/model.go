// Package composition implements Loom's deterministic, in-process composition
// kernel. It composes bounded service ports; it never grants execution authority.
package composition

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
)

var (
	ErrInvalidComposition    = errors.New("invalid composition")
	ErrCompositionConflict   = errors.New("composition conflict")
	ErrCompositionNotReady   = errors.New("composition not ready")
	ErrCapabilityUnavailable = errors.New("capability unavailable")
	ErrCapabilityProtected   = errors.New("protected capability")
	ErrScopeClosed           = errors.New("capability scope closed")
)

type ProfileID string

const (
	ProfileDesktop  ProfileID = "desktop"
	ProfileHeadless ProfileID = "headless"
	ProfileTest     ProfileID = "test"
)

type CapabilityID string

const (
	CapabilitySnapshotActivation     CapabilityID = "core.composition-activation.v1"
	CapabilityCoreReadiness          CapabilityID = "core.readiness.v1"
	CapabilityJournalAppendAuthority CapabilityID = "core.journal-append-authority.v1"
	CapabilityProjectionActivation   CapabilityID = "core.projection-activation.v1"
	CapabilityStateWriter            CapabilityID = "core.state-writer.v1"
	CapabilityPolicyAuthority        CapabilityID = "core.policy-authority.v1"
	CapabilityGrantAuthority         CapabilityID = "core.grant-authority.v1"
	CapabilityApprovalAuthority      CapabilityID = "core.approval-authority.v1"
	CapabilityTerminalAuthority      CapabilityID = "core.terminal-authority.v1"
	CapabilityVaultRoot              CapabilityID = "core.vault-root.v1"
	CapabilityCredentialMigration    CapabilityID = "core.credential-migration-authority.v1"
	CapabilityIPCAttestation         CapabilityID = "core.ipc-attestation-policy.v1"
	CapabilityObservabilityRecorder  CapabilityID = "observability.recorder.v1"
	CapabilityAssetsReader           CapabilityID = "assets.reader.v1"
	CapabilityCredentialLeaseIssuer  CapabilityID = "vault.credential-lease-issuer.v1"
	CapabilityConversationRouter     CapabilityID = "conversation.router.v1"
	CapabilityRuntimeDispatcher      CapabilityID = "runtime.dispatcher.v1"
	CapabilityScopedContextRetriever CapabilityID = "context.scoped-retriever.v1"
	CapabilityApprovedToolInvoker    CapabilityID = "governance.approved-tool-invoker.v1"
	CapabilityWorkCoordinator        CapabilityID = "work.coordinator.v1"
	CapabilityLocalIPCHandler        CapabilityID = "local-ipc.handler.v1"
)

type capabilityDefinition struct {
	Ref       CapabilityRef
	Protected bool
}

var capabilityCatalog = map[CapabilityID]capabilityDefinition{
	CapabilitySnapshotActivation:     {Ref: CapabilityRef{ID: CapabilitySnapshotActivation, Version: 1}, Protected: true},
	CapabilityCoreReadiness:          {Ref: CapabilityRef{ID: CapabilityCoreReadiness, Version: 1}},
	CapabilityJournalAppendAuthority: {Ref: CapabilityRef{ID: CapabilityJournalAppendAuthority, Version: 1}, Protected: true},
	CapabilityProjectionActivation:   {Ref: CapabilityRef{ID: CapabilityProjectionActivation, Version: 1}, Protected: true},
	CapabilityStateWriter:            {Ref: CapabilityRef{ID: CapabilityStateWriter, Version: 1}, Protected: true},
	CapabilityPolicyAuthority:        {Ref: CapabilityRef{ID: CapabilityPolicyAuthority, Version: 1}, Protected: true},
	CapabilityGrantAuthority:         {Ref: CapabilityRef{ID: CapabilityGrantAuthority, Version: 1}, Protected: true},
	CapabilityApprovalAuthority:      {Ref: CapabilityRef{ID: CapabilityApprovalAuthority, Version: 1}, Protected: true},
	CapabilityTerminalAuthority:      {Ref: CapabilityRef{ID: CapabilityTerminalAuthority, Version: 1}, Protected: true},
	CapabilityVaultRoot:              {Ref: CapabilityRef{ID: CapabilityVaultRoot, Version: 1}, Protected: true},
	CapabilityCredentialMigration:    {Ref: CapabilityRef{ID: CapabilityCredentialMigration, Version: 1}, Protected: true},
	CapabilityIPCAttestation:         {Ref: CapabilityRef{ID: CapabilityIPCAttestation, Version: 1}, Protected: true},
	CapabilityObservabilityRecorder:  {Ref: CapabilityRef{ID: CapabilityObservabilityRecorder, Version: 1}},
	CapabilityAssetsReader:           {Ref: CapabilityRef{ID: CapabilityAssetsReader, Version: 1}},
	CapabilityCredentialLeaseIssuer:  {Ref: CapabilityRef{ID: CapabilityCredentialLeaseIssuer, Version: 1}},
	CapabilityConversationRouter:     {Ref: CapabilityRef{ID: CapabilityConversationRouter, Version: 1}},
	CapabilityRuntimeDispatcher:      {Ref: CapabilityRef{ID: CapabilityRuntimeDispatcher, Version: 1}},
	CapabilityScopedContextRetriever: {Ref: CapabilityRef{ID: CapabilityScopedContextRetriever, Version: 1}},
	CapabilityApprovedToolInvoker:    {Ref: CapabilityRef{ID: CapabilityApprovedToolInvoker, Version: 1}},
	CapabilityWorkCoordinator:        {Ref: CapabilityRef{ID: CapabilityWorkCoordinator, Version: 1}},
	CapabilityLocalIPCHandler:        {Ref: CapabilityRef{ID: CapabilityLocalIPCHandler, Version: 1}},
}

var builtInBundleVersions = map[string]string{
	"loom-core":          "1.0.0",
	"loom-vault":         "1.0.0",
	"loom-conversation":  "1.0.0",
	"loom-agent-runtime": "1.0.0",
	"loom-governance":    "1.0.0",
	"loom-work":          "1.0.0",
	"loom-assets":        "1.0.0",
	"loom-observability": "1.0.0",
	"loom-local-ipc":     "1.0.0",
}

type CapabilityRef struct {
	ID      CapabilityID `json:"id"`
	Version int          `json:"version"`
}

func (id CapabilityID) Ref() CapabilityRef {
	definition, ok := capabilityCatalog[id]
	if !ok {
		return CapabilityRef{}
	}
	return definition.Ref
}

type CapabilityKey[T any] struct {
	ref       CapabilityRef
	valueType reflect.Type
}

func MustCapabilityKey[T any](id CapabilityID) CapabilityKey[T] {
	key, err := NewCapabilityKey[T](id)
	if err != nil {
		panic(err)
	}
	return key
}

func NewCapabilityKey[T any](id CapabilityID) (CapabilityKey[T], error) {
	definition, ok := capabilityCatalog[id]
	valueType := reflect.TypeOf((*T)(nil)).Elem()
	if !ok || valueType == nil || valueType.Kind() != reflect.Interface ||
		valueType.NumMethod() == 0 {
		return CapabilityKey[T]{}, ErrInvalidComposition
	}
	return CapabilityKey[T]{ref: definition.Ref, valueType: valueType}, nil
}

func (key CapabilityKey[T]) Ref() CapabilityRef { return key.ref }

func ProtectedCapabilities() []CapabilityRef {
	result := make([]CapabilityRef, 0)
	for _, definition := range capabilityCatalog {
		if definition.Protected {
			result = append(result, definition.Ref)
		}
	}
	sortCapabilityRefs(result)
	return result
}

func capabilityProtected(ref CapabilityRef) bool {
	definition, ok := capabilityCatalog[ref.ID]
	return ok && definition.Ref == ref && definition.Protected
}

func validCapabilityRef(ref CapabilityRef) bool {
	definition, ok := capabilityCatalog[ref.ID]
	return ok && definition.Ref == ref
}

type RouteMethod string

const RouteJournalAppend RouteMethod = "journal.append"

var protectedRoutes = map[RouteMethod]bool{RouteJournalAppend: true}

type PrivacyClass string

const (
	PrivacyMetadataOnly PrivacyClass = "metadata_only"
	PrivacyLocalContent PrivacyClass = "local_content"
)

type RouteDescriptor struct {
	Method               RouteMethod     `json:"method"`
	SchemaVersion        int             `json:"schema_version"`
	OwnerBundle          string          `json:"owner_bundle"`
	RequiredCapabilities []CapabilityRef `json:"required_capabilities,omitempty"`
	HandlerCapability    CapabilityRef   `json:"handler_capability"`
	AvailabilityFailure  string          `json:"availability_failure,omitempty"`
	IncidentPolicy       string          `json:"incident_policy,omitempty"`
	PrivacyClass         PrivacyClass    `json:"privacy_class"`
}

type BundleConstraint struct {
	BundleID string `json:"bundle_id"`
	Version  string `json:"version"`
}

type BundleDescriptor struct {
	SchemaVersion    int                `json:"schema_version"`
	ID               string             `json:"id"`
	Version          string             `json:"version"`
	Requires         []CapabilityRef    `json:"requires,omitempty"`
	OptionalRequires []CapabilityRef    `json:"optional_requires,omitempty"`
	Provides         []CapabilityRef    `json:"provides,omitempty"`
	Routes           []RouteDescriptor  `json:"routes,omitempty"`
	LifecycleID      string             `json:"lifecycle_id"`
	CoreProtected    bool               `json:"core_protected"`
	Compatibility    []BundleConstraint `json:"compatibility,omitempty"`
}

type ProfileBundle struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Mandatory bool   `json:"mandatory"`
}

type RouteUnavailablePolicy string

const (
	RouteUnavailableFailStartup RouteUnavailablePolicy = "fail_startup"
	RouteUnavailableOmit        RouteUnavailablePolicy = "omit"
)

type LaunchProfile struct {
	SchemaVersion          int                    `json:"schema_version"`
	ID                     ProfileID              `json:"id"`
	Bundles                []ProfileBundle        `json:"bundles"`
	RequiredRoutes         []RouteMethod          `json:"required_routes,omitempty"`
	UnavailableRoutePolicy RouteUnavailablePolicy `json:"unavailable_route_policy"`
	ShutdownTimeoutMillis  int64                  `json:"shutdown_timeout_millis"`
	DiagnosticMode         string                 `json:"diagnostic_mode,omitempty"`
	NativeUIRequired       bool                   `json:"native_ui_required"`
	ProtectedCapabilities  []CapabilityRef        `json:"protected_capabilities"`
	FeatureFlags           []string               `json:"feature_flags,omitempty"`
}

type Bundle interface {
	Descriptor() BundleDescriptor
	Register(context.Context, *Registration) (Effect, error)
	Start(context.Context, *BundleContext) (Effect, error)
	Ready(context.Context, *BundleContext) error
	Stop(context.Context, *BundleContext) error
}

type Snapshot struct {
	SchemaVersion int                `json:"schema_version"`
	Profile       LaunchProfile      `json:"profile"`
	Bundles       []BundleDescriptor `json:"bundles"`
	Routes        []RouteDescriptor  `json:"routes"`
	Digest        string             `json:"digest"`
	Canonical     []byte             `json:"-"`
}

func (snapshot Snapshot) Clone() Snapshot {
	result := snapshot
	result.Profile = cloneProfile(snapshot.Profile)
	result.Bundles = cloneDescriptors(snapshot.Bundles)
	result.Routes = cloneRoutes(snapshot.Routes)
	result.Canonical = append([]byte(nil), snapshot.Canonical...)
	return result
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:/-", character) {
			continue
		}
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"api_key", "authorization", "bearer", "password", "private-prompt",
		"provider-response", "secret-value",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func validVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 9 {
			return false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return true
}

func sortCapabilityRefs(values []CapabilityRef) {
	sort.Slice(values, func(left, right int) bool {
		if values[left].ID != values[right].ID {
			return values[left].ID < values[right].ID
		}
		return values[left].Version < values[right].Version
	})
}

func cloneProfile(value LaunchProfile) LaunchProfile {
	result := value
	result.Bundles = append([]ProfileBundle(nil), value.Bundles...)
	result.RequiredRoutes = append([]RouteMethod(nil), value.RequiredRoutes...)
	result.ProtectedCapabilities = append([]CapabilityRef(nil), value.ProtectedCapabilities...)
	result.FeatureFlags = append([]string(nil), value.FeatureFlags...)
	return result
}

func cloneDescriptors(values []BundleDescriptor) []BundleDescriptor {
	result := make([]BundleDescriptor, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Requires = append([]CapabilityRef(nil), value.Requires...)
		result[index].OptionalRequires = append([]CapabilityRef(nil), value.OptionalRequires...)
		result[index].Provides = append([]CapabilityRef(nil), value.Provides...)
		result[index].Routes = cloneRoutes(value.Routes)
		result[index].Compatibility = append([]BundleConstraint(nil), value.Compatibility...)
	}
	return result
}

func cloneRoutes(values []RouteDescriptor) []RouteDescriptor {
	result := make([]RouteDescriptor, len(values))
	for index, value := range values {
		result[index] = value
		result[index].RequiredCapabilities = append([]CapabilityRef(nil), value.RequiredCapabilities...)
	}
	return result
}
