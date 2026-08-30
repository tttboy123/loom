package controltool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidRegistry    = errors.New("invalid Loom control tool registry")
	ErrInvalidCall        = errors.New("invalid Loom control tool call")
	ErrGatewayUnavailable = errors.New("Loom control tool gateway unavailable")
	ErrGatewayConflict    = errors.New("Loom control tool gateway already bound")
)

type ToolID string

const (
	ToolSessionsSearch                     ToolID = "loom.sessions.search"
	ToolSessionsAlignPreview               ToolID = "loom.sessions.align.preview"
	ToolMissionsCreatePreview              ToolID = "loom.missions.create.preview"
	ToolMissionsContinuePreview            ToolID = "loom.missions.continue.preview"
	ToolTeamsCreatePreview                 ToolID = "loom.teams.create.preview"
	ToolRoundtablesOpenPreview             ToolID = "loom.roundtables.open.preview"
	ToolMissionsSearch                     ToolID = "loom.missions.search"
	ToolMissionsStatus                     ToolID = "loom.missions.status"
	ToolTeamsSearch                        ToolID = "loom.teams.search"
	ToolTeamsStatus                        ToolID = "loom.teams.status"
	ToolRoundtablesStatus                  ToolID = "loom.roundtables.status"
	ToolGovernanceNeedsYou                 ToolID = "loom.governance.needs_you"
	ToolRuntimesStatus                     ToolID = "loom.runtimes.status"
	ToolProvidersStatus                    ToolID = "loom.providers.status"
	ToolDiagnosticsIncident                ToolID = "loom.diagnostics.incident"
	ToolWorkspaceStatus                    ToolID = "loom.workspace.status"
	ToolConversationRouteStatus            ToolID = "loom.conversation.route.status"
	ToolLibrarySearch                      ToolID = "loom.library.search"
	ToolConversationRouteChangePreview     ToolID = "loom.conversation.route.change.preview"
	ToolConversationModelChangePreview     ToolID = "loom.conversation.model.change.preview"
	ToolConversationReasoningChangePreview ToolID = "loom.conversation.reasoning.change.preview"
	ToolWorkspaceChoosePreview             ToolID = "loom.workspace.choose.preview"
	ToolTeamsEditPreview                   ToolID = "loom.teams.edit.preview"
	ToolRoundtablesPausePreview            ToolID = "loom.roundtables.pause.preview"
	ToolRoundtablesSteerPreview            ToolID = "loom.roundtables.steer.preview"
	ToolRoundtablesRetryPreview            ToolID = "loom.roundtables.retry.preview"
	ToolRoundtablesSkipPreview             ToolID = "loom.roundtables.skip.preview"
	ToolRoundtablesReplacePreview          ToolID = "loom.roundtables.replace.preview"
)

type Effect string

const (
	EffectRead     Effect = "read"
	EffectProposal Effect = "proposal"
)

type ConfirmationPolicy string

const (
	ConfirmationNever ConfirmationPolicy = "never"
	ConfirmationUser  ConfirmationPolicy = "user"
)

type Definition struct {
	ID           ToolID
	MCPName      string
	Version      int
	Description  string
	Effect       Effect
	Confirmation ConfirmationPolicy
	InputSchema  json.RawMessage
}

type Registry struct {
	definitions []Definition
	byID        map[ToolID]Definition
	byMCPName   map[string]Definition
	digest      string
}

func NewBuiltinRegistry() (*Registry, error) {
	return NewRegistry([]Definition{
		{
			ID: ToolSessionsSearch, MCPName: "loom_sessions_search", Version: 1,
			Description: "Find Loom conversations by title or exact conversation ID. Returns metadata only, never transcript content.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","description":"Conversation title text or exact Conversation ID to match.","maxLength":256},"limit":{"type":"integer","description":"Maximum number of Conversations to return, from 1 to 20.","minimum":1,"maximum":20}},"required":["query"]}`),
		},
		{
			ID: ToolSessionsAlignPreview, MCPName: "loom_sessions_align_preview", Version: 2,
			Description: "Prepare a digest-bound proposal to align selected Loom conversations into the current conversation. This never applies context; the user must confirm in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_ids":{"type":"array","description":"Exact Conversation IDs to align into the current Conversation.","minItems":1,"maxItems":8,"uniqueItems":true,"items":{"type":"string"}},"context_mode":{"type":"string","description":"Alignment mode: summary only or continue with prior context.","enum":["summary_only","continue_with_context"]}},"required":["session_ids","context_mode"]}`),
		},
		{
			ID: ToolMissionsCreatePreview, MCPName: "loom_missions_create_preview", Version: 2,
			Description: "Prepare a governed Loom Mission draft from an objective. This only proposes opening Mission review; no Mission or Run is created until the user confirms in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string","description":"Objective that the proposed Mission should accomplish.","minLength":1,"maxLength":4096}},"required":["objective"]}`),
		},
		{
			ID: ToolMissionsContinuePreview, MCPName: "loom_missions_continue_preview", Version: 2,
			Description: "Prepare guidance for one exact blocked Mission. This never resumes or retries work until the user confirms in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"mission_id":{"type":"string","description":"Exact blocked Mission ID to continue.","minLength":1,"maxLength":128},"guidance":{"type":"string","description":"Guidance for the next governed Mission attempt.","minLength":1,"maxLength":4096}},"required":["mission_id","guidance"]}`),
		},
		{
			ID: ToolTeamsCreatePreview, MCPName: "loom_teams_create_preview", Version: 2,
			Description: "Prepare a governed Agent Team draft from a purpose. This only opens Team review; no Team instance or Run is created until the user confirms in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"purpose":{"type":"string","description":"Purpose and responsibilities for the proposed Agent Team.","minLength":1,"maxLength":4096}},"required":["purpose"]}`),
		},
		{
			ID: ToolRoundtablesOpenPreview, MCPName: "loom_roundtables_open_preview", Version: 2,
			Description: "Prepare opening the RoundTable for one exact Mission beside this conversation. This never creates seats or starts deliberation until the user confirms in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"mission_id":{"type":"string","description":"Exact Mission ID to open a RoundTable for.","minLength":1,"maxLength":128}},"required":["mission_id"]}`),
		},
		{
			ID: ToolMissionsSearch, MCPName: "loom_missions_search", Version: 1,
			Description: "Find Loom Missions by title or exact Mission ID. Returns bounded workflow metadata only, never prompts or Agent output.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","description":"Mission title text or exact Mission ID to match.","maxLength":256},"limit":{"type":"integer","description":"Maximum number of Missions to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolMissionsStatus, MCPName: "loom_missions_status", Version: 1,
			Description: "Inspect one Loom Mission's frozen status, progress counts, current node and non-secret block reason.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"mission_id":{"type":"string","description":"Exact Mission ID to inspect.","minLength":1,"maxLength":128}},"required":["mission_id"]}`),
		},
		{
			ID: ToolTeamsSearch, MCPName: "loom_teams_search", Version: 1,
			Description: "Find governed Loom Agent Teams by name or exact Team instance ID. Returns metadata only.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","description":"Agent Team name text or exact Team instance ID to match.","maxLength":256},"limit":{"type":"integer","description":"Maximum number of Agent Teams to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolTeamsStatus, MCPName: "loom_teams_status", Version: 1,
			Description: "Inspect one Agent Team's state and each role's frozen Harness, Provider Account, credential revision and model metadata.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"team_instance_id":{"type":"string","description":"Exact Agent Team instance ID to inspect.","minLength":1,"maxLength":128}},"required":["team_instance_id"]}`),
		},
		{
			ID: ToolRoundtablesStatus, MCPName: "loom_roundtables_status", Version: 1,
			Description: "Inspect bounded metadata for one Mission-linked RoundTable session. Returns no deliberation transcript or private Agent state.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID to inspect.","minLength":1,"maxLength":128}},"required":["session_id"]}`),
		},
		{
			ID: ToolGovernanceNeedsYou, MCPName: "loom_governance_needs_you", Version: 1,
			Description: "List bounded non-secret decisions and blocked work currently awaiting user intervention.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"limit":{"type":"integer","description":"Maximum number of awaiting decisions or blocked items to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolRuntimesStatus, MCPName: "loom_runtimes_status", Version: 1,
			Description: "List Loom Runtime health, adapters, capacity and model identifiers. Returns no local environment or credential data.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"adapter_type":{"type":"string","description":"Runtime adapter type to filter by; omit to include all types.","maxLength":64},"limit":{"type":"integer","description":"Maximum number of Runtimes to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolProvidersStatus, MCPName: "loom_providers_status", Version: 1,
			Description: "List non-secret Provider and Provider Account status, policy posture and available conversation profiles. Never returns credential references or keys.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"provider_id":{"type":"string","description":"Exact Provider ID to filter by; omit to include all Providers.","maxLength":64},"provider_account_id":{"type":"string","description":"Exact Provider Account ID to filter by; omit to include all accounts.","maxLength":128},"limit":{"type":"integer","description":"Maximum number of Provider records to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolDiagnosticsIncident, MCPName: "loom_diagnostics_incident", Version: 1,
			Description: "Read privacy-safe stages and recovery metadata for one exact Loom Incident ID. Returns no prompt, response body, path, header or secret.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"incident_id":{"type":"string","description":"Exact Loom Incident ID to inspect.","minLength":1,"maxLength":128}},"required":["incident_id"]}`),
		},
		{
			ID: ToolWorkspaceStatus, MCPName: "loom_workspace_status", Version: 1,
			Description: "Inspect the current Conversation workspace identity and digest without exposing an absolute local path.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
		},
		{
			ID: ToolConversationRouteStatus, MCPName: "loom_conversation_route_status", Version: 1,
			Description: "Inspect the current frozen Conversation Harness, Provider Account, credential revision, model and reasoning metadata.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
		},
		{
			ID: ToolLibrarySearch, MCPName: "loom_library_search", Version: 1,
			Description: "Find accepted Loom Evidence and reusable result metadata by identifier. Returns digests and references, never artifact contents.",
			Effect:      EffectRead, Confirmation: ConfirmationNever,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","description":"Identifier text used to find accepted Evidence or reusable results.","maxLength":256},"limit":{"type":"integer","description":"Maximum number of Library records to return, from 1 to 20.","minimum":1,"maximum":20}}}`),
		},
		{
			ID: ToolConversationRouteChangePreview, MCPName: "loom_conversation_route_change_preview", Version: 2,
			Description: "Prepare a user-reviewed change to an exact configured Conversation Route. This never changes the active Segment and still requires any trust-domain review in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"profile_id":{"type":"string","description":"Exact Conversation Profile ID for the proposed Route.","minLength":1,"maxLength":256}},"required":["profile_id"]}`),
		},
		{
			ID: ToolConversationModelChangePreview, MCPName: "loom_conversation_model_change_preview", Version: 2,
			Description: "Prepare a user-reviewed model selection on the current configured Conversation Route. The new model applies only to a new immutable Segment.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"model_id":{"type":"string","description":"Exact model ID proposed for the current Conversation Route.","minLength":1,"maxLength":256}},"required":["model_id"]}`),
		},
		{
			ID: ToolConversationReasoningChangePreview, MCPName: "loom_conversation_reasoning_change_preview", Version: 2,
			Description: "Prepare a user-reviewed reasoning-effort selection for new turns. Use provider-default only when the current Route advertises it.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"reasoning_effort":{"type":"string","description":"Reasoning-effort value advertised by the current Conversation Route.","minLength":1,"maxLength":64}},"required":["reasoning_effort"]}`),
		},
		{
			ID: ToolWorkspaceChoosePreview, MCPName: "loom_workspace_choose_preview", Version: 2,
			Description: "Prepare opening Loom's user-owned workspace folder picker. The model never sees or chooses an absolute local path.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
		},
		{
			ID: ToolTeamsEditPreview, MCPName: "loom_teams_edit_preview", Version: 2,
			Description: "Prepare opening one exact governed Agent Team in review with a bounded edit instruction. This never changes a Team definition or instance by itself.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"team_instance_id":{"type":"string","description":"Exact Agent Team instance ID to open for review.","minLength":1,"maxLength":128},"instruction":{"type":"string","description":"Requested edit for the user-reviewed Agent Team draft.","minLength":1,"maxLength":4096}},"required":["team_instance_id","instruction"]}`),
		},
		{
			ID: ToolRoundtablesPausePreview, MCPName: "loom_roundtables_pause_preview", Version: 2,
			Description: "Prepare pausing one exact active RoundTable round. Loom validates current authority again after the user confirms.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID containing the active round.","minLength":1,"maxLength":128},"round_id":{"type":"string","description":"Exact active RoundTable round ID to pause.","minLength":1,"maxLength":128}},"required":["session_id","round_id"]}`),
		},
		{
			ID: ToolRoundtablesSteerPreview, MCPName: "loom_roundtables_steer_preview", Version: 2,
			Description: "Prepare bounded guidance for one exact running RoundTable Agent Attempt. The user must review and confirm the guidance in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID containing the running attempt.","minLength":1,"maxLength":128},"round_id":{"type":"string","description":"Exact RoundTable round ID containing the running attempt.","minLength":1,"maxLength":128},"seat_id":{"type":"string","description":"Exact Agent seat ID for the running attempt.","minLength":1,"maxLength":128},"attempt_id":{"type":"string","description":"Exact running Agent Attempt ID to steer.","minLength":1,"maxLength":128},"guidance":{"type":"string","description":"Guidance for the selected running Agent Attempt.","minLength":1,"maxLength":4096}},"required":["session_id","round_id","seat_id","attempt_id","guidance"]}`),
		},
		{
			ID: ToolRoundtablesRetryPreview, MCPName: "loom_roundtables_retry_preview", Version: 2,
			Description: "Prepare a governed retry with bounded guidance for one exact failed or cancelled RoundTable Agent Attempt.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID containing the failed attempt.","minLength":1,"maxLength":128},"round_id":{"type":"string","description":"Exact RoundTable round ID containing the failed attempt.","minLength":1,"maxLength":128},"seat_id":{"type":"string","description":"Exact Agent seat ID for the failed attempt.","minLength":1,"maxLength":128},"attempt_id":{"type":"string","description":"Exact failed or cancelled Agent Attempt ID to retry.","minLength":1,"maxLength":128},"guidance":{"type":"string","description":"Guidance for the governed retry attempt.","minLength":1,"maxLength":4096}},"required":["session_id","round_id","seat_id","attempt_id","guidance"]}`),
		},
		{
			ID: ToolRoundtablesSkipPreview, MCPName: "loom_roundtables_skip_preview", Version: 2,
			Description: "Prepare skipping one exact RoundTable seat in the active round. This requires explicit user confirmation.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID containing the seat.","minLength":1,"maxLength":128},"round_id":{"type":"string","description":"Exact active RoundTable round ID containing the seat.","minLength":1,"maxLength":128},"seat_id":{"type":"string","description":"Exact Agent seat ID to skip.","minLength":1,"maxLength":128}},"required":["session_id","round_id","seat_id"]}`),
		},
		{
			ID: ToolRoundtablesReplacePreview, MCPName: "loom_roundtables_replace_preview", Version: 2,
			Description: "Prepare opening replacement review for one exact RoundTable seat. The user chooses and confirms the replacement Agent and Route in Loom.",
			Effect:      EffectProposal, Confirmation: ConfirmationUser,
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"session_id":{"type":"string","description":"Exact RoundTable Session ID containing the seat.","minLength":1,"maxLength":128},"round_id":{"type":"string","description":"Exact RoundTable round ID containing the seat.","minLength":1,"maxLength":128},"seat_id":{"type":"string","description":"Exact Agent seat ID to open for replacement review.","minLength":1,"maxLength":128}},"required":["session_id","round_id","seat_id"]}`),
		},
	})
}

func IsProductMetadataReadTool(toolID ToolID) bool {
	switch toolID {
	case ToolMissionsSearch, ToolMissionsStatus,
		ToolTeamsSearch, ToolTeamsStatus, ToolRoundtablesStatus,
		ToolGovernanceNeedsYou, ToolRuntimesStatus, ToolProvidersStatus,
		ToolDiagnosticsIncident, ToolWorkspaceStatus,
		ToolConversationRouteStatus, ToolLibrarySearch:
		return true
	default:
		return false
	}
}

func NewRegistry(definitions []Definition) (*Registry, error) {
	if len(definitions) == 0 || len(definitions) > 64 {
		return nil, ErrInvalidRegistry
	}
	registry := &Registry{
		definitions: make([]Definition, 0, len(definitions)),
		byID:        make(map[ToolID]Definition, len(definitions)),
		byMCPName:   make(map[string]Definition, len(definitions)),
	}
	for _, definition := range definitions {
		definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
		if !validDefinition(definition) {
			return nil, ErrInvalidRegistry
		}
		if _, duplicate := registry.byID[definition.ID]; duplicate {
			return nil, ErrInvalidRegistry
		}
		if _, duplicate := registry.byMCPName[definition.MCPName]; duplicate {
			return nil, ErrInvalidRegistry
		}
		registry.definitions = append(registry.definitions, definition)
		registry.byID[definition.ID] = definition
		registry.byMCPName[definition.MCPName] = definition
	}
	canonical := append([]Definition(nil), registry.definitions...)
	sort.Slice(canonical, func(left, right int) bool { return canonical[left].ID < canonical[right].ID })
	body, err := json.Marshal(canonicalDefinitions(canonical))
	if err != nil {
		return nil, ErrInvalidRegistry
	}
	registry.digest = DigestBytes(body)
	return registry, nil
}

func (registry *Registry) Definitions() []Definition {
	if registry == nil {
		return nil
	}
	result := make([]Definition, len(registry.definitions))
	copy(result, registry.definitions)
	for index := range result {
		result[index].InputSchema = append(json.RawMessage(nil), result[index].InputSchema...)
	}
	return result
}

func (registry *Registry) DefinitionByMCPName(name string) (Definition, bool) {
	if registry == nil {
		return Definition{}, false
	}
	definition, found := registry.byMCPName[name]
	definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	return definition, found
}

func (registry *Registry) Definition(id ToolID) (Definition, bool) {
	if registry == nil {
		return Definition{}, false
	}
	definition, found := registry.byID[id]
	definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	return definition, found
}

func (registry *Registry) Digest() string {
	if registry == nil {
		return ""
	}
	return registry.digest
}

type canonicalDefinition struct {
	ID           ToolID             `json:"id"`
	MCPName      string             `json:"mcp_name"`
	Version      int                `json:"version"`
	Description  string             `json:"description"`
	Effect       Effect             `json:"effect"`
	Confirmation ConfirmationPolicy `json:"confirmation"`
	InputSchema  json.RawMessage    `json:"input_schema"`
}

func canonicalDefinitions(definitions []Definition) []canonicalDefinition {
	result := make([]canonicalDefinition, len(definitions))
	for index, definition := range definitions {
		result[index] = canonicalDefinition{
			ID: definition.ID, MCPName: definition.MCPName, Version: definition.Version,
			Description: definition.Description, Effect: definition.Effect,
			Confirmation: definition.Confirmation, InputSchema: definition.InputSchema,
		}
	}
	return result
}

func validDefinition(definition Definition) bool {
	if !validIdentifier(string(definition.ID), 128, true) ||
		!validIdentifier(definition.MCPName, 128, false) || definition.Version <= 0 ||
		definition.Version > 1_000 || !validText(definition.Description, 1_024) ||
		definition.Effect != EffectRead && definition.Effect != EffectProposal ||
		definition.Confirmation != ConfirmationNever &&
			definition.Confirmation != ConfirmationUser ||
		definition.Effect == EffectRead && definition.Confirmation != ConfirmationNever ||
		definition.Effect == EffectProposal && definition.Confirmation != ConfirmationUser {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(definition.InputSchema))
	decoder.UseNumber()
	var schema map[string]any
	if decoder.Decode(&schema) != nil || decoder.Decode(&struct{}{}) == nil ||
		schema["type"] != "object" || schema["additionalProperties"] != false {
		return false
	}
	return true
}

func validIdentifier(value string, maximum int, dotted bool) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' ||
			char == '_' || char == '-' || dotted && char == '.' {
			continue
		}
		return false
	}
	return true
}

func validText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func DigestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%x", digest)
}

type ContextMode string

const (
	ContextModeSummaryOnly         ContextMode = "summary_only"
	ContextModeContinueWithContext ContextMode = "continue_with_context"
)

type ProposalStatus string

const (
	ProposalPending   ProposalStatus = "pending"
	ProposalConfirmed ProposalStatus = "confirmed"
	ProposalCancelled ProposalStatus = "cancelled"
	ProposalExpired   ProposalStatus = "expired"
)

type SessionReference struct {
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type SessionSource struct {
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	ContentDigest  string `json:"content_digest"`
	MessageCount   int    `json:"message_count"`
}

type SessionAlignmentProposal struct {
	SchemaVersion        int                       `json:"schema_version"`
	ProposalID           string                    `json:"proposal_id"`
	ToolID               ToolID                    `json:"tool_id"`
	ToolVersion          int                       `json:"tool_version"`
	Confirmation         ConfirmationPolicy        `json:"confirmation"`
	TargetConversationID string                    `json:"target_conversation_id"`
	TargetContentDigest  string                    `json:"target_content_digest"`
	Sources              []SessionSource           `json:"sources"`
	ContextMode          ContextMode               `json:"context_mode"`
	CatalogDigest        string                    `json:"catalog_digest"`
	Route                *FrozenRouteReference     `json:"route,omitempty"`
	Workspace            *FrozenWorkspaceReference `json:"workspace,omitempty"`
	RegistryDigest       string                    `json:"registry_digest,omitempty"`
	IncidentID           string                    `json:"incident_id,omitempty"`
	SegmentID            string                    `json:"segment_id"`
	AttemptID            string                    `json:"attempt_id"`
	MessageID            string                    `json:"message_id,omitempty"`
	Status               ProposalStatus            `json:"status"`
	CreatedAt            time.Time                 `json:"created_at"`
	ExpiresAt            time.Time                 `json:"expires_at"`
	ProposalDigest       string                    `json:"proposal_digest"`
}

type SessionAlignmentProposalInput struct {
	ProposalID           string
	TargetConversationID string
	TargetContentDigest  string
	Sources              []SessionSource
	ContextMode          ContextMode
	CatalogDigest        string
	Route                *FrozenRouteReference
	Workspace            *FrozenWorkspaceReference
	RegistryDigest       string
	IncidentID           string
	SegmentID            string
	AttemptID            string
	CreatedAt            time.Time
	ExpiresAt            time.Time
}

type ConversationAction string

const (
	ConversationActionMission           ConversationAction = "mission"
	ConversationActionContinueMission   ConversationAction = "continue_mission"
	ConversationActionTeam              ConversationAction = "team"
	ConversationActionRoundTable        ConversationAction = "roundtable"
	ConversationActionRoute             ConversationAction = "route"
	ConversationActionModel             ConversationAction = "model"
	ConversationActionReasoning         ConversationAction = "reasoning"
	ConversationActionWorkspace         ConversationAction = "workspace"
	ConversationActionTeamEdit          ConversationAction = "team_edit"
	ConversationActionRoundTablePause   ConversationAction = "roundtable_pause"
	ConversationActionRoundTableSteer   ConversationAction = "roundtable_steer"
	ConversationActionRoundTableRetry   ConversationAction = "roundtable_retry"
	ConversationActionRoundTableSkip    ConversationAction = "roundtable_skip"
	ConversationActionRoundTableReplace ConversationAction = "roundtable_replace"
)

// ConversationActionPayload is a closed, non-secret target for proposal-only
// governance actions. No field grants execution authority.
type ConversationActionPayload struct {
	MissionID          string `json:"mission_id,omitempty"`
	ProfileID          string `json:"profile_id,omitempty"`
	ModelID            string `json:"model_id,omitempty"`
	ReasoningEffort    string `json:"reasoning_effort,omitempty"`
	TeamInstanceID     string `json:"team_instance_id,omitempty"`
	Instruction        string `json:"instruction,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	RoundID            string `json:"round_id,omitempty"`
	SeatID             string `json:"seat_id,omitempty"`
	AttemptID          string `json:"attempt_id,omitempty"`
	Guidance           string `json:"guidance,omitempty"`
	MembershipRevision int    `json:"membership_revision,omitempty"`
	SeatBindingDigest  string `json:"seat_binding_digest,omitempty"`
}

func (payload ConversationActionPayload) ValidFrozenRoundTableSeatBinding() bool {
	return payload.MembershipRevision > 0 && validDigest(payload.SeatBindingDigest)
}

type ConversationActionProposal struct {
	SchemaVersion        int                        `json:"schema_version"`
	ProposalID           string                     `json:"proposal_id"`
	ToolID               ToolID                     `json:"tool_id"`
	ToolVersion          int                        `json:"tool_version"`
	Confirmation         ConfirmationPolicy         `json:"confirmation"`
	Action               ConversationAction         `json:"action"`
	Argument             string                     `json:"argument,omitempty"`
	Payload              *ConversationActionPayload `json:"payload,omitempty"`
	Route                *FrozenRouteReference      `json:"route,omitempty"`
	Workspace            *FrozenWorkspaceReference  `json:"workspace,omitempty"`
	RegistryDigest       string                     `json:"registry_digest,omitempty"`
	IncidentID           string                     `json:"incident_id,omitempty"`
	TargetConversationID string                     `json:"target_conversation_id"`
	TargetContentDigest  string                     `json:"target_content_digest"`
	SegmentID            string                     `json:"segment_id"`
	AttemptID            string                     `json:"attempt_id"`
	MessageID            string                     `json:"message_id,omitempty"`
	Status               ProposalStatus             `json:"status"`
	CreatedAt            time.Time                  `json:"created_at"`
	ExpiresAt            time.Time                  `json:"expires_at"`
	ProposalDigest       string                     `json:"proposal_digest"`
}

type ConversationActionProposalInput struct {
	ProposalID           string
	ToolID               ToolID
	TargetConversationID string
	TargetContentDigest  string
	Argument             string
	Payload              *ConversationActionPayload
	Route                *FrozenRouteReference
	Workspace            *FrozenWorkspaceReference
	RegistryDigest       string
	IncidentID           string
	SegmentID            string
	AttemptID            string
	CreatedAt            time.Time
	ExpiresAt            time.Time
}

func NewConversationActionProposal(
	input ConversationActionProposalInput,
) (ConversationActionProposal, error) {
	action, found := conversationActionForTool(input.ToolID)
	if !found {
		return ConversationActionProposal{}, ErrInvalidCall
	}
	proposal := ConversationActionProposal{
		SchemaVersion: 2, ProposalID: input.ProposalID,
		ToolID: input.ToolID, ToolVersion: 2, Confirmation: ConfirmationUser,
		Action: action, Argument: input.Argument,
		Payload:              cloneConversationActionPayload(input.Payload),
		Route:                cloneFrozenRouteReference(input.Route),
		Workspace:            cloneFrozenWorkspaceReference(input.Workspace),
		RegistryDigest:       input.RegistryDigest,
		IncidentID:           input.IncidentID,
		TargetConversationID: input.TargetConversationID,
		TargetContentDigest:  input.TargetContentDigest,
		SegmentID:            input.SegmentID, AttemptID: input.AttemptID,
		Status: ProposalPending, CreatedAt: input.CreatedAt.UTC(),
		ExpiresAt: input.ExpiresAt.UTC(),
	}
	if !proposal.validPayload() {
		return ConversationActionProposal{}, ErrInvalidCall
	}
	proposal.ProposalDigest = proposal.payloadDigest()
	return proposal, nil
}

func (proposal ConversationActionProposal) Valid() bool {
	return proposal.validPayload() && validDigest(proposal.ProposalDigest) &&
		proposal.ProposalDigest == proposal.payloadDigest() && validProposalStatus(proposal.Status)
}

// Confirmable is stricter than Valid so persisted migration records remain
// readable and cancellable without regaining execution authority.
func (proposal ConversationActionProposal) Confirmable() bool {
	if !proposal.Valid() || proposal.SchemaVersion != 2 ||
		proposal.Status != ProposalPending {
		return false
	}
	switch proposal.Action {
	case ConversationActionRoundTableSteer,
		ConversationActionRoundTableRetry,
		ConversationActionRoundTableSkip,
		ConversationActionRoundTableReplace:
		return proposal.Payload != nil &&
			proposal.Payload.ValidFrozenRoundTableSeatBinding()
	default:
		return true
	}
}

func (proposal ConversationActionProposal) Clone() ConversationActionProposal {
	proposal.Payload = cloneConversationActionPayload(proposal.Payload)
	proposal.Route = cloneFrozenRouteReference(proposal.Route)
	proposal.Workspace = cloneFrozenWorkspaceReference(proposal.Workspace)
	return proposal
}

func CloneConversationActionProposals(
	proposals []ConversationActionProposal,
) []ConversationActionProposal {
	if proposals == nil {
		return nil
	}
	result := make([]ConversationActionProposal, len(proposals))
	for index, proposal := range proposals {
		result[index] = proposal.Clone()
	}
	return result
}

func (proposal ConversationActionProposal) validPayload() bool {
	action, found := conversationActionForTool(proposal.ToolID)
	if !found || proposal.Action != action || proposal.Confirmation != ConfirmationUser ||
		!validIdentifier(proposal.ProposalID, 128, true) ||
		!validIdentifier(proposal.TargetConversationID, 256, true) ||
		!validIdentifier(proposal.SegmentID, 128, true) ||
		!validIdentifier(proposal.AttemptID, 128, true) ||
		!validDigest(proposal.TargetContentDigest) ||
		proposal.CreatedAt.IsZero() || proposal.ExpiresAt.IsZero() ||
		!proposal.ExpiresAt.After(proposal.CreatedAt) ||
		proposal.ExpiresAt.Sub(proposal.CreatedAt) > 15*time.Minute ||
		!validConversationActionArgument(proposal.Action, proposal.Argument) {
		return false
	}
	switch proposal.SchemaVersion {
	case 1:
		if proposal.ToolVersion != 1 || proposal.Route != nil || proposal.Workspace != nil ||
			proposal.RegistryDigest != "" || proposal.IncidentID != "" ||
			!validConversationActionPayloadV1(proposal.Action, proposal.Payload) {
			return false
		}
	case 2:
		if proposal.ToolVersion != 2 || proposal.Route == nil || !proposal.Route.Valid() ||
			proposal.Workspace == nil || !proposal.Workspace.Valid() ||
			!validDigest(proposal.RegistryDigest) ||
			!validIdentifier(proposal.IncidentID, 128, true) ||
			!validConversationActionPayloadV2(proposal.Action, proposal.Payload) {
			return false
		}
	default:
		return false
	}
	return proposal.MessageID == "" || validIdentifier(proposal.MessageID, 128, true)
}

func (proposal ConversationActionProposal) payloadDigest() string {
	if proposal.SchemaVersion == 1 {
		return proposal.payloadDigestV1()
	}
	body, _ := json.Marshal(struct {
		SchemaVersion        int                        `json:"schema_version"`
		ProposalID           string                     `json:"proposal_id"`
		ToolID               ToolID                     `json:"tool_id"`
		ToolVersion          int                        `json:"tool_version"`
		Confirmation         ConfirmationPolicy         `json:"confirmation"`
		Action               ConversationAction         `json:"action"`
		Argument             string                     `json:"argument,omitempty"`
		Payload              *ConversationActionPayload `json:"payload,omitempty"`
		Route                *FrozenRouteReference      `json:"route"`
		Workspace            *FrozenWorkspaceReference  `json:"workspace"`
		RegistryDigest       string                     `json:"registry_digest"`
		IncidentID           string                     `json:"incident_id"`
		TargetConversationID string                     `json:"target_conversation_id"`
		TargetContentDigest  string                     `json:"target_content_digest"`
		SegmentID            string                     `json:"segment_id"`
		AttemptID            string                     `json:"attempt_id"`
		CreatedAt            time.Time                  `json:"created_at"`
		ExpiresAt            time.Time                  `json:"expires_at"`
	}{
		proposal.SchemaVersion, proposal.ProposalID, proposal.ToolID,
		proposal.ToolVersion, proposal.Confirmation, proposal.Action,
		proposal.Argument, proposal.Payload, proposal.Route, proposal.Workspace,
		proposal.RegistryDigest, proposal.IncidentID,
		proposal.TargetConversationID, proposal.TargetContentDigest,
		proposal.SegmentID, proposal.AttemptID, proposal.CreatedAt.UTC(),
		proposal.ExpiresAt.UTC(),
	})
	return DigestBytes(body)
}

func (proposal ConversationActionProposal) payloadDigestV1() string {
	body, _ := json.Marshal(struct {
		SchemaVersion        int                        `json:"schema_version"`
		ProposalID           string                     `json:"proposal_id"`
		ToolID               ToolID                     `json:"tool_id"`
		ToolVersion          int                        `json:"tool_version"`
		Confirmation         ConfirmationPolicy         `json:"confirmation"`
		Action               ConversationAction         `json:"action"`
		Argument             string                     `json:"argument,omitempty"`
		Payload              *ConversationActionPayload `json:"payload,omitempty"`
		TargetConversationID string                     `json:"target_conversation_id"`
		TargetContentDigest  string                     `json:"target_content_digest"`
		SegmentID            string                     `json:"segment_id"`
		AttemptID            string                     `json:"attempt_id"`
		CreatedAt            time.Time                  `json:"created_at"`
		ExpiresAt            time.Time                  `json:"expires_at"`
	}{
		proposal.SchemaVersion, proposal.ProposalID, proposal.ToolID,
		proposal.ToolVersion, proposal.Confirmation, proposal.Action,
		proposal.Argument, proposal.Payload, proposal.TargetConversationID,
		proposal.TargetContentDigest, proposal.SegmentID, proposal.AttemptID,
		proposal.CreatedAt.UTC(), proposal.ExpiresAt.UTC(),
	})
	return DigestBytes(body)
}

type ProposalDecision string

const (
	ProposalDecisionConfirm   ProposalDecision = "confirm"
	ProposalDecisionCancel    ProposalDecision = "cancel"
	ProposalDecisionExpire    ProposalDecision = "expire"
	ProposalDecisionSupersede ProposalDecision = "supersede"
)

// ProposalDecisionReceipt is the immutable, non-secret proof of one terminal
// decision over an exact schema-v2 Proposal. It records no execution authority;
// consumers must still revalidate the corresponding Proposal and product state.
type ProposalDecisionReceipt struct {
	SchemaVersion          int              `json:"schema_version"`
	ProposalID             string           `json:"proposal_id"`
	ProposalDigest         string           `json:"proposal_digest"`
	ToolID                 ToolID           `json:"tool_id"`
	Decision               ProposalDecision `json:"decision"`
	DecisionIncidentID     string           `json:"decision_incident_id"`
	TargetConversationID   string           `json:"target_conversation_id"`
	SegmentID              string           `json:"segment_id"`
	AttemptID              string           `json:"attempt_id"`
	RegistryDigest         string           `json:"registry_digest"`
	WorkspaceDigest        string           `json:"workspace_digest"`
	ExecutionBindingDigest string           `json:"execution_binding_digest"`
	ContextCapsuleDigest   string           `json:"context_capsule_digest"`
	DecidedAt              time.Time        `json:"decided_at"`
	ReceiptDigest          string           `json:"receipt_digest"`
}

func NewConversationActionDecisionReceipt(
	proposal ConversationActionProposal,
	decision ProposalDecision,
	decisionIncidentID string,
	decidedAt time.Time,
) (ProposalDecisionReceipt, error) {
	if !proposal.Valid() || proposal.SchemaVersion != 2 ||
		proposal.Status != ProposalPending || proposal.Route == nil ||
		proposal.Workspace == nil ||
		decision == ProposalDecisionConfirm && !proposal.Confirmable() ||
		!validProposalDecisionTime(decision, decidedAt, proposal.ExpiresAt) {
		return ProposalDecisionReceipt{}, ErrInvalidCall
	}
	receipt := newProposalDecisionReceipt(
		proposal.ProposalID,
		proposal.ProposalDigest,
		proposal.ToolID,
		decision,
		decisionIncidentID,
		proposal.TargetConversationID,
		proposal.SegmentID,
		proposal.AttemptID,
		proposal.RegistryDigest,
		proposal.Workspace.WorkspaceDigest,
		proposal.Route.ExecutionBindingDigest,
		proposal.Route.ContextCapsuleDigest,
		decidedAt,
	)
	if !receipt.Valid() {
		return ProposalDecisionReceipt{}, ErrInvalidCall
	}
	return receipt, nil
}

func NewSessionAlignmentDecisionReceipt(
	proposal SessionAlignmentProposal,
	decision ProposalDecision,
	decisionIncidentID string,
	decidedAt time.Time,
) (ProposalDecisionReceipt, error) {
	if !proposal.Valid() || proposal.SchemaVersion != 2 ||
		proposal.Status != ProposalPending || proposal.Route == nil ||
		proposal.Workspace == nil ||
		decision == ProposalDecisionConfirm && !proposal.Confirmable() ||
		!validProposalDecisionTime(decision, decidedAt, proposal.ExpiresAt) {
		return ProposalDecisionReceipt{}, ErrInvalidCall
	}
	receipt := newProposalDecisionReceipt(
		proposal.ProposalID,
		proposal.ProposalDigest,
		proposal.ToolID,
		decision,
		decisionIncidentID,
		proposal.TargetConversationID,
		proposal.SegmentID,
		proposal.AttemptID,
		proposal.RegistryDigest,
		proposal.Workspace.WorkspaceDigest,
		proposal.Route.ExecutionBindingDigest,
		proposal.Route.ContextCapsuleDigest,
		decidedAt,
	)
	if !receipt.Valid() {
		return ProposalDecisionReceipt{}, ErrInvalidCall
	}
	return receipt, nil
}

func newProposalDecisionReceipt(
	proposalID string,
	proposalDigest string,
	toolID ToolID,
	decision ProposalDecision,
	decisionIncidentID string,
	targetConversationID string,
	segmentID string,
	attemptID string,
	registryDigest string,
	workspaceDigest string,
	executionBindingDigest string,
	contextCapsuleDigest string,
	decidedAt time.Time,
) ProposalDecisionReceipt {
	receipt := ProposalDecisionReceipt{
		SchemaVersion: 1, ProposalID: proposalID, ProposalDigest: proposalDigest,
		ToolID: toolID, Decision: decision, DecisionIncidentID: decisionIncidentID,
		TargetConversationID: targetConversationID, SegmentID: segmentID,
		AttemptID: attemptID, RegistryDigest: registryDigest,
		WorkspaceDigest:        workspaceDigest,
		ExecutionBindingDigest: executionBindingDigest,
		ContextCapsuleDigest:   contextCapsuleDigest, DecidedAt: decidedAt.UTC(),
	}
	receipt.ReceiptDigest = receipt.payloadDigest()
	return receipt
}

func (receipt ProposalDecisionReceipt) Valid() bool {
	return receipt.SchemaVersion == 1 &&
		validIdentifier(receipt.ProposalID, 128, true) &&
		validDigest(receipt.ProposalDigest) &&
		validIdentifier(string(receipt.ToolID), 128, true) &&
		validProposalDecision(receipt.Decision) &&
		validDecisionIncidentID(receipt.DecisionIncidentID) &&
		validIdentifier(receipt.TargetConversationID, 256, true) &&
		validIdentifier(receipt.SegmentID, 128, true) &&
		validIdentifier(receipt.AttemptID, 128, true) &&
		validDigest(receipt.RegistryDigest) &&
		validDigest(receipt.WorkspaceDigest) &&
		validDigest(receipt.ExecutionBindingDigest) &&
		validDigest(receipt.ContextCapsuleDigest) &&
		!receipt.DecidedAt.IsZero() && validDigest(receipt.ReceiptDigest) &&
		receipt.ReceiptDigest == receipt.payloadDigest()
}

func (receipt ProposalDecisionReceipt) MatchesConversationActionProposal(
	proposal ConversationActionProposal,
) bool {
	return proposal.Valid() && proposal.SchemaVersion == 2 &&
		proposal.Route != nil && proposal.Workspace != nil &&
		receipt.matchesProposal(
			proposal.ProposalID,
			proposal.ProposalDigest,
			proposal.ToolID,
			proposal.TargetConversationID,
			proposal.SegmentID,
			proposal.AttemptID,
			proposal.RegistryDigest,
			proposal.Workspace.WorkspaceDigest,
			proposal.Route.ExecutionBindingDigest,
			proposal.Route.ContextCapsuleDigest,
			proposal.ExpiresAt,
		)
}

func (receipt ProposalDecisionReceipt) MatchesSessionAlignmentProposal(
	proposal SessionAlignmentProposal,
) bool {
	return proposal.Valid() && proposal.SchemaVersion == 2 &&
		proposal.Route != nil && proposal.Workspace != nil &&
		receipt.matchesProposal(
			proposal.ProposalID,
			proposal.ProposalDigest,
			proposal.ToolID,
			proposal.TargetConversationID,
			proposal.SegmentID,
			proposal.AttemptID,
			proposal.RegistryDigest,
			proposal.Workspace.WorkspaceDigest,
			proposal.Route.ExecutionBindingDigest,
			proposal.Route.ContextCapsuleDigest,
			proposal.ExpiresAt,
		)
}

func (receipt ProposalDecisionReceipt) matchesProposal(
	proposalID string,
	proposalDigest string,
	toolID ToolID,
	targetConversationID string,
	segmentID string,
	attemptID string,
	registryDigest string,
	workspaceDigest string,
	executionBindingDigest string,
	contextCapsuleDigest string,
	expiresAt time.Time,
) bool {
	return receipt.Valid() && receipt.ProposalID == proposalID &&
		receipt.ProposalDigest == proposalDigest && receipt.ToolID == toolID &&
		receipt.TargetConversationID == targetConversationID &&
		receipt.SegmentID == segmentID && receipt.AttemptID == attemptID &&
		receipt.RegistryDigest == registryDigest &&
		receipt.WorkspaceDigest == workspaceDigest &&
		receipt.ExecutionBindingDigest == executionBindingDigest &&
		receipt.ContextCapsuleDigest == contextCapsuleDigest &&
		validProposalDecisionTime(receipt.Decision, receipt.DecidedAt, expiresAt)
}

func (receipt ProposalDecisionReceipt) payloadDigest() string {
	body, _ := json.Marshal(struct {
		SchemaVersion          int              `json:"schema_version"`
		ProposalID             string           `json:"proposal_id"`
		ProposalDigest         string           `json:"proposal_digest"`
		ToolID                 ToolID           `json:"tool_id"`
		Decision               ProposalDecision `json:"decision"`
		DecisionIncidentID     string           `json:"decision_incident_id"`
		TargetConversationID   string           `json:"target_conversation_id"`
		SegmentID              string           `json:"segment_id"`
		AttemptID              string           `json:"attempt_id"`
		RegistryDigest         string           `json:"registry_digest"`
		WorkspaceDigest        string           `json:"workspace_digest"`
		ExecutionBindingDigest string           `json:"execution_binding_digest"`
		ContextCapsuleDigest   string           `json:"context_capsule_digest"`
		DecidedAt              time.Time        `json:"decided_at"`
	}{
		receipt.SchemaVersion,
		receipt.ProposalID,
		receipt.ProposalDigest,
		receipt.ToolID,
		receipt.Decision,
		receipt.DecisionIncidentID,
		receipt.TargetConversationID,
		receipt.SegmentID,
		receipt.AttemptID,
		receipt.RegistryDigest,
		receipt.WorkspaceDigest,
		receipt.ExecutionBindingDigest,
		receipt.ContextCapsuleDigest,
		receipt.DecidedAt.UTC(),
	})
	return DigestBytes(body)
}

func validProposalDecision(decision ProposalDecision) bool {
	switch decision {
	case ProposalDecisionConfirm, ProposalDecisionCancel,
		ProposalDecisionExpire, ProposalDecisionSupersede:
		return true
	default:
		return false
	}
}

func validProposalDecisionTime(
	decision ProposalDecision,
	decidedAt time.Time,
	expiresAt time.Time,
) bool {
	if decidedAt.IsZero() || expiresAt.IsZero() || !validProposalDecision(decision) {
		return false
	}
	if decision == ProposalDecisionExpire {
		return !decidedAt.Before(expiresAt)
	}
	return decidedAt.Before(expiresAt)
}

func validDecisionIncidentID(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}

func conversationActionForTool(toolID ToolID) (ConversationAction, bool) {
	switch toolID {
	case ToolMissionsCreatePreview:
		return ConversationActionMission, true
	case ToolMissionsContinuePreview:
		return ConversationActionContinueMission, true
	case ToolTeamsCreatePreview:
		return ConversationActionTeam, true
	case ToolRoundtablesOpenPreview:
		return ConversationActionRoundTable, true
	case ToolConversationRouteChangePreview:
		return ConversationActionRoute, true
	case ToolConversationModelChangePreview:
		return ConversationActionModel, true
	case ToolConversationReasoningChangePreview:
		return ConversationActionReasoning, true
	case ToolWorkspaceChoosePreview:
		return ConversationActionWorkspace, true
	case ToolTeamsEditPreview:
		return ConversationActionTeamEdit, true
	case ToolRoundtablesPausePreview:
		return ConversationActionRoundTablePause, true
	case ToolRoundtablesSteerPreview:
		return ConversationActionRoundTableSteer, true
	case ToolRoundtablesRetryPreview:
		return ConversationActionRoundTableRetry, true
	case ToolRoundtablesSkipPreview:
		return ConversationActionRoundTableSkip, true
	case ToolRoundtablesReplacePreview:
		return ConversationActionRoundTableReplace, true
	default:
		return "", false
	}
}

func validConversationActionArgument(action ConversationAction, argument string) bool {
	if action != ConversationActionMission && action != ConversationActionContinueMission &&
		action != ConversationActionTeam {
		return argument == ""
	}
	if !validText(argument, 4_096) || looksLikeCredential(argument) {
		return false
	}
	return action == ConversationActionMission ||
		action == ConversationActionContinueMission ||
		action == ConversationActionTeam
}

func validConversationActionPayloadV1(
	action ConversationAction,
	payload *ConversationActionPayload,
) bool {
	switch action {
	case ConversationActionMission, ConversationActionContinueMission,
		ConversationActionTeam, ConversationActionRoundTable,
		ConversationActionWorkspace:
		return payload == nil
	case ConversationActionRoute:
		return payload != nil &&
			validConversationActionIdentifier(payload.ProfileID, 256) &&
			payload.only("profile_id")
	case ConversationActionModel:
		return payload != nil &&
			validConversationActionIdentifier(payload.ModelID, 256) &&
			payload.only("model_id")
	case ConversationActionReasoning:
		return payload != nil &&
			validConversationActionIdentifier(payload.ReasoningEffort, 64) &&
			payload.only("reasoning_effort")
	case ConversationActionTeamEdit:
		return payload != nil &&
			validConversationActionIdentifier(payload.TeamInstanceID, 128) &&
			validText(payload.Instruction, 4_096) &&
			!looksLikeCredential(payload.Instruction) &&
			payload.only("team_instance_id", "instruction")
	case ConversationActionRoundTablePause:
		return payload != nil && validConversationRoundTableTarget(payload, false) &&
			payload.only("session_id", "round_id")
	case ConversationActionRoundTableSteer,
		ConversationActionRoundTableRetry:
		return payload != nil && validConversationRoundTableTarget(payload, true) &&
			validText(payload.Guidance, 4_096) && !looksLikeCredential(payload.Guidance) &&
			payload.only("session_id", "round_id", "seat_id", "attempt_id", "guidance")
	case ConversationActionRoundTableSkip,
		ConversationActionRoundTableReplace:
		return payload != nil && validConversationRoundTableTarget(payload, false) &&
			validConversationRoundTableIdentifier(payload.SeatID, 128) &&
			payload.only("session_id", "round_id", "seat_id")
	default:
		return false
	}
}

func validConversationActionPayloadV2(
	action ConversationAction,
	payload *ConversationActionPayload,
) bool {
	switch action {
	case ConversationActionContinueMission, ConversationActionRoundTable:
		return payload != nil &&
			validConversationActionIdentifier(payload.MissionID, 128) &&
			payload.only("mission_id")
	case ConversationActionMission, ConversationActionTeam, ConversationActionWorkspace:
		return payload == nil
	case ConversationActionRoundTableSteer,
		ConversationActionRoundTableRetry:
		if payload == nil || !validConversationRoundTableTarget(payload, true) ||
			!validText(payload.Guidance, 4_096) || looksLikeCredential(payload.Guidance) {
			return false
		}
		return payload.only("session_id", "round_id", "seat_id", "attempt_id", "guidance") ||
			payload.ValidFrozenRoundTableSeatBinding() && payload.only(
				"session_id", "round_id", "seat_id", "attempt_id", "guidance",
				"membership_revision", "seat_binding_digest",
			)
	case ConversationActionRoundTableSkip,
		ConversationActionRoundTableReplace:
		if payload == nil || !validConversationRoundTableTarget(payload, false) ||
			!validConversationRoundTableIdentifier(payload.SeatID, 128) {
			return false
		}
		return payload.only("session_id", "round_id", "seat_id") ||
			payload.ValidFrozenRoundTableSeatBinding() && payload.only(
				"session_id", "round_id", "seat_id",
				"membership_revision", "seat_binding_digest",
			)
	default:
		return validConversationActionPayloadV1(action, payload)
	}
}

func validConversationRoundTableTarget(
	payload *ConversationActionPayload,
	requireAttempt bool,
) bool {
	if payload == nil ||
		!validConversationRoundTableIdentifier(payload.SessionID, 128) ||
		!validConversationRoundTableIdentifier(payload.RoundID, 128) {
		return false
	}
	if !requireAttempt {
		return true
	}
	return validConversationRoundTableIdentifier(payload.SeatID, 128) &&
		validConversationRoundTableIdentifier(payload.AttemptID, 128)
}

func (payload ConversationActionPayload) only(fields ...string) bool {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	values := map[string]string{
		"mission_id": payload.MissionID,
		"profile_id": payload.ProfileID, "model_id": payload.ModelID,
		"reasoning_effort": payload.ReasoningEffort,
		"team_instance_id": payload.TeamInstanceID, "instruction": payload.Instruction,
		"session_id": payload.SessionID, "round_id": payload.RoundID,
		"seat_id": payload.SeatID, "attempt_id": payload.AttemptID,
		"guidance": payload.Guidance, "seat_binding_digest": payload.SeatBindingDigest,
	}
	for field, value := range values {
		_, expected := allowed[field]
		if expected != (value != "") {
			return false
		}
	}
	_, membershipExpected := allowed["membership_revision"]
	if membershipExpected != (payload.MembershipRevision != 0) {
		return false
	}
	return true
}

func cloneFrozenRouteReference(reference *FrozenRouteReference) *FrozenRouteReference {
	if reference == nil {
		return nil
	}
	cloned := *reference
	return &cloned
}

func cloneFrozenWorkspaceReference(reference *FrozenWorkspaceReference) *FrozenWorkspaceReference {
	if reference == nil {
		return nil
	}
	cloned := *reference
	return &cloned
}

func cloneConversationActionPayload(
	payload *ConversationActionPayload,
) *ConversationActionPayload {
	if payload == nil {
		return nil
	}
	cloned := *payload
	return &cloned
}

func validConversationActionIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	normalized := strings.ToLower(value)
	for _, marker := range []string{
		"sk-", "sk_", "api_key=", "api-key=", "apikey=", "bearer ",
	} {
		if strings.HasPrefix(normalized, marker) {
			return false
		}
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' ||
			character == '/' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func validConversationRoundTableIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 ||
		looksLikeExplicitCredential(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func looksLikeCredential(value string) bool {
	for _, token := range strings.Fields(value) {
		normalized := strings.ToLower(strings.TrimLeft(token, "\"'([{<"))
		if explicitCredentialToken(normalized) {
			return true
		}
		if len(token) < 40 {
			continue
		}
		hasLetter := false
		hasNumber := false
		for _, current := range token {
			hasLetter = hasLetter || unicode.IsLetter(current)
			hasNumber = hasNumber || unicode.IsNumber(current)
		}
		if hasLetter && hasNumber && (strings.Contains(token, "_") || strings.Contains(token, "-")) {
			return true
		}
	}
	return false
}

func looksLikeExplicitCredential(value string) bool {
	for _, token := range strings.Fields(value) {
		normalized := strings.ToLower(strings.TrimLeft(token, "\"'([{<"))
		if explicitCredentialToken(normalized) {
			return true
		}
	}
	return false
}

func explicitCredentialToken(normalized string) bool {
	if normalized == "bearer" {
		return true
	}
	for _, marker := range []string{
		"sk-", "sk_", "api_key=", "api-key=", "apikey=", "--api-key=",
		"authorization:", "x-api-key:",
	} {
		if strings.HasPrefix(normalized, marker) {
			return true
		}
	}
	return false
}

func NewSessionAlignmentProposal(
	input SessionAlignmentProposalInput,
) (SessionAlignmentProposal, error) {
	proposal := SessionAlignmentProposal{
		SchemaVersion: 2, ProposalID: input.ProposalID,
		ToolID: ToolSessionsAlignPreview, ToolVersion: 2,
		Confirmation:         ConfirmationUser,
		TargetConversationID: input.TargetConversationID,
		TargetContentDigest:  input.TargetContentDigest,
		Sources:              append([]SessionSource(nil), input.Sources...),
		ContextMode:          input.ContextMode, CatalogDigest: input.CatalogDigest,
		Route:          cloneFrozenRouteReference(input.Route),
		Workspace:      cloneFrozenWorkspaceReference(input.Workspace),
		RegistryDigest: input.RegistryDigest, IncidentID: input.IncidentID,
		SegmentID: input.SegmentID, AttemptID: input.AttemptID,
		Status: ProposalPending, CreatedAt: input.CreatedAt.UTC(),
		ExpiresAt: input.ExpiresAt.UTC(),
	}
	sort.Slice(proposal.Sources, func(left, right int) bool {
		return proposal.Sources[left].ConversationID < proposal.Sources[right].ConversationID
	})
	if !proposal.validPayload() {
		return SessionAlignmentProposal{}, ErrInvalidCall
	}
	proposal.ProposalDigest = proposal.payloadDigest()
	return proposal, nil
}

func (proposal SessionAlignmentProposal) Valid() bool {
	return proposal.validPayload() && validDigest(proposal.ProposalDigest) &&
		proposal.ProposalDigest == proposal.payloadDigest() && validProposalStatus(proposal.Status)
}

// Confirmable separates migration readability from current execution authority.
func (proposal SessionAlignmentProposal) Confirmable() bool {
	return proposal.Valid() && proposal.SchemaVersion == 2 &&
		proposal.Status == ProposalPending
}

func (proposal SessionAlignmentProposal) Clone() SessionAlignmentProposal {
	proposal.Sources = append([]SessionSource(nil), proposal.Sources...)
	proposal.Route = cloneFrozenRouteReference(proposal.Route)
	proposal.Workspace = cloneFrozenWorkspaceReference(proposal.Workspace)
	return proposal
}

func CloneSessionAlignmentProposals(
	proposals []SessionAlignmentProposal,
) []SessionAlignmentProposal {
	if proposals == nil {
		return nil
	}
	result := make([]SessionAlignmentProposal, len(proposals))
	for index, proposal := range proposals {
		result[index] = proposal.Clone()
	}
	return result
}

func (proposal SessionAlignmentProposal) validPayload() bool {
	if proposal.ToolID != ToolSessionsAlignPreview ||
		proposal.Confirmation != ConfirmationUser ||
		!validIdentifier(proposal.ProposalID, 128, true) ||
		!validIdentifier(proposal.TargetConversationID, 256, true) ||
		!validIdentifier(proposal.SegmentID, 128, true) ||
		!validIdentifier(proposal.AttemptID, 128, true) ||
		!validDigest(proposal.TargetContentDigest) || !validDigest(proposal.CatalogDigest) ||
		proposal.ContextMode != ContextModeSummaryOnly &&
			proposal.ContextMode != ContextModeContinueWithContext ||
		proposal.CreatedAt.IsZero() || proposal.ExpiresAt.IsZero() ||
		!proposal.ExpiresAt.After(proposal.CreatedAt) ||
		proposal.ExpiresAt.Sub(proposal.CreatedAt) > 15*time.Minute ||
		len(proposal.Sources) == 0 || len(proposal.Sources) > 8 {
		return false
	}
	switch proposal.SchemaVersion {
	case 1:
		if proposal.ToolVersion != 1 || proposal.Route != nil || proposal.Workspace != nil ||
			proposal.RegistryDigest != "" || proposal.IncidentID != "" {
			return false
		}
	case 2:
		if proposal.ToolVersion != 2 || proposal.Route == nil || !proposal.Route.Valid() ||
			proposal.Workspace == nil || !proposal.Workspace.Valid() ||
			!validDigest(proposal.RegistryDigest) ||
			!validIdentifier(proposal.IncidentID, 128, true) {
			return false
		}
	default:
		return false
	}
	previous := ""
	for _, source := range proposal.Sources {
		if !validIdentifier(source.ConversationID, 256, true) ||
			source.ConversationID == proposal.TargetConversationID ||
			!validText(source.Title, 256) || !validDigest(source.ContentDigest) ||
			source.MessageCount < 0 || source.MessageCount > 256 ||
			previous != "" && source.ConversationID <= previous {
			return false
		}
		previous = source.ConversationID
	}
	return proposal.MessageID == "" || validIdentifier(proposal.MessageID, 128, true)
}

func (proposal SessionAlignmentProposal) payloadDigest() string {
	if proposal.SchemaVersion == 1 {
		return proposal.payloadDigestV1()
	}
	body, _ := json.Marshal(struct {
		SchemaVersion        int                       `json:"schema_version"`
		ProposalID           string                    `json:"proposal_id"`
		ToolID               ToolID                    `json:"tool_id"`
		ToolVersion          int                       `json:"tool_version"`
		Confirmation         ConfirmationPolicy        `json:"confirmation"`
		TargetConversationID string                    `json:"target_conversation_id"`
		TargetContentDigest  string                    `json:"target_content_digest"`
		Sources              []SessionSource           `json:"sources"`
		ContextMode          ContextMode               `json:"context_mode"`
		CatalogDigest        string                    `json:"catalog_digest"`
		Route                *FrozenRouteReference     `json:"route"`
		Workspace            *FrozenWorkspaceReference `json:"workspace"`
		RegistryDigest       string                    `json:"registry_digest"`
		IncidentID           string                    `json:"incident_id"`
		SegmentID            string                    `json:"segment_id"`
		AttemptID            string                    `json:"attempt_id"`
		CreatedAt            time.Time                 `json:"created_at"`
		ExpiresAt            time.Time                 `json:"expires_at"`
	}{
		proposal.SchemaVersion, proposal.ProposalID, proposal.ToolID,
		proposal.ToolVersion, proposal.Confirmation, proposal.TargetConversationID,
		proposal.TargetContentDigest, proposal.Sources, proposal.ContextMode,
		proposal.CatalogDigest, proposal.Route, proposal.Workspace,
		proposal.RegistryDigest, proposal.IncidentID,
		proposal.SegmentID, proposal.AttemptID,
		proposal.CreatedAt.UTC(), proposal.ExpiresAt.UTC(),
	})
	return DigestBytes(body)
}

func (proposal SessionAlignmentProposal) payloadDigestV1() string {
	body, _ := json.Marshal(struct {
		SchemaVersion        int                `json:"schema_version"`
		ProposalID           string             `json:"proposal_id"`
		ToolID               ToolID             `json:"tool_id"`
		ToolVersion          int                `json:"tool_version"`
		Confirmation         ConfirmationPolicy `json:"confirmation"`
		TargetConversationID string             `json:"target_conversation_id"`
		TargetContentDigest  string             `json:"target_content_digest"`
		Sources              []SessionSource    `json:"sources"`
		ContextMode          ContextMode        `json:"context_mode"`
		CatalogDigest        string             `json:"catalog_digest"`
		SegmentID            string             `json:"segment_id"`
		AttemptID            string             `json:"attempt_id"`
		CreatedAt            time.Time          `json:"created_at"`
		ExpiresAt            time.Time          `json:"expires_at"`
	}{
		proposal.SchemaVersion, proposal.ProposalID, proposal.ToolID,
		proposal.ToolVersion, proposal.Confirmation, proposal.TargetConversationID,
		proposal.TargetContentDigest, proposal.Sources, proposal.ContextMode,
		proposal.CatalogDigest, proposal.SegmentID, proposal.AttemptID,
		proposal.CreatedAt.UTC(), proposal.ExpiresAt.UTC(),
	})
	return DigestBytes(body)
}

func validProposalStatus(status ProposalStatus) bool {
	switch status {
	case ProposalPending, ProposalConfirmed, ProposalCancelled, ProposalExpired:
		return true
	default:
		return false
	}
}

type TurnContext struct {
	ConversationID string
	SegmentID      string
	AttemptID      string
	IncidentID     string
	RegistryDigest string
	Catalog        []SessionReference
	CatalogDigest  string
	Route          FrozenRouteReference
	Workspace      FrozenWorkspaceReference
}

type FrozenRouteReference struct {
	HarnessAdapter         string `json:"harness_adapter"`
	ProviderID             string `json:"provider_id"`
	ProviderAccountID      string `json:"provider_account_id,omitempty"`
	CredentialRevision     int64  `json:"credential_revision"`
	ModelID                string `json:"model_id"`
	ReasoningEffort        string `json:"reasoning_effort,omitempty"`
	ExecutionBindingDigest string `json:"execution_binding_digest"`
	ContextCapsuleDigest   string `json:"context_capsule_digest"`
}

func (reference FrozenRouteReference) Valid() bool {
	if !validIdentifier(reference.HarnessAdapter, 64, false) ||
		!validIdentifier(reference.ProviderID, 64, false) ||
		!validText(reference.ModelID, 256) ||
		!validDigest(reference.ExecutionBindingDigest) ||
		!validDigest(reference.ContextCapsuleDigest) ||
		reference.CredentialRevision < 0 ||
		reference.ReasoningEffort != "" && !validText(reference.ReasoningEffort, 64) {
		return false
	}
	if reference.ProviderAccountID == "" {
		return reference.CredentialRevision == 0
	}
	return validIdentifier(reference.ProviderAccountID, 128, true) &&
		reference.CredentialRevision > 0
}

func (reference FrozenRouteReference) Empty() bool {
	return reference == (FrozenRouteReference{})
}

type FrozenWorkspaceReference struct {
	WorkspaceID     string `json:"workspace_id"`
	WorkspaceDigest string `json:"workspace_digest"`
}

func (reference FrozenWorkspaceReference) Valid() bool {
	return validIdentifier(reference.WorkspaceID, 128, false) &&
		validDigest(reference.WorkspaceDigest)
}

func (reference FrozenWorkspaceReference) Empty() bool {
	return reference == (FrozenWorkspaceReference{})
}

type Call struct {
	ToolID    ToolID
	Arguments json.RawMessage
}

// CompletedCall is the bounded, non-secret audit projection for one tool call
// that completed inside a frozen Harness turn. Arguments and results are
// intentionally absent.
type CompletedCall struct {
	ToolID      ToolID `json:"tool_id"`
	ToolVersion int    `json:"tool_version"`
	Effect      Effect `json:"effect"`
}

func (call CompletedCall) Valid() bool {
	return validIdentifier(string(call.ToolID), 128, true) &&
		call.ToolVersion > 0 && call.ToolVersion <= 1_000 &&
		(call.Effect == EffectRead || call.Effect == EffectProposal)
}

func CloneCompletedCalls(calls []CompletedCall) []CompletedCall {
	if calls == nil {
		return nil
	}
	return append([]CompletedCall(nil), calls...)
}

// ValidCompletedCalls binds each audit record to the exact admitted Registry
// definition. Repeated calls are preserved because a model may legitimately
// invoke the same read more than once within the bounded turn.
func (registry *Registry) ValidCompletedCalls(calls []CompletedCall, maximum int) bool {
	if registry == nil || maximum <= 0 || len(calls) > maximum {
		return false
	}
	for _, call := range calls {
		definition, found := registry.Definition(call.ToolID)
		if !found || !call.Valid() || definition.Version != call.ToolVersion ||
			definition.Effect != call.Effect {
			return false
		}
	}
	return true
}

type Result struct {
	Content        json.RawMessage
	Proposal       *SessionAlignmentProposal
	ActionProposal *ConversationActionProposal
}

type ProposalBatch struct {
	SessionAlignments   []SessionAlignmentProposal
	ConversationActions []ConversationActionProposal
	CompletedCalls      []CompletedCall
}

type Gateway interface {
	Call(context.Context, TurnContext, Call) (Result, error)
}

type GatewaySlot struct {
	mu      sync.RWMutex
	gateway Gateway
}

func NewGatewaySlot() *GatewaySlot { return &GatewaySlot{} }

func (slot *GatewaySlot) Bind(gateway Gateway) error {
	if slot == nil || nilInterface(gateway) {
		return ErrGatewayUnavailable
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.gateway != nil {
		return ErrGatewayConflict
	}
	slot.gateway = gateway
	return nil
}

func (slot *GatewaySlot) Call(
	ctx context.Context,
	turn TurnContext,
	call Call,
) (Result, error) {
	if slot == nil || ctx == nil || ctx.Err() != nil {
		return Result{}, ErrGatewayUnavailable
	}
	slot.mu.RLock()
	gateway := slot.gateway
	slot.mu.RUnlock()
	if nilInterface(gateway) {
		return Result{}, ErrGatewayUnavailable
	}
	return gateway.Call(ctx, turn, call)
}

func nilInterface(value any) bool {
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
