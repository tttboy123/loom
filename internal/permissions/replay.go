package permissions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/journal"
)

const (
	streamProfile    = "permission-profile/"
	streamRule       = "permission-rule/"
	streamBinding    = "permission-binding/"
	streamGrant      = "permission-grant/"
	streamActivation = "permission-activation"
	streamAdminLock  = "permission-admin-lock"
	streamDecision   = "permission-decision/"
)

type profileEventPayload struct {
	ProfileID  string   `json:"profile_id"`
	Generation int64    `json:"generation"`
	Digest     string   `json:"digest"`
	Mode       Mode     `json:"mode"`
	Rules      []Rule   `json:"rules"`
	OwnedPaths []string `json:"owned_paths"`
}

type retiredPayload struct {
	ProfileID string `json:"profile_id"`
	RetiredAt string `json:"retired_at"`
}

type ruleEventPayload struct {
	RuleID  string     `json:"rule_id"`
	Scope   ScopeKind  `json:"scope"`
	ScopeID string     `json:"scope_id"`
	Action  RuleAction `json:"action"`
	Tool    ToolKind   `json:"tool"`
	Pattern string     `json:"pattern"`
}

type ruleRevokedPayload struct {
	RuleID    string `json:"rule_id"`
	RevokedAt string `json:"revoked_at"`
}

type bindingPayload struct {
	JobID             string `json:"job_id"`
	ProfileID         string `json:"profile_id"`
	ProfileDigest     string `json:"profile_digest"`
	ProfileGeneration int64  `json:"profile_generation"`
	BoundAt           string `json:"bound_at"`
}

type grantEventPayload struct {
	GrantID  string    `json:"grant_id"`
	Scope    ScopeKind `json:"scope"`
	ScopeID  string    `json:"scope_id"`
	Tool     ToolKind  `json:"tool"`
	Pattern  string    `json:"pattern"`
	IssuedAt string    `json:"issued_at"`
}

type grantRevokedPayload struct {
	GrantID   string `json:"grant_id"`
	RevokedAt string `json:"revoked_at"`
}

type activationPayload struct {
	Mode         Mode      `json:"mode"`
	Scope        ScopeKind `json:"scope"`
	ScopeID      string    `json:"scope_id"`
	ActivatedAt  string    `json:"activated_at"`
	AuthorizedBy string    `json:"authorized_by"`
}

type adminLockPayload struct {
	EnabledAt    string `json:"enabled_at,omitempty"`
	DisabledAt   string `json:"disabled_at,omitempty"`
	AuthorizedBy string `json:"authorized_by"`
}

type decisionPayload struct {
	JobID             string   `json:"job_id"`
	ApprovalID        string   `json:"approval_id,omitempty"`
	Verdict           Verdict  `json:"verdict"`
	Tool              ToolKind `json:"tool"`
	Command           string   `json:"command,omitempty"`
	Path              string   `json:"path,omitempty"`
	Reason            string   `json:"reason"`
	AuthorizationPath string   `json:"authorization_path"`
	RecordedAt        string   `json:"recorded_at"`
}

// Replay rebuilds the permission projection strictly from Journal events in
// order. Any invalid, unknown or inconsistent event returns an error; callers
// must keep the previous view (no swap) on failure.
func Replay(events []journal.Event) (*Projection, error) {
	projection := &Projection{
		Profiles:     make(map[string]PermissionProfile),
		Bindings:     make(map[string]JobBinding),
		Rules:        make(map[string]Rule),
		Grants:       make(map[string]Grant),
		Activations:  make(map[string]Activation),
		ApprovalView: make(map[string]Approval),
		retired:      make(map[string]bool),
	}
	// The Journal is shared across domains: events on non-permission streams
	// (queue, work, runtime, agent-grant, ...) are unrelated and must be
	// skipped, not rejected. An unknown or invalid event on a permission
	// stream still fails the replay (RED #10, RED #15).
	// Journal ReadAll orders by (stream_id, seq), so cross-stream dependency
	// order is not preserved. Apply profile facts first, then all other facts,
	// then bindings (which reference the final profile state) — per-stream
	// sequence order inside each bucket is preserved.
	var profiles, others, bindings []journal.Event
	for _, event := range events {
		if !isPermissionStream(event.StreamID) {
			continue
		}
		switch {
		case event.Type == "PermissionProfileDefined" ||
			event.Type == "PermissionProfileRevised" ||
			event.Type == "PermissionProfileRetired":
			profiles = append(profiles, event)
		case event.Type == "JobPermissionBound":
			bindings = append(bindings, event)
		default:
			others = append(others, event)
		}
	}
	for _, event := range profiles {
		if err := applyEvent(projection, event); err != nil {
			return nil, err
		}
	}
	for _, event := range others {
		if err := applyEvent(projection, event); err != nil {
			return nil, err
		}
	}
	for _, event := range bindings {
		if err := applyEvent(projection, event); err != nil {
			return nil, err
		}
	}
	return projection, nil
}

func isPermissionStream(streamID string) bool {
	return strings.HasPrefix(streamID, streamProfile) ||
		strings.HasPrefix(streamID, streamRule) ||
		strings.HasPrefix(streamID, streamBinding) ||
		strings.HasPrefix(streamID, streamGrant) ||
		streamID == streamActivation ||
		streamID == streamAdminLock ||
		strings.HasPrefix(streamID, streamDecision)
}

func applyEvent(projection *Projection, event journal.Event) error {
	if event.SchemaVersion != 1 {
		return fmt.Errorf("%w: schema_version=%d", ErrInvalidEventPayload, event.SchemaVersion)
	}
	switch event.Type {
	case "PermissionProfileDefined", "PermissionProfileRevised":
		var payload profileEventPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		return applyProfileEvent(projection, event, payload)
	case "PermissionProfileRetired":
		var payload retiredPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if payload.ProfileID == "" {
			return fmt.Errorf("%w: empty profile_id", ErrInvalidEventPayload)
		}
		if event.StreamID != streamProfile+payload.ProfileID {
			return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
		}
		if _, ok := projection.Profiles[payload.ProfileID]; !ok {
			return fmt.Errorf("%w: retire unknown profile", ErrInvalidEventPayload)
		}
		projection.retired[payload.ProfileID] = true
		return nil
	case "PermissionRuleAdded":
		var payload ruleEventPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if err := validateRule(Rule(payload)); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEventPayload, err)
		}
		if event.StreamID != streamRule+payload.RuleID {
			return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
		}
		projection.Rules[payload.RuleID] = Rule(payload)
		return nil
	case "PermissionRuleRevoked":
		var payload ruleRevokedPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if _, ok := projection.Rules[payload.RuleID]; !ok {
			return fmt.Errorf("%w: revoke unknown rule", ErrInvalidEventPayload)
		}
		delete(projection.Rules, payload.RuleID)
		return nil
	case "JobPermissionBound":
		var payload bindingPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if payload.JobID == "" || payload.ProfileID == "" || payload.BoundAt == "" {
			return fmt.Errorf("%w: empty binding field", ErrInvalidEventPayload)
		}
		if event.StreamID != streamBinding+payload.JobID {
			return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
		}
		profile, ok := projection.Profiles[payload.ProfileID]
		if !ok {
			return fmt.Errorf("%w: binding to missing profile", ErrInvalidEventPayload)
		}
		// A binding to a retired profile stays replayable; resolution fails
		// fail-closed via ResolveEffectiveProfile (ErrProfileRetired).
		if profile.Digest != payload.ProfileDigest || profile.Generation != payload.ProfileGeneration {
			return fmt.Errorf("%w: binding digest/generation mismatch", ErrInvalidEventPayload)
		}
		projection.Bindings[payload.JobID] = JobBinding{
			JobID: payload.JobID, ProfileID: payload.ProfileID,
			ProfileDigest:     payload.ProfileDigest,
			ProfileGeneration: payload.ProfileGeneration,
			BoundAt:           payload.BoundAt,
		}
		return nil
	case "PermissionGrantIssued":
		var payload grantEventPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		grant := Grant{GrantID: payload.GrantID, Scope: payload.Scope, ScopeID: payload.ScopeID,
			Tool: payload.Tool, Pattern: payload.Pattern, IssuedAt: payload.IssuedAt}
		if err := validateGrant(grant); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEventPayload, err)
		}
		if event.StreamID != streamGrant+payload.GrantID {
			return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
		}
		projection.Grants[payload.GrantID] = grant
		return nil
	case "PermissionGrantRevoked":
		var payload grantRevokedPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		grant, ok := projection.Grants[payload.GrantID]
		if !ok {
			return fmt.Errorf("%w: revoke unknown grant", ErrInvalidEventPayload)
		}
		grant.RevokedAt = payload.RevokedAt
		projection.Grants[payload.GrantID] = grant
		return nil
	case "PermissionActivationActivated":
		var payload activationPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if event.StreamID != streamActivation {
			return fmt.Errorf("%w: activation stream mismatch", ErrInvalidEventPayload)
		}
		if !ValidMode(string(payload.Mode)) || !ValidScopeKind(string(payload.Scope)) || payload.ScopeID == "" {
			return fmt.Errorf("%w: invalid activation", ErrInvalidEventPayload)
		}
		if payload.Mode == ModeBypassPermissions && payload.AuthorizedBy == "" {
			return fmt.Errorf("%w: bypass activation without authorized_by", ErrInvalidEventPayload)
		}
		projection.Activations[string(payload.Scope)+"/"+payload.ScopeID] = Activation{
			Mode: payload.Mode, Scope: payload.Scope, ScopeID: payload.ScopeID,
			ActivatedAt: payload.ActivatedAt, AuthorizedBy: payload.AuthorizedBy,
		}
		return nil
	case "PermissionActivationDeactivated":
		var payload activationPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if event.StreamID != streamActivation {
			return fmt.Errorf("%w: activation stream mismatch", ErrInvalidEventPayload)
		}
		delete(projection.Activations, string(payload.Scope)+"/"+payload.ScopeID)
		return nil
	case "PermissionAdminLockEnabled", "PermissionAdminLockDisabled":
		var payload adminLockPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if event.StreamID != streamAdminLock {
			return fmt.Errorf("%w: admin lock stream mismatch", ErrInvalidEventPayload)
		}
		if payload.AuthorizedBy == "" {
			return fmt.Errorf("%w: admin lock without authorized_by", ErrInvalidEventPayload)
		}
		projection.AdminLock = event.Type == "PermissionAdminLockEnabled"
		return nil
	case "PermissionDecisionRecorded":
		var payload decisionPayload
		if err := decodeStrict(event.PayloadJSON, &payload); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidEventPayload, event.Type, err)
		}
		if payload.JobID == "" ||
			(payload.Verdict != VerdictAllow && payload.Verdict != VerdictAsk && payload.Verdict != VerdictDeny) {
			return fmt.Errorf("%w: invalid decision", ErrInvalidEventPayload)
		}
		if !ValidToolKind(string(payload.Tool)) {
			return fmt.Errorf("%w: invalid decision tool", ErrInvalidEventPayload)
		}
		if event.StreamID != streamDecision+payload.JobID {
			return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnknownEvent, event.Type)
	}
}

func applyProfileEvent(projection *Projection, event journal.Event, payload profileEventPayload) error {
	if payload.ProfileID == "" || !ValidMode(string(payload.Mode)) || payload.Digest == "" {
		return fmt.Errorf("%w: invalid profile event", ErrInvalidEventPayload)
	}
	if event.StreamID != streamProfile+payload.ProfileID {
		return fmt.Errorf("%w: stream mismatch", ErrInvalidEventPayload)
	}
	for _, rule := range payload.Rules {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEventPayload, err)
		}
	}
	expectedDigest, err := digestProfile(payload.ProfileID, payload.Mode, payload.Rules, payload.OwnedPaths)
	if err != nil {
		return err
	}
	if expectedDigest != payload.Digest {
		return fmt.Errorf("%w: digest mismatch", ErrInvalidEventPayload)
	}
	existing, exists := projection.Profiles[payload.ProfileID]
	if event.Type == "PermissionProfileDefined" {
		if payload.Generation != 1 {
			return fmt.Errorf("%w: defined generation must be 1", ErrInvalidEventPayload)
		}
		if exists {
			return fmt.Errorf("%w: profile redefined", ErrInvalidEventPayload)
		}
	} else {
		if !exists {
			return fmt.Errorf("%w: revise unknown profile", ErrInvalidEventPayload)
		}
		if projection.retired[payload.ProfileID] {
			return fmt.Errorf("%w: revise retired profile", ErrInvalidEventPayload)
		}
		if payload.Generation != existing.Generation+1 {
			return fmt.Errorf("%w: revised generation not monotonic", ErrInvalidEventPayload)
		}
	}
	projection.Profiles[payload.ProfileID] = PermissionProfile{
		ProfileID: payload.ProfileID, Generation: payload.Generation,
		Digest: payload.Digest, Mode: payload.Mode,
		Rules: cloneRules(payload.Rules), OwnedPaths: clonePaths(payload.OwnedPaths),
	}
	return nil
}

func validateGrant(grant Grant) error {
	if !validID(grant.GrantID) || !ValidScopeKind(string(grant.Scope)) ||
		!validID(grant.ScopeID) || !ValidToolKind(string(grant.Tool)) ||
		strings.TrimSpace(grant.Pattern) == "" || grant.IssuedAt == "" {
		return ErrInvalidInput
	}
	if len(DangerousSegments(grant.Pattern)) > 0 {
		return ErrDangerousGrant
	}
	return nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data")
	}
	return nil
}

// EffectiveProfile resolves the effective authorization context for a Job.
// Unbound jobs use the conservative root policy + default mode (read-only and
// ask). A binding to a missing or retired profile, or with a stale
// digest/generation, returns an error (fail-closed).
func ResolveEffectiveProfile(projection *Projection, jobID string) (EffectiveProfile, error) {
	if projection == nil {
		return EffectiveProfile{}, ErrInvalidInput
	}
	binding, ok := projection.Bindings[jobID]
	if !ok {
		return EffectiveProfile{
			Profile:     PermissionProfile{Mode: ModeDefault},
			Mode:        ModeDefault,
			MergedRules: rootRules(projection),
			AdminLock:   projection.AdminLock,
		}, nil
	}
	profile, ok := projection.Profiles[binding.ProfileID]
	if !ok {
		return EffectiveProfile{}, ErrProfileNotFound
	}
	if projection.retired[binding.ProfileID] {
		return EffectiveProfile{}, ErrProfileRetired
	}
	if profile.Digest != binding.ProfileDigest || profile.Generation != binding.ProfileGeneration {
		return EffectiveProfile{}, ErrStaleBinding
	}
	mode, err := EffectiveMode(projection, jobID)
	if err != nil {
		return EffectiveProfile{}, err
	}
	// Profile-embedded rules are the Job's own rules (contract §3.2 "Job
	// profile 规则"); they apply with job scope regardless of the scope tags
	// stored inside the profile, and join Journal PermissionRuleAdded facts.
	merged := rulesForJob(projection, jobID, binding.ProfileID)
	for _, rule := range profile.Rules {
		rule.Scope = ScopeJob
		rule.ScopeID = jobID
		merged = append(merged, rule)
	}
	return EffectiveProfile{
		Profile:     profile,
		Mode:        mode,
		MergedRules: merged,
		Grants:      grantsForJob(projection, jobID),
		AdminLock:   projection.AdminLock,
	}, nil
}

// EffectiveMode precedence: bound profile mode > root-scope activation >
// default. Personal/project activations require the job-to-project scope
// mapping supplied by the daemon service layer (bounded amendment at B-P1
// integration; tracked in REVIEW-NOTES.md).
func EffectiveMode(projection *Projection, jobID string) (Mode, error) {
	if projection == nil {
		return "", ErrInvalidInput
	}
	if binding, ok := projection.Bindings[jobID]; ok {
		profile, exists := projection.Profiles[binding.ProfileID]
		if !exists {
			return "", ErrProfileNotFound
		}
		if projection.retired[binding.ProfileID] {
			return "", ErrProfileRetired
		}
		return profile.Mode, nil
	}
	// Personal > project > root activation precedence (single-project
	// daemon; scopeID resolution lands with the job->project mapping).
	// Multiple activations within one scope kind resolve deterministically:
	// the lexicographically smallest scopeID wins (map iteration must never
	// influence the verdict).
	for _, scopeKind := range []ScopeKind{ScopePersonal, ScopeProject} {
		var candidates []Activation
		for _, activation := range projection.Activations {
			if activation.Scope == scopeKind && activation.ScopeID != "" {
				candidates = append(candidates, activation)
			}
		}
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].ScopeID < candidates[j].ScopeID
			})
			return candidates[0].Mode, nil
		}
	}
	if activation, ok := projection.Activations[string(ScopeRoot)+"/global"]; ok {
		return activation.Mode, nil
	}
	return ModeDefault, nil
}

func rootRules(projection *Projection) []Rule {
	var rules []Rule
	for _, rule := range projection.Rules {
		if rule.Scope == ScopeRoot {
			rules = append(rules, rule)
		}
	}
	return rules
}

func rulesForJob(projection *Projection, jobID, profileID string) []Rule {
	var rules []Rule
	for _, rule := range projection.Rules {
		switch {
		case rule.Scope == ScopeJob && rule.ScopeID == jobID:
			rules = append(rules, rule)
		case rule.Scope == ScopeProject:
			// Single-project daemon: project rules apply to every job. In a
			// multi-project deployment the job->project mapping must scope
			// this to rule.ScopeID (bounded amendment, tracked in REVIEW-NOTES).
			rules = append(rules, rule)
		case rule.Scope == ScopePersonal:
			// Personal (user-local, gitignored) rules apply to every job in
			// the same daemon session, scoped by project mapping when it lands.
			rules = append(rules, rule)
		case rule.Scope == ScopeRoot:
			rules = append(rules, rule)
		}
	}
	return rules
}

// ActiveGrants returns non-revoked grants for a scope and scope ID.
func ActiveGrants(projection *Projection, scope ScopeKind, scopeID string) []Grant {
	if projection == nil {
		return nil
	}
	var grants []Grant
	for _, grant := range projection.Grants {
		if grant.RevokedAt == "" && grant.Scope == scope && grant.ScopeID == scopeID {
			grants = append(grants, grant)
		}
	}
	return grants
}

func grantsForJob(projection *Projection, jobID string) []Grant {
	var grants []Grant
	grants = append(grants, ActiveGrants(projection, ScopeJob, jobID)...)
	for _, grant := range projection.Grants {
		if grant.RevokedAt != "" {
			continue
		}
		if (grant.Scope == ScopeProject || grant.Scope == ScopePersonal) && grant.ScopeID != "" {
			grants = append(grants, grant)
		}
	}
	grants = append(grants, ActiveGrants(projection, ScopeRoot, "global")...)
	return grants
}
