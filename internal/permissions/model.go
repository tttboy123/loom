// Package permissions implements the Loom tool-call authorization pipeline:
// permission modes, deny/ask/allow rules, read-only whitelist, dangerous
// command table, per-Job permission profiles with generation fencing, a
// deterministic fail-closed evaluator, and the admin lock. All authorization
// facts are Journal events; configuration files are materialized projections,
// never a second authority. This package contains no os/exec.
package permissions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type Mode string

const (
	ModeDefault           Mode = "default"
	ModeAcceptEdits       Mode = "accept_edits"
	ModeAuto              Mode = "auto"
	ModeDontAsk           Mode = "dont_ask"
	ModeBypassPermissions Mode = "bypass_permissions"
)

type ToolKind string

const (
	ToolBash      ToolKind = "Bash"
	ToolRead      ToolKind = "Read"
	ToolEdit      ToolKind = "Edit"
	ToolGrep      ToolKind = "Grep"
	ToolMCPTool   ToolKind = "MCPTool"
	ToolWebFetch  ToolKind = "WebFetch"
	ToolWebSearch ToolKind = "WebSearch"
)

type RuleAction string

const (
	ActionAllow RuleAction = "allow"
	ActionAsk   RuleAction = "ask"
	ActionDeny  RuleAction = "deny"
)

type ScopeKind string

const (
	ScopeJob      ScopeKind = "job"
	ScopeProject  ScopeKind = "project"
	ScopePersonal ScopeKind = "personal"
	ScopeRoot     ScopeKind = "root"
)

var (
	ErrInvalidInput          = errors.New("invalid permission input")
	ErrInvalidMode           = errors.New("invalid permission mode")
	ErrInvalidTool           = errors.New("invalid tool kind")
	ErrInvalidScope          = errors.New("invalid scope kind")
	ErrInvalidRule           = errors.New("invalid permission rule")
	ErrProfileExists         = errors.New("permission profile already exists")
	ErrProfileNotFound       = errors.New("permission profile not found")
	ErrProfileRetired        = errors.New("permission profile retired")
	ErrRuleNotFound          = errors.New("permission rule not found")
	ErrRuleExists            = errors.New("permission rule already exists")
	ErrGrantNotFound         = errors.New("permission grant not found")
	ErrGrantExists           = errors.New("permission grant already exists")
	ErrDangerousGrant        = errors.New("grant pattern covers a dangerous command")
	ErrBypassRequiresAuth    = errors.New("bypass activation requires a non-empty authorized_by")
	ErrRootRequiresAuth      = errors.New("root scope write requires a non-empty authorized_by")
	ErrAdminLockEnabled      = errors.New("admin lock enabled")
	ErrApprovalForwardOnly   = errors.New("approval resolution is forwarded to the existing rules authority")
	ErrStaleBinding          = errors.New("stale job permission binding")
	ErrUnknownTool           = errors.New("unknown tool kind")
	ErrEmptyCommand          = errors.New("empty bash command")
	ErrEmptyPath             = errors.New("empty tool path")
	ErrUnknownEvent          = errors.New("unknown permission event type")
	ErrInvalidEventPayload   = errors.New("invalid permission event payload")
	ErrInvalidAuthorityInput = errors.New("invalid permission authority input")
	ErrUnboundJobDefault     = errors.New("unbound job has no permission profile")
)

type Rule struct {
	RuleID  string     `json:"rule_id"`
	Scope   ScopeKind  `json:"scope"`
	ScopeID string     `json:"scope_id"`
	Action  RuleAction `json:"action"`
	Tool    ToolKind   `json:"tool"`
	Pattern string     `json:"pattern"`
}

type ProfileInput struct {
	ProfileID  string   `json:"profile_id"`
	Mode       Mode     `json:"mode"`
	Rules      []Rule   `json:"rules"`
	OwnedPaths []string `json:"owned_paths"`
}

type PermissionProfile struct {
	ProfileID  string   `json:"profile_id"`
	Generation int64    `json:"generation"`
	Digest     string   `json:"digest"`
	Mode       Mode     `json:"mode"`
	Rules      []Rule   `json:"rules"`
	OwnedPaths []string `json:"owned_paths"`
}

type ProposedCall struct {
	Tool    ToolKind `json:"tool"`
	Command string   `json:"command"`
	Path    string   `json:"path"`
	Pattern string   `json:"pattern,omitempty"`
}

// EffectiveProfile is the only input to Evaluate: the Job-bound profile merged
// with all applicable scope rules, active grants, mode and admin lock.
type EffectiveProfile struct {
	Profile     PermissionProfile
	Mode        Mode
	MergedRules []Rule
	Grants      []Grant
	AdminLock   bool
}

type Verdict string

const (
	VerdictAllow Verdict = "allow"
	VerdictAsk   Verdict = "ask"
	VerdictDeny  Verdict = "deny"
)

type Denial struct {
	Reason            string   `json:"reason"`
	AuthorizationPath string   `json:"authorization_path"`
	RuleIDs           []string `json:"rule_ids"`
}

type JobBinding struct {
	JobID             string `json:"job_id"`
	ProfileID         string `json:"profile_id"`
	ProfileDigest     string `json:"profile_digest"`
	ProfileGeneration int64  `json:"profile_generation"`
	BoundAt           string `json:"bound_at"`
}

type Grant struct {
	GrantID   string    `json:"grant_id"`
	Scope     ScopeKind `json:"scope"`
	ScopeID   string    `json:"scope_id"`
	Tool      ToolKind  `json:"tool"`
	Pattern   string    `json:"pattern"`
	IssuedAt  string    `json:"issued_at"`
	RevokedAt string    `json:"revoked_at"`
}

// Approval is a read-only view of the existing rules/approval authority
// (ApprovalRequested/ApprovalResolved in internal/rules and
// internal/projection/approval.go). This layer never writes approval events.
type Approval struct {
	ApprovalID string   `json:"approval_id"`
	JobID      string   `json:"job_id"`
	Tool       ToolKind `json:"tool"`
	Command    string   `json:"command"`
	Path       string   `json:"path"`
	AskedAt    string   `json:"asked_at"`
	ExpiresAt  string   `json:"expires_at"`
	Status     string   `json:"status"`
	Resolution string   `json:"resolution"`
	ResolvedAt string   `json:"resolved_at"`
	ResolvedBy string   `json:"resolved_by"`
}

type Activation struct {
	Mode          Mode      `json:"mode"`
	Scope         ScopeKind `json:"scope"`
	ScopeID       string    `json:"scope_id"`
	ActivatedAt   string    `json:"activated_at"`
	DeactivatedAt string    `json:"deactivated_at"`
	AuthorizedBy  string    `json:"authorized_by"`
}

type Projection struct {
	Profiles    map[string]PermissionProfile
	Bindings    map[string]JobBinding
	Rules       map[string]Rule
	Grants      map[string]Grant
	Activations map[string]Activation // key: scope + "/" + scopeID
	AdminLock   bool
	// ApprovalView read-only reference to the existing approval projection.
	ApprovalView map[string]Approval

	retired map[string]bool
}

func ValidMode(value string) bool {
	switch Mode(value) {
	case ModeDefault, ModeAcceptEdits, ModeAuto, ModeDontAsk, ModeBypassPermissions:
		return true
	default:
		return false
	}
}

func ValidToolKind(value string) bool {
	switch ToolKind(value) {
	case ToolBash, ToolRead, ToolEdit, ToolGrep, ToolMCPTool, ToolWebFetch, ToolWebSearch:
		return true
	default:
		return false
	}
}

func ValidScopeKind(value string) bool {
	switch ScopeKind(value) {
	case ScopeJob, ScopeProject, ScopePersonal, ScopeRoot:
		return true
	default:
		return false
	}
}

func ValidRuleAction(value string) bool {
	switch RuleAction(value) {
	case ActionAllow, ActionAsk, ActionDeny:
		return true
	default:
		return false
	}
}

func validID(value string) bool {
	return value != "" && len(value) <= 128 && !strings.ContainsAny(value, "\n\r")
}

func validateRule(rule Rule) error {
	if !validID(rule.RuleID) {
		return ErrInvalidRule
	}
	if !ValidScopeKind(string(rule.Scope)) {
		return ErrInvalidRule
	}
	if !validID(rule.ScopeID) {
		return ErrInvalidRule
	}
	if !ValidRuleAction(string(rule.Action)) {
		return ErrInvalidRule
	}
	if !ValidToolKind(string(rule.Tool)) {
		return ErrInvalidRule
	}
	if strings.TrimSpace(rule.Pattern) == "" {
		return ErrInvalidRule
	}
	return nil
}

func cloneRules(rules []Rule) []Rule {
	out := make([]Rule, len(rules))
	copy(out, rules)
	return out
}

func clonePaths(paths []string) []string {
	out := make([]string, len(paths))
	copy(out, paths)
	return out
}

// ProposedCallDigest is the canonical, content-free identity for one exact
// tool proposal. Journal facts use this digest instead of persisting arguments.
func ProposedCallDigest(call ProposedCall) string {
	body, _ := json.Marshal(call)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// digestProfile computes the canonical profile digest over sorted rules so
// identical logical profiles always produce identical digests.
func digestProfile(profileID string, mode Mode, rules []Rule, ownedPaths []string) (string, error) {
	sorted := cloneRules(rules)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RuleID < sorted[j].RuleID })
	paths := clonePaths(ownedPaths)
	sort.Strings(paths)
	canonical := struct {
		ProfileID  string   `json:"profile_id"`
		Mode       Mode     `json:"mode"`
		Rules      []Rule   `json:"rules"`
		OwnedPaths []string `json:"owned_paths"`
	}{ProfileID: profileID, Mode: mode, Rules: sorted, OwnedPaths: paths}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func isoNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}
