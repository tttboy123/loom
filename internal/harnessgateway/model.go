package harnessgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ConfiguredHarnessSchemaVersion     = 1
	SegmentSessionBindingSchemaVersion = 1
	ResponseAuthoritySchemaVersion     = 2
	maxGatewayInputBytes               = 8 << 20
)

var (
	ErrInvalidRegistry   = errors.New("invalid Harness backend registry")
	ErrInvalidGateway    = errors.New("invalid Harness Gateway request")
	ErrAuthorityConflict = errors.New("Harness Gateway authority conflict")
	ErrResponseNotFound  = errors.New("Harness Gateway response not found")
	ErrSessionUnhealthy  = errors.New("Harness Gateway Session unhealthy")
)

type HarnessID string
type BackendID string
type Capability string

const (
	HarnessCodex      HarnessID = "codex"
	HarnessClaudeCode HarnessID = "claude-code"
	HarnessOpenCode   HarnessID = "opencode"
	HarnessPi         HarnessID = "pi"
	HarnessLoomNative HarnessID = "loom-native"

	CapabilitySegmentSession Capability = "segment_session"
	CapabilityResponseCancel Capability = "response_cancel"
)

func BuiltInHarnessIDs() []HarnessID {
	return []HarnessID{
		HarnessCodex, HarnessClaudeCode, HarnessOpenCode, HarnessPi, HarnessLoomNative,
	}
}

type ConfiguredHarness struct {
	SchemaVersion         int
	HarnessID             HarnessID
	Version               int
	BackendID             BackendID
	BackendVersion        int
	ConfigurationDigest   string
	Capabilities          []Capability
	MaxConcurrentSessions int
	IdleTimeout           time.Duration
	MaxSessionAge         time.Duration
}

func (configured ConfiguredHarness) valid() bool {
	if configured.SchemaVersion != ConfiguredHarnessSchemaVersion ||
		!validIdentifier(string(configured.HarnessID)) || configured.Version < 1 ||
		!validIdentifier(string(configured.BackendID)) || configured.BackendVersion < 1 ||
		!validDigest(configured.ConfigurationDigest) ||
		configured.MaxConcurrentSessions < 1 || configured.MaxConcurrentSessions > 1_024 ||
		configured.IdleTimeout <= 0 || configured.MaxSessionAge < configured.IdleTimeout ||
		len(configured.Capabilities) == 0 || len(configured.Capabilities) > 32 {
		return false
	}
	seen := make(map[Capability]struct{}, len(configured.Capabilities))
	for _, capability := range configured.Capabilities {
		if capability != CapabilitySegmentSession && capability != CapabilityResponseCancel {
			return false
		}
		if _, duplicate := seen[capability]; duplicate {
			return false
		}
		seen[capability] = struct{}{}
	}
	return true
}

type SegmentSessionBinding struct {
	SchemaVersion               int
	ConfiguredHarnessID         HarnessID
	ConfiguredHarnessVersion    int
	BackendID                   BackendID
	BackendVersion              int
	ConversationID              string
	SegmentID                   string
	WorkspaceID                 string
	WorkspaceDigest             string
	ExecutionBindingDigest      string
	ProviderID                  string
	ProviderAccountID           string
	CredentialRevision          int64
	ModelID                     string
	ReasoningEffort             string
	SegmentContextCapsuleDigest string
	GovernancePolicyDigest      string
	RouteTransitionReviewDigest string
}

func (binding SegmentSessionBinding) valid() bool {
	return binding.SchemaVersion == SegmentSessionBindingSchemaVersion &&
		validIdentifier(string(binding.ConfiguredHarnessID)) && binding.ConfiguredHarnessVersion > 0 &&
		validIdentifier(string(binding.BackendID)) && binding.BackendVersion > 0 &&
		validIdentifier(binding.ConversationID) && validIdentifier(binding.SegmentID) &&
		validIdentifier(binding.WorkspaceID) && validDigest(binding.WorkspaceDigest) &&
		validDigest(binding.ExecutionBindingDigest) && validIdentifier(binding.ProviderID) &&
		validProviderAuthority(binding.ProviderAccountID, binding.CredentialRevision) &&
		validIdentifier(binding.ModelID) &&
		(binding.ReasoningEffort == "" || validIdentifier(binding.ReasoningEffort)) &&
		validDigest(binding.SegmentContextCapsuleDigest) &&
		optionalDigest(binding.GovernancePolicyDigest) &&
		optionalDigest(binding.RouteTransitionReviewDigest) &&
		(binding.ProviderAccountID != "" || binding.GovernancePolicyDigest == "")
}

func validProviderAuthority(providerAccountID string, credentialRevision int64) bool {
	return providerAccountID == "" && credentialRevision == 0 ||
		validIdentifier(providerAccountID) && credentialRevision > 0
}

func optionalDigest(value string) bool { return value == "" || validDigest(value) }

func (binding SegmentSessionBinding) SessionID() string {
	if !binding.valid() {
		return ""
	}
	body, err := json.Marshal(struct {
		SchemaVersion            int       `json:"schema_version"`
		ConfiguredHarnessID      HarnessID `json:"configured_harness_id"`
		ConfiguredHarnessVersion int       `json:"configured_harness_version"`
		BackendID                BackendID `json:"backend_id"`
		BackendVersion           int       `json:"backend_version"`
		ConversationID           string    `json:"conversation_id"`
		SegmentID                string    `json:"segment_id"`
		WorkspaceID              string    `json:"workspace_id"`
		WorkspaceDigest          string    `json:"workspace_digest"`
		ExecutionBindingDigest   string    `json:"execution_binding_digest"`
		ProviderID               string    `json:"provider_id"`
		ProviderAccountID        string    `json:"provider_account_id"`
		CredentialRevision       int64     `json:"credential_revision"`
		ModelID                  string    `json:"model_id"`
		ReasoningEffort          string    `json:"reasoning_effort,omitempty"`
		ContextCapsuleDigest     string    `json:"context_capsule_digest"`
		GovernancePolicyDigest   string    `json:"governance_policy_digest"`
		RouteTransitionDigest    string    `json:"route_transition_review_digest,omitempty"`
	}{
		binding.SchemaVersion, binding.ConfiguredHarnessID, binding.ConfiguredHarnessVersion,
		binding.BackendID, binding.BackendVersion, binding.ConversationID, binding.SegmentID,
		binding.WorkspaceID, binding.WorkspaceDigest, binding.ExecutionBindingDigest,
		binding.ProviderID, binding.ProviderAccountID, binding.CredentialRevision,
		binding.ModelID, binding.ReasoningEffort, binding.SegmentContextCapsuleDigest,
		binding.GovernancePolicyDigest, binding.RouteTransitionReviewDigest,
	})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(body)
	return "session-" + hex.EncodeToString(digest[:])
}

type Workspace struct {
	ID     string
	Digest string
	Path   string
}

func (workspace Workspace) validFor(binding SegmentSessionBinding) bool {
	return validIdentifier(workspace.ID) && validDigest(workspace.Digest) &&
		workspace.ID == binding.WorkspaceID && workspace.Digest == binding.WorkspaceDigest &&
		filepath.IsAbs(workspace.Path) && filepath.Clean(workspace.Path) == workspace.Path &&
		len(workspace.Path) <= 4_096
}

type ResponseAuthority struct {
	SchemaVersion               int
	ResponseID                  string
	IncidentID                  string
	ExecutionBindingDigest      string
	ContextCapsuleDigest        string
	SegmentContextCapsuleDigest string
	GovernancePolicyDigest      string
	RouteTransitionReviewDigest string
	ProviderID                  string
	ProviderAccountID           string
	CredentialRevision          int64
	ModelID                     string
	ReasoningEffort             string
}

func (authority ResponseAuthority) validFor(binding SegmentSessionBinding) bool {
	return authority.SchemaVersion == ResponseAuthoritySchemaVersion &&
		validIdentifier(authority.ResponseID) && validIdentifier(authority.IncidentID) &&
		validDigest(authority.ContextCapsuleDigest) &&
		authority.SegmentContextCapsuleDigest == binding.SegmentContextCapsuleDigest &&
		authority.ExecutionBindingDigest == binding.ExecutionBindingDigest &&
		authority.GovernancePolicyDigest == binding.GovernancePolicyDigest &&
		authority.RouteTransitionReviewDigest == binding.RouteTransitionReviewDigest &&
		authority.ProviderID == binding.ProviderID &&
		authority.ProviderAccountID == binding.ProviderAccountID &&
		authority.CredentialRevision == binding.CredentialRevision &&
		authority.ModelID == binding.ModelID &&
		authority.ReasoningEffort == binding.ReasoningEffort
}

type ResponseRequest struct {
	Authority ResponseAuthority
	Input     []byte
}

type Response struct{ Content []byte }

type Backend interface {
	ID() BackendID
	Version() int
	OpenSession(context.Context, ConfiguredHarness, SegmentSessionBinding, Workspace) (BackendSession, error)
}

type BackendSession interface {
	Respond(context.Context, ResponseRequest) (Response, error)
	Close(context.Context) error
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if current > 127 || !(current >= 'a' && current <= 'z' ||
			current >= 'A' && current <= 'Z' || current >= '0' && current <= '9' ||
			strings.ContainsRune("._:/-", current)) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func normalizedCapabilities(values []Capability) []Capability {
	result := append([]Capability(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
